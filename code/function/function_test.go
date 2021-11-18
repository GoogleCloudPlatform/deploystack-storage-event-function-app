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
