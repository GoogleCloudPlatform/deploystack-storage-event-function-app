package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type CloudStorage struct {
	Client storage.Client
	Bucket string
	ctx    context.Context
}

func NewCloudStorage(bucket string) (CloudStorage, error) {
	cs := CloudStorage{}

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return cs, fmt.Errorf("failed to create client: %v", err)
	}
	cs.Client = *client
	cs.Bucket = bucket
	cs.ctx = ctx

	return cs, nil
}

func (cs *CloudStorage) Close() error {
	return cs.Client.Close()
}

func (cs CloudStorage) List() (CSFiles, error) {
	i := CSFiles{}
	bucket := cs.Client.Bucket(cs.Bucket)

	query := &storage.Query{}
	it := bucket.Objects(cs.ctx, query)
	for {
		obj, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return i, fmt.Errorf("error iterating over bucket query: %s", err)
		}

		u, err := url.Parse(obj.MediaLink)
		if err != nil {
			return i, fmt.Errorf("cannot create url from %s: %s", obj.MediaLink, err)
		}
		img := CSFile{obj.Name, cs.Bucket, u}
		i = append(i, img)

	}

	return i, nil
}

type CSFile struct {
	Name   string
	Bucket string
	URL    *url.URL
}

type CSFiles []CSFile

type Image struct {
	Name      string `json:"name"`
	Original  string `json:"original"`
	Thumbnail string `json:"thumbnail"`
}

type Images []Image

func NewImages(fs CSFiles) (Images, error) {
	is := Images{}
	err := is.Load(fs)
	return is, err
}

func (is *Images) Load(fs CSFiles) error {
	for _, v := range fs {
		if strings.Index(v.Name, "original.") > -1 {
			dir := filepath.Dir(v.Name)
			base := filepath.Base(v.Name)
			name := strings.Replace(dir, "processed/", "", 1)
			o := fmt.Sprintf("https://storage.googleapis.com/%s/%s/%s", v.Bucket, dir, base)
			t := fmt.Sprintf("https://storage.googleapis.com/%s/%s/%s", v.Bucket, dir, strings.Replace(base, "original.", "thumbnail.", 1))
			i := Image{name, o, t}
			*is = append(*is, i)
		}
	}

	return nil
}

// JSON marshalls the content of Images to json.
func (is Images) JSON() (string, error) {
	bytes, err := json.Marshal(is)
	if err != nil {
		return "", fmt.Errorf("could not marshal json for response: %s", err)
	}

	return string(bytes), nil
}

// JSONBytes marshalls the content of Images to json.
func (is Images) JSONBytes() ([]byte, error) {
	bytes, err := json.Marshal(is)
	if err != nil {
		return []byte{}, fmt.Errorf("could not marshal json for response: %s", err)
	}

	return bytes, nil
}
