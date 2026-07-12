package plan_test

import (
	"os"
	"path/filepath"
	"testing"

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
	if len(fm.ADRs) != 2 || fm.ADRs[0] != 97 || fm.ADRs[1] != 98 {
		t.Errorf("ADRs = %v, want [97 98]", fm.ADRs)
	}

	legacy := byName["2026-06-24-legacy.md"]
	if legacy.HasFrontmatter {
		t.Error("expected HasFrontmatter false for the frontmatter-less plan")
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
