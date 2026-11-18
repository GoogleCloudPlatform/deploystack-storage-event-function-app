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

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

var cs CloudStorage

var allowedImageMIMETypes = []string{"image/png", "image/jpeg", "image/gif"}

var blockedFilenameExtensions = map[string]bool{
	".html":  true,
	".htm":   true,
	".xhtml": true,
	".svg":   true,
	".xml":   true,
	".js":    true,
	".mjs":   true,
}

func main() {
	port := os.Getenv("PORT")
	bucket := os.Getenv("BUCKET")

	if port == "" {
		port = "8080"
	}

	fmt.Printf("Port: %s\n", port)

	var err error
	cs, err = NewCloudStorage(bucket)
	if err != nil {
		log.Printf("failed to create client: %v", err)
		return
	}
	defer cs.Close()

	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/api/v1/image", listHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/v1/image", createHandler).Methods(http.MethodPost)
	router.HandleFunc("/api/v1/image/{id}", readHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/v1/image/{id}", deleteHandler).Methods(http.MethodDelete)
	router.HandleFunc("/api/v1/image/{id}", updateHandler).Methods(http.MethodPost, http.MethodPut)

	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./static/")))

	var handler http.Handler = router
	if allowedOrigin := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGIN")); allowedOrigin != "" && allowedOrigin != "*" {
		headersOk := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"})
		originsOk := handlers.AllowedOrigins([]string{allowedOrigin})
		methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS", "DELETE"})
		handler = handlers.CORS(originsOk, headersOk, methodsOk)(router)
	}

	log.Fatal(http.ListenAndServe(":"+port, handler))
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	fs, err := cs.List()
	if err != nil {
		writeErrorMsg(w, fmt.Errorf("failed to list files: %v", err))
		return
	}

	is, err := NewImages(fs)
	if err != nil {
		writeErrorMsg(w, fmt.Errorf("failed to convert files to images images: %v", err))
		return
	}

	writeJSON(w, is, http.StatusOK)
}

func createHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeErrorMsg(w, fmt.Errorf("error retrieving file: %v", err))
		return
	}
	// FormFile returns the first file for the given key `myFile`
	// it also returns the FileHeader so we can get the Filename,
	// the Header and the size of the file
	file, handler, err := r.FormFile("myFile")
	if err != nil {
		writeErrorMsg(w, fmt.Errorf("error retrieving file: %v", err))
		return
	}
	defer file.Close()

	mimemap := NewMimeMap(allowedImageMIMETypes)

	mimetype, err := sniffContentType(file)
	if err != nil {
		writeErrorMsg(w, fmt.Errorf("error detecting file type: %v", err))
		return
	}

	if !mimemap.Valid(mimetype) {
		mimelist := mimemap.List()
		writeErrorMsg(w, fmt.Errorf("invalid image type, want one of %s got : %s", mimelist, mimetype))
		return
	}

	filename, err := sanitizeFilename(handler.Filename)
	if err != nil {
		writeErrorMsg(w, err)
		return
	}

	if err := cs.Create(filename, file); err != nil {
		writeErrorMsg(w, fmt.Errorf("image couldn't be created: %v", err))
		return
	}

	writeResponse(w, http.StatusCreated, "")
}

// sniffContentType determines the MIME type of an uploaded file from its
// leading magic bytes rather than the client-supplied multipart header.
// The file's read offset is reset to zero on return.
func sniffContentType(f multipart.File) (string, error) {
	buf := make([]byte, 512)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	if n == 0 {
		return "application/octet-stream", nil
	}
	return http.DetectContentType(buf[:n]), nil
}

// sanitizeFilename strips directory components and rejects empty, traversal,
// hidden, control-character, or HTML/URL-metacharacter filenames.
func sanitizeFilename(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.Contains(trimmed, "..") {
		return "", fmt.Errorf("invalid filename")
	}
	normalized := strings.ReplaceAll(trimmed, "\\", "/")
	name := strings.TrimSpace(filepath.Base(normalized))
	if name == "" || name == "." || name == ".." || name == "/" || strings.HasPrefix(name, ".") || len(name) > 255 {
		return "", fmt.Errorf("invalid filename")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("invalid filename")
		}
		switch r {
		case '/', '\\', '<', '>', '"', '\'', '&', '`', '?', '#', '%':
			return "", fmt.Errorf("invalid filename")
		}
	}
	ext := strings.ToLower(filepath.Ext(name))
	if blockedFilenameExtensions[ext] {
		return "", fmt.Errorf("invalid filename extension: %s", ext)
	}
	return name, nil
}

// validateImageID ensures an image identifier is a safe single path segment.
func validateImageID(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" || id == "." || id == ".." || strings.Contains(id, "..") || len(id) > 255 {
		return fmt.Errorf("invalid image id")
	}
	for _, r := range id {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("invalid image id")
		}
		switch r {
		case '/', '\\', '<', '>', '"', '\'', '&', '`', '?', '#', '%':
			return fmt.Errorf("invalid image id")
		}
	}
	return nil
}

type MimeMap map[string]bool

func NewMimeMap(s []string) MimeMap {
	m := make(MimeMap)
	for _, v := range s {
		m[v] = true
	}
	return m
}

func (m MimeMap) Valid(mimetype string) bool {
	_, ok := m[mimetype]
	return ok
}

func (m MimeMap) List() string {
	var sb strings.Builder
	for i := range m {
		sb.WriteString(i)
		sb.WriteString(", ")
	}

	return strings.TrimRight(sb.String(), ", ")
}

func updateHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := validateImageID(id); err != nil {
		writeErrorMsg(w, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeErrorMsg(w, fmt.Errorf("error retrieving file: %v", err))
		return
	}
	// FormFile returns the first file for the given key `myFile`
	// it also returns the FileHeader so we can get the Filename,
	// the Header and the size of the file
	file, handler, err := r.FormFile("myFile")
	if err != nil {
		writeErrorMsg(w, fmt.Errorf("error retrieving file: %v", err))
		return
	}
	defer file.Close()

	mimemap := NewMimeMap(allowedImageMIMETypes)

	mimetype, err := sniffContentType(file)
	if err != nil {
		writeErrorMsg(w, fmt.Errorf("error detecting file type: %v", err))
		return
	}

	if !mimemap.Valid(mimetype) {
		mimelist := mimemap.List()
		writeErrorMsg(w, fmt.Errorf("invalid image type, want one of %s got : %s", mimelist, mimetype))
		return
	}

	filename, err := sanitizeFilename(handler.Filename)
	if err != nil {
		writeErrorMsg(w, err)
		return
	}

	if err := cs.Delete(id); err != nil {
		writeErrorMsg(w, fmt.Errorf("error replacing file: %s", err))
		return
	}

	if err := cs.Create(filename, file); err != nil {
		writeErrorMsg(w, fmt.Errorf("image couldn't be created: %v", err))
		return
	}

	writeResponse(w, http.StatusOK, "")
}

func readHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := validateImageID(id); err != nil {
		writeErrorMsg(w, err)
		return
	}

	fs, err := cs.Read(id)
	if err != nil {
		writeErrorMsg(w, fmt.Errorf("failed to read files %s: %v", id, err))
		return
	}

	is, err := NewImages(fs)
	if err != nil {
		writeErrorMsg(w, fmt.Errorf("failed to convert files to images images: %v", err))
		return
	}
	if len(is) < 1 {
		writeResponse(w, http.StatusNoContent, "")
		return
	}

	writeJSON(w, is[0], http.StatusOK)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := validateImageID(id); err != nil {
		writeErrorMsg(w, err)
		return
	}

	if err := cs.Delete(id); err != nil {
		writeErrorMsg(w, err)
		return
	}
	msg := Message{"image deleted", fmt.Sprintf("image id: %s", id)}

	writeJSON(w, msg, http.StatusNoContent)
}

// JSONProducer is an interface that spits out a JSON string version of itself
type JSONProducer interface {
	JSON() (string, error)
	JSONBytes() ([]byte, error)
}

func writeJSON(w http.ResponseWriter, j JSONProducer, status int) {
	jsonStr, err := j.JSON()
	if err != nil {
		writeErrorMsg(w, err)
		return
	}
	writeResponse(w, status, jsonStr)
}

func writeErrorMsg(w http.ResponseWriter, err error) {
	b, marshalErr := json.Marshal(struct {
		Error string `json:"error"`
	}{Error: err.Error()})
	if marshalErr != nil {
		writeResponse(w, http.StatusInternalServerError, `{"error":"internal server error"}`)
		return
	}
	writeResponse(w, http.StatusInternalServerError, string(b))
}

func writeResponse(w http.ResponseWriter, status int, msg string) {
	if status != http.StatusOK {
		weblog(msg)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	w.Write([]byte(msg))
}

func weblog(msg string) {
	log.Printf("Webserver : %s", msg)
}

// Message is a structure for communicating additional data to API consumer.
type Message struct {
	Text    string `json:"text"`
	Details string `json:"details"`
}

// JSON marshalls the content of a todo to json.
func (m Message) JSON() (string, error) {
	bytes, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("could not marshal json for response: %s", err)
	}

	return string(bytes), nil
}

// JSONBytes marshalls the content of a todo to json as a byte array.
func (m Message) JSONBytes() ([]byte, error) {
	bytes, err := json.Marshal(m)
	if err != nil {
		return []byte{}, fmt.Errorf("could not marshal json for response: %s", err)
	}

	return bytes, nil
}
