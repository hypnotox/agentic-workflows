package adr_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invariant: adr-system/adr-lifecycle:corpus-single-identity-key
func TestParseDirExtractsStatusAndTitle(t *testing.T) {
	dir := t.TempDir()
	content := testsupport.ADR("Accepted", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
		testsupport.WithTitle("0007: Example Title"), testsupport.WithBody("## Context\nx\n"))
	if err := os.WriteFile(filepath.Join(dir, "0007-example.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// A non-ADR file must be skipped.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adrs, err := adr.ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(adrs) != 1 {
		t.Fatalf("expected 1 ADR, got %d", len(adrs))
	}
	got := adrs[0]
	if got.Number != "0007" || got.Status != "Accepted" || got.Title != "ADR-0007: Example Title" || got.Filename != "0007-example.md" {
		t.Errorf("unexpected ADR: %+v", got)
	}
}

// TestParseDirExtractsTagsAndRelated confirms the revived tags:/related:
// frontmatter is lifted into adr.ADR (previously parsed past and dropped).
func TestParseRecordRoutesV1AndV2Cutoffs(t *testing.T) {
	_, body, found := strings.Cut(adrTemplateFixture, "---\n")
	if !found {
		t.Fatal("fixture frontmatter delimiter missing")
	}
	v1 := strings.Replace("---\n"+body, "YYYY-MM-DD", "2026-07-21", 2)
	v1 = strings.Replace(v1, "ADR-NNNN", "ADR-0005", 1)
	v2 := strings.Replace(v1, "current-state-v1", "current-state-v2", 1)
	for _, tc := range []struct {
		name, file, doc string
		v2From          int
		wantV1, wantV2  bool
	}{
		{"V1 region", "0005-v1.md", v1, 8, true, false},
		{"V2 boundary", "0008-v2.md", strings.Replace(v2, "ADR-0005", "ADR-0008", 1), 8, false, true},
		{"missing V2 cutoff remains V1", "0005-v1.md", v1, 0, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := adr.ParseRecord(tc.file, []byte(tc.doc), adr.FormatBoundaries{V1From: 5, V2From: tc.v2From})
			if err != nil || a.IsV1() != tc.wantV1 || a.IsV2() != tc.wantV2 {
				t.Fatalf("record=%#v err=%v", a, err)
			}
		})
	}
	if _, err := adr.ParseRecord("0004-stray.md", []byte(strings.Replace(v2, "ADR-0005", "ADR-0004", 1)), adr.FormatBoundaries{V1From: 5, V2From: 8}); err == nil || !strings.Contains(err.Error(), "current-state-v2") {
		t.Fatalf("stray V2 below cutoff error=%v", err)
	}
}

func TestParseBytesRecognizesV2Marker(t *testing.T) {
	a, found, err := adr.ParseBytes("0007-v2.md", []byte("---\nformat: current-state-v2\nstatus: Proposed\ndate: 2026-07-21\n---\n# ADR-0007: V2\n"))
	if err != nil || !found || !a.IsV2() || a.IsV1() {
		t.Fatalf("ParseBytes V2 = %#v found=%v err=%v", a, found, err)
	}
}

func TestParseDirExtractsTagsAndRelated(t *testing.T) {
	dir := t.TempDir()
	content := testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
		testsupport.WithTags("context", "config"), testsupport.WithRelated(1, 92),
		testsupport.WithTitle("0007: Tagged"), testsupport.WithBody("## Context\nx\n"))
	if err := os.WriteFile(filepath.Join(dir, "0007-tagged.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	adrs, err := adr.ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(adrs) != 1 {
		t.Fatalf("expected 1 ADR, got %d", len(adrs))
	}
	got := adrs[0]
	if len(got.Tags) != 2 || got.Tags[0] != "context" || got.Tags[1] != "config" {
		t.Errorf("tags: got %#v", got.Tags)
	}
	if len(got.Related) != 2 || got.Related[0] != 1 || got.Related[1] != 92 {
		t.Errorf("related: got %#v", got.Related)
	}
}

// parseOne writes content as a single ADR fixture and returns its parse.
func parseOne(t *testing.T, content string) adr.ADR {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "0001-fixture.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	adrs, err := adr.ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(adrs) != 1 {
		t.Fatalf("expected 1 ADR, got %d", len(adrs))
	}
	return adrs[0]
}

// TestDecisionItems covers column-0 item enumeration: indented sub-lists
// skipped, multi-digit items matched, and fenced syntax ignored.
func TestDecisionItems(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []int
	}{
		{"indented sub-item not enumerated", "## Decision\n\n1. First.\n   1. Sub-item.\n2. Second.\n", []int{1, 2}},
		{"multi-digit item enumerated", "## Decision\n\n13. Thirteenth.\n", []int{13}},
		{"fenced column-0 numbered line is inert", "## Decision\n\n1. Example:\n\n```\n1. fenced line\n```\n\n2. Real.\n", []int{1, 2}},
		{"tilde fenced column-0 numbered line is inert", "## Decision\n\n~~~\n1. fenced line\n~~~\n\n1. Real.\n", []int{1}},
		{"no items", "## Decision\n\nProse only.\n", nil},
		{"final item without trailing newline", "## Decision\n\n1. Final.", []int{1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := parseOne(t, testsupport.ADR("Implemented", testsupport.WithTitle("0001: Fixture"), testsupport.WithBody(tc.body)))
			if got := a.DecisionItems(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("DecisionItems: got %#v, want %#v", got, tc.want)
			}
		})
	}
}

// TestDecisionSectionOffsetsIgnoreFencedHeadings pins that a fenced `## `
// heading inside the Decision section does not end it. The offsets are what the
// schema migrations perform byte surgery against, so a short read here truncates
// the section a migration then appends into.
func TestDecisionSectionOffsetsIgnoreFencedHeadings(t *testing.T) {
	body := "## Decision\n\n1. Real.\n\n```\n## Fake\n```\n\n2. Still real.\n\n## Consequences\n\nx\n"
	a := parseOne(t, testsupport.ADR("Implemented", testsupport.WithTitle("0001: Fixture"), testsupport.WithBody(body)))
	if got, want := a.DecisionEnd-a.DecisionStart, len("## Decision\n\n1. Real.\n\n```\n## Fake\n```\n\n2. Still real.\n\n"); got != want {
		t.Errorf("Decision section length = %d, want %d", got, want)
	}
}

// TestParseDirGlobError exercises the glob-pattern failure path: a directory
// whose name contains an unterminated "[" yields an ErrBadPattern from Glob.
func TestParseDirGlobError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bad[")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.ParseDir(dir); err == nil {
		t.Fatal("expected glob error for malformed pattern, got nil")
	}
}

// TestParseDirReadError exercises the os.ReadFile failure path: a directory
// squatting on a path that matches the ADR filename pattern cannot be read as a
// file (fails for all users, including root).
func TestParseDirReadError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "0001-squatter.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.ParseDir(dir); err == nil {
		t.Fatal("expected read error for directory in file's place, got nil")
	}
}

// TestParseDirParseError exercises the parse failure path: malformed YAML
// frontmatter in an ADR file makes parse return an error.
func TestParseDirParseError(t *testing.T) {
	dir := t.TempDir()
	content := "---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n"
	if err := os.WriteFile(filepath.Join(dir, "0001-broken.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.ParseDir(dir); err == nil {
		t.Fatal("expected parse error for malformed frontmatter, got nil")
	}
}

// TestLoadCorpusParseError ensures the construction seam propagates a parse
// error rather than yielding a partial corpus. The render entry points take a
// Corpus (ADR-0130 item 1), so this is the one place a malformed ADR can
// surface: they no longer parse and so no longer fail.
func TestLoadCorpusParseError(t *testing.T) {
	dir := t.TempDir()
	content := "---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n"
	if err := os.WriteFile(filepath.Join(dir, "0001-broken.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := adr.LoadCorpus(dir)
	if err == nil {
		t.Fatal("expected error from malformed frontmatter, got nil")
	}
	if len(got.All()) != 0 {
		t.Errorf("expected an empty corpus on error, got %d ADR(s)", len(got.All()))
	}
}

// invariant: adr-system/adr-lifecycle:adr-sections-parsed
func TestParseDirExtractsSections(t *testing.T) {
	dir := t.TempDir()
	content := testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
		testsupport.WithTitle("0009: S"), testsupport.WithBody("## Context\nctx body\n## Invariants\n- `invariant: example-slug` - a thing.\n## Consequences\ncons\n"))
	if err := os.WriteFile(filepath.Join(dir, "0009-s.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	adrs, err := adr.ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(adrs) != 1 {
		t.Fatalf("expected 1 ADR, got %d", len(adrs))
	}
	inv := adrs[0].Sections["Invariants"]
	if !strings.Contains(inv, "invariant: example-slug") {
		t.Errorf("Invariants section missing tag; got: %q", inv)
	}
	if !strings.Contains(adrs[0].Sections["Context"], "ctx body") {
		t.Errorf("Context section wrong: %q", adrs[0].Sections["Context"])
	}
}

// adrTemplateFixture mirrors the current-state-v1 shape of the rendered
// docs/decisions/template.md while keeping every section valid without author edits.
const adrTemplateFixture = `<!-- GENERATED by awf -- do not edit -->
<!-- awf:edit frontmatter -- default -->
---
format: current-state-v1
status: Proposed
date: YYYY-MM-DD
---
# ADR-NNNN: Title

<!-- awf:edit body -- default -->
## Context

Fixture context.

## Decision

1. Use the fixture decision.

## State changes

None.

## Consequences

Fixture consequence.

## Alternatives Considered

Fixture alternative.

## Status history

- YYYY-MM-DD: Proposed
`

func writeTemplateFixture(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte(adrTemplateFixture), 0o644); err != nil {
		t.Fatal(err)
	}
}

// swapNow overrides the adr package's now seam for the duration of a test.
func swapNow(t *testing.T, fn func() time.Time) {
	t.Helper()
	orig := adr.SetNowForTest(fn)
	t.Cleanup(func() { adr.SetNowForTest(orig) })
}

func fixedNow() time.Time { return time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC) }

func TestAdoptionBoundary(t *testing.T) {
	legacy := func(number string) string {
		return testsupport.ADR("Accepted", testsupport.WithDate("2026-07-21"), testsupport.WithTitle(number+": Legacy"))
	}
	t.Run("empty", func(t *testing.T) {
		cutoff, gaps, err := adr.AdoptionBoundary(t.TempDir())
		if err != nil || cutoff != 1 || gaps == nil || len(gaps) != 0 {
			t.Fatalf("got %d %v %v", cutoff, gaps, err)
		}
	})
	t.Run("contiguous and gapped", func(t *testing.T) {
		for _, tc := range []struct {
			names  []string
			cutoff int
			gaps   []int
		}{
			{[]string{"0001-one.md", "0002-two.md"}, 3, []int{}},
			{[]string{"0001-one.md", "0003-three.md"}, 4, []int{2}},
		} {
			dir := t.TempDir()
			for _, name := range tc.names {
				testsupport.WriteFile(t, filepath.Join(dir, name), legacy(name[:4]))
			}
			cutoff, gaps, err := adr.AdoptionBoundary(dir)
			if err != nil || cutoff != tc.cutoff || !reflect.DeepEqual(gaps, tc.gaps) {
				t.Fatalf("got %d %v %v, want %d %v", cutoff, gaps, err, tc.cutoff, tc.gaps)
			}
		}
	})
	t.Run("malformed", func(t *testing.T) {
		dir := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(dir, "0001-bad.md"), "---\nstatus: [bad\n---\n")
		if _, _, err := adr.AdoptionBoundary(dir); err == nil {
			t.Fatal("expected parse error")
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		dir := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(dir, "0001-one.md"), legacy("0001"))
		testsupport.WriteFile(t, filepath.Join(dir, "0001-two.md"), legacy("0001"))
		if _, _, err := adr.AdoptionBoundary(dir); err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("v1 below cutoff", func(t *testing.T) {
		dir := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(dir, "0001-v1.md"), "---\nformat: current-state-v1\nstatus: Proposed\ndate: 2026-07-21\n---\n# ADR-0001: V1\n")
		if _, _, err := adr.AdoptionBoundary(dir); err == nil || !strings.Contains(err.Error(), "current-state-v1") {
			t.Fatalf("error=%v", err)
		}
	})
}

func TestNextNumberEmptyDir(t *testing.T) {
	dir := t.TempDir()
	got, err := adr.NextNumber(dir)
	if err != nil {
		t.Fatalf("NextNumber: %v", err)
	}
	if got != "0001" {
		t.Errorf("NextNumber(empty) = %q, want 0001", got)
	}
}

func TestNextNumberSkipsGapToMaxPlusOne(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"0001-first.md", "0003-third.md"} {
		content := testsupport.ADR("Accepted", testsupport.WithTitle("title"))
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := adr.NextNumber(dir)
	if err != nil {
		t.Fatalf("NextNumber: %v", err)
	}
	// invariant: adr-system/adr-lifecycle:adr-new-sequential-numbering
	if got != "0004" {
		t.Errorf("NextNumber = %q, want 0004", got)
	}
}

func TestNextNumberPropagatesParseDirError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bad[")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.NextNumber(dir); err == nil {
		t.Fatal("expected glob error to propagate")
	}
}

func TestNewFileHappyPath(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFixture(t, dir)
	clockCalls := 0
	swapNow(t, func() time.Time {
		clockCalls++
		return time.Date(2026, 7, clockCalls, 0, 0, 0, 0, time.UTC)
	})

	path, err := adr.NewFile(dir, "My Cool Title", adr.CurrentStateV1)
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	want := filepath.Join(dir, "0001-my-cool-title.md")
	if path != want {
		t.Errorf("NewFile path = %q, want %q", path, want)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	body := string(got)
	// invariant: adr-system/adr-lifecycle:adr-new-strips-markers
	if strings.Contains(body, "GENERATED by awf") || strings.Contains(body, "awf:edit") {
		t.Errorf("marker comment survived: %q", body)
	}
	// invariant: adr-system/adr-lifecycle:adr-new-heading-matches-file
	if !strings.Contains(body, "# ADR-0001: My Cool Title") {
		t.Errorf("heading not filled in: %q", body)
	}
	if strings.Count(body, "date: 2026-07-01") != 1 {
		t.Errorf("frontmatter date count = %d, want 1: %q", strings.Count(body, "date: 2026-07-01"), body)
	}
	if strings.Count(body, "- 2026-07-01: Proposed") != 1 {
		t.Errorf("Proposed history date count = %d, want 1: %q", strings.Count(body, "- 2026-07-01: Proposed"), body)
	}
	if clockCalls != 1 {
		t.Errorf("clock calls = %d, want 1", clockCalls)
	}
	if _, err := adr.ParseV1(filepath.Base(path), got); err != nil {
		t.Fatalf("scaffolded ADR does not parse as current-state-v1: %v", err)
	}
}

// TestNewFileSequentialCallsGetDifferentNumbers documents why NewFile's
// overwrite guard can never fire from repeated same-process calls: NextNumber
// always returns one more than every existing NNNN-*.md file, so a second call
// with the same title lands at the next number instead of colliding.
func TestNewFileReplacesEitherGovernedTemplateMarkerWithRequestedFormat(t *testing.T) {
	swapNow(t, fixedNow)
	for _, templateMarker := range []string{adr.V1FormatMarker, adr.V2FormatMarker} {
		for _, requested := range []adr.Format{adr.CurrentStateV1, adr.CurrentStateV2} {
			requestedName := adr.V1FormatMarker
			if requested == adr.CurrentStateV2 {
				requestedName = adr.V2FormatMarker
			}
			t.Run(templateMarker+" to "+requestedName, func(t *testing.T) {
				dir := t.TempDir()
				template := strings.Replace(adrTemplateFixture, "format: current-state-v1", "format: "+templateMarker, 1)
				template = strings.Replace(template, "## Context\n", "## Context\n\nformat: current-state-v1 remains prose.\n", 1)
				if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte(template), 0o644); err != nil {
					t.Fatal(err)
				}
				path, err := adr.NewFile(dir, "Format Title", requested)
				if err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				wantMarker := adr.V1FormatMarker
				parse := adr.ParseV1
				if requested == adr.CurrentStateV2 {
					wantMarker = adr.V2FormatMarker
					parse = adr.ParseV2
				}
				wantCount := 1
				if requested == adr.CurrentStateV1 {
					wantCount = 2 // frontmatter plus the deliberately similar body prose
				}
				if strings.Count(string(data), "format: "+wantMarker) != wantCount || !strings.Contains(string(data), "format: current-state-v1 remains prose.") {
					t.Fatalf("format selection rewrote outside frontmatter or emitted the wrong marker:\n%s", data)
				}
				if _, err := parse(filepath.Base(path), data); err != nil {
					t.Fatalf("requested %s scaffold does not parse: %v", requestedName, err)
				}
			})
		}
	}
}

func TestNewFileRejectsNonGovernedFormatAndMissingFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFixture(t, dir)
	if _, err := adr.NewFile(dir, "Legacy", adr.Legacy); err == nil || !strings.Contains(err.Error(), "scaffold format") {
		t.Fatalf("legacy format error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte("# ADR-NNNN: Title\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.NewFile(dir, "No Frontmatter", adr.CurrentStateV1); err == nil || !strings.Contains(err.Error(), "missing frontmatter") {
		t.Fatalf("missing frontmatter error = %v", err)
	}
}

func TestNewFileSequentialCallsGetDifferentNumbers(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFixture(t, dir)
	swapNow(t, fixedNow)
	first, err := adr.NewFile(dir, "Same Title", adr.CurrentStateV1)
	if err != nil {
		t.Fatalf("first NewFile: %v", err)
	}
	second, err := adr.NewFile(dir, "Same Title", adr.CurrentStateV1)
	if err != nil {
		t.Fatalf("second NewFile: %v", err)
	}
	if first == second {
		t.Fatalf("expected distinct paths, both were %q", first)
	}
	wantFirst := filepath.Join(dir, "0001-same-title.md")
	wantSecond := filepath.Join(dir, "0002-same-title.md")
	if first != wantFirst || second != wantSecond {
		t.Errorf("got (%q, %q), want (%q, %q)", first, second, wantFirst, wantSecond)
	}
}

func TestNewFilePropagatesNextNumberError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bad[")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.NewFile(dir, "Some Title", adr.CurrentStateV1); err == nil {
		t.Fatal("expected NextNumber's glob error to propagate")
	}
}

func TestNewFileMissingTemplate(t *testing.T) {
	dir := t.TempDir()
	if _, err := adr.NewFile(dir, "No Template Here", adr.CurrentStateV1); err == nil {
		t.Fatal("expected error for missing template.md")
	}
}

func TestNewFileEmptySlug(t *testing.T) {
	dir := t.TempDir()
	// No template.md needed - slugify errors before the file is ever read.
	if _, err := adr.NewFile(dir, "!!!", adr.CurrentStateV1); err == nil {
		t.Fatal("expected slugify error for an all-punctuation title")
	}
}

func TestNewFileRejectsInvalidGovernedTemplateMarkers(t *testing.T) {
	for _, tc := range []struct {
		name, formatLines, want string
	}{
		{"missing", "", "missing governed format marker"},
		{"multiple same", "format: current-state-v1\nformat: current-state-v1\n", "exactly one"},
		{"multiple mixed", "format: current-state-v1\nformat: current-state-v2\n", "exactly one"},
		{"unsupported", "format: current-state-v3\n", "unsupported governed format marker"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			broken := strings.Replace(adrTemplateFixture, "format: current-state-v1\n", tc.formatLines, 1)
			if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte(broken), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := adr.NewFile(dir, "Bad Format", adr.CurrentStateV1); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("format marker error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestNewFileMissingDatePlaceholder(t *testing.T) {
	dir := t.TempDir()
	broken := strings.Replace(adrTemplateFixture, "date: YYYY-MM-DD\n", "", 1)
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := adr.NewFile(dir, "Some Title", adr.CurrentStateV1)
	if err == nil || !strings.Contains(err.Error(), `template missing expected "date: YYYY-MM-DD"`) {
		t.Fatalf("err = %v, want missing frontmatter date placeholder", err)
	}
}

func TestNewFileMissingProposedHistoryDatePlaceholder(t *testing.T) {
	dir := t.TempDir()
	broken := strings.Replace(adrTemplateFixture, "- YYYY-MM-DD: Proposed\n", "- Proposed\n", 1)
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := adr.NewFile(dir, "Some Title", adr.CurrentStateV1)
	if err == nil || !strings.Contains(err.Error(), `ADR template Proposed history: adr: template missing expected "- YYYY-MM-DD: Proposed"`) {
		t.Fatalf("err = %v, want missing Proposed history date placeholder", err)
	}
}

func TestNewFileMissingTitlePlaceholder(t *testing.T) {
	dir := t.TempDir()
	broken := strings.Replace(adrTemplateFixture, "# ADR-NNNN: Title\n", "", 1)
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.NewFile(dir, "Some Title", adr.CurrentStateV1); err == nil {
		t.Fatal("expected error for missing title placeholder")
	}
}

// TestStatusLiteralsOwnedByADRPackage enforces ADR-0130 item 3 mechanically:
// the ADR status vocabulary is compared in internal/adr and nowhere else, so
// every other consumer asks a predicate. This is what stops the three-way "is
// live" and five-way "is superseded" divergences the ADR was written to end.
//
// The scan covers comparisons against an ADR's .Status field and against any
// status literal, so neither an ADR-typed field read nor a local can
// reintroduce the drift.
//
// internal/project/context.go carries the single enumerated exception: ADR-0129
// item 4 keeps its Tier-2 exclusion on a direct prefix test.
// invariant: adr-system/adr-lifecycle:corpus-owns-status-literals
func TestBridgeLegacyFacts(t *testing.T) {
	for _, tc := range []struct {
		status            string
		shipped, inflight bool
	}{
		{"Implemented", true, false}, {"Superseded", true, false}, {"Accepted", false, true}, {"Proposed", false, true}, {"Superseded by ADR-0001", false, false},
	} {
		a, _, err := adr.ParseBytes("0001-test.md", []byte("---\nstatus: "+tc.status+"\ndate: 2026-07-19\n---\n# ADR-0001: Test\n\n## Invariants\n\n- `invariant: backed` - x.\n- `unbacked-invariant: manual` - y.\n"))
		if err != nil {
			t.Fatal(err)
		}
		if a.Date != "2026-07-19" || a.IsLegacyShipped() != tc.shipped || a.IsInflight() != tc.inflight {
			t.Errorf("%s: %#v", tc.status, a)
		}
		if got := a.InvariantDecls(); len(got) != 2 || got[0].Unbacked || !got[1].Unbacked {
			t.Errorf("declaration class drift: %#v", got)
		}
	}
	a, _, err := adr.ParseBytes("0002-no-date.md", []byte("---\nstatus: Implemented\n---\n# ADR-0002: No date\n"))
	if err != nil || a.Date != "" {
		t.Fatalf("missing date must remain valid and empty: %#v %v", a, err)
	}
}

// TestInvariantDeclsBulletShapes covers a non-declaration bullet (skipped) and a
// declaration whose text wraps onto a continuation line (joined into one bullet).
func TestInvariantDeclsBulletShapes(t *testing.T) {
	section := "- `invariant: wrapped` - a declaration whose sentence\n" +
		"  continues onto a second physical line.\n" +
		"- a plain reminder bullet that declares no invariant\n"
	a := adr.ADR{Sections: map[string]string{"Invariants": section}}
	decls := a.InvariantDecls()
	if len(decls) != 1 {
		t.Fatalf("expected one declaration, got %d: %#v", len(decls), decls)
	}
	if decls[0].Slug != "wrapped" || decls[0].Unbacked {
		t.Errorf("declaration = %#v", decls[0])
	}
	if !strings.Contains(decls[0].Bullet, "second physical line") {
		t.Errorf("continuation line was not joined into the bullet: %q", decls[0].Bullet)
	}
}

// TestLegacyStatusPredicates exercises the live/superseded/proposed classifiers
// and legacy Bucket grouping across each status shape, including the suffixed
// superseded form the prefix test tolerates.
func TestLegacyStatusPredicates(t *testing.T) {
	for _, tc := range []struct {
		status                     string
		live, superseded, proposed bool
		bucket                     string
	}{
		{"Implemented", true, false, false, "Implemented"},
		{"Accepted", true, false, false, "Accepted"},
		{"Proposed", false, false, true, "Proposed"},
		{"Superseded by ADR-0001", false, true, false, "Superseded"},
	} {
		a := adr.ADR{Status: tc.status}
		if a.IsLive() != tc.live {
			t.Errorf("%q IsLive = %v, want %v", tc.status, a.IsLive(), tc.live)
		}
		if a.IsSuperseded() != tc.superseded {
			t.Errorf("%q IsSuperseded = %v, want %v", tc.status, a.IsSuperseded(), tc.superseded)
		}
		if a.IsProposed() != tc.proposed {
			t.Errorf("%q IsProposed = %v, want %v", tc.status, a.IsProposed(), tc.proposed)
		}
		if a.Bucket() != tc.bucket {
			t.Errorf("%q Bucket = %q, want %q", tc.status, a.Bucket(), tc.bucket)
		}
	}
}

func TestStatusLiteralsOwnedByADRPackage(t *testing.T) {
	// Two shapes: a comparison against an ADR's .Status field, and a comparison
	// of any local against a status literal. internal/audit used to hold the
	// second shape on a local `st`; Phase 3 of the coverage-derived-supersession
	// plan gave it parsed records, so the scan covers both from here on.
	statusCmp := regexp.MustCompile(
		`\.Status\s*[!=]=\s*"` +
			`|HasPrefix\([^)]*\.Status\s*,\s*"` +
			`|[!=]=\s*"(Accepted|Implemented|Proposed|Superseded)"` +
			// A switch arm and a prefix test on a local both compare a status
			// without an ADR field in the line; internal/audit held the latter
			// shape before it had parsed records.
			`|case\s+"(Accepted|Implemented|Proposed|Superseded)"` +
			`|HasPrefix\([^,]*,\s*"Superseded"`)

	seen := 0
	root := filepath.Join("..", "..", "internal")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// internal/adr owns the literals; the predicates themselves live there.
		if strings.HasPrefix(path, filepath.Join(root, "adr")+string(filepath.Separator)) {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		seen++
		for i, line := range strings.Split(string(data), "\n") {
			if !statusCmp.MatchString(line) {
				continue
			}
			t.Errorf("%s:%d compares an ADR status literal directly - use an adr.ADR predicate (ADR-0130 item 3):\n\t%s",
				path, i+1, strings.TrimSpace(line))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Guard against a vacuous pass if internal/ is ever relocated.
	if seen < 10 {
		t.Fatalf("inspected only %d non-test source file(s) under internal/; the scan is not reaching the tree", seen)
	}
}

// mustCorpus loads a decisions directory into the view for a render test.
// RenderIndexMD takes a Corpus rather than a directory (ADR-0130 item 1), so
// the parse is the caller's, made once.
func mustCorpus(t *testing.T, dir string) adr.Corpus {
	t.Helper()
	c, err := adr.LoadCorpus(dir)
	if err != nil {
		t.Fatalf("LoadCorpus(%s): %v", dir, err)
	}
	return c
}
