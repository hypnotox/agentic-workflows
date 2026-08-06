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
func TestReadBackInPlaceBodyExcludesStructuralHeading(t *testing.T) {
	for _, heading := range []string{"## Owned heading", "## "} {
		out := "<!-- awf:edit-in-place body: pointer -->\n" + heading + "\nuser body\n"
		got, ok := readBackInPlaceBody(out, "body", []string{"body"}, render.HTMLComment, heading)
		if !ok || got != "user body" {
			t.Fatalf("heading %q read-back = %q, found %v; want body without structural heading", heading, got, ok)
		}
		// Reassembly with the rendered structural line is a fixpoint rather than
		// duplicating a whitespace-bearing empty-value fallback into the body.
		segs := render.ParseSections("<!-- awf:section body inplace -->\n" + heading + "\ndefault\n<!-- awf:end -->\n")
		assembled, _ := render.Assemble(segs, map[string]render.SectionPlan{"body": {InPlace: true, InPlaceFound: true, InPlaceBody: got}}, render.HTMLComment)
		if strings.Count(assembled, heading+"\n") != 1 {
			t.Fatalf("heading %q duplicated after read-back and reassembly: %q", heading, assembled)
		}
	}
}

func TestReadBackInPlaceBodyChangedStructuralHeading(t *testing.T) {
	out := "<!-- awf:edit-in-place body: pointer -->\n## Tampered heading\nbody\n"
	got, ok := readBackInPlaceBody(out, "body", []string{"body"}, render.HTMLComment, "## Expected heading")
	if !ok || got != "body" {
		t.Fatalf("changed heading = %q, found %v", got, ok)
	}
	for _, changed := range []string{"### Changed heading", "# Changed heading"} {
		out = "<!-- awf:edit-in-place body: pointer -->\n" + changed + "\nbody\n"
		got, _ = readBackInPlaceBody(out, "body", []string{"body"}, render.HTMLComment, "## Expected heading")
		if got != "body" {
			t.Fatalf("changed structural heading %q = %q", changed, got)
		}
	}
	out = "<!-- awf:edit-in-place body: pointer -->\nbody heading is absent\n### Body heading\n"
	got, _ = readBackInPlaceBody(out, "body", []string{"body"}, render.HTMLComment, "## Expected heading")
	if got != "body heading is absent\n### Body heading" {
		t.Fatalf("missing structural slot must preserve subordinate body heading = %q", got)
	}
	out = "<!-- awf:edit-in-place body: pointer -->\n#not a heading\nbody\n"
	got, _ = readBackInPlaceBody(out, "body", []string{"body"}, render.HTMLComment, "## Expected heading")
	if got != "#not a heading\nbody" {
		t.Fatalf("non-heading body line = %q", got)
	}
}

func TestReadBackInPlaceBody(t *testing.T) {
	// invariant: rendering/inplace-and-placeholders:in-place-readback (exact interior, internal blank preserved)
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

	// invariant: rendering/inplace-and-placeholders:in-place-spacing-owned (leading and trailing blank framing trimmed)
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
	// invariant: rendering/inplace-and-placeholders:section-source-exclusive (TestPlanSectionsInPlacePartExclusive)
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
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

// planSections surfaces a read error on an in-place section's convention-part
// probe (the part path readable as neither file nor absent) instead of
// silently treating the part as absent.
func TestPlanSectionsInPlacePartReadError(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	part := p.Cfg.PartPath("skills", "foo", "s")
	if err := os.MkdirAll(part, 0o755); err != nil {
		t.Fatal(err)
	}
	segs := render.ParseSections("<!-- awf:section s inplace -->\nDEFAULT\n<!-- awf:end -->\n")
	if _, err := p.planSections("skills", "foo", []string{"s"}, nil, segs, "out.md", render.HTMLComment); err == nil {
		t.Fatal("in-place part read error accepted")
	}
}

func TestPlanSectionsInPlaceOutputReadError(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "out.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	segs := render.ParseSections("<!-- awf:section s inplace -->\nDEFAULT\n<!-- awf:end -->\n")
	if _, err := p.planSections("skills", "foo", []string{"s"}, nil, segs, "out.md", render.HTMLComment); err == nil || !strings.Contains(err.Error(), "read output out.md") {
		t.Fatalf("in-place output read error = %v", err)
	}
}

// observeRenderInputs records an existing output as a managed-output input when
// the section plan carries an in-place section, so the read-back channel shows
// up among the node's observed inputs.
func TestObserveRenderInputsInPlaceOutput(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "out.md"), []byte("existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inputs, err := p.observeRenderInputs("skills", "foo", "skills/foo/SKILL.md.tmpl", "out.md", map[string]render.SectionPlan{"s": {InPlace: true}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, in := range inputs {
		if in.Path == "out.md" && in.Role == ArtifactManagedOutput {
			found = true
		}
	}
	if !found {
		t.Errorf("in-place plan must observe the existing output as a managed-output input: %#v", inputs)
	}
	if err := os.Remove(filepath.Join(root, "out.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "out.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := p.observeRenderInputs("skills", "foo", "skills/foo/SKILL.md.tmpl", "out.md", map[string]render.SectionPlan{"s": {InPlace: true}}); err == nil {
		t.Fatal("observed in-place output read fault was erased")
	}
}

// planSections sources an in-place section's body from the existing output, and
// falls back to the default when the output is absent (first render).
func TestPlanSectionsInPlaceReadBack(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
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
	// invariant: rendering/inplace-and-placeholders:in-place-tamper-drift (TestCheckLockedFilesInPlaceRegenDrift)
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	canonical := "#!/bin/sh\n# awf:edit-in-place s: your edits\nadopter line\n"
	xPath := filepath.Join(root, "x")
	lock := &manifest.Lock{Files: map[string]manifest.Entry{
		"x": {RegenChecked: true, OutputHash: manifest.Hash([]byte(canonical))},
	}}
	rendered := map[string]RenderedFile{"x": {Path: "x", Content: canonical, RegenChecked: true, TemplateID: "in-place/mock.tmpl", Policy: OutputPolicy{Regenerate: true}}}

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

// End-to-end fixpoint over the real in-place producer-consumer composition.
// invariant: rendering/inplace-and-placeholders:in-place-readback (TestInPlaceComposedSyncCheckFixpoint)
// invariant: rendering/inplace-and-placeholders:in-place-tamper-drift (TestInPlaceComposedSyncCheckFixpoint)
// invariant: rendering/inplace-and-placeholders:in-place-spacing-owned (TestInPlaceComposedSyncCheckFixpoint)
func TestInPlaceComposedSyncCheckFixpoint(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	segs := render.ParseSections(
		"banner\n" +
			"<!-- awf:section s inplace -->\n## Owned heading\nDEFAULT\n<!-- awf:end -->\n" +
			"<!-- awf:section tail -->\nOWNED\n<!-- awf:end -->\n")
	declared := []string{"s", "tail"}
	outPath := filepath.Join(root, "out.md")

	regenerate := func() string {
		t.Helper()
		plan, err := p.planSections("skills", "foo", declared, nil, segs, "out.md", render.HTMLComment, map[string]string{"s": "## Owned heading"})
		if err != nil {
			t.Fatal(err)
		}
		asm, _ := render.Assemble(segs, plan, render.HTMLComment)
		return asm
	}
	drift := func(regenerated string) []manifest.Drift {
		t.Helper()
		lock := &manifest.Lock{Files: map[string]manifest.Entry{
			"out.md": {RegenChecked: true, OutputHash: manifest.Hash([]byte(regenerated))},
		}}
		rendered := map[string]RenderedFile{"out.md": {Path: "out.md", Content: regenerated, RegenChecked: true, TemplateID: "in-place/composed.tmpl", Policy: OutputPolicy{Regenerate: true}}}
		return p.checkLockedFiles(lock, rendered)
	}
	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	first := regenerate()
	if !strings.Contains(first, "the heading immediately below is awf-owned") ||
		!strings.Contains(first, "## Owned heading\nDEFAULT") {
		t.Fatalf("first render must emit pointer, owned heading, then fallback body:\n%s", first)
	}
	write(first)

	write(strings.Replace(first, "DEFAULT", "adopter one\n\n### Adopter subordinate heading\nadopter two", 1))
	resynced := regenerate()
	if !strings.Contains(resynced, "adopter one\n\n### Adopter subordinate heading\nadopter two") ||
		strings.Count(resynced, "## Owned heading") != 1 || !strings.Contains(resynced, "OWNED") {
		t.Fatalf("read-back lost or duplicated owned structure or adopter body:\n%s", resynced)
	}
	if d := drift(resynced); len(d) != 0 {
		t.Fatalf("an edit confined to the in-place body must report clean, got %v", d)
	}
	write(resynced)
	if again := regenerate(); again != resynced {
		t.Fatalf("regeneration is not an idempotent fixpoint:\ngot  %q\nwant %q", again, resynced)
	}
	if d := drift(regenerate()); len(d) != 0 {
		t.Fatalf("sync then check must report no drift, got %v", d)
	}

	write(strings.Replace(resynced, "## Owned heading", "## Tampered heading", 1))
	headingRegen := regenerate()
	if strings.Contains(headingRegen, "Tampered heading") || strings.Count(headingRegen, "## Owned heading") != 1 {
		t.Fatalf("regeneration must restore exactly one structural heading:\n%s", headingRegen)
	}
	if d := drift(headingRegen); len(d) != 1 || d[0].Kind != "hand-edited" {
		t.Fatalf("a changed structural heading must report hand-edited drift, got %v", d)
	}
	for _, changed := range []string{"### Tampered heading", "# Tampered heading"} {
		write(strings.Replace(resynced, "## Owned heading", changed, 1))
		levelRegen := regenerate()
		if strings.Contains(levelRegen, changed) || strings.Count(levelRegen, "## Owned heading") != 1 {
			t.Fatalf("%q must be consumed as owned-slot tamper:\n%s", changed, levelRegen)
		}
		if d := drift(levelRegen); len(d) != 1 || d[0].Kind != "hand-edited" {
			t.Fatalf("%q drift = %v", changed, d)
		}
	}

	write(strings.Replace(resynced, "## Owned heading\n", "", 1))
	missingRegen := regenerate()
	if strings.Count(missingRegen, "## Owned heading") != 1 ||
		!strings.Contains(missingRegen, "### Adopter subordinate heading") {
		t.Fatalf("missing-heading recovery lost structure or body:\n%s", missingRegen)
	}
	if d := drift(missingRegen); len(d) != 1 || d[0].Kind != "hand-edited" {
		t.Fatalf("a missing structural heading must report hand-edited drift, got %v", d)
	}

	write(strings.Replace(resynced, "OWNED", "TAMPERED", 1))
	tamperRegen := regenerate()
	if strings.Contains(tamperRegen, "TAMPERED") {
		t.Fatalf("regeneration must restore another awf-owned region:\n%s", tamperRegen)
	}
	if d := drift(tamperRegen); len(d) != 1 || d[0].Kind != "hand-edited" {
		t.Fatalf("an awf-owned edit must report hand-edited drift, got %v", d)
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

// An in-place region is read back from rendered output, a channel the
// authoring-comment strip never touches (ADR-0121 Decision 2): a
// directive-shaped line an adopter writes inside the region survives
// re-render byte-for-byte.
// invariant: rendering/inplace-and-placeholders:authoring-comment-inplace-inert (TestInPlaceRegionKeepsAuthoringCommentShapedLine)
func TestInPlaceRegionKeepsAuthoringCommentShapedLine(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	segs := render.ParseSections(
		"<!-- awf:section s inplace -->\nDEFAULT\n<!-- awf:end -->\n" +
			"<!-- awf:section next -->\nN\n<!-- awf:end -->\n")
	declared := []string{"s", "next"}
	out := "banner\n" +
		"<!-- awf:edit-in-place s: your edits -->\n" +
		"kept above\n<!-- awf:comment shaped, but user-owned output -->\nkept below\n" +
		"<!-- awf:edit next: default; create x -->\nN\n"
	if err := os.WriteFile(filepath.Join(root, "out.md"), []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := p.planSections("skills", "foo", declared, nil, segs, "out.md", render.HTMLComment)
	if err != nil {
		t.Fatal(err)
	}
	want := "kept above\n<!-- awf:comment shaped, but user-owned output -->\nkept below"
	if plan["s"].InPlaceBody != want {
		t.Errorf("in-place body must survive verbatim\ngot  %q\nwant %q", plan["s"].InPlaceBody, want)
	}
}
