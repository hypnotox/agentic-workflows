package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"gopkg.in/yaml.v3"
)

// fixtureMonolith is a representative pre-ADR-0009 .claude/awf.yaml exercising
// every shape the tree-layout migration must port: skeleton fields, a non-default
// docsDir, per-target data, sections (drop and explicit replaceWith), a local
// target, hooks, the agents-doc ownership/identity/invariants/docMap, and a doc
// sidecar. Convention-part bodies referenced by replaceWith are written alongside.
const fixtureMonolith = `prefix: ex
docsDir: documentation
invariants:
  sources:
    - globs: ["*.go"]
      marker: "//"
vars:
  testCmd: go test ./...
  gateCmd: ./x gate
skills:
  tdd:
    data:
      testSurfaces:
        - {name: Logic, location: internal, kind: unit}
  debugging:
    sections:
      debugging-surfaces:
        replaceWith: parts/debugging-surfaces.md
  refactor-coupling-audit:
    sections:
      category-4-codegen:
        drop: true
  brainstorming: {}
  local-skill: {local: true}
agents:
  code-reviewer:
    data:
      correctnessTraps:
        - {description: "wrap errors with %w"}
hooks:
  - pre-commit
  - pre-push
agentsDoc:
  data:
    ownership: |
      You own this.
    identity: |
      ex is a thing.
    invariants:
      - {text: "**Be safe.**", ref: ADR-0001}
    docMap: []
docs:
  architecture:
    sections:
      body:
        replaceWith: parts/doc-architecture.md
`

// writeMonolith lays down a temp project root containing the fixture monolith and
// the convention parts its sections point at, returning the root.
func writeMonolith(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	awfDir := filepath.Join(root, ".claude", "awf")
	if err := os.MkdirAll(filepath.Join(awfDir, "parts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "awf.yaml"), []byte(fixtureMonolith), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "awf.lock"), []byte(`{"awfVersion":"0.1.0","files":{}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awfDir, "parts", "debugging-surfaces.md"), []byte("DEBUG SURFACES BODY\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awfDir, "parts", "doc-architecture.md"), []byte("ARCH BODY\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestGateBlocksWhenBehind(t *testing.T) {
	// invariant: upgrade-gate
	root := writeMonolith(t) // legacy layout → generation 0, current 1
	if got := Generation(root); got != 0 {
		t.Fatalf("Generation(legacy) = %d, want 0", got)
	}
	if got := GateState(root); got != "gate" {
		t.Errorf("GateState(legacy) = %q, want gate", got)
	}
	// A project already at the current schema does not gate.
	upgraded := t.TempDir()
	mustMkdir(t, filepath.Join(upgraded, ".claude", "awf"))
	mustWrite(t, filepath.Join(upgraded, ".claude", "awf", "config.yaml"), "prefix: ex\n")
	stampLock(t, upgraded, Current())
	if got := GateState(upgraded); got != "ok" {
		t.Errorf("GateState(current) = %q, want ok", got)
	}
}

func TestUpgradeAppliesInOrderIdempotent(t *testing.T) {
	// invariant: migration-ordering
	root := writeMonolith(t)
	applied, err := Upgrade(root)
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if strings.Join(applied, ",") != "tree-layout" {
		t.Errorf("first Upgrade applied = %v, want [tree-layout]", applied)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "awf", "config.yaml")); err != nil {
		t.Errorf("tree not produced: %v", err)
	}
	// runUpgrade's terminal sync stamps the lock; simulate it, then re-running
	// upgrade at the current schema applies nothing and exits zero.
	stampLock(t, root, Current())
	again, err := Upgrade(root)
	if err != nil {
		t.Fatalf("second Upgrade: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("second Upgrade applied = %v, want []", again)
	}
}

func TestNoopGapAutoBumps(t *testing.T) {
	// invariant: noop-autobump
	// A gap covered by no registered migration auto-bumps rather than gating.
	if got := gateStateFor(2, 5, []int{1, 2}); got != "autobump" {
		t.Errorf("gateStateFor(2,5,[1,2]) = %q, want autobump", got)
	}
	// A gap with a covering migration gates; at/above current is ok.
	if got := gateStateFor(0, 5, []int{1, 3}); got != "gate" {
		t.Errorf("gateStateFor(0,5,[1,3]) = %q, want gate", got)
	}
	if got := gateStateFor(5, 5, []int{1, 5}); got != "ok" {
		t.Errorf("gateStateFor(5,5,...) = %q, want ok", got)
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stampLock(t *testing.T, root string, schema int) {
	t.Helper()
	l := &manifest.Lock{AWFVersion: "0.1.0", SchemaVersion: schema, Files: map[string]manifest.Entry{}}
	if err := l.Save(filepath.Join(root, ".claude", "awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
}

func TestTreeLayoutPortsMonolith(t *testing.T) {
	root := writeMonolith(t)
	if err := applyTreeLayout(root); err != nil {
		t.Fatalf("applyTreeLayout: %v", err)
	}
	awfDir := filepath.Join(root, ".claude", "awf")

	// The legacy file is gone.
	if _, err := os.Stat(filepath.Join(root, ".claude", "awf.yaml")); !os.IsNotExist(err) {
		t.Errorf("legacy .claude/awf.yaml should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "awf.lock")); !os.IsNotExist(err) {
		t.Errorf("legacy .claude/awf.lock should be removed, stat err = %v", err)
	}

	// config.yaml: skeleton with sorted enable arrays and a carried-through docsDir.
	var skel struct {
		Prefix  string   `yaml:"prefix"`
		DocsDir string   `yaml:"docsDir"`
		Skills  []string `yaml:"skills"`
		Agents  []string `yaml:"agents"`
		Docs    []string `yaml:"docs"`
		Hooks   []string `yaml:"hooks"`
	}
	readYAML(t, filepath.Join(awfDir, "config.yaml"), &skel)
	if skel.Prefix != "ex" {
		t.Errorf("config.prefix = %q", skel.Prefix)
	}
	if skel.DocsDir != "documentation" {
		t.Errorf("config.docsDir = %q, want documentation (non-default carried through)", skel.DocsDir)
	}
	wantSkills := []string{"brainstorming", "debugging", "local-skill", "refactor-coupling-audit", "tdd"}
	if strings.Join(skel.Skills, ",") != strings.Join(wantSkills, ",") {
		t.Errorf("config.skills = %v, want %v", skel.Skills, wantSkills)
	}
	if strings.Join(skel.Agents, ",") != "code-reviewer" {
		t.Errorf("config.agents = %v", skel.Agents)
	}
	if strings.Join(skel.Docs, ",") != "architecture" {
		t.Errorf("config.docs = %v", skel.Docs)
	}

	// A representative data sidecar.
	var tddSc struct {
		Data map[string]any `yaml:"data"`
	}
	readYAML(t, filepath.Join(awfDir, "skills", "tdd.yaml"), &tddSc)
	if tddSc.Data["testSurfaces"] == nil {
		t.Errorf("tdd sidecar missing testSurfaces data")
	}

	// A drop-only sidecar keeps its drop and carries no convention part.
	var rcaSc struct {
		Sections map[string]map[string]any `yaml:"sections"`
	}
	readYAML(t, filepath.Join(awfDir, "skills", "refactor-coupling-audit.yaml"), &rcaSc)
	if d, _ := rcaSc.Sections["category-4-codegen"]["drop"].(bool); !d {
		t.Errorf("refactor-coupling-audit drop not preserved: %v", rcaSc.Sections)
	}

	// A replaceWith section became a convention part; its sidecar is absent.
	gotPart := readFile(t, filepath.Join(awfDir, "skills", "parts", "debugging", "debugging-surfaces.md"))
	if gotPart != "DEBUG SURFACES BODY\n" {
		t.Errorf("debugging convention part = %q", gotPart)
	}
	if _, err := os.Stat(filepath.Join(awfDir, "skills", "debugging.yaml")); !os.IsNotExist(err) {
		t.Errorf("debugging should have no sidecar (sole section converted to a part)")
	}
	if _, err := os.Stat(filepath.Join(awfDir, "parts", "debugging-surfaces.md")); !os.IsNotExist(err) {
		t.Errorf("legacy flat part should be removed after conversion")
	}
	// Doc convention part.
	gotDoc := readFile(t, filepath.Join(awfDir, "docs", "parts", "architecture", "body.md"))
	if gotDoc != "ARCH BODY\n" {
		t.Errorf("architecture convention part = %q", gotDoc)
	}

	// agents-doc: prose parts carry their ## headings; data minus ownership/identity.
	yp := readFile(t, filepath.Join(awfDir, "parts", "agents-doc", "you-and-this-project.md"))
	if !strings.HasPrefix(yp, "## You and this project\n\n") {
		t.Errorf("you-and-this-project part must begin with its heading, got %q", yp)
	}
	if !strings.Contains(yp, "You own this.") {
		t.Errorf("you-and-this-project part missing prose: %q", yp)
	}
	ip := readFile(t, filepath.Join(awfDir, "parts", "agents-doc", "identity.md"))
	if !strings.HasPrefix(ip, "## Identity\n\n") {
		t.Errorf("identity part must begin with its heading, got %q", ip)
	}
	var adSc struct {
		Data map[string]any `yaml:"data"`
	}
	readYAML(t, filepath.Join(awfDir, "agents-doc.yaml"), &adSc)
	if adSc.Data["ownership"] != nil || adSc.Data["identity"] != nil {
		t.Errorf("agents-doc.yaml must not carry ownership/identity scalars: %v", adSc.Data)
	}
	if adSc.Data["invariants"] == nil {
		t.Errorf("agents-doc.yaml should retain invariants data")
	}
}

func readYAML(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := yaml.Unmarshal(b, v); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestLegacyReadOnlyInMigrate(t *testing.T) {
	// invariant: legacy-read-isolation
	// (a) readLegacy parses a fixture monolith.
	root := writeMonolith(t)
	lc, err := readLegacy(filepath.Join(root, ".claude", "awf.yaml"))
	if err != nil {
		t.Fatalf("readLegacy: %v", err)
	}
	if lc.Prefix != "ex" {
		t.Errorf("prefix = %q, want ex", lc.Prefix)
	}
	if lc.DocsDir != "documentation" {
		t.Errorf("docsDir = %q, want documentation", lc.DocsDir)
	}
	if _, ok := lc.Skills["tdd"]; !ok {
		t.Errorf("tdd skill missing")
	}
	if !lc.Skills["local-skill"].Local {
		t.Errorf("local-skill should be local")
	}
	if lc.Skills["debugging"].Sections["debugging-surfaces"].ReplaceWith != "parts/debugging-surfaces.md" {
		t.Errorf("debugging replaceWith not parsed")
	}
	if !lc.Skills["refactor-coupling-audit"].Sections["category-4-codegen"].Drop {
		t.Errorf("drop override not parsed")
	}
	if lc.AgentsDoc == nil || lc.AgentsDoc.Data["ownership"] == nil {
		t.Errorf("agentsDoc.data.ownership missing")
	}

	// (b) Comment contract: legacy.go documents itself as the sole reader of the
	// legacy path (ADR-0010 inv: legacy-read-isolation). The import-graph assertion
	// that no non-migrate package reads the legacy path co-owns this exemption and
	// lives in config's TestLoadReadsTreeRoot (inv: config-root), which becomes true
	// at the Phase-3 cutover; the two are written to agree.
	src, err := os.ReadFile("legacy.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"legacy-read-isolation", "SOLE reader"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("legacy.go missing documented contract %q", want)
		}
	}
}
