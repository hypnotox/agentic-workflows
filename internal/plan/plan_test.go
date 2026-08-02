package plan_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/plan"
)

// TestParseDirParsesFrontmatterAndSkipsNonPlans covers the happy path (a plan
// with frontmatter, a frontmatter-less plan) and the FilenameRe exclusions
// (template.md, README.md, and a non-plan .md are skipped).
func TestParseDirParsesFrontmatterAndSkipsNonPlans(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("2026-07-12-with-frontmatter.md", "---\ndate: 2026-07-12\nadrs: [97, 98]\nstatus: Proposed\n---\n# Plan: With Frontmatter\n")
	write("2026-06-24-legacy.md", "# Plan: Legacy\n\nNo frontmatter here.\n")
	write("template.md", "---\ndate: YYYY-MM-DD\n---\n# Plan: Title\n")
	write("README.md", "# Plans\n")
	write("notes.md", "# scratch\n")

	plans, err := plan.ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	byName := map[string]plan.Plan{}
	for _, pl := range plans {
		byName[pl.Filename] = pl
	}
	if len(byName) != 2 {
		t.Fatalf("expected 2 plans (template.md, README.md, notes.md skipped), got %d: %v", len(byName), plans)
	}

	fm := byName["2026-07-12-with-frontmatter.md"]
	if !fm.HasFrontmatter {
		t.Error("expected HasFrontmatter true for the frontmatter plan")
	}
	if fm.Date != "2026-07-12" {
		t.Errorf("Date = %q, want 2026-07-12", fm.Date)
	}
	if fm.Status != "Proposed" {
		t.Errorf("Status = %q, want Proposed", fm.Status)
	}
	if len(fm.ADRs) != 2 || fm.ADRs[0].Identity() != "0097" || fm.ADRs[1].Identity() != "0098" {
		t.Errorf("ADRs = %v, want [0097 0098]", fm.ADRs)
	}

	legacy := byName["2026-06-24-legacy.md"]
	if legacy.HasFrontmatter {
		t.Error("expected HasFrontmatter false for the frontmatter-less plan")
	}
}

// TestParseDirReadsNumberAndSlugADRLinks covers the `adrs:` entry grammar
// (ADR-0202 item 14): a number and a pending record's slug both parse into the
// field their spelling names, and every zero-padded spelling the live plans use
// stays a number whichever tag yaml.v3 resolves it to. Each case asserts the
// filled field as well as the identity, because a numeric slug and a number
// share an identity string and only the field distinguishes them.
func TestParseDirAcceptsEmptyLegacyFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "2026-07-12-empty-frontmatter.md", "---\n---\n# Plan: Legacy\n")
	plans, err := plan.ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(plans) != 1 || !plans[0].HasFrontmatter || plans[0].Format != "" {
		t.Fatalf("plans = %#v, want one marker-absent legacy plan", plans)
	}
}

func TestParseDirReadsNumberAndSlugADRLinks(t *testing.T) {
	for _, tc := range []struct {
		name       string
		entry      string
		wantNumber int
		wantSlug   string
		want       string
	}{
		{name: "plain number", entry: "97", wantNumber: 97, want: "0097"},
		{name: "zero-padded number resolves as !!float", entry: "0186", wantNumber: 186, want: "0186"},
		{name: "zero-padded octal-valid number is no longer read as octal", entry: "0153", wantNumber: 153, want: "0153"},
		{name: "four-digit number", entry: "0194", wantNumber: 194, want: "0194"},
		{name: "quoted number stays a number", entry: `"0186"`, wantNumber: 186, want: "0186"},
		{name: "bare slug", entry: "pending-record-slug", wantSlug: "pending-record-slug", want: "pending-record-slug"},
		{name: "quoted slug", entry: `"pending-record-slug"`, wantSlug: "pending-record-slug", want: "pending-record-slug"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writePlan(t, dir, "2026-07-31-links.md", "---\ndate: 2026-07-31\nadrs: ["+tc.entry+"]\nstatus: Proposed\n---\n# Plan: Links\n")
			plans, err := plan.ParseDir(dir)
			if err != nil {
				t.Fatalf("ParseDir: %v", err)
			}
			if len(plans) != 1 || len(plans[0].ADRs) != 1 {
				t.Fatalf("plans = %#v", plans)
			}
			link := plans[0].ADRs[0]
			if link.Number != tc.wantNumber || link.Slug != tc.wantSlug {
				t.Errorf("link = {Number:%d Slug:%q}, want {Number:%d Slug:%q}", link.Number, link.Slug, tc.wantNumber, tc.wantSlug)
			}
			if got := link.Identity(); got != tc.want {
				t.Errorf("Identity() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestParseDirRejectsUnusableADRLinks covers the `adrs:` entries that are
// neither a number nor a slug. A slug that names no record is deliberately not
// here: it parses, and fails link validation as a scoped finding instead.
func TestParseDirRejectsUnusableADRLinks(t *testing.T) {
	for _, tc := range []struct {
		name  string
		entry string
		want  string
	}{
		{"mapping node", "{a: 1}", "must be an ADR number or slug"},
		{"fractional scalar", "1.5", "neither an ADR number nor a slug"},
		{"boolean scalar", "true", "neither an ADR number nor a slug"},
		{"empty scalar", `""`, "neither an ADR number nor a slug"},
		{"zero", "0", "not a usable ADR number"},
		{"past the four-digit identity width", "10000", "not a usable ADR number"},
		{"number past int range", "99999999999999999999", "not a usable ADR number"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writePlan(t, dir, "2026-07-31-links.md", "---\ndate: 2026-07-31\nadrs: ["+tc.entry+"]\nstatus: Proposed\n---\n# Plan: Links\n")
			_, err := plan.ParseDir(dir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ParseDir error = %v, want one containing %q", err, tc.want)
			}
		})
	}
}

func writePlan(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestParseDirExtractsCommitSubjects covers commit-subject extraction: a ```commit
// block is captured (first non-empty line, multi-line body ignored); a bare fence, a
// language fence, a ```commit awf-ignore opt-out, and an empty ```commit block are
// all skipped.
// invariant: adr-system/plan-artifacts:plan-commit-subject-marker-scoped (TestParseDirExtractsCommitSubjects)
// invariant: adr-system/plan-artifacts:plan-commit-subject-optout-honored (TestParseDirExtractsCommitSubjects)
func TestParseDirExtractsCommitSubjects(t *testing.T) {
	dir := t.TempDir()
	body := "---\ndate: 2026-07-14\nadrs: []\nstatus: Proposed\n---\n# Plan: X\n\n" +
		"```commit\nfeat(awf): real subject\nbody line ignored\n```\n\n" +
		"```\nfeat(awf): bare fence not captured\n```\n\n" +
		"```go\nfmt.Println()\n```\n\n" +
		"```commit awf-ignore\nfeat(awf): opted-out example\n```\n\n" +
		"```commit\n\n```\n"
	if err := os.WriteFile(filepath.Join(dir, "2026-07-14-x.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	plans, err := plan.ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("want 1 plan, got %d", len(plans))
	}
	got := plans[0].CommitSubjects
	want := []string{"feat(awf): real subject"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("CommitSubjects = %#v, want %#v", got, want)
	}
}

func TestParseDirResolveErrors(t *testing.T) {
	parent := t.TempDir()
	loop := filepath.Join(parent, "loop")
	if err := os.Symlink("loop", loop); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.ParseDir(loop); err == nil || !strings.Contains(err.Error(), "resolve plans directory") {
		t.Fatalf("directory resolve error = %v", err)
	}

	dir := t.TempDir()
	if err := os.Symlink("missing.md", filepath.Join(dir, "2026-08-02-broken.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.ParseDir(dir); err == nil || !strings.Contains(err.Error(), "resolve 2026-08-02-broken.md") {
		t.Fatalf("plan resolve error = %v", err)
	}
}

// TestParseDirGlobError exercises the glob-pattern failure path: a directory
// whose name contains an unterminated "[" yields an ErrBadPattern from Glob.
func TestParseDirGlobError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bad[")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.ParseDir(dir); err == nil {
		t.Fatal("expected glob error for malformed pattern, got nil")
	}
}

// TestParseDirReadError exercises the os.ReadFile failure path: a directory
// squatting on a path that matches the plan filename pattern cannot be read as
// a file.
func TestParseDirReadError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "2026-07-12-squatter.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.ParseDir(dir); err == nil {
		t.Fatal("expected read error for directory in file's place, got nil")
	}
}

// TestParseDirParseError exercises the frontmatter parse failure path:
// malformed YAML makes frontmatter.Parse return an error.
func TestParseDirParseError(t *testing.T) {
	dir := t.TempDir()
	content := "---\nstatus: [unterminated\n---\n# Plan: Broken\n"
	if err := os.WriteFile(filepath.Join(dir, "2026-07-12-broken.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.ParseDir(dir); err == nil {
		t.Fatal("expected parse error for malformed frontmatter, got nil")
	}
}

// swapNow overrides the plan package's now seam for the duration of a test.
func swapNow(t *testing.T, fn func() time.Time) {
	t.Helper()
	orig := plan.SetNowForTest(fn)
	t.Cleanup(func() { plan.SetNowForTest(orig) })
}

func fixedNow() time.Time { return time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC) }

// planTemplateFixture mirrors the shape NewFile expects: a rendered template
// with marker comments, the date placeholder, and the title heading.
const planTemplateFixture = `---
format: plan-v1
date: YYYY-MM-DD
adrs: []
status: Proposed
---
<!-- GENERATED by awf -->
# Plan: Title

<!-- awf:edit positioning: default -->
## Goal

Fixture outcome.

## Architecture summary

Fixture structure.

## Phase 1: Build

**Execution mode: inline.**

### Task 1.1: Implement

Create the fixture.

### Phase close

Close the transaction.

` + "```commit" + `
feat(plans): build fixture
` + "```" + `

## Definition of done

- The fixture parses.

## Notes

No deviations.
`

func writePlanTemplate(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte(planTemplateFixture), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNewFileHappyPath(t *testing.T) {
	dir := t.TempDir()
	writePlanTemplate(t, dir)
	swapNow(t, fixedNow)

	path, err := plan.NewFile(dir, "My Cool Plan")
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	want := filepath.Join(dir, "2026-07-12-my-cool-plan.md")
	// invariant: adr-system/plan-artifacts:plan-new-unnumbered (TestNewFileHappyPath)
	if path != want {
		t.Errorf("NewFile path = %q, want %q", path, want)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	s := string(body)
	if strings.Contains(s, "GENERATED by awf") || strings.Contains(s, "awf:edit") {
		t.Errorf("marker comment survived: %q", s)
	}
	if !strings.Contains(s, "# Plan: My Cool Plan") {
		t.Errorf("heading not filled in: %q", s)
	}
	if !strings.Contains(s, "format: plan-v1\n") {
		t.Errorf("plan-v1 marker missing: %q", s)
	}
	if !strings.Contains(s, "date: 2026-07-12") {
		t.Errorf("date not filled in: %q", s)
	}
	plans, err := plan.ParseDir(dir)
	if err != nil {
		t.Fatalf("new scaffold does not parse cleanly: %v", err)
	}
	if len(plans) != 1 || plans[0].Format != "plan-v1" || len(plans[0].Phases) != 1 {
		t.Fatalf("parsed scaffold = %#v, want one plan-v1 phase", plans)
	}
}

func TestNewFileRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	writePlanTemplate(t, dir)
	swapNow(t, fixedNow)
	if _, err := plan.NewFile(dir, "Same Plan"); err != nil {
		t.Fatalf("first NewFile: %v", err)
	}
	if _, err := plan.NewFile(dir, "Same Plan"); err == nil {
		t.Fatal("expected overwrite refusal on same-day same-title plan, got nil")
	}
}

func TestNewFileRejectsUnusableTitle(t *testing.T) {
	dir := t.TempDir()
	writePlanTemplate(t, dir)
	if _, err := plan.NewFile(dir, "!!! ???"); err == nil {
		t.Fatal("expected error for a title with no usable characters, got nil")
	}
}

func TestNewFileErrorsOnMissingTemplate(t *testing.T) {
	dir := t.TempDir() // no template.md written
	swapNow(t, fixedNow)
	if _, err := plan.NewFile(dir, "No Template"); err == nil {
		t.Fatal("expected error when template.md is absent, got nil")
	}
}

func TestNewFileErrorsOnDriftedTemplate(t *testing.T) {
	dir := t.TempDir()
	// A template missing the date: placeholder trips replaceOnce.
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte("---\nstatus: Proposed\n---\n# Plan: Title\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	swapNow(t, fixedNow)
	if _, err := plan.NewFile(dir, "Drifted"); err == nil {
		t.Fatal("expected error for template missing the date: placeholder, got nil")
	}
}

func TestNewFileErrorsOnMissingTitlePlaceholder(t *testing.T) {
	dir := t.TempDir()
	// Has date: but no "# Plan: Title" heading - the second replaceOnce trips.
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte("---\ndate: YYYY-MM-DD\n---\n# Wrong Heading\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	swapNow(t, fixedNow)
	if _, err := plan.NewFile(dir, "No Heading"); err == nil {
		t.Fatal("expected error for template missing the # Plan: Title heading, got nil")
	}
}
