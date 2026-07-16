package render_test

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/render"
)

// TestStripAuthoringComments pins the whole-line-only, exact-literal strip
// contract (ADR-0121 Decision 1): a directive line vanishes with its newline,
// everything else - mid-line occurrences, prefix-sharing tokens, fenced
// demos - renders verbatim.
// invariant: authoring-comment-whole-line-only
func TestStripAuthoringComments(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"mid-file, no blank residue",
			"a\n<!-- awf:comment note -->\nb\n", "a\nb\n"},
		{"comment-only input strips empty",
			"<!-- awf:comment note -->\n", ""},
		{"directive as unterminated last line strips empty",
			"<!-- awf:comment note -->", ""},
		{"indented directive stripped",
			"a\n  <!-- awf:comment note -->\nb\n", "a\nb\n"},
		{"immediate close stripped",
			"a\n<!-- awf:comment-->\nb\n", "a\nb\n"},
		{"mid-line occurrence preserved",
			"text <!-- awf:comment note -->\n", "text <!-- awf:comment note -->\n"},
		{"prefix-sharing token preserved (closed)",
			"<!-- awf:commentary -->\n", "<!-- awf:commentary -->\n"},
		{"prefix-sharing token preserved (unclosed, never an error)",
			"<!-- awf:commentary no close\n", "<!-- awf:commentary no close\n"},
		{"backtick fence preserves directive",
			"```\n<!-- awf:comment demo -->\n```\n", "```\n<!-- awf:comment demo -->\n```\n"},
		{"tilde fence preserves directive",
			"~~~\n<!-- awf:comment demo -->\n~~~\n", "~~~\n<!-- awf:comment demo -->\n~~~\n"},
		{"fence preserves malformed opener",
			"```\n<!-- awf:comment unclosed\n```\n", "```\n<!-- awf:comment unclosed\n```\n"},
		{"strip resumes after fence closes",
			"```\nx\n```\n<!-- awf:comment gone -->\ny\n", "```\nx\n```\ny\n"},
	}
	for _, c := range cases {
		got, err := render.StripAuthoringComments(c.in)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

// TestStripAuthoringCommentsMalformed pins the hard-error contract (ADR-0121
// Decision 3): outside a fence, a whole line opening at the directive token
// boundary that does not end with "-->" fails, naming the line, and the input
// comes back unchanged.
// invariant: authoring-comment-malformed-fails
func TestStripAuthoringCommentsMalformed(t *testing.T) {
	cases := []struct {
		name, in string
	}{
		{"bare opener", "a\n<!-- awf:comment\n"},
		{"missing close", "a\n<!-- awf:comment no close\n"},
		{"text trailing the close", "a\n<!-- awf:comment x --> extra\n"},
	}
	for _, c := range cases {
		got, err := render.StripAuthoringComments(c.in)
		if err == nil {
			t.Errorf("%s: expected an error", c.name)
			continue
		}
		if !strings.Contains(err.Error(), "line 2") || !strings.Contains(err.Error(), "malformed awf:comment") {
			t.Errorf("%s: error must name the line and the directive, got %v", c.name, err)
		}
		if got != c.in {
			t.Errorf("%s: input must come back unchanged on error, got %q", c.name, got)
		}
	}
}
