package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig writes config.yaml into a fresh awf dir and returns that dir.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadParsesSkeletonFields(t *testing.T) {
	dir := writeConfig(t, `prefix: example
vars:
  testCmd: go test ./...
skills:
  - tdd
  - bugfix
agents:
  - code-reviewer
docs:
  - architecture
hooks: [pre-commit]
`)
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Prefix != "example" {
		t.Errorf("prefix = %q", c.Prefix)
	}
	if c.Vars["testCmd"] != "go test ./..." {
		t.Errorf("vars.testCmd = %v", c.Vars["testCmd"])
	}
	if strings.Join(c.Skills, ",") != "tdd,bugfix" {
		t.Errorf("skills = %v", c.Skills)
	}
	if strings.Join(c.Agents, ",") != "code-reviewer" {
		t.Errorf("agents = %v", c.Agents)
	}
	if strings.Join(c.Docs, ",") != "architecture" {
		t.Errorf("docs = %v", c.Docs)
	}
}

// invariant: enable-arrays
func TestEnableListsAreArrays(t *testing.T) {
	dir := writeConfig(t, "prefix: example\nskills:\n  - tdd\n  - bugfix\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Skills) != 2 || c.Skills[0] != "tdd" || c.Skills[1] != "bugfix" {
		t.Errorf("skills did not parse to []string{tdd,bugfix}: %v", c.Skills)
	}
	// A per-target data/sections/local key at the root is rejected (KnownFields).
	for _, bad := range []string{"data", "sections", "local"} {
		d := writeConfig(t, "prefix: example\n"+bad+": {}\n")
		if _, err := Load(d); err == nil {
			t.Errorf("expected a root %q key to be rejected", bad)
		}
	}
	// The map enable shape (name: {}) no longer parses as a string array.
	d := writeConfig(t, "prefix: example\nskills:\n  tdd: {}\n")
	if _, err := Load(d); err == nil {
		t.Error("expected map-shaped skills to be rejected (must be a string array)")
	}
}

// invariant: config-root
// TestLoadReadsTreeRoot pins the config root to .claude/awf/config.yaml and
// co-owns (with the migrate package's TestLegacyReadOnlyInMigrate, ADR-0010
// inv: legacy-read-isolation) the exemption that ONLY internal/migrate reads the
// legacy .claude/awf.yaml: the import-graph assertion below scans the repo and
// fails if any non-migrate, non-test source references the legacy path.
func TestLoadReadsTreeRoot(t *testing.T) {
	root := t.TempDir()
	awfDir := filepath.Join(root, ".claude", "awf")
	if err := os.MkdirAll(awfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awfDir, "config.yaml"), []byte("prefix: tree-root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A legacy decoy sibling; Load must ignore it.
	if err := os.WriteFile(filepath.Join(root, ".claude", "awf.yaml"), []byte("prefix: legacy-decoy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(awfDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Prefix != "tree-root" {
		t.Errorf("Load read the wrong file: prefix = %q, want tree-root", c.Prefix)
	}

	repo := repoRoot(t)
	legacyRefs := scanLegacyRefs(t, repo)
	if len(legacyRefs) != 0 {
		t.Errorf("only internal/migrate may reference the legacy .claude/awf.yaml; found refs in: %v", legacyRefs)
	}
}

// repoRoot ascends from the test's working directory to the directory holding go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test working directory")
		}
		dir = parent
	}
}

// scanLegacyRefs returns non-test, non-migrate Go files that mention the legacy
// awf.yaml filename.
func scanLegacyRefs(t *testing.T, repo string) []string {
	t.Helper()
	var hits []string
	migrateDir := filepath.Join("internal", "migrate")
	err := filepath.WalkDir(repo, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(repo, path)
		if strings.HasPrefix(rel, migrateDir) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), "awf.yaml") {
			hits = append(hits, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hits
}

func TestSidecarReadsDataSectionsLocal(t *testing.T) {
	dir := writeConfig(t, "prefix: example\nskills:\n  - tdd\n")
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	sidecar := "data:\n  foo: bar\nsections:\n  notes:\n    drop: true\nlocal: false\n"
	if err := os.WriteFile(filepath.Join(dir, "skills", "tdd.yaml"), []byte(sidecar), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	sc, err := c.Sidecar("skills", "tdd")
	if err != nil {
		t.Fatalf("Sidecar: %v", err)
	}
	if sc.Data["foo"] != "bar" {
		t.Errorf("sidecar data.foo = %v", sc.Data["foo"])
	}
	if !sc.Sections["notes"].Drop {
		t.Errorf("sidecar sections.notes.drop should be true")
	}
}

// invariant: sidecar-optional
func TestSidecarAbsentIsEmpty(t *testing.T) {
	dir := writeConfig(t, "prefix: example\nskills:\n  - tdd\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	sc, err := c.Sidecar("skills", "tdd")
	if err != nil {
		t.Fatalf("absent sidecar should be empty, not an error: %v", err)
	}
	if sc.Data != nil || sc.Sections != nil || sc.Local {
		t.Errorf("absent sidecar should be the zero Sidecar, got %#v", sc)
	}
}

func TestSidecarRejectsDropAndReplace(t *testing.T) {
	dir := writeConfig(t, "prefix: example\nskills:\n  - tdd\n")
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	bad := "sections:\n  notes:\n    drop: true\n    replaceWith: parts/x.md\n"
	if err := os.WriteFile(filepath.Join(dir, "skills", "tdd.yaml"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	c, _ := Load(dir)
	if _, err := c.Sidecar("skills", "tdd"); err == nil {
		t.Error("expected error: section cannot both drop and replaceWith")
	}
}

func TestSidecarRejectsUnknownKey(t *testing.T) {
	dir := writeConfig(t, "prefix: example\nskills:\n  - tdd\n")
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skills", "tdd.yaml"), []byte("dat: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, _ := Load(dir)
	if _, err := c.Sidecar("skills", "tdd"); err == nil {
		t.Error("expected error for unknown sidecar key 'dat'")
	}
}

func TestPartPath(t *testing.T) {
	dir := writeConfig(t, "prefix: example\n")
	c, _ := Load(dir)
	if got := c.PartPath("skills", "debugging", "surfaces"); got != filepath.Join(dir, "skills", "parts", "debugging", "surfaces.md") {
		t.Errorf("PartPath skills = %q", got)
	}
	if got := c.PartPath("agents-doc", "", "identity"); got != filepath.Join(dir, "parts", "agents-doc", "identity.md") {
		t.Errorf("PartPath agents-doc = %q", got)
	}
}

func TestValidateRejectsEmptyPrefix(t *testing.T) {
	dir := writeConfig(t, "prefix: \"\"\nskills: []\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err == nil {
		t.Errorf("expected error for empty prefix")
	}
}

func TestValidateRejectsPathInPrefix(t *testing.T) {
	cases := []string{"../evil", "foo/bar", "a\\b"}
	for _, prefix := range cases {
		dir := writeConfig(t, "prefix: "+prefix+"\nskills: []\n")
		c, err := Load(dir)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if err := c.Validate(); err == nil {
			t.Errorf("expected error for prefix %q containing path separator", prefix)
		}
	}
}

func TestLoadRejectsUnknownTopLevelKey(t *testing.T) {
	dir := writeConfig(t, "prefix: example\nskils: []\n")
	if _, err := Load(dir); err == nil {
		t.Errorf("expected error for unknown top-level key 'skils'")
	}
}

// invariant: docsdir-default
func TestDocsDirDefaultsToDocs(t *testing.T) {
	dir := writeConfig(t, "prefix: example\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.DocsDir != "docs" {
		t.Errorf("DocsDir = %q, want \"docs\"", c.DocsDir)
	}
}

func TestDocsDirExplicitValue(t *testing.T) {
	dir := writeConfig(t, "prefix: example\ndocsDir: documentation\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.DocsDir != "documentation" {
		t.Errorf("DocsDir = %q, want \"documentation\"", c.DocsDir)
	}
}

func TestDocsDirRejectsEscapingPath(t *testing.T) {
	c := &Config{Prefix: "example", DocsDir: "../escape"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for escaping docsDir")
	}
}

// invariant: invariants-glob-basename
func TestInvariantGlobValidation(t *testing.T) {
	ok := &Config{Prefix: "x", DocsDir: "docs", Invariants: &InvariantConfig{
		Sources: []InvariantSource{{Globs: []string{"*.go", "*_test.py"}, Marker: "//"}},
	}}
	if err := ok.Validate(); err != nil {
		t.Errorf("valid basename globs rejected: %v", err)
	}
	pathGlob := &Config{Prefix: "x", DocsDir: "docs", Invariants: &InvariantConfig{
		Sources: []InvariantSource{{Globs: []string{"**/*.go"}, Marker: "//"}},
	}}
	if err := pathGlob.Validate(); err == nil {
		t.Error("expected path-separator glob to be rejected")
	}
	bad := &Config{Prefix: "x", DocsDir: "docs", Invariants: &InvariantConfig{
		Sources: []InvariantSource{{Globs: []string{"[", "*.go"}, Marker: "//"}},
	}}
	if err := bad.Validate(); err == nil {
		t.Error("expected malformed glob to be rejected")
	}
	emptyMarker := &Config{Prefix: "x", DocsDir: "docs", Invariants: &InvariantConfig{
		Sources: []InvariantSource{{Globs: []string{"*.go"}}},
	}}
	if err := emptyMarker.Validate(); err == nil {
		t.Error("expected empty marker to be rejected (a bare marker would match prose)")
	}
}
