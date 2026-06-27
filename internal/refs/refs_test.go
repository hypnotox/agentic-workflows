package refs_test

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/refs"
)

func TestLinks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"simple", "see [a](b.md) here", []string{"b.md"}},
		{"http skipped", "[x](http://e.com)", nil},
		{"https skipped", "[x](https://e.com)", nil},
		{"mailto skipped", "[x](mailto:a@b.c)", nil},
		{"tel skipped", "[x](tel:123)", nil},
		{"anchor-only skipped", "[x](#frag)", nil},
		{"trailing anchor stripped", "[x](b.md#sec)", []string{"b.md"}},
		{"title stripped double", "[x](b.md \"T\")", []string{"b.md"}},
		{"title stripped single", "[x](b.md 'T')", []string{"b.md"}},
		{"angle-bracket dest", "[x](<b.md>)", []string{"b.md"}},
		{"multiple", "[a](x.md) and [c](y.md)", []string{"x.md", "y.md"}},
		{"none", "no links here", nil},
		{"bracket without link", "a [bracket only", nil},
		{"unterminated dest", "[a](b.md and more", nil},
		{"fenced backtick", "```\n[a](b.md)\n```", nil},
		{"fenced tilde", "~~~\n[a](b.md)\n~~~", nil},
		{"link after fence", "```\n[a](skip.md)\n```\n[b](keep.md)", []string{"keep.md"}},
		{"image extracted", "![alt](img.png)", []string{"img.png"}},
		{"code span dropped", "`[x](y.md)`", nil},
		{"code span kept real link", "`[x](skip.md)` and [r](keep.md)", []string{"keep.md"}},
		{"unpaired backtick keeps link", "a `b [r](keep.md)", []string{"keep.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := refs.Links(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("Links(%q) = %v, want %v", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("Links(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
				}
			}
		})
	}
}
