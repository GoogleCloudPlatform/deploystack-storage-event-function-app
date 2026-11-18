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
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func makeMultipartRequest(t *testing.T, method, target, filename, declaredType string, payload []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name="myFile"; filename=%q`, filename))
	if declaredType != "" {
		hdr.Set("Content-Type", declaredType)
	}
	part, err := mw.CreatePart(hdr)
	if err != nil {
		t.Fatalf("CreatePart: %v", err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	req := httptest.NewRequest(method, target, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestSniffContentType(t *testing.T) {
	// 43-byte minimal valid 1x1 GIF89a (< 512 bytes, triggers io.ErrUnexpectedEOF on ReadFull).
	tinyGIF := []byte{
		'G', 'I', 'F', '8', '9', 'a',
		0x01, 0x00, 0x01, 0x00, 0x80, 0x00, 0x00,
		0xff, 0xff, 0xff, 0x00, 0x00, 0x00,
		0x21, 0xf9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00,
		0x2c, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00,
		0x02, 0x02, 0x44, 0x01, 0x00, 0x3b,
	}
	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R'}
	miffPayload := []byte("id=ImageMagick\nprofile=icc:/proc/self/environ\ncolumns=1 rows=1 depth=8\n:\n\xff\xff\xff")

	tests := []struct {
		name         string
		declaredType string
		payload      []byte
		wantMIME     string
	}{
		{
			name:         "tiny GIF under 512 bytes",
			declaredType: "application/octet-stream",
			payload:      tinyGIF,
			wantMIME:     "image/gif",
		},
		{
			name:         "valid PNG header",
			declaredType: "application/octet-stream",
			payload:      pngHeader,
			wantMIME:     "image/png",
		},
		{
			name:         "MIFF with spoofed image/png header",
			declaredType: "image/png",
			payload:      miffPayload,
			wantMIME:     "application/octet-stream",
		},
		{
			name:         "empty payload",
			declaredType: "image/png",
			payload:      []byte{},
			wantMIME:     "application/octet-stream",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := makeMultipartRequest(t, http.MethodPost, "/api/v1/image", "test.png", tc.declaredType, tc.payload)
			f, _, err := req.FormFile("myFile")
			if err != nil {
				t.Fatalf("FormFile: %v", err)
			}
			defer f.Close()

			got, err := sniffContentType(f)
			if err != nil {
				t.Fatalf("sniffContentType error: %v", err)
			}
			if got != tc.wantMIME {
				t.Fatalf("sniffContentType = %q, want %q", got, tc.wantMIME)
			}

			// Verify read offset was reset to 0 so full bytes are still readable.
			var remaining bytes.Buffer
			if _, err := io.Copy(&remaining, f); err != nil {
				t.Fatalf("io.Copy after seek: %v", err)
			}
			if !bytes.Equal(remaining.Bytes(), tc.payload) {
				t.Fatalf("post-sniff read got %d bytes, want exact %d bytes", remaining.Len(), len(tc.payload))
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	valid := map[string]string{
		"ColtReto.png":             "ColtReto.png",
		`C:\fakepath\ColtReto.png`: "ColtReto.png",
		"subdir/photo_1.jpg":       "photo_1.jpg",
	}
	for in, want := range valid {
		got, err := sanitizeFilename(in)
		if err != nil {
			t.Fatalf("sanitizeFilename(%q) unexpected error: %v", in, err)
		}
		if got != want {
			t.Fatalf("sanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}

	invalid := []string{
		"",
		"   ",
		".",
		"..",
		"../ColtReto.png",
		"../../etc/passwd",
		".hidden.png",
		"<img src=x onerror=alert(1)>.png",
		`bad"quote.png`,
		"exploit.html",
		"exploit.svg",
	}
	for _, in := range invalid {
		if got, err := sanitizeFilename(in); err == nil {
			t.Fatalf("sanitizeFilename(%q) succeeded with %q, expected error", in, got)
		}
	}
}

func TestHandlersRejectSpoofedMIMEAndXSSFilename(t *testing.T) {
	miffPayload := []byte("id=ImageMagick\nprofile=icc:/proc/self/environ\ncolumns=1 rows=1 depth=8\n:\n\xff\xff\xff")

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/image", createHandler).Methods(http.MethodPost)
	router.HandleFunc("/api/v1/image/{id}", updateHandler).Methods(http.MethodPut)

	// 1. POST with spoofed Content-Type: image/png and MIFF body must fail before cs.Create.
	reqPost := makeMultipartRequest(t, http.MethodPost, "/api/v1/image", "notimage.bin", "image/png", miffPayload)
	rrPost := httptest.NewRecorder()
	router.ServeHTTP(rrPost, reqPost)
	if rrPost.Code == http.StatusCreated {
		t.Fatalf("createHandler accepted spoofed MIME payload")
	}
	if !strings.Contains(rrPost.Body.String(), "invalid image type") {
		t.Fatalf("unexpected createHandler body: %s", rrPost.Body.String())
	}

	// 2. PUT with MIFF body must fail before cs.Delete / cs.Create.
	reqPut := makeMultipartRequest(t, http.MethodPut, "/api/v1/image/victim", "notimage.bin", "", miffPayload)
	rrPut := httptest.NewRecorder()
	router.ServeHTTP(rrPut, reqPut)
	if rrPut.Code == http.StatusOK {
		t.Fatalf("updateHandler accepted non-image payload")
	}
	if !strings.Contains(rrPut.Body.String(), "invalid image type") {
		t.Fatalf("unexpected updateHandler body: %s", rrPut.Body.String())
	}

	// 3. POST with valid PNG header but XSS filename must fail before cs.Create.
	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R'}
	reqXSS := makeMultipartRequest(t, http.MethodPost, "/api/v1/image", "<img src=x onerror=alert(1)>.png", "image/png", pngHeader)
	rrXSS := httptest.NewRecorder()
	router.ServeHTTP(rrXSS, reqXSS)
	if rrXSS.Code == http.StatusCreated {
		t.Fatalf("createHandler accepted XSS filename")
	}
	if !strings.Contains(rrXSS.Body.String(), "invalid filename") {
		t.Fatalf("unexpected XSS filename error body: %s", rrXSS.Body.String())
	}
}
