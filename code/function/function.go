// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package p contains a Google Cloud Storage Cloud Function.
package p

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
)

// Global API clients used across function invocations.
var (
	storageClient   *storage.Client
	storageClientMu sync.Mutex
)

func init() {
	// Initialize client during cold start when credentials are available.
	// If credentials are unavailable (e.g., during unit tests), defer initialization
	// to getStorageClient so pure unit tests can run without live GCP credentials.
	client, err := storage.NewClient(context.Background())
	if err != nil {
		log.Printf("storage.NewClient deferred: %v", err)
		return
	}
	storageClient = client
}

func getStorageClient(ctx context.Context) (*storage.Client, error) {
	storageClientMu.Lock()
	defer storageClientMu.Unlock()
	if storageClient != nil {
		return storageClient, nil
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %v", err)
	}
	storageClient = client
	return storageClient, nil
}

// GCSEvent is the payload of a GCS event. Please refer to the docs for
// additional information regarding GCS events.
type GCSEvent struct {
	Bucket   string `json:"bucket"`
	Name     string `json:"name"`
	SelfLink string `json:"selfLink"`
}

// OnFileUpload prints a message when a file is changed in a Cloud Storage bucket.
func OnFileUpload(ctx context.Context, e GCSEvent) error {
	log.Printf("Processing file: %s", e.Name)

	if !strings.HasPrefix(e.Name, "uploads/") {
		return nil
	}

	relName := strings.TrimPrefix(e.Name, "uploads/")
	if relName == "" || relName == "." || relName == ".." || strings.HasPrefix(relName, ".") || strings.Contains(relName, "/") || strings.Contains(relName, "\\") {
		err := fmt.Errorf("invalid upload object path: %q", e.Name)
		log.Printf("error: %s", err)
		return err
	}

	if _, err := getStorageClient(ctx); err != nil {
		log.Printf("error: %s", err)
		return err
	}

	tPath, oPath, err := newPaths(ctx, e)
	if err != nil {
		log.Printf("error: %s", err)
		return err
	}

	if err := thumbnail(ctx, e, tPath); err != nil {
		log.Printf("error: %s", err)
		return err
	}
	if err := move(ctx, e, oPath); err != nil {
		log.Printf("error: %s", err)
		return err
	}

	if err := makePublic(ctx, e.Bucket, oPath); err != nil {
		log.Printf("error: %s", err)
		return err
	}

	if err := makePublic(ctx, e.Bucket, tPath); err != nil {
		log.Printf("error: %s", err)
		return err
	}

	return nil
}

func makePublic(ctx context.Context, bucket, file string) error {
	client, err := getStorageClient(ctx)
	if err != nil {
		return err
	}
	obj := client.Bucket(bucket).Object(file)
	return obj.ACL().Set(ctx, storage.AllUsers, "READER")
}

// newPaths figures out the paths for both the original images and their
// thumbnails. It ensures that duplicate uploads will be given unique suffixes
func newPaths(ctx context.Context, e GCSEvent) (string, string, error) {
	t := thumbnailPath(e.Name)
	o := originalPath(e.Name)

	doesExist, err := exists(ctx, e.Bucket, t)
	if err != nil {
		return "", "", err
	}

	i := 0
	for doesExist {
		i++
		t = thumbnailPath(e.Name)
		t = strings.Replace(t, "/thumbnail", fmt.Sprintf("_%d/thumbnail", i), 1)
		doesExist, err = exists(ctx, e.Bucket, t)
		if err != nil {
			return "", "", err
		}

		if !doesExist {
			o = originalPath(e.Name)
			o = strings.Replace(o, "/original", fmt.Sprintf("_%d/original", i), 1)
		}

	}

	return t, o, nil
}

// exists sees if a file exists already in a Cloud Storage
func exists(ctx context.Context, bucket, file string) (bool, error) {
	client, err := getStorageClient(ctx)
	if err != nil {
		return false, err
	}
	obj := client.Bucket(bucket).Object(file)

	_, err = obj.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("error checking existence of %s/%s: %s", bucket, file, err)
	}

	return true, nil
}

// move copies a file to the destination and deletes the original
func move(ctx context.Context, e GCSEvent, dest string) error {
	client, err := getStorageClient(ctx)
	if err != nil {
		return err
	}
	src := client.Bucket(e.Bucket).Object(e.Name)
	dst := client.Bucket(e.Bucket).Object(dest)

	if _, err := dst.CopierFrom(src).Run(ctx); err != nil {
		return fmt.Errorf("error copying  %s to %s: %s", e.Name, dest, err)
	}

	if err := src.Delete(ctx); err != nil {
		return fmt.Errorf("error deleting  %s: %s", e.Name, err)
	}

	return nil
}

// coderForContent inspects magic bytes and returns the explicit ImageMagick
// coder prefix ("png:", "jpeg:", or "gif:") for supported image formats.
func coderForContent(head []byte) (string, error) {
	if len(head) == 0 {
		return "", fmt.Errorf("unsupported image type: empty input")
	}
	contentType := http.DetectContentType(head)
	switch contentType {
	case "image/png":
		return "png:", nil
	case "image/jpeg":
		return "jpeg:", nil
	case "image/gif":
		return "gif:", nil
	default:
		return "", fmt.Errorf("unsupported image type: %s", contentType)
	}
}

// prepareThumbnailInput reads up to the first 512 bytes from r to sniff the
// actual image format (properly handling valid images smaller than 512 bytes
// where io.ReadFull returns io.ErrUnexpectedEOF), restricts the allowed types
// to PNG, JPEG, and GIF, and returns the explicit ImageMagick coder prefix
// together with a reassembled io.Reader over the full input stream.
func prepareThumbnailInput(r io.Reader) (string, io.Reader, error) {
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", nil, fmt.Errorf("error reading image header: %v", err)
	}
	head = head[:n]
	coder, err := coderForContent(head)
	if err != nil {
		return "", nil, err
	}
	return coder, io.MultiReader(bytes.NewReader(head), r), nil
}

// thumbnail creates a smaller version of an image
func thumbnail(ctx context.Context, e GCSEvent, dest string) error {
	client, err := getStorageClient(ctx)
	if err != nil {
		return err
	}

	inputBlob := client.Bucket(e.Bucket).Object(e.Name)
	r, err := inputBlob.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("error in getting reading input from bucket: %v", err)
	}
	defer r.Close()

	// Sniff the actual image type from the first 512 bytes and force
	// ImageMagick to use only that coder. Do not let convert auto-detect
	// the format from attacker-controlled magic bytes.
	coder, stdinReader, err := prepareThumbnailInput(r)
	if err != nil {
		return err
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "convert", coder+"-", "-thumbnail", "x100", "-strip", coder+"-")
	cmd.Stdin = stdinReader
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error in imagemagick call: %s", stderr.String())
	}

	outputBlob := client.Bucket(e.Bucket).Object(dest)
	w := outputBlob.NewWriter(ctx)
	if _, err := io.Copy(w, &stdout); err != nil {
		_ = w.Close()
		return fmt.Errorf("error writing thumbnail to bucket: %v", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("error finalizing thumbnail in bucket: %v", err)
	}

	return nil
}

func thumbnailPath(name string) string {
	base := filepath.Base(name)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return "processed/" + stem + "/thumbnail" + ext
}

func originalPath(name string) string {
	base := filepath.Base(name)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return "processed/" + stem + "/original" + ext
}
