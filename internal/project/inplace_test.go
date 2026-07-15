package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/render"
)

// readBackInPlaceBody extracts the interior between an in-place section's pointer
// and awf's next registered section pointer, verbatim (internal blank lines kept),
// trimming only the awf-owned leading/trailing framing.
func TestReadBackInPlaceBody(t *testing.T) {
	// invariant: in-place-readback
	t.Run("exact interior, internal blank preserved", func(t *testing.T) {
		out := "head\n" +
			"<!-- awf:edit-in-place body: your edits -->\n" +
			"line one\n\nline two\n" +
			"<!-- awf:edit next: default; create y -->\n" +
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
		out := "<!-- awf:edit-in-place body: x -->\n" +
			"before\n<!-- awf:edit bogus: not a registered section -->\nafter\n" +
			"<!-- awf:edit next: default -->\ntail\n"
		got, _ := readBackInPlaceBody(out, "body", []string{"body", "next"}, render.HTMLComment)
		want := "before\n<!-- awf:edit bogus: not a registered section -->\nafter"
		if got != want {
			t.Errorf("boundary matched a non-registered pointer shape\ngot  %q\nwant %q", got, want)
		}
	})

	// invariant: in-place-spacing-owned
	t.Run("leading and trailing blank framing trimmed", func(t *testing.T) {
		out := "<!-- awf:edit-in-place body: x -->\n\n \nCONTENT\n\n\n<!-- awf:edit next: d -->\ntail\n"
		got, _ := readBackInPlaceBody(out, "body", []string{"body", "next"}, render.HTMLComment)
		if got != "CONTENT" {
			t.Errorf("framing not trimmed: got %q", got)
		}
	})

	t.Run("last section reads to EOF", func(t *testing.T) {
		out := "<!-- awf:edit-in-place body: x -->\nonly content\n"
		got, ok := readBackInPlaceBody(out, "body", []string{"body"}, render.HTMLComment)
		if !ok || got != "only content" {
			t.Errorf("got %q ok=%v, want %q", got, ok, "only content")
		}
	})

	// A shell (#!-shebang) target uses #-comment pointers; read-back bounds on the
	// #-style pointer and is not truncated by a #-pointer-shaped adopter line.
	t.Run("hash-comment target", func(t *testing.T) {
		out := "#!/usr/bin/env bash\n" +
			"# awf:edit-in-place setup: your edits\n" +
			"helper() { :; }\n# awf:edit bogus: nope\n" +
			"# awf:edit dispatch: default; create z\n" +
			"case x in esac\n"
		got, _ := readBackInPlaceBody(out, "setup", []string{"setup", "dispatch"}, render.HashComment)
		want := "helper() { :; }\n# awf:edit bogus: nope"
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
	if !plan["s"].InPlace || plan["s"].InPlaceFound || plan["s"].InPlaceBody != "" {
		t.Errorf("absent output: want InPlace, not found, empty body, got %#v", plan["s"])
	}

	// Present output → the section body is read back from disk.
	out := "banner\n" +
		"<!-- awf:edit-in-place s: your edits -->\n" +
		"adopter line\n\nsecond line\n" +
		"<!-- awf:edit next: default; create x -->\nN\n"
	if err := os.WriteFile(filepath.Join(root, "out.md"), []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err = p.planSections("skills", "foo", declared, nil, segs, "out.md", render.HTMLComment)
	if err != nil {
		t.Fatal(err)
	}
	if !plan["s"].InPlace || !plan["s"].InPlaceFound || plan["s"].InPlaceBody != "adopter line\n\nsecond line" {
		t.Errorf("present output: got found=%v InPlaceBody %q", plan["s"].InPlaceFound, plan["s"].InPlaceBody)
	}
}

// setRegion rewrites the lines of an in-place region — between its
// `# awf:edit-in-place <section>` pointer and the next `# awf:edit`-family
// pointer — to body, mimicking an adopter editing the rendered output in place.
func setRegion(t *testing.T, content, section, body string) string {
	t.Helper()
	lines := strings.Split(content, "\n")
	start, end := -1, -1
	for i, ln := range lines {
		tl := strings.TrimSpace(ln)
		if strings.HasPrefix(tl, "# awf:edit-in-place "+section+": ") {
			start = i
			continue
		}
		if start >= 0 && strings.HasPrefix(tl, "# awf:edit") {
			end = i
			break
		}
	}
	if start < 0 || end < 0 {
		t.Fatalf("region %q not found between pointers", section)
	}
	out := append([]string{}, lines[:start+1]...)
	if body != "" {
		out = append(out, body)
	}
	out = append(out, lines[end:]...)
	return strings.Join(out, "\n")
}

// End-to-end fixpoint over a real Sync→edit→Sync→Check cycle on the rendered
// runner: an in-place edit (including emptying the region) survives re-sync and
// is drift-free, while an edit to an awf-owned region surfaces as drift.
// invariant: in-place-tamper-drift
// invariant: in-place-spacing-owned
func TestRunnerInPlaceFixpoint(t *testing.T) {
	root := scaffold(t, "prefix: example\nrunner:\n  enabled: true\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	xPath := filepath.Join(root, "x")
	mustSync := func() {
		t.Helper()
		if err := p.Sync(); err != nil {
			t.Fatal(err)
		}
	}
	read := func() string {
		t.Helper()
		b, err := os.ReadFile(xPath)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	// runnerDrift returns the drift entries the full check reports for the runner.
	runnerDrift := func() []manifest.Drift {
		t.Helper()
		all, err := p.Check()
		if err != nil {
			t.Fatal(err)
		}
		var d []manifest.Drift
		for _, dr := range all {
			if dr.Path == "x" {
				d = append(d, dr)
			}
		}
		return d
	}
	mustClean := func(when string) {
		t.Helper()
		if d := runnerDrift(); len(d) != 0 {
			t.Errorf("%s: want no runner drift, got %v", when, d)
		}
	}
	mustSync()

	// (a) An in-place edit to the project verbs survives re-sync and is clean.
	edited := setRegion(t, read(), "runner-project-verbs", "gate)\n\tgo test ./... ;;")
	if err := os.WriteFile(xPath, []byte(edited), 0o755); err != nil {
		t.Fatal(err)
	}
	mustSync()
	got := read()
	if !strings.Contains(got, "go test ./... ;;") {
		t.Errorf("in-place edit not preserved across sync:\n%s", got)
	}
	if !strings.Contains(got, `"$(bash .awf/bootstrap.sh)" "$cmd" "$@" ;;`) {
		t.Errorf("awf-owned dispatch lost:\n%s", got)
	}
	mustClean("after in-place edit + sync")

	// (b) Emptying the region stays empty — NOT reverted to the default stubs.
	if err := os.WriteFile(xPath, []byte(setRegion(t, read(), "runner-project-verbs", "")), 0o755); err != nil {
		t.Fatal(err)
	}
	mustSync()
	if strings.Contains(read(), "define the 'gate' project verb") {
		t.Errorf("emptied region reverted to the default stubs:\n%s", read())
	}
	mustClean("after emptying the region")

	// (c) Tampering an awf-owned region surfaces as drift.
	tampered := strings.Replace(read(), "set -euo pipefail", "set -euo pipefail\n# adopter tampering an awf-owned region", 1)
	if err := os.WriteFile(xPath, []byte(tampered), 0o755); err != nil {
		t.Fatal(err)
	}
	if d := runnerDrift(); len(d) == 0 {
		t.Error("tampering an awf-owned region must surface as drift")
	}
}

func TestAnyInPlace(t *testing.T) {
	if !anyInPlace(map[string]render.SectionPlan{"a": {}, "b": {InPlace: true}}) {
		t.Error("a plan with an in-place section must report true")
	}
	if anyInPlace(map[string]render.SectionPlan{"a": {}, "b": {HasPart: true}}) {
		t.Error("a plan with no in-place section must report false")
	}
}

// An in-place file is drift-checked by regeneration-with-read-back: on-disk is
// compared to the freshly regenerated content, not the frozen OutputHash. An edit
// to an awf-owned region surfaces as drift; a matching file does not.
func TestCheckLockedFilesInPlaceRegenDrift(t *testing.T) {
	// invariant: in-place-tamper-drift
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	canonical := "#!/bin/sh\n# awf:edit-in-place s: your edits\nadopter line\n"
	xPath := filepath.Join(root, "x")
	lock := &manifest.Lock{Files: map[string]manifest.Entry{
		"x": {RegenChecked: true, OutputHash: manifest.Hash([]byte(canonical))},
	}}
	rendered := map[string]RenderedFile{"x": {Path: "x", Content: canonical, RegenChecked: true}}

	// On-disk equals the regenerated content (in-place body already read back) → clean.
	if err := os.WriteFile(xPath, []byte(canonical), 0o644); err != nil {
		t.Fatal(err)
	}
	if d := p.checkLockedFiles(lock, rendered); len(d) != 0 {
		t.Errorf("a matching in-place file must not drift, got %v", d)
	}

	// An awf-owned region edited on disk → regenerated content differs → hand-edited.
	tampered := "#!/bin/sh\n# awf owns this and it was TAMPERED\n# awf:edit-in-place s: your edits\nadopter line\n"
	if err := os.WriteFile(xPath, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	d := p.checkLockedFiles(lock, rendered)
	if len(d) != 1 || d[0].Kind != "hand-edited" {
		t.Fatalf("a tampered awf region must drift hand-edited, got %v", d)
	}

	// Absent file → missing.
	if err := os.Remove(xPath); err != nil {
		t.Fatal(err)
	}
	d = p.checkLockedFiles(lock, rendered)
	if len(d) != 1 || d[0].Kind != "missing" {
		t.Fatalf("an absent in-place file → missing, got %v", d)
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
			map[string]render.SectionPlan{"s": {InPlace: true, InPlaceFound: true, InPlaceBody: "x"}}, style)
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
