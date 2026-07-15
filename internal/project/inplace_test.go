package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/render"
)

// readBackInPlaceBody extracts the interior between an in-place section's pointer
// and awf's next registered section pointer, verbatim (internal blank lines kept),
// trimming only the awf-owned leading/trailing framing.
func TestReadBackInPlaceBody(t *testing.T) {
	// invariant: in-place-readback
	t.Run("exact interior, internal blank preserved", func(t *testing.T) {
		out := "head\n" +
			"<!-- awf:edit-in-place body — your edits -->\n" +
			"line one\n\nline two\n" +
			"<!-- awf:edit next — default; create y -->\n" +
			"tail\n"
		got, ok := readBackInPlaceBody(out, "body", []string{"body", "next"}, render.HTMLComment)
		if !ok || got != "line one\n\nline two" {
			t.Errorf("got %q ok=%v, want %q", got, ok, "line one\n\nline two")
		}
	})

	t.Run("absent pointer falls back", func(t *testing.T) {
		if _, ok := readBackInPlaceBody("no pointers here\n", "body", []string{"body"}, render.HTMLComment); ok {
			t.Error("a missing own pointer must return ok=false so the caller uses the default")
		}
	})

	t.Run("a pointer-shaped line for a non-registered name does not truncate", func(t *testing.T) {
		out := "<!-- awf:edit-in-place body — x -->\n" +
			"before\n<!-- awf:edit bogus — not a registered section -->\nafter\n" +
			"<!-- awf:edit next — default -->\ntail\n"
		got, _ := readBackInPlaceBody(out, "body", []string{"body", "next"}, render.HTMLComment)
		want := "before\n<!-- awf:edit bogus — not a registered section -->\nafter"
		if got != want {
			t.Errorf("boundary matched a non-registered pointer shape\ngot  %q\nwant %q", got, want)
		}
	})

	// invariant: in-place-spacing-owned
	t.Run("leading and trailing blank framing trimmed", func(t *testing.T) {
		out := "<!-- awf:edit-in-place body — x -->\n\n \nCONTENT\n\n\n<!-- awf:edit next — d -->\ntail\n"
		got, _ := readBackInPlaceBody(out, "body", []string{"body", "next"}, render.HTMLComment)
		if got != "CONTENT" {
			t.Errorf("framing not trimmed: got %q", got)
		}
	})

	t.Run("last section reads to EOF", func(t *testing.T) {
		out := "<!-- awf:edit-in-place body — x -->\nonly content\n"
		got, ok := readBackInPlaceBody(out, "body", []string{"body"}, render.HTMLComment)
		if !ok || got != "only content" {
			t.Errorf("got %q ok=%v, want %q", got, ok, "only content")
		}
	})

	// A shell (#!-shebang) target uses #-comment pointers; read-back bounds on the
	// #-style pointer and is not truncated by a #-pointer-shaped adopter line.
	t.Run("hash-comment target", func(t *testing.T) {
		out := "#!/usr/bin/env bash\n" +
			"# awf:edit-in-place setup — your edits\n" +
			"helper() { :; }\n# awf:edit bogus — nope\n" +
			"# awf:edit dispatch — default; create z\n" +
			"case x in esac\n"
		got, _ := readBackInPlaceBody(out, "setup", []string{"setup", "dispatch"}, render.HashComment)
		want := "helper() { :; }\n# awf:edit bogus — nope"
		if got != want {
			t.Errorf("hash-style read-back\ngot  %q\nwant %q", got, want)
		}
	})
}

// planSections refuses a section that is both in-place-editable and part-backed.
func TestPlanSectionsInPlacePartExclusive(t *testing.T) {
	// invariant: section-source-exclusive
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// Author a convention part for a section the template declares in-place.
	part := p.Cfg.PartPath("skills", "foo", "s")
	if err := os.MkdirAll(filepath.Dir(part), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, []byte("conflicting part body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	segs := render.ParseSections("<!-- awf:section s inplace -->\nDEFAULT\n<!-- awf:end -->\n")
	_, err = p.planSections("skills", "foo", []string{"s"}, nil, segs, "out.md", render.HTMLComment)
	if err == nil || !strings.Contains(err.Error(), "in-place-editable and must not also have a convention part") {
		t.Fatalf("want a section-source-exclusive error, got %v", err)
	}
}

// planSections sources an in-place section's body from the existing output, and
// falls back to the default when the output is absent (first render).
func TestPlanSectionsInPlaceReadBack(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	segs := render.ParseSections(
		"<!-- awf:section s inplace -->\nDEFAULT\n<!-- awf:end -->\n" +
			"<!-- awf:section next -->\nN\n<!-- awf:end -->\n")
	declared := []string{"s", "next"}

	// Absent output → in-place with an empty body (Assemble uses the default).
	plan, err := p.planSections("skills", "foo", declared, nil, segs, "out.md", render.HTMLComment)
	if err != nil {
		t.Fatal(err)
	}
	if !plan["s"].InPlace || plan["s"].InPlaceBody != "" {
		t.Errorf("absent output: want InPlace with empty body, got %#v", plan["s"])
	}

	// Present output → the section body is read back from disk.
	out := "banner\n" +
		"<!-- awf:edit-in-place s — your edits -->\n" +
		"adopter line\n\nsecond line\n" +
		"<!-- awf:edit next — default; create x -->\nN\n"
	if err := os.WriteFile(filepath.Join(root, "out.md"), []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err = p.planSections("skills", "foo", declared, nil, segs, "out.md", render.HTMLComment)
	if err != nil {
		t.Fatal(err)
	}
	if !plan["s"].InPlace || plan["s"].InPlaceBody != "adopter line\n\nsecond line" {
		t.Errorf("present output: got InPlaceBody %q", plan["s"].InPlaceBody)
	}
}

// PointerLinePrefixes stays in lockstep with editPointer: every rendered pointer
// begins with one of the prefixes read-back matches on, in both comment styles.
func TestPointerPrefixesMatchRenderedPointers(t *testing.T) {
	for _, style := range []render.CommentStyle{render.HTMLComment, render.HashComment} {
		src := "<!-- awf:section s inplace -->\nD\n<!-- awf:end -->\n"
		if style == render.HashComment {
			src = "#!/bin/sh\n" + src
		}
		asm, _ := render.Assemble(render.ParseSections(src),
			map[string]render.SectionPlan{"s": {InPlace: true, InPlaceBody: "x"}}, style)
		var ptrLine string
		for _, ln := range strings.Split(asm, "\n") {
			if strings.Contains(ln, "awf:edit-in-place s") {
				ptrLine = strings.TrimSpace(ln)
			}
		}
		if !hasAnyPrefix(ptrLine, render.PointerLinePrefixes("s", style)) {
			t.Errorf("style %v: rendered pointer %q not matched by PointerLinePrefixes", style, ptrLine)
		}
	}
}
