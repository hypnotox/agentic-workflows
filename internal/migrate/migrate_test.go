package migrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
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
	// invariant: config/migrations-and-locks:upgrade-gate (TestGateBlocksWhenBehind)
	root := writeMonolith(t) // legacy layout → generation 0
	if got := mustGeneration(t, root); got != 0 {
		t.Fatalf("Generation(legacy) = %d, want 0", got)
	}
	if got := mustGateState(t, root); got != "gate" {
		t.Errorf("GateState(legacy) = %q, want gate", got)
	}
	// A .awf/ tree already at the current schema does not gate.
	upgraded := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(upgraded, ".awf", "config.yaml"), "prefix: ex\n")
	stampLockAt(t, filepath.Join(upgraded, ".awf", "awf.lock"), Current())
	if got := mustGateState(t, upgraded); got != "ok" {
		t.Errorf("GateState(current) = %q, want ok", got)
	}

	// A .awf/ tree with no lock yet (a fresh awf init, or a just-upgraded project
	// before its terminal sync) reports Current() and must not gate - otherwise
	// sync/check would falsely block the very command that stamps the lock.
	noLock := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(noLock, ".awf", "config.yaml"), "prefix: ex\n")
	if got := mustGeneration(t, noLock); got != Current() {
		t.Errorf("Generation(.awf, no lock) = %d, want Current()=%d", got, Current())
	}
	if got := mustGateState(t, noLock); got != "ok" {
		t.Errorf("GateState(.awf, no lock) = %q, want ok (fresh init/post-upgrade must not gate)", got)
	}

	// A pre-relocation .claude/awf/ tree with no lock returns 1 (the tree-layout
	// port's output) and gates: drop-replacewith and the relocation still apply.
	preReloc := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(preReloc, ".claude", "awf", "config.yaml"), "prefix: ex\n")
	if got := mustGeneration(t, preReloc); got != 1 {
		t.Errorf("Generation(.claude/awf, no lock) = %d, want 1", got)
	}
	if got := mustGateState(t, preReloc); got != "gate" {
		t.Errorf("GateState(.claude/awf, no lock) = %q, want gate", got)
	}

	// Nothing present at all reports Current() (a bare dir is treated as current).
	if got := mustGeneration(t, t.TempDir()); got != Current() {
		t.Errorf("Generation(empty) = %d, want Current()=%d", got, Current())
	}
}

// A lockless pre-relocation .claude/awf/ tree is the tree-layout port's output
// (the port deletes the legacy lock), so Upgrade must still run every later
// migration - most importantly the To:3 relocation. The old Current()-1
// generation drifted upward as later migrations registered, leaving such a
// tree permanently gated while Upgrade only ran post-relocation no-ops.
func TestUpgradeRelocatesLocklessPreRelocationTree(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".claude", "awf", "config.yaml"), "prefix: ex\nskills: []\nagents: []\n")
	applied, _, err := Upgrade(testContext(t), root)
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "config.yaml")); err != nil {
		t.Errorf("relocation did not run (no .awf/config.yaml); applied = %v", applied)
	}
	if got := mustGateState(t, root); got != "ok" {
		t.Errorf("GateState after upgrade = %q, want ok", got)
	}
}

func TestNoopGapAutoBumps(t *testing.T) {
	// invariant: config/migrations-and-locks:noop-autobump (TestNoopGapAutoBumps)
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
	// gen strictly above current → the binary is behind the project (ADR-0039).
	if got := gateStateFor(6, 5, []int{1, 5}); got != "ahead" {
		t.Errorf("gateStateFor(6,5,...) = %q, want ahead", got)
	}
	if got := gateStateFor(5, 4, []int{1, 4}); got != "ahead" {
		t.Errorf("gateStateFor(5,4,...) = %q, want ahead", got)
	}
}

// stampLockAt writes a schema-stamped lock at an explicit path (for .awf/ trees).
func stampLockAt(t *testing.T, lockPath string, schema int) {
	t.Helper()
	l := &manifest.Lock{AWFVersion: "0.1.0", SchemaVersion: schema, Files: map[string]manifest.Entry{}}
	if err := l.Save(lockPath); err != nil {
		t.Fatal(err)
	}
}

func TestTreeLayoutPortsMonolith(t *testing.T) {
	root := writeMonolith(t)
	var changes Changes
	if err := applyTreeLayout(root, &changes); err != nil {
		t.Fatalf("applyTreeLayout: %v", err)
	}
	wantFacts := []string{
		"tree-layout: wrote .claude/awf/config.yaml",
		"tree-layout: wrote skills/tdd.yaml",
		"tree-layout: copied .claude/awf/parts/debugging-surfaces.md to .claude/awf/skills/parts/debugging/debugging-surfaces.md",
		"tree-layout: removed .claude/awf/parts/debugging-surfaces.md",
		"tree-layout: wrote skills/refactor-coupling-audit.yaml",
		"tree-layout: wrote skills/local-skill.yaml",
		"tree-layout: wrote agents/code-reviewer.yaml",
		"tree-layout: copied .claude/awf/parts/doc-architecture.md to .claude/awf/docs/parts/architecture/body.md",
		"tree-layout: removed .claude/awf/parts/doc-architecture.md",
		"tree-layout: wrote parts/agents-doc/you-and-this-project.md",
		"tree-layout: wrote parts/agents-doc/identity.md",
		"tree-layout: wrote agents-doc.yaml",
		"tree-layout: removed .claude/awf.yaml",
		"tree-layout: removed .claude/awf.lock",
	}
	for _, want := range wantFacts {
		if !strings.Contains(changes.String(), want+"\n") {
			t.Errorf("changes missing proven fact %q:\n%s", want, changes.String())
		}
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

// writeLegacyRoot lays down a temp project root containing a .claude/awf dir and
// the given legacy awf.yaml body, returning the root. Convention parts and squat
// directories are added by individual tests.
func writeLegacyRoot(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, ".claude", "awf.yaml"), body)
	return root
}

func TestApplyTreeLayoutNoopWhenLegacyAbsent(t *testing.T) {
	// No .claude/awf.yaml → already ported (or never legacy): a nil no-op.
	root := t.TempDir()
	if err := applyTreeLayout(root, &Changes{}); err != nil {
		t.Fatalf("applyTreeLayout(no legacy) = %v, want nil", err)
	}
}

func TestReadLegacyReadError(t *testing.T) {
	// A path that cannot be read (absent) surfaces a wrapped read error.
	_, err := readLegacy(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err == nil || !strings.Contains(err.Error(), "read legacy config") {
		t.Fatalf("readLegacy(absent) = %v, want a read-legacy-config error", err)
	}
}

func TestReadLegacyParseError(t *testing.T) {
	// An unknown field trips the strict (KnownFields) decoder.
	p := filepath.Join(t.TempDir(), "awf.yaml")
	testsupport.WriteFile(t, p, "prefix: ex\nbogusUnknownField: 1\n")
	_, err := readLegacy(p)
	if err == nil || !strings.Contains(err.Error(), "parse legacy config") {
		t.Fatalf("readLegacy(unknown field) = %v, want a parse-legacy-config error", err)
	}
}

func TestUpgradePropagatesMigrationError(t *testing.T) {
	// A malformed legacy file at generation 0 makes the tree-layout migration
	// fail; Upgrade wraps it with the migration name and returns it.
	root := writeLegacyRoot(t, "prefix: ex\nbogusUnknownField: 1\n")
	if got := mustGeneration(t, root); got != 0 {
		t.Fatalf("Generation(malformed legacy) = %d, want 0", got)
	}
	applied, _, err := Upgrade(testContext(t), root)
	if err == nil || !strings.Contains(err.Error(), "tree-layout") {
		t.Fatalf("Upgrade(testContext(t), malformed legacy) err = %v, want a tree-layout migration error", err)
	}
	if len(applied) != 0 {
		t.Errorf("Upgrade applied = %v, want [] on failure", applied)
	}
}

func TestApplyTreeLayoutConfigWriteError(t *testing.T) {
	// A directory squatting on config.yaml's path makes the skeleton write fail.
	root := writeLegacyRoot(t, "prefix: ex\n")
	if err := os.MkdirAll(filepath.Join(root, ".claude", "awf", "config.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := applyTreeLayout(root, &Changes{}); err == nil {
		t.Fatal("applyTreeLayout with config.yaml dir = nil, want a write error")
	}
}

func TestApplyTreeLayoutSidecarMkdirError(t *testing.T) {
	// A regular file squatting on the skills/ dir makes the sidecar MkdirAll fail.
	root := writeLegacyRoot(t, "prefix: ex\nskills:\n  alpha:\n    data:\n      k: v\n")
	testsupport.WriteFile(t, filepath.Join(root, ".claude", "awf", "skills"), "not a dir\n")
	var changes Changes
	if err := applyTreeLayout(root, &changes); err == nil {
		t.Fatal("applyTreeLayout with skills/ as a file = nil, want a mkdir error")
	}
	if got, want := changes.String(), "tree-layout: wrote .claude/awf/config.yaml\n"; got != want {
		t.Fatalf("changes = %q, want only the successful config write %q", got, want)
	}
}

func TestApplyTreeLayoutMissingPartSource(t *testing.T) {
	// A replaceWith pointing at an absent legacy part makes copyPart's read fail.
	root := writeLegacyRoot(t, "prefix: ex\nskills:\n  gamma:\n    sections:\n      sec:\n        replaceWith: parts/missing.md\n")
	err := applyTreeLayout(root, &Changes{})
	if err == nil || !strings.Contains(err.Error(), "read part") {
		t.Fatalf("applyTreeLayout(missing part) = %v, want a read-part error", err)
	}
}

func TestApplyTreeLayoutCopyPartWriteError(t *testing.T) {
	// A directory squatting on a convention part's destination makes copyPart's
	// write fail (the source reads fine; only the write target is hostile).
	root := writeLegacyRoot(t, "prefix: ex\nskills:\n  beta:\n    sections:\n      sec:\n        replaceWith: parts/p.md\n")
	testsupport.WriteFile(t, filepath.Join(root, ".claude", "awf", "parts", "p.md"), "BODY\n")
	if err := os.MkdirAll(filepath.Join(root, ".claude", "awf", "skills", "parts", "beta", "sec.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := applyTreeLayout(root, &Changes{}); err == nil {
		t.Fatal("applyTreeLayout with squatted part dst = nil, want a write error")
	}
}

func TestApplyTreeLayoutLockRemoveError(t *testing.T) {
	// A non-empty directory squatting on the legacy awf.lock makes the terminal
	// os.Remove fail with a real (non-NotExist) error after a successful port.
	root := writeMonolith(t)
	lock := filepath.Join(root, ".claude", "awf.lock")
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(lock, 0o755); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(lock, "occupant"), "x\n")
	if err := applyTreeLayout(root, &Changes{}); err == nil {
		t.Fatal("applyTreeLayout with non-empty awf.lock dir = nil, want a remove error")
	}
}

func TestPortAgentsDocSectionsLocalAndData(t *testing.T) {
	// agentsDoc with a successful replaceWith section, a drop section, extra data,
	// and local:true exercises the full sidecar-assembly path.
	root := writeLegacyRoot(t, "prefix: ex\n"+
		"agentsDoc:\n"+
		"  local: true\n"+
		"  data:\n"+
		"    ownership: \"Own.\"\n"+
		"    identity: \"Id.\"\n"+
		"    extra: keep\n"+
		"  sections:\n"+
		"    sec-a:\n"+
		"      replaceWith: parts/adp.md\n"+
		"    sec-b:\n"+
		"      drop: true\n")
	testsupport.WriteFile(t, filepath.Join(root, ".claude", "awf", "parts", "adp.md"), "AD PART\n")
	if err := applyTreeLayout(root, &Changes{}); err != nil {
		t.Fatalf("applyTreeLayout: %v", err)
	}
	awfDir := filepath.Join(root, ".claude", "awf")
	if got := readFile(t, filepath.Join(awfDir, "parts", "agents-doc", "sec-a.md")); got != "AD PART\n" {
		t.Errorf("sec-a convention part = %q", got)
	}
	var ad struct {
		Data     map[string]any            `yaml:"data"`
		Sections map[string]map[string]any `yaml:"sections"`
		Local    bool                      `yaml:"local"`
	}
	readYAML(t, filepath.Join(awfDir, "agents-doc.yaml"), &ad)
	if ad.Data["extra"] != "keep" {
		t.Errorf("agents-doc.yaml data.extra = %v, want keep", ad.Data["extra"])
	}
	if d, _ := ad.Sections["sec-b"]["drop"].(bool); !d {
		t.Errorf("agents-doc.yaml sec-b drop not preserved: %v", ad.Sections)
	}
	if !ad.Local {
		t.Errorf("agents-doc.yaml local = false, want true")
	}
}

func TestPortAgentsDocRetainsEarlierWriteEvidenceWhenSidecarWriteFails(t *testing.T) {
	root := writeLegacyRoot(t, "prefix: ex\nagentsDoc:\n  data:\n    ownership: \"Own.\"\n    extra: keep\n")
	if err := os.MkdirAll(filepath.Join(root, ".claude", "awf", "agents-doc.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	var changes Changes
	if err := applyTreeLayout(root, &changes); err == nil {
		t.Fatal("applyTreeLayout with agents-doc sidecar directory = nil, want a write error")
	}
	want := "tree-layout: wrote .claude/awf/config.yaml\n" +
		"tree-layout: wrote parts/agents-doc/you-and-this-project.md\n"
	if got := changes.String(); got != want {
		t.Fatalf("changes = %q, want successful writes before the failed sidecar write %q", got, want)
	}
}

func TestPortAgentsDocSkipsAbsentOwnershipIdentity(t *testing.T) {
	// agentsDoc whose data lacks ownership/identity skips the prose-part writes
	// and keeps the remaining data in the sidecar.
	root := writeLegacyRoot(t, "prefix: ex\nagentsDoc:\n  data:\n    extra: keep\n")
	if err := applyTreeLayout(root, &Changes{}); err != nil {
		t.Fatalf("applyTreeLayout: %v", err)
	}
	awfDir := filepath.Join(root, ".claude", "awf")
	if _, err := os.Stat(filepath.Join(awfDir, "parts", "agents-doc")); !os.IsNotExist(err) {
		t.Errorf("no prose parts expected without ownership/identity, stat err = %v", err)
	}
	var ad struct {
		Data map[string]any `yaml:"data"`
	}
	readYAML(t, filepath.Join(awfDir, "agents-doc.yaml"), &ad)
	if ad.Data["extra"] != "keep" {
		t.Errorf("agents-doc.yaml data.extra = %v, want keep", ad.Data["extra"])
	}
}

func TestPortAgentsDocEmptySidecarOmitted(t *testing.T) {
	// agentsDoc with only ownership/identity yields prose parts and no sidecar.
	root := writeLegacyRoot(t, "prefix: ex\nagentsDoc:\n  data:\n    ownership: \"Own.\"\n    identity: \"Id.\"\n")
	if err := applyTreeLayout(root, &Changes{}); err != nil {
		t.Fatalf("applyTreeLayout: %v", err)
	}
	awfDir := filepath.Join(root, ".claude", "awf")
	if _, err := os.Stat(filepath.Join(awfDir, "agents-doc.yaml")); !os.IsNotExist(err) {
		t.Errorf("agents-doc.yaml should be omitted when the sidecar is empty, stat err = %v", err)
	}
	if got := readFile(t, filepath.Join(awfDir, "parts", "agents-doc", "you-and-this-project.md")); !strings.HasPrefix(got, "## You and this project\n\n") {
		t.Errorf("you-and-this-project part = %q", got)
	}
}

func TestPortAgentsDocSectionCopyError(t *testing.T) {
	// A replaceWith section pointing at an absent part surfaces a copy error
	// through portAgentsDoc (after the ownership prose part writes fine).
	root := writeLegacyRoot(t, "prefix: ex\n"+
		"agentsDoc:\n"+
		"  data:\n"+
		"    ownership: \"Own.\"\n"+
		"  sections:\n"+
		"    sec:\n"+
		"      replaceWith: parts/missing.md\n")
	err := applyTreeLayout(root, &Changes{})
	if err == nil || !strings.Contains(err.Error(), "read part") {
		t.Fatalf("applyTreeLayout(agentsDoc missing part) = %v, want a read-part error", err)
	}
}

func TestPortAgentsDocProseWriteError(t *testing.T) {
	// A directory squatting on the you-and-this-project prose part makes the
	// ownership write fail.
	root := writeLegacyRoot(t, "prefix: ex\nagentsDoc:\n  data:\n    ownership: \"Own.\"\n")
	if err := os.MkdirAll(filepath.Join(root, ".claude", "awf", "parts", "agents-doc", "you-and-this-project.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := applyTreeLayout(root, &Changes{}); err == nil {
		t.Fatal("applyTreeLayout with squatted prose part = nil, want a write error")
	}
}

func TestLegacyReadOnlyInMigrate(t *testing.T) {
	// invariant: config/migrations-and-locks:legacy-read-isolation (TestLegacyReadOnlyInMigrate)
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

// invariant: config/migrations-and-locks:awf-relocation-migration (TestAwfRelocationGatesAndMoves)
func TestAwfRelocationGatesAndMoves(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, ".claude", "awf")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "config.yaml"), []byte("prefix: awf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "awf.lock"),
		[]byte(`{"awfVersion":"0.1.0","schemaVersion":2,"files":{}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := mustGeneration(t, root); got != 2 {
		t.Fatalf("pre-relocation Generation = %d, want 2", got)
	}
	if mustGateState(t, root) != "gate" {
		t.Fatalf("expected gate state, got %q", mustGateState(t, root))
	}
	_, changes, err := Upgrade(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	wantChanges := []string{
		"awf-dir-relocation: moved .claude/awf to .awf",
		"awf-dir-relocation: moved authority lock .claude/awf/awf.lock to .awf/awf.lock",
	}
	for i, want := range wantChanges {
		if i >= len(changes) || changes[i].Text != want {
			t.Fatalf("changes = %#v, want relocation facts in order", changes)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "config.yaml")); err != nil {
		t.Fatalf("config not relocated: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("old .claude/awf not removed")
	}
	if got := mustGeneration(t, root); got != Current() {
		t.Fatalf("post-relocation Generation = %d, want %d", got, Current())
	}
}

func TestAwfRelocationNoopWhenAbsent(t *testing.T) {
	if err := applyAwfRelocation(t.TempDir(), &Changes{}); err != nil {
		t.Fatal(err)
	}
}

func TestAwfRelocationRenameFailurePublishesNoChanges(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, ".claude", "awf")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("rename failed")
	var changes Changes
	if err := applyAwfRelocationWithRename(root, &changes, func(string, string) error { return failure }); !errors.Is(err, failure) {
		t.Fatalf("error = %v, want %v", err, failure)
	}
	if got := changes.String(); got != "" {
		t.Fatalf("changes = %q, want no facts before rename succeeds", got)
	}
}

func TestAwfRelocationRefusesExistingTarget(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := applyAwfRelocation(root, &Changes{}); err == nil {
		t.Fatal("expected error when .awf already exists")
	}
}

// awfFile writes a file under <root>/.claude/awf/<rel>.
func awfFile(t *testing.T, root, rel, body string) {
	t.Helper()
	testsupport.WriteFile(t, filepath.Join(root, ".claude", "awf", rel), body)
}

// invariant: config/migrations-and-locks:hooks-config-dropped (TestDropHooksStrips)
func TestDropHooksStrips(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, ".awf", "config.yaml")
	testsupport.WriteFile(t, cfg, "prefix: ex\nhooks:\n  - pre-commit\n  - pre-push\nskills:\n  - tdd\n")
	if err := applyDropHooks(root, &Changes{}); err != nil {
		t.Fatalf("applyDropHooks: %v", err)
	}
	out, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "hooks") {
		t.Errorf("hooks key not stripped:\n%s", out)
	}
	if !strings.Contains(string(out), "prefix: ex") || !strings.Contains(string(out), "- tdd") {
		t.Errorf("untouched keys lost:\n%s", out)
	}
}

// The 3→4 migration targets the legacy hooks *array*; the modern ADR-0048
// hooks mapping ({enabled: true}) must survive a replay from a degraded lock
// (schemaVersion 0/absent) instead of being silently stripped.
func TestDropHooksKeepsModernMapping(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, ".awf", "config.yaml")
	src := "prefix: ex\nhooks:\n  enabled: true\nskills:\n  - tdd\n"
	testsupport.WriteFile(t, cfg, src)
	if err := applyDropHooks(root, &Changes{}); err != nil {
		t.Fatalf("applyDropHooks: %v", err)
	}
	out, _ := os.ReadFile(cfg)
	if string(out) != src {
		t.Errorf("modern hooks mapping stripped on replay:\n got %q\nwant %q", out, src)
	}
}

func TestDropHooksIdempotent(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, ".awf", "config.yaml")
	src := "prefix: ex\nskills:\n  - tdd\n"
	testsupport.WriteFile(t, cfg, src)
	if err := applyDropHooks(root, &Changes{}); err != nil {
		t.Fatalf("applyDropHooks: %v", err)
	}
	out, _ := os.ReadFile(cfg)
	if string(out) != src {
		t.Errorf("idempotent run changed a hooks-less config:\n got %q\nwant %q", out, src)
	}
}

func TestDropHooksAbsentConfig(t *testing.T) {
	if err := applyDropHooks(t.TempDir(), &Changes{}); err != nil {
		t.Errorf("applyDropHooks with no .awf/config.yaml should be a no-op, got %v", err)
	}
}

func TestDropHooksMalformedConfig(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "skills: [a, b\n")
	if err := applyDropHooks(root, &Changes{}); err == nil {
		t.Error("expected error surfaced from RemoveKey for malformed config.yaml")
	}
}

func TestEnableBootstrapAdds(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, ".awf", "config.yaml")
	testsupport.WriteFile(t, cfg, "prefix: ex\nskills:\n  - tdd\n")
	if err := applyEnableBootstrap(root, &Changes{}); err != nil {
		t.Fatalf("applyEnableBootstrap: %v", err)
	}
	out, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "bootstrap:") || !strings.Contains(string(out), "enabled: true") {
		t.Errorf("bootstrap.enabled not added:\n%s", out)
	}
	if !strings.Contains(string(out), "prefix: ex") || !strings.Contains(string(out), "- tdd") {
		t.Errorf("untouched keys lost:\n%s", out)
	}
}

// A config already carrying a bootstrap key made a choice - a replay from a
// degraded lock must not override a deliberate opt-out with the upgrade
// default. The genuine 4→5 path has no bootstrap key and still gets true.
func TestEnableBootstrapKeepsExplicitOptOut(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, ".awf", "config.yaml")
	src := "prefix: ex\nbootstrap:\n  enabled: false\n"
	testsupport.WriteFile(t, cfg, src)
	if err := applyEnableBootstrap(root, &Changes{}); err != nil {
		t.Fatalf("applyEnableBootstrap: %v", err)
	}
	out, _ := os.ReadFile(cfg)
	if string(out) != src {
		t.Errorf("explicit bootstrap opt-out overridden on replay:\n got %q\nwant %q", out, src)
	}
}

func TestEnableBootstrapAbsentConfig(t *testing.T) {
	if err := applyEnableBootstrap(t.TempDir(), &Changes{}); err != nil {
		t.Errorf("applyEnableBootstrap with no .awf/config.yaml should be a no-op, got %v", err)
	}
}

func TestEnableBootstrapMalformedConfig(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "skills: [a, b\n")
	if err := applyEnableBootstrap(root, &Changes{}); err == nil {
		t.Error("expected error surfaced from SetMappingScalar for malformed config.yaml")
	}
}

func TestDropReplaceWithNoop(t *testing.T) {
	root := t.TempDir()
	awfFile(t, root, "config.yaml", "prefix: ex\n")
	awfFile(t, root, "skills/a.yaml", "sections:\n  s:\n    drop: true\n")
	before, _ := os.ReadFile(filepath.Join(root, ".claude", "awf", "skills", "a.yaml"))
	if err := applyDropReplaceWith(root, &Changes{}); err != nil {
		t.Fatalf("applyDropReplaceWith: %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(root, ".claude", "awf", "skills", "a.yaml"))
	if string(before) != string(after) {
		t.Errorf("sidecar changed:\n%s\n---\n%s", before, after)
	}
}

func TestDropReplaceWithConverts(t *testing.T) {
	root := t.TempDir()
	awfFile(t, root, "config.yaml", "prefix: ex\n")
	// skills/x: replaceWith + drop + data + local → rewritten, retaining the rest.
	awfFile(t, root, "skills/x.yaml", "data:\n  k: v\nsections:\n  s:\n    replaceWith: skills/parts/x/legacy.md\n  d:\n    drop: true\nlocal: true\n")
	awfFile(t, root, "skills/parts/x/legacy.md", "LEGACY BODY\n")
	// agents/y: only a replaceWith section → sidecar becomes empty and is removed.
	awfFile(t, root, "agents/y.yaml", "sections:\n  s2:\n    replaceWith: agents/parts/y/legacy2.md\n")
	awfFile(t, root, "agents/parts/y/legacy2.md", "Y BODY\n")
	// agents-doc singleton → part lands under parts/agents-doc/.
	awfFile(t, root, "agents-doc.yaml", "sections:\n  identity:\n    replaceWith: parts/agents-doc/legacy3.md\n")
	awfFile(t, root, "parts/agents-doc/legacy3.md", "AD BODY\n")

	if err := applyDropReplaceWith(root, &Changes{}); err != nil {
		t.Fatalf("applyDropReplaceWith: %v", err)
	}
	awf := filepath.Join(root, ".claude", "awf")
	if b, _ := os.ReadFile(filepath.Join(awf, "skills", "parts", "x", "s.md")); string(b) != "LEGACY BODY\n" {
		t.Errorf("convention part s.md = %q", b)
	}
	sc, _ := os.ReadFile(filepath.Join(awf, "skills", "x.yaml"))
	if strings.Contains(string(sc), "replaceWith") {
		t.Errorf("rewritten sidecar still has replaceWith:\n%s", sc)
	}
	if !strings.Contains(string(sc), "k: v") || !strings.Contains(string(sc), "drop: true") || !strings.Contains(string(sc), "local: true") {
		t.Errorf("rewritten sidecar dropped data/drop/local:\n%s", sc)
	}
	if _, err := os.Stat(filepath.Join(awf, "agents", "y.yaml")); !os.IsNotExist(err) {
		t.Errorf("emptied agents/y.yaml should be removed, stat err = %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(awf, "agents", "parts", "y", "s2.md")); string(b) != "Y BODY\n" {
		t.Errorf("agents convention part s2.md = %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(awf, "parts", "agents-doc", "identity.md")); string(b) != "AD BODY\n" {
		t.Errorf("agents-doc convention part identity.md = %q", b)
	}
	if _, err := os.Stat(filepath.Join(awf, "agents-doc.yaml")); !os.IsNotExist(err) {
		t.Errorf("emptied agents-doc.yaml should be removed, stat err = %v", err)
	}
}

func TestDropReplaceWithRetainsCopiedEvidenceWhenSidecarRewriteFails(t *testing.T) {
	root := t.TempDir()
	awfFile(t, root, "config.yaml", "prefix: ex\n")
	awfFile(t, root, "skills/x.yaml", "sections:\n  s:\n    replaceWith: skills/parts/x/legacy.md\n")
	awfFile(t, root, "skills/parts/x/legacy.md", "BODY\n")
	failure := errors.New("rewrite sidecar")
	var changes Changes
	err := applyDropReplaceWithSidecarWrite(root, &changes, func(string, map[string]any, map[string]any, bool, bool) error {
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want %v", err, failure)
	}
	if got, want := changes.String(), "drop-replacewith: copied skills/parts/x/legacy.md to .claude/awf/skills/parts/x/s.md\n"; got != want {
		t.Fatalf("changes = %q, want %q", got, want)
	}
	if got := readFile(t, filepath.Join(root, ".claude", "awf", "skills", "parts", "x", "s.md")); got != "BODY\n" {
		t.Fatalf("moved part = %q", got)
	}
}

func TestDropReplaceWithIdempotent(t *testing.T) {
	root := t.TempDir()
	awfFile(t, root, "config.yaml", "prefix: ex\n")
	awfFile(t, root, "skills/x.yaml", "sections:\n  s:\n    replaceWith: skills/parts/x/legacy.md\n")
	awfFile(t, root, "skills/parts/x/legacy.md", "BODY\n")
	awfFile(t, root, "skills/parts/x/s.md", "BODY\n") // dst already present, identical
	var changes Changes
	if err := applyDropReplaceWith(root, &changes); err != nil {
		t.Fatalf("applyDropReplaceWith: %v", err)
	}
	if got := changes.Items(); len(got) != 1 || got[0].Text != "drop-replacewith: rewrote .claude/awf/skills/x.yaml" {
		t.Fatalf("changes = %v, want only the sidecar rewrite", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "awf", "skills", "x.yaml")); !os.IsNotExist(err) {
		t.Errorf("emptied sidecar should be removed, stat err = %v", err)
	}
}

func TestRelocatePartReportsNoCopyOnDestinationFailure(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src.md")
	testsupport.WriteFile(t, src, "BODY\n")

	destinationDir := filepath.Join(root, "destination")
	if err := os.Mkdir(destinationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if copied, err := relocatePart(src, destinationDir, writeFile); err == nil || copied {
		t.Fatalf("relocatePart directory destination = (copied=%v, err=%v), want (false, error)", copied, err)
	}

	failure := errors.New("write destination")
	if copied, err := relocatePart(src, filepath.Join(root, "missing", "destination.md"), func(string, []byte) error { return failure }); !errors.Is(err, failure) || copied {
		t.Fatalf("relocatePart failed write = (copied=%v, err=%v), want (false, %v)", copied, err, failure)
	}
}

func TestDropReplaceWithConflict(t *testing.T) {
	root := t.TempDir()
	awfFile(t, root, "config.yaml", "prefix: ex\n")
	awfFile(t, root, "skills/x.yaml", "sections:\n  s:\n    replaceWith: skills/parts/x/legacy.md\n")
	awfFile(t, root, "skills/parts/x/legacy.md", "NEW\n")
	awfFile(t, root, "skills/parts/x/s.md", "OLD DIFFERENT\n")
	err := applyDropReplaceWith(root, &Changes{})
	if err == nil || !strings.Contains(err.Error(), "already exists with different content") {
		t.Errorf("want conflict error, got: %v", err)
	}
}

func TestDropReplaceWithMissingPart(t *testing.T) {
	root := t.TempDir()
	awfFile(t, root, "config.yaml", "prefix: ex\n")
	awfFile(t, root, "skills/x.yaml", "sections:\n  s:\n    replaceWith: skills/parts/x/legacy.md\n")
	err := applyDropReplaceWith(root, &Changes{})
	if err == nil || !strings.Contains(err.Error(), "legacy.md") {
		t.Errorf("want missing-part error mentioning legacy.md, got: %v", err)
	}
}

// migration-ordering is what makes `awf upgrade` a function of the project's
// detected generation rather than of how many times anyone has run it: exactly
// the registered migrations whose target exceeds that generation run, in
// ascending target order, and a tree already at the current schema is left
// alone. Recording migrations make both halves observable - which ones ran, and
// in what order - and the real registry's ascending declaration is asserted
// here too, because Upgrade iterates it in declaration order and would silently
// apply a misdeclared registry out of target order.
// invariant: config/migrations-and-locks:migration-ordering (TestUpgradeAppliesExactlyTheMigrationsAboveTheDetectedGeneration)
func TestUpgradeAppliesExactlyTheMigrationsAboveTheDetectedGeneration(t *testing.T) {
	original := registry
	t.Cleanup(func() { registry = original })
	for i := 1; i < len(original); i++ {
		if original[i].To <= original[i-1].To {
			t.Fatalf("registry is not ascending by To: %q (to %d) follows %q (to %d)",
				original[i].Name, original[i].To, original[i-1].Name, original[i-1].To)
		}
	}

	var ran []string
	record := func(name string) func(context.Context, string, *Changes) error {
		return func(context.Context, string, *Changes) error {
			ran = append(ran, name)
			return nil
		}
	}
	registry = []Migration{
		{To: 1, Name: "first", Apply: record("first")},
		{To: 2, Name: "second", Apply: record("second")},
		{To: 3, Name: "third", Apply: record("third")},
		{To: 4, Name: "fourth", Apply: record("fourth")},
	}

	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "prefix: ex\n")
	stampLockAt(t, filepath.Join(root, ".awf", "awf.lock"), 2)

	applied, _, err := Upgrade(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	const want = "third,fourth"
	if strings.Join(applied, ",") != want || strings.Join(ran, ",") != want {
		t.Fatalf("applied=%v ran=%v, want exactly [%s] and nothing at or below generation 2", applied, ran, want)
	}

	// Upgrade stamped the lock at the current generation, so the re-run has
	// nothing above it left to apply and still exits zero.
	ran = nil
	applied, _, err = Upgrade(testContext(t), root)
	if err != nil {
		t.Fatalf("re-run at the current schema errored: %v", err)
	}
	if len(applied) != 0 || len(ran) != 0 {
		t.Fatalf("re-run at the current schema applied %v (ran %v), want nothing", applied, ran)
	}
}

// A tree→tree upgrade keeps its lock; Upgrade must restamp it to Current() so the
// terminal sync's schema gate passes.
func TestUpgradeFallbackStampsWhenHighestMigrationDoesNotOwnStamp(t *testing.T) {
	if stamped, err := stampLockSchemaWithSave(t.TempDir(), func(lock *manifest.Lock, path string) error { return lock.Save(path) }); err != nil || stamped {
		t.Fatalf("missing-lock stamp = %t, %v; want false, nil", stamped, err)
	}
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "prefix: ex\n")
	stampLockAt(t, filepath.Join(root, ".awf", "awf.lock"), Current())
	original := registry
	next := Current() + 1
	registry = append(append([]Migration(nil), registry...), Migration{To: next, Name: "test-fallback-stamp", Apply: func(context.Context, string, *Changes) error { return nil }})
	t.Cleanup(func() { registry = original })
	applied, _, err := Upgrade(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 1 || applied[0] != "test-fallback-stamp" {
		t.Fatalf("applied = %v", applied)
	}
	lock, err := manifest.Load(filepath.Join(root, ".awf", "awf.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if lock.SchemaVersion != next {
		t.Fatalf("fallback schema = %d, want %d", lock.SchemaVersion, next)
	}
}

func TestStampLockSchemaWritesExistingLock(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "prefix: ex\n")
	stampLockAt(t, filepath.Join(root, ".awf", "awf.lock"), 0)
	if stamped, err := stampLockSchemaWithSave(root, func(lock *manifest.Lock, path string) error { return lock.Save(path) }); err != nil || !stamped {
		t.Fatalf("existing-lock stamp = %t, %v; want true, nil", stamped, err)
	}
	lock, err := manifest.Load(filepath.Join(root, ".awf", "awf.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if lock.SchemaVersion != Current() {
		t.Fatalf("schema = %d, want %d", lock.SchemaVersion, Current())
	}
}

func TestUpgradeReturnsAppliedChangesWhenFallbackStampSaveFails(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "prefix: ex\n")
	stampLockAt(t, filepath.Join(root, ".awf", "awf.lock"), 0)

	originalRegistry := registry
	registry = []Migration{
		{To: 1, Name: "first", Apply: func(_ context.Context, _ string, changes *Changes) error {
			changes.Add("first: changed config")
			return nil
		}},
		{To: 2, Name: "second", Apply: func(_ context.Context, _ string, changes *Changes) error {
			changes.Add("second: changed config")
			return nil
		}},
	}
	failure := errors.New("save stamped lock")
	t.Cleanup(func() { registry = originalRegistry })

	applied, changes, err := upgradeWithStampSave(testContext(t), root, func(*manifest.Lock, string) error { return failure })
	if !errors.Is(err, failure) {
		t.Fatalf("Upgrade error = %v, want %v", err, failure)
	}
	if want := []string{"first", "second"}; !slices.Equal(applied, want) {
		t.Fatalf("applied = %v, want %v", applied, want)
	}
	wantChanges := []Change{{Text: "first: changed config"}, {Text: "second: changed config"}}
	if !slices.Equal(changes, wantChanges) {
		t.Fatalf("changes = %#v, want %#v", changes, wantChanges)
	}
}

func TestDropReplaceWithMalformedSidecar(t *testing.T) {
	root := t.TempDir()
	awfFile(t, root, "config.yaml", "prefix: ex\n")
	awfFile(t, root, "skills/x.yaml", "sections: [not-a-map\n")
	err := applyDropReplaceWith(root, &Changes{})
	if err == nil || !strings.Contains(err.Error(), "parse sidecar") {
		t.Errorf("want parse-sidecar error, got: %v", err)
	}
}

// mustGeneration / mustGateState assert the error-free path for fixtures whose
// locks are readable or absent - the pre-ADR-0076 call shape.
func mustGeneration(t *testing.T, root string) int {
	t.Helper()
	gen, err := Generation(root)
	if err != nil {
		t.Fatalf("Generation(%s): %v", root, err)
	}
	return gen
}

func mustGateState(t *testing.T, root string) string {
	t.Helper()
	state, _, err := GateState(root)
	if err != nil {
		t.Fatalf("GateState(%s): %v", root, err)
	}
	return state
}

func writeCorruptLock(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{\"awfVersion\": \"0.1"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAuthorityLockPath(t *testing.T) {
	for _, tc := range []struct{ name, marker, want string }{
		{"absent", "", ".awf/awf.lock"},
		{"current config", ".awf/config.yaml", ".awf/awf.lock"},
		{"current lock", ".awf/awf.lock", ".awf/awf.lock"},
		{"legacy", ".claude/awf.yaml", ".claude/awf.lock"},
		{"old tree", ".claude/awf/config.yaml", ".claude/awf/awf.lock"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.marker != "" {
				testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(tc.marker)), "x")
			}
			want := filepath.Join(root, filepath.FromSlash(tc.want))
			if got := AuthorityLockPath(root); got != want {
				t.Fatalf("got %s, want %s", got, want)
			}
		})
	}
}

func TestGenerationCorruptTreeLockErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(root), []byte("prefix: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCorruptLock(t, config.LockPath(root))
	if _, err := Generation(root); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("want corrupt-lock error, got %v", err)
	}
	if _, _, err := Upgrade(testContext(t), root); err == nil {
		t.Fatal("Upgrade must refuse a corrupt lock upfront")
	}
}

func TestGenerationCorruptLegacyLockErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "awf", "config.yaml"), []byte("prefix: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCorruptLock(t, filepath.Join(root, ".claude", "awf", "awf.lock"))
	if _, err := Generation(root); err == nil {
		t.Fatal("want corrupt legacy-lock error")
	}
}

func TestGenerationMissingLockSemanticsPreserved(t *testing.T) {
	// Tree + no lock → Current(); nothing present → Current(); both err-free
	// (the documented standing ambiguity, ADR-0076 Decision 2 last sentence).
	root := t.TempDir()
	if gen, err := Generation(root); err != nil || gen != Current() {
		t.Fatalf("empty root: gen=%d err=%v", gen, err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(root), []byte("prefix: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if gen, err := Generation(root); err != nil || gen != Current() {
		t.Fatalf("lockless tree: gen=%d err=%v", gen, err)
	}
}

func TestProjectPresent(t *testing.T) {
	root := t.TempDir()
	if ProjectPresent(root) {
		t.Fatal("empty root must not be present")
	}
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(root), []byte("prefix: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !ProjectPresent(root) {
		t.Fatal("tree root must be present")
	}
}

// invariant: config/migrations-and-locks:migration-ordering (TestMigrationOrderingAscendingAndIdempotent)
func TestMigrationOrderingAscendingAndIdempotent(t *testing.T) {
	// The registry is the ordering contract: Upgrade walks it in slice order and
	// skips by To, so an entry appended out of order would silently run early.
	for i := 1; i < len(registry); i++ {
		if registry[i].To <= registry[i-1].To {
			t.Fatalf("registry is not strictly ascending: entry %d targets %d after %d", i, registry[i].To, registry[i-1].To)
		}
	}

	// Selection and order, checked at points derived from the registry itself so
	// no generation number is written into this test.
	for _, idx := range []int{0, len(registry) / 2, len(registry) - 2} {
		from := registry[idx].To
		var want []string
		for _, m := range registry {
			if m.To > from {
				want = append(want, m.Name)
			}
		}
		root := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "prefix: ex\n")
		stampLockAt(t, filepath.Join(root, ".awf", "awf.lock"), from)
		applied, _, err := Upgrade(testContext(t), root)
		if err != nil {
			t.Fatalf("Upgrade from %d: %v", from, err)
		}
		if !slices.Equal(applied, want) {
			t.Errorf("Upgrade from %d applied %v, want exactly the registered migrations above it, ascending: %v", from, applied, want)
		}

		// Re-running at the schema the first run reached applies nothing and exits
		// zero, so upgrade is safe to repeat.
		again, _, err := Upgrade(testContext(t), root)
		if err != nil {
			t.Errorf("re-running Upgrade from %d: %v", from, err)
		}
		if len(again) != 0 {
			t.Errorf("re-running Upgrade from %d applied %v, want nothing", from, again)
		}
	}
}
