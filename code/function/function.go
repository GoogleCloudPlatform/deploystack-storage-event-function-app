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
	"log"
	"os/exec"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
)

// Global API clients used across function invocations.
var (
	storageClient *storage.Client
)

func init() {
	// Declare a separate err variable to avoid shadowing the client variables.
	var err error

	storageClient, err = storage.NewClient(context.Background())
	if err != nil {
		log.Fatalf("storage.NewClient: %v", err)
	}
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

	tPath, oPath, err := newPaths(ctx, e)
	if err != nil {
		log.Printf("error: %s", err)
		return err
	}

	if strings.Index(e.Name, "uploads/") == 0 {
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

	}
	return nil
}

func makePublic(ctx context.Context, bucket, file string) error {
	obj := storageClient.Bucket(bucket).Object(file)
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
	obj := storageClient.Bucket(bucket).Object(file)

	_, err := obj.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("error checking existence of %s/%s: %s", bucket, file, err)
	}

	return true, nil
}

// move copies afile to the destination and deletes the original
func move(ctx context.Context, e GCSEvent, dest string) error {
	src := storageClient.Bucket(e.Bucket).Object(e.Name)
	dst := storageClient.Bucket(e.Bucket).Object(dest)

	if _, err := dst.CopierFrom(src).Run(ctx); err != nil {
		return fmt.Errorf("error copying  %s to %s: %s", e.Name, dest, err)
	}

	if err := src.Delete(ctx); err != nil {
		return fmt.Errorf("error deleting  %s: %s", e.Name, err)
	}

	return nil
}

// thumbnail creates a smaller version of an image
func thumbnail(ctx context.Context, e GCSEvent, dest string) error {
	inputBlob := storageClient.Bucket(e.Bucket).Object(e.Name)
	r, err := inputBlob.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("error in getting reading input from bucket: %v", err)
	}

	outputBlob := storageClient.Bucket(e.Bucket).Object(dest)
	w := outputBlob.NewWriter(ctx)
	defer w.Close()

	// Use - as input and output to use stdin and stdout.
	var stderr bytes.Buffer
	cmd := exec.Command("convert", "-", "-thumbnail", "x100", "-")
	cmd.Stdin = r
	cmd.Stdout = w
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error in imagemagick call: %s", stderr.String())
	}

	return nil
}

func thumbnailPath(name string) string {
	ext := filepath.Ext(name)
	newBase := strings.Replace(filepath.Base(name), ext, "/thumbnail"+ext, 1)
	newPath := "processed/" + newBase
	return newPath
}

func originalPath(name string) string {
	ext := filepath.Ext(name)
	newBase := strings.Replace(filepath.Base(name), ext, "/original"+ext, 1)
	newPath := "processed/" + newBase
	return newPath
}
