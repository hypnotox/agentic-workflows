package adr_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestRenderActiveMDGroupsByStatus is a hermetic unit test using a temp dir.
// invariant: render-active-md
func TestRenderActiveMDGroupsByStatus(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"0001-first-accepted.md": testsupport.ADR("Accepted",
			testsupport.WithDate("2026-06-24"), testsupport.WithTags("tooling"),
			testsupport.WithTitle("0001: First Accepted"), testsupport.WithBody("## Context\nSomething.\n")),
		"0002-a-proposal.md": testsupport.ADR("Proposed",
			testsupport.WithDate("2026-06-24"),
			testsupport.WithTitle("0002: A Proposal"), testsupport.WithBody("## Context\nAnother thing.\n")),
		"0003-already-implemented.md": testsupport.ADR("Implemented",
			testsupport.WithDate("2026-06-24"),
			testsupport.WithTitle("0003: Already Implemented"), testsupport.WithBody("## Context\nDone.\n")),
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}

	got, err := adr.RenderActiveMD(dir)
	if err != nil {
		t.Fatalf("RenderActiveMD: %v", err)
	}

	// RenderActiveMD is banner-free (the banner is injected downstream by
	// internal/project, like every other rendered artifact) - content starts
	// directly with the first status heading.
	if !strings.HasPrefix(got, "## ") {
		t.Errorf("expected content to start with a status heading, no embedded banner; got start: %q", got[:min(60, len(got))])
	}

	// Accepted must appear before Proposed.
	acceptedPos := strings.Index(got, "## Accepted")
	implementedPos := strings.Index(got, "## Implemented")
	proposedPos := strings.Index(got, "## Proposed")

	if acceptedPos < 0 {
		t.Error("missing ## Accepted section")
	}
	if implementedPos < 0 {
		t.Error("missing ## Implemented section")
	}
	if proposedPos < 0 {
		t.Error("missing ## Proposed section")
	}
	if acceptedPos > 0 && implementedPos > 0 && acceptedPos > implementedPos {
		t.Errorf("Accepted (%d) should come before Implemented (%d)", acceptedPos, implementedPos)
	}
	if implementedPos > 0 && proposedPos > 0 && implementedPos > proposedPos {
		t.Errorf("Implemented (%d) should come before Proposed (%d)", implementedPos, proposedPos)
	}

	// Each ADR entry should appear under the correct section.
	checkEntry := func(section, title, filename string) {
		t.Helper()
		sectionHeader := "## " + section
		sectionStart := strings.Index(got, sectionHeader)
		if sectionStart < 0 {
			t.Errorf("section %q not found", sectionHeader)
			return
		}
		// Find next section after this one.
		nextSection := strings.Index(got[sectionStart+len(sectionHeader):], "\n## ")
		var sectionBody string
		if nextSection < 0 {
			sectionBody = got[sectionStart:]
		} else {
			sectionBody = got[sectionStart : sectionStart+len(sectionHeader)+nextSection]
		}
		if !strings.Contains(sectionBody, title) {
			t.Errorf("expected %q in section %q; section body:\n%s", title, section, sectionBody)
		}
		if !strings.Contains(sectionBody, filename) {
			t.Errorf("expected filename %q in section %q", filename, section)
		}
	}

	checkEntry("Accepted", "ADR-0001: First Accepted", "0001-first-accepted.md")
	checkEntry("Implemented", "ADR-0003: Already Implemented", "0003-already-implemented.md")
	checkEntry("Proposed", "ADR-0002: A Proposal", "0002-a-proposal.md")
}

// TestRenderActiveMDGroupsSupersededVariants locks the lifecycle convention's
// suffixed status ("Superseded by ADR-NNNN") into one Superseded group: a
// single section header ordered by statusOrder, per-entry full status kept -
// not one alphabetical section per successor.
// invariant: render-active-md
func TestRenderActiveMDGroupsSupersededVariants(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"0001-old-one.md": testsupport.ADR("Superseded by ADR-0003",
			testsupport.WithDate("2026-06-24"), testsupport.WithTitle("0001: Old One")),
		"0002-old-two.md": testsupport.ADR("Superseded by ADR-0004",
			testsupport.WithDate("2026-06-24"), testsupport.WithTitle("0002: Old Two")),
		"0003-current.md": testsupport.ADR("Implemented",
			testsupport.WithDate("2026-06-24"), testsupport.WithTitle("0003: Current")),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
	got, err := adr.RenderActiveMD(dir)
	if err != nil {
		t.Fatalf("RenderActiveMD: %v", err)
	}
	if strings.Contains(got, "## Superseded by") {
		t.Errorf("per-successor section header rendered instead of one Superseded group:\n%s", got)
	}
	if !strings.Contains(got, "## Superseded\n") {
		t.Errorf("missing single ## Superseded section:\n%s", got)
	}
	for _, entry := range []string{"(Superseded by ADR-0003)", "(Superseded by ADR-0004)"} {
		if !strings.Contains(got, entry) {
			t.Errorf("entry lost its full status %q:\n%s", entry, got)
		}
	}
	if imp, sup := strings.Index(got, "## Implemented"), strings.Index(got, "## Superseded"); imp > sup {
		t.Errorf("Superseded (%d) must sort after Implemented (%d) per statusOrder", sup, imp)
	}
}

// invariant: render-active-md
func TestRenderActiveMDPlaceholderWhenNoADRs(t *testing.T) {
	dir := t.TempDir()
	// A non-ADR markdown file must not count.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := adr.RenderActiveMD(dir)
	if err != nil {
		t.Fatalf("RenderActiveMD: %v", err)
	}
	if !strings.Contains(got, "No decisions recorded yet") {
		t.Errorf("expected placeholder index for an ADR-less dir, got:\n%s", got)
	}
}

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

// TestSupersessionRefExtraction covers the ADR-0120 token grammar: both keys
// extracted from the Decision section with the kind named by the key, tokens
// elsewhere inert, and the anchor shape rules (no leading-zero items, no
// uppercase slugs).
func TestSupersessionRefExtraction(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []adr.SupersessionRef
	}{
		{
			name: "both kinds extracted",
			body: "## Decision\n\n1. Overrides `supersedes: ADR-0116#2`.\n2. Retires `supersedes-invariant: ADR-0031#retired-slug`.\n",
			want: []adr.SupersessionRef{{Target: "0116", Item: 2}, {Target: "0031", Slug: "retired-slug"}},
		},
		{
			name: "token outside the Decision section is inert",
			body: "## Context\n\nMentions `supersedes: ADR-0116#2` in passing.\n\n## Decision\n\n1. No tokens.\n",
			want: nil,
		},
		{
			name: "leading-zero item and uppercase slug do not match",
			body: "## Decision\n\n1. `supersedes: ADR-0116#03` and `supersedes-invariant: ADR-0031#Bad-Slug`.\n",
			want: nil,
		},
		{
			name: "backticked token inside a fenced block is read raw",
			body: "## Decision\n\n1. Grammar example:\n\n```\n`supersedes: ADR-0999#7`\n```\n",
			want: []adr.SupersessionRef{{Target: "0999", Item: 7}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := parseOne(t, testsupport.ADR("Implemented", testsupport.WithTitle("0001: Fixture"), testsupport.WithBody(tc.body)))
			if !reflect.DeepEqual(a.Refs, tc.want) {
				t.Errorf("Refs: got %#v, want %#v", a.Refs, tc.want)
			}
		})
	}
}

// TestDecisionItems covers column-0 item enumeration: indented sub-lists
// skipped, multi-digit items matched, and a fenced column-0 numbered line
// counted - pinning that the Decision body is read raw, fences included (the
// corpus is fence-clean at column 0 today; the pin makes any future surprise
// a test failure, not silent drift).
func TestDecisionItems(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []int
	}{
		{"indented sub-item not enumerated", "## Decision\n\n1. First.\n   1. Sub-item.\n2. Second.\n", []int{1, 2}},
		{"multi-digit item enumerated", "## Decision\n\n13. Thirteenth.\n", []int{13}},
		{"fenced column-0 numbered line counts (raw-read pin)", "## Decision\n\n1. Example:\n\n```\n1. fenced line\n```\n", []int{1, 1}},
		{"no items", "## Decision\n\nProse only.\n", nil},
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

// TestParseDirExtractsSupersedes confirms `supersedes:` frontmatter round-trips
// into adr.ADR (full-supersession claims, ADR-0120).
func TestParseDirExtractsSupersedes(t *testing.T) {
	content := "---\nstatus: Implemented\nsupersedes: [31]\n---\n# ADR-0120: T\n\n## Decision\n\n1. x.\n"
	a := parseOne(t, content)
	if !reflect.DeepEqual(a.Supersedes, []int{31}) {
		t.Errorf("Supersedes: got %#v, want [31]", a.Supersedes)
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

// TestRenderActiveMDParseError ensures RenderActiveMD propagates a ParseDir
// error instead of producing output.
func TestRenderActiveMDParseError(t *testing.T) {
	dir := t.TempDir()
	content := "---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n"
	if err := os.WriteFile(filepath.Join(dir, "0001-broken.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := adr.RenderActiveMD(dir)
	if err == nil {
		t.Fatal("expected error from malformed frontmatter, got nil")
	}
	if got != "" {
		t.Errorf("expected empty output on error, got: %q", got)
	}
}

// TestRenderActiveMDSortsWithinStatusAndOrdersExtra covers two paths: sorting
// multiple ADRs within one status group (by number), and appending a status not
// present in statusOrder (sorted after the known statuses).
func TestRenderActiveMDSortsWithinStatusAndOrdersExtra(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"0002-second-accepted.md": testsupport.ADR("Accepted", testsupport.WithTitle("0002: Second Accepted")),
		"0001-first-accepted.md":  testsupport.ADR("Accepted", testsupport.WithTitle("0001: First Accepted")),
		"0003-draft-status.md":    testsupport.ADR("Draft", testsupport.WithTitle("0003: Draft Status")),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}

	got, err := adr.RenderActiveMD(dir)
	if err != nil {
		t.Fatalf("RenderActiveMD: %v", err)
	}

	// Within the Accepted group, 0001 must be listed before 0002.
	first := strings.Index(got, "ADR-0001: First Accepted")
	second := strings.Index(got, "ADR-0002: Second Accepted")
	if first < 0 || second < 0 {
		t.Fatalf("missing accepted entries; got:\n%s", got)
	}
	if first > second {
		t.Errorf("0001 (%d) should be listed before 0002 (%d)", first, second)
	}

	// The unknown "Draft" status must appear as its own section, after the
	// known statuses.
	acceptedPos := strings.Index(got, "## Accepted")
	draftPos := strings.Index(got, "## Draft")
	if draftPos < 0 {
		t.Fatalf("missing ## Draft section; got:\n%s", got)
	}
	if acceptedPos > draftPos {
		t.Errorf("Accepted (%d) should come before extra status Draft (%d)", acceptedPos, draftPos)
	}
}

// invariant: adr-sections-parsed
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

// adrTemplateFixture mirrors the shape of the rendered docs/decisions/template.md
// (marker comments, frontmatter placeholders, title heading) without the
// Invariants section's backtick-heavy prose, which NewFile never touches.
const adrTemplateFixture = `<!-- GENERATED by awf -- do not edit -->
<!-- awf:edit frontmatter -- default -->
---
status: Proposed
date: YYYY-MM-DD
supersedes: []
retires_invariants: []
superseded_by: ""
tags: []
related: []
domains: []
---
# ADR-NNNN: Title


<!-- awf:edit body -- default -->
## Context

Fixture body.
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
	// invariant: adr-new-sequential-numbering
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
	swapNow(t, fixedNow)

	path, err := adr.NewFile(dir, "My Cool Title")
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
	// invariant: adr-new-strips-markers
	if strings.Contains(body, "GENERATED by awf") || strings.Contains(body, "awf:edit") {
		t.Errorf("marker comment survived: %q", body)
	}
	// invariant: adr-new-heading-matches-file
	if !strings.Contains(body, "# ADR-0001: My Cool Title") {
		t.Errorf("heading not filled in: %q", body)
	}
	if !strings.Contains(body, "date: 2026-07-01") {
		t.Errorf("date not filled in: %q", body)
	}
}

// TestNewFileSequentialCallsGetDifferentNumbers documents why NewFile's
// overwrite guard can never fire from repeated same-process calls: NextNumber
// always returns one more than every existing NNNN-*.md file, so a second call
// with the same title lands at the next number instead of colliding.
func TestNewFileSequentialCallsGetDifferentNumbers(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFixture(t, dir)
	swapNow(t, fixedNow)
	first, err := adr.NewFile(dir, "Same Title")
	if err != nil {
		t.Fatalf("first NewFile: %v", err)
	}
	second, err := adr.NewFile(dir, "Same Title")
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
	if _, err := adr.NewFile(dir, "Some Title"); err == nil {
		t.Fatal("expected NextNumber's glob error to propagate")
	}
}

func TestNewFileMissingTemplate(t *testing.T) {
	dir := t.TempDir()
	if _, err := adr.NewFile(dir, "No Template Here"); err == nil {
		t.Fatal("expected error for missing template.md")
	}
}

func TestNewFileEmptySlug(t *testing.T) {
	dir := t.TempDir()
	// No template.md needed - slugify errors before the file is ever read.
	if _, err := adr.NewFile(dir, "!!!"); err == nil {
		t.Fatal("expected slugify error for an all-punctuation title")
	}
}

func TestNewFileMissingDatePlaceholder(t *testing.T) {
	dir := t.TempDir()
	broken := strings.Replace(adrTemplateFixture, "date: YYYY-MM-DD\n", "", 1)
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.NewFile(dir, "Some Title"); err == nil {
		t.Fatal("expected error for missing date placeholder")
	}
}

func TestNewFileMissingTitlePlaceholder(t *testing.T) {
	dir := t.TempDir()
	broken := strings.Replace(adrTemplateFixture, "# ADR-NNNN: Title\n", "", 1)
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.NewFile(dir, "Some Title"); err == nil {
		t.Fatal("expected error for missing title placeholder")
	}
}
