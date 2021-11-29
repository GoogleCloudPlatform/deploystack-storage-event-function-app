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
	"testing"
)

func TestThumbnailPath(t *testing.T) {
	type test struct {
		input string
		want  string
	}

	tests := []test{
		{input: "uploads/ColtReto.png", want: "processed/ColtReto/thumbnail.png"},
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
	}

	for _, c := range tests {
		got := originalPath(c.input)
		if !(c.want == got) {
			t.Fatalf("expected: %v, got: %v", c.want, got)
		}
	}
}
