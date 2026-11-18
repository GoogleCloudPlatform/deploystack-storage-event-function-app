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

package p

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestThumbnailPath(t *testing.T) {
	type test struct {
		input string
		want  string
	}

	tests := []test{
		{input: "uploads/ColtReto.png", want: "processed/ColtReto/thumbnail.png"},
		{input: "uploads/my.photo.v2.jpg", want: "processed/my.photo.v2/thumbnail.jpg"},
	}

	for _, c := range tests {
		got := thumbnailPath(c.input)
		if !(c.want == got) {
			t.Fatalf("expected: %v, got: %v", c.want, got)
		}
	}
}

func TestOrginalPath(t *testing.T) {
	type test struct {
		input string
		want  string
	}

	tests := []test{
		{input: "uploads/ColtReto.png", want: "processed/ColtReto/original.png"},
		{input: "uploads/my.photo.v2.jpg", want: "processed/my.photo.v2/original.jpg"},
	}

	for _, c := range tests {
		got := originalPath(c.input)
		if !(c.want == got) {
			t.Fatalf("expected: %v, got: %v", c.want, got)
		}
	}
}

func TestCoderForContent(t *testing.T) {
	tests := []struct {
		name      string
		head      []byte
		wantCoder string
		wantErr   bool
	}{
		{
			name:      "PNG",
			head:      []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R'},
			wantCoder: "png:",
		},
		{
			name:      "JPEG",
			head:      []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00},
			wantCoder: "jpeg:",
		},
		{
			name:      "GIF89a",
			head:      []byte("GIF89a\x01\x00\x01\x00"),
			wantCoder: "gif:",
		},
		{
			name:    "MIFF profile=icc exploit",
			head:    []byte("id=ImageMagick\nprofile=icc:/proc/self/environ\ncolumns=1 rows=1 depth=8\n:\n\xff\xff\xff"),
			wantErr: true,
		},
		{
			name:    "SVG XML",
			head:    []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`),
			wantErr: true,
		},
		{
			name:    "empty",
			head:    []byte{},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := coderForContent(tc.head)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("coderForContent(%s) succeeded with %q, want error", tc.name, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("coderForContent(%s) unexpected error: %v", tc.name, err)
			}
			if got != tc.wantCoder {
				t.Fatalf("coderForContent(%s) = %q, want %q", tc.name, got, tc.wantCoder)
			}
		})
	}
}

type chunkedReader struct {
	data []byte
	pos  int
	step int
}

func (c *chunkedReader) Read(p []byte) (int, error) {
	if c.pos >= len(c.data) {
		return 0, io.EOF
	}
	n := c.step
	if n > len(p) {
		n = len(p)
	}
	if rem := len(c.data) - c.pos; n > rem {
		n = rem
	}
	copy(p[:n], c.data[c.pos:c.pos+n])
	c.pos += n
	return n, nil
}

func TestPrepareThumbnailInput(t *testing.T) {
	// 43-byte valid 1x1 GIF89a (< 512 bytes -> triggers io.ErrUnexpectedEOF inside io.ReadFull).
	tinyGIF := []byte{
		'G', 'I', 'F', '8', '9', 'a',
		0x01, 0x00, 0x01, 0x00, 0x80, 0x00, 0x00,
		0xff, 0xff, 0xff, 0x00, 0x00, 0x00,
		0x21, 0xf9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00,
		0x2c, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00,
		0x02, 0x02, 0x44, 0x01, 0x00, 0x3b,
	}

	coder, stream, err := prepareThumbnailInput(bytes.NewReader(tinyGIF))
	if err != nil {
		t.Fatalf("prepareThumbnailInput(tinyGIF) unexpected error: %v", err)
	}
	if coder != "gif:" {
		t.Fatalf("coder = %q, want %q", coder, "gif:")
	}
	var gotGIF bytes.Buffer
	if _, err := io.Copy(&gotGIF, stream); err != nil {
		t.Fatalf("reading reassembled tinyGIF: %v", err)
	}
	if !bytes.Equal(gotGIF.Bytes(), tinyGIF) {
		t.Fatalf("reassembled tinyGIF mismatch: got %d bytes, want %d bytes", gotGIF.Len(), len(tinyGIF))
	}

	// >= 512-byte PNG delivered in small 17-byte chunks to verify io.ReadFull + io.MultiReader.
	largePNG := make([]byte, 1024)
	copy(largePNG, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R'})
	for i := 16; i < len(largePNG); i++ {
		largePNG[i] = byte(i % 251)
	}
	coder, stream, err = prepareThumbnailInput(&chunkedReader{data: largePNG, step: 17})
	if err != nil {
		t.Fatalf("prepareThumbnailInput(largePNG) unexpected error: %v", err)
	}
	if coder != "png:" {
		t.Fatalf("coder = %q, want %q", coder, "png:")
	}
	var gotPNG bytes.Buffer
	if _, err := io.Copy(&gotPNG, stream); err != nil {
		t.Fatalf("reading reassembled largePNG: %v", err)
	}
	if !bytes.Equal(gotPNG.Bytes(), largePNG) {
		t.Fatalf("reassembled largePNG mismatch: got %d bytes, want %d bytes", gotPNG.Len(), len(largePNG))
	}

	// MIFF exploit payload must be rejected before ImageMagick is invoked.
	miff := []byte("id=ImageMagick\nprofile=icc:/proc/self/environ\ncolumns=1 rows=1 depth=8\n:\n\xff\xff\xff")
	if _, _, err := prepareThumbnailInput(bytes.NewReader(miff)); err == nil || !strings.Contains(err.Error(), "unsupported image type") {
		t.Fatalf("prepareThumbnailInput(miff) = %v, want unsupported image type error", err)
	}

	// Empty input must be rejected.
	if _, _, err := prepareThumbnailInput(bytes.NewReader(nil)); err == nil {
		t.Fatalf("prepareThumbnailInput(empty) succeeded, want error")
	}
}
