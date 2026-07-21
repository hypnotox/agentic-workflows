package project

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// scaffold writes a .awf/config.yaml tree under a fresh temp root.
func scaffold(t *testing.T, configYAML string) string {
	return scaffoldFiles(t, configYAML, nil)
}

// testLayout returns a complete .layout template map - every key
// Layout.templateMap produces, with canonical docs/-rooted values - so a test
// that renders a template directly doesn't need to hand-pick which keys that
// template happens to reference today. A future Layout field addition only
// needs updating here, not at every hand-built fixture across the package.
func testLayout() map[string]any {
	return map[string]any{
		"docsDir":          "docs",
		"adrDir":           "docs/decisions",
		"indexMd":          "docs/decisions/INDEX.md",
		"adrReadme":        "docs/decisions/README.md",
		"adrTemplate":      "docs/decisions/template.md",
		"plansDir":         "docs/plans",
		"plansReadme":      "docs/plans/README.md",
		"plansTemplate":    "docs/plans/template.md",
		"docs":             map[string]any{},
		"workflowRef":      "docs/workflow.md",
		"docStandard":      "docs/doc-standard.md",
		"agentsMdStandard": "docs/agents-md-standard.md",
		"workingWithAwf":   "docs/working-with-awf.md",
		"configReference":  "docs/config-reference.md",
		"domainsDir":       "docs/domains",
	}
}

// scaffoldFiles writes config.yaml plus optional sidecar/part files keyed by path
// relative to .awf/ (e.g. "skills/tdd.yaml", "skills/parts/x/y.md").
func scaffoldFiles(t *testing.T, configYAML string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, configYAML)
	for rel, body := range files {
		testsupport.WriteFile(t, filepath.Join(root, ".awf", rel), body)
	}
	return root
}

// lockFile is the relocated lock path under the tree.
func lockFile(root string) string {
	return filepath.Join(root, ".awf", "awf.lock")
}

// configPath is the tree config file path.
func configPath(root string) string {
	return filepath.Join(root, ".awf", "config.yaml")
}

const sampleYAML = `prefix: example
vars:
  testCmd: go test ./...
  gateCmd: make gate
  gateCmdFull: make gate full
skills:
  - tdd
agents:
  - code-reviewer
`

func TestInitializeAndSyncAuthorityRefusals(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.SyncReport(); err == nil || !strings.Contains(err.Error(), "pre-tracking") {
		t.Fatalf("missing-lock sync error=%v", err)
	}
	if _, _, _, err := p.InitializeReport(InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.InitializeReport(InitAuthority{InitializedWithVersion: Version}); err == nil || !strings.Contains(err.Error(), "absent lock") {
		t.Fatalf("repeat initialize error=%v", err)
	}
	lock := &manifest.Lock{AWFVersion: Version, SchemaVersion: 14, Files: map[string]manifest.Entry{}}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.SyncReport(); err == nil || !strings.Contains(err.Error(), "permanent") {
		t.Fatalf("pre-tracking sync error=%v", err)
	}
}

func TestSyncPreservesPermanentCurrentStateCutoff(t *testing.T) {
	for _, initializedWithVersion := range []string{"0.18.0", ""} {
		name := "initialized"
		if initializedWithVersion == "" {
			name = "migrated"
		}
		t.Run(name, func(t *testing.T) {
			root := scaffold(t, sampleYAML)
			prior := &manifest.Lock{
				AWFVersion:             "0.18.0",
				SchemaVersion:          14,
				Files:                  map[string]manifest.Entry{},
				ADRFormatV1From:        137,
				ADRFormatV2From:        200,
				LegacyADRGaps:          []int{2, 9},
				InitializedWithVersion: initializedWithVersion,
			}
			raw, err := prior.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(lockFile(root), raw, 0o644); err != nil {
				t.Fatal(err)
			}
			p, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := p.Sync(); err != nil {
				t.Fatal(err)
			}
			got, err := manifest.Load(lockFile(root))
			if err != nil {
				t.Fatal(err)
			}
			if got.InitializedWithVersion != initializedWithVersion || got.ADRFormatV1From != 137 || got.ADRFormatV2From != 200 || !slices.Equal(got.LegacyADRGaps, []int{2, 9}) {
				t.Fatalf("permanent current-state authority was not preserved: initialized=%q cutoffs=%d/%d gaps=%v", got.InitializedWithVersion, got.ADRFormatV1From, got.ADRFormatV2From, got.LegacyADRGaps)
			}
		})
	}
}

func TestSyncWritesFilesAndLock(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")
	b, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("skill not written: %v", err)
	}
	if !strings.Contains(string(b), "# example-tdd") || strings.Contains(string(b), "awf:section") {
		t.Errorf("rendered skill wrong:\n%s", b)
	}
	if !strings.Contains(string(b), "GENERATED by awf") || !strings.Contains(string(b), "<!-- awf:edit ") {
		t.Errorf("rendered skill missing provenance banner/pointer:\n%s", b)
	}
	for _, rel := range []string{".claude/agents/code-reviewer.md", ".awf/awf.lock"} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
}

// invariant: rendering/project-output-plan:target-prune-ancestors
func TestSyncPrunesRemovedTargetTree(t *testing.T) {
	root := scaffold(t, sampleYAML+"targets:\n  - claude\n  - cursor\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".cursor/skills/example-tdd/SKILL.md")); err != nil {
		t.Fatalf("expected cursor skill rendered on first sync: %v", err)
	}
	// Drop the cursor target (sampleYAML has no targets: key → defaults to claude).
	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(sampleYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	p2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p2.Sync(); err != nil {
		t.Fatal(err)
	}
	// The whole .cursor/ tree is gone - every empty ancestor, not just the leaf parent.
	for _, dir := range []string{".cursor/skills/example-tdd", ".cursor/skills", ".cursor/agents", ".cursor"} {
		if _, err := os.Stat(filepath.Join(root, dir)); !os.IsNotExist(err) {
			t.Errorf("expected %s removed, stat err = %v", dir, err)
		}
	}
}

func TestCheckCleanAfterSync(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(root)
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 0 {
		t.Errorf("expected clean, got drift: %#v", drift)
	}
}

func TestCheckDetectsHandEdit(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(root)
	_ = p.Sync()
	skill := filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")
	_ = os.WriteFile(skill, []byte("hand edited\n"), 0o644)
	drift, _ := p.Check()
	if len(drift) == 0 || drift[0].Kind != "hand-edited" {
		t.Errorf("expected hand-edited drift, got %#v", drift)
	}
}

func TestCheckStaleTakesPrecedence(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(root)
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	skillPath := ".claude/skills/example-tdd/SKILL.md"
	// Make the lock entry stale by corrupting its TemplateHash.
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	e := lock.Files[skillPath]
	e.TemplateHash = "sha256:bogus"
	lock.Files[skillPath] = e
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	// Also hand-edit the rendered file so its on-disk content differs too.
	if err := os.WriteFile(filepath.Join(root, skillPath), []byte("hand edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	var forPath []manifest.Drift
	for _, d := range drift {
		if d.Path == skillPath {
			forPath = append(forPath, d)
		}
	}
	if len(forPath) != 1 {
		t.Fatalf("expected exactly one drift entry for %s, got %#v", skillPath, forPath)
	}
	if forPath[0].Kind != "stale" {
		t.Errorf("expected stale, got %q", forPath[0].Kind)
	}
}

func TestSyncSkipsLocalSkill(t *testing.T) {
	cfg := `prefix: example
vars:
  testCmd: go test ./...
  gateCmd: make gate
skills:
  - adding-thing
  - tdd
agents:
  - code-reviewer
`
	root := scaffoldFiles(t, cfg, map[string]string{
		"skills/adding-thing.yaml": "local: true\n",
	})
	// A local skill is hand-authored; provide its on-disk file with valid frontmatter.
	localPath := ".claude/skills/example-adding-thing/SKILL.md"
	writeLocalSkill(t, root, localPath)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lock.Files[localPath]; ok {
		t.Errorf("local skill should be absent from lock")
	}
	if _, ok := lock.Files[".claude/skills/example-tdd/SKILL.md"]; !ok {
		t.Errorf("tdd skill should still be present in lock")
	}
}

// TestSyncKeepsLocalConvertedSkill guards the managed→local prune bug: converting
// a previously-managed skill to local must not delete its on-disk file. RenderAll
// skips local artifacts, so without protection Sync's prune treats the (now hand-
// authored) file as a stale managed output and removes it - breaking every later
// sync/check with "local skill file absent".
func TestSyncKeepsLocalConvertedSkill(t *testing.T) {
	cfg := `prefix: example
vars:
  testCmd: go test ./...
  gateCmd: make gate
skills:
  - tdd
agents:
  - code-reviewer
`
	root := scaffoldFiles(t, cfg, nil)
	// First sync renders tdd as a managed skill and locks its output path.
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("managed skill not rendered: %v", err)
	}
	// Convert tdd to local: its rendered file becomes the hand-authored local one.
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "skills", "tdd.yaml"), "local: true\n")
	p, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("sync after local conversion: %v", err)
	}
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("local-converted skill file was pruned: %v", err)
	}
	// The follow-up sync must still find the local file (the regression symptom).
	if err := p.Sync(); err != nil {
		t.Fatalf("second sync after local conversion: %v", err)
	}
}

// writeLocalSkill writes a hand-authored local skill file with valid frontmatter.
func writeLocalSkill(t *testing.T, root, rel string) {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: local-skill\ndescription: a hand-authored local skill\n---\nbody\n"
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOpenRejectsUnknownAgent(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: [does-not-exist]\n")
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error should mention the offending agent name, got: %v", err)
	}
}

func TestOpenRejectsUnknownDoc(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ndocs: [nonexistent]\n")
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected error for unknown doc")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention the offending doc name, got: %v", err)
	}
}

func TestSyncRendersDeclaredDoc(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ndocs: [architecture]\n")
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "docs", "architecture.md"))
	if err != nil {
		t.Fatalf("docs/architecture.md not written: %v", err)
	}
	if !strings.Contains(string(b), "# Architecture") {
		t.Errorf("docs/architecture.md missing heading:\n%s", b)
	}
}

// TestSyncAutoLinksDocsInAgentsDoc covers the project-level wiring that the
// template golden cannot: RenderAll injects resolvedDocs() into the agents-doc
// data map so the Document map auto-links every declared (non-local) doc with
// its catalog title/desc. A local doc must not appear.
func TestSyncAutoLinksDocsInAgentsDoc(t *testing.T) {
	cfg := `prefix: example
vars:
  gateCmd: ""
skills: []
agents: []
docs:
  - architecture
  - glossary
`
	root := scaffoldFiles(t, cfg, map[string]string{
		"docs/glossary.yaml": "local: true\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md not written: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, "[docs/architecture.md](docs/architecture.md)") {
		t.Errorf("Document map should auto-link the declared architecture doc:\n%s", got)
	}
	if !strings.Contains(got, "system shape, packages, key components, dependencies") {
		t.Errorf("Document map should carry the catalog desc for architecture:\n%s", got)
	}
	if strings.Contains(got, "docs/glossary.md") {
		t.Errorf("local doc must not appear in the Document map:\n%s", got)
	}
}

func TestOpenRejectsUnknownSkill(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: [no-such-skill]\nagents: []\n")
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
	if !strings.Contains(err.Error(), "no-such-skill") {
		t.Errorf("error should mention the offending skill name, got: %v", err)
	}
}

func TestOpenValidConfigSucceeds(t *testing.T) {
	root := scaffold(t, sampleYAML)
	_, err := Open(root)
	if err != nil {
		t.Fatalf("expected valid config to open cleanly, got: %v", err)
	}
}

func TestOpenAllowsLocalSkillNotInCatalog(t *testing.T) {
	cfg := strings.Replace(sampleYAML, "skills:\n  - tdd\n", "skills:\n  - totally-unknown-local\n  - tdd\n", 1)
	root := scaffoldFiles(t, cfg, map[string]string{
		"skills/totally-unknown-local.yaml": "local: true\n",
	})
	_, err := Open(root)
	if err != nil {
		t.Fatalf("local skill not in catalog should be allowed, got: %v", err)
	}
}

func TestSyncPrunesRemovedSkill(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(root)
	_ = p.Sync()
	// Rewrite config without the tdd skill, re-open, re-sync.
	noTDD := strings.Replace(sampleYAML, "skills:\n  - tdd\n", "skills: []\n", 1)
	_ = os.WriteFile(configPath(root), []byte(noTDD), 0o644)
	p2, _ := Open(root)
	_, _, pruned, err := p2.SyncReport()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")); !os.IsNotExist(err) {
		t.Errorf("removed skill should be pruned")
	}
	if !slices.Contains(pruned, ".claude/skills/example-tdd/SKILL.md") {
		t.Errorf("pruned report missing the removed skill: %v", pruned)
	}
	if !slices.IsSorted(pruned) {
		t.Errorf("pruned report must be sorted for deterministic output: %v", pruned)
	}
}

func TestSyncPruneReportSkipsAlreadyGoneFile(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(root)
	_ = p.Sync()
	// Hand-delete the rendered file before the pruning sync: the report must
	// not claim a removal the prune did not perform.
	if err := os.Remove(filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")); err != nil {
		t.Fatal(err)
	}
	noTDD := strings.Replace(sampleYAML, "skills:\n  - tdd\n", "skills: []\n", 1)
	_ = os.WriteFile(configPath(root), []byte(noTDD), 0o644)
	p2, _ := Open(root)
	_, _, pruned, err := p2.SyncReport()
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(pruned, ".claude/skills/example-tdd/SKILL.md") {
		t.Errorf("already-gone file must not be reported pruned: %v", pruned)
	}
}

// TestSyncReportClassifiesChangedOutput stages every provenance cause by
// authoring the prior lock directly - the classification compares the old
// entry against the fresh render, so a tweaked stored hash simulates the
// corresponding real change (an upstream template edit, a config edit, a
// non-hashed input) without mutating the embedded templates.
func TestSyncReportClassifiesChangedOutput(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(root)
	_, changes, _, err := p.InitializeReport(InitAuthority{InitializedWithVersion: Version})
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Errorf("first sync has no baseline and must report no changes, got %v", changes)
	}
	lock, err := manifest.Load(p.lockPath())
	if err != nil {
		t.Fatal(err)
	}
	mutate := func(path string, f func(e *manifest.Entry)) {
		t.Helper()
		e, ok := lock.Files[path]
		if !ok {
			t.Fatalf("no lock entry for %s; have %v", path, slices.Sorted(maps.Keys(lock.Files)))
		}
		f(&e)
		lock.Files[path] = e
	}
	// Output moved + template hash moved → upstream churn.
	mutate("AGENTS.md", func(e *manifest.Entry) { e.OutputHash = "x"; e.TemplateHash = "x" })
	// Output moved + config hash moved → the project's own inputs.
	mutate(".claude/skills/example-tdd/SKILL.md", func(e *manifest.Entry) { e.OutputHash = "x"; e.ConfigHash = "x" })
	// Both hashes moved.
	mutate("CLAUDE.md", func(e *manifest.Entry) { e.OutputHash = "x"; e.TemplateHash = "x"; e.ConfigHash = "x" })
	// Output moved, real hashes unmoved → a non-hashed input.
	mutate(".awf/memory/.gitignore", func(e *manifest.Entry) { e.OutputHash = "x" })
	// Output moved on a generated index (no hashes by design) → regenerated.
	mutate("docs/decisions/INDEX.md", func(e *manifest.Entry) { e.OutputHash = "x" })
	// No prior entry → added.
	delete(lock.Files, "docs/workflow.md")
	if err := lock.Save(p.lockPath()); err != nil {
		t.Fatal(err)
	}
	p2, _ := Open(root)
	_, changes, _, err = p2.SyncReport()
	if err != nil {
		t.Fatal(err)
	}
	want := []Change{
		{Path: ".awf/memory/.gitignore", Cause: "internal"},
		{Path: ".claude/skills/example-tdd/SKILL.md", Cause: "config"},
		{Path: "AGENTS.md", Cause: "template"},
		{Path: "CLAUDE.md", Cause: "template+config"},
		{Path: "docs/decisions/INDEX.md", Cause: "regenerated"},
		{Path: "docs/workflow.md", Cause: "added"},
	}
	if !slices.Equal(changes, want) {
		t.Errorf("changes = %v\nwant %v (path-sorted; untouched files silent)", changes, want)
	}
}

func TestOpenRejectsUnknownSectionOverride(t *testing.T) {
	// tdd in the catalog has sections [surfaces, notes]; "bogus" is not declared.
	cfg := "prefix: example\nskills: [tdd]\nagents: [code-reviewer]\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"skills/tdd.yaml": "sections:\n  bogus:\n    drop: true\n",
	})
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected error for unknown section override 'bogus'")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention 'bogus', got: %v", err)
	}
	// The label carries the artifact name for a named artifact (name != ""), so
	// the message identifies which skill; assert it so that branch is pinned.
	if !strings.Contains(err.Error(), `"tdd"`) {
		t.Errorf("error should name the offending skill \"tdd\", got: %v", err)
	}
}

func TestOpenAllowsValidSectionOverride(t *testing.T) {
	// "notes" is a declared section for tdd.
	cfg := "prefix: example\nskills: [tdd]\nagents: [code-reviewer]\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"skills/tdd.yaml": "sections:\n  notes:\n    drop: true\n",
	})
	_, err := Open(root)
	if err != nil {
		t.Fatalf("valid section override 'notes' should succeed, got: %v", err)
	}
}

func TestOpenAllowsLocalAgentNotInCatalog(t *testing.T) {
	cfg := "prefix: example\nskills: []\nagents: [my-custom-agent]\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"agents/my-custom-agent.yaml": "local: true\n",
	})
	_, err := Open(root)
	if err != nil {
		t.Fatalf("local agent not in catalog should be allowed, got: %v", err)
	}
}

func TestOpenRejectsUnknownAgentSectionOverride(t *testing.T) {
	// code-reviewer in the catalog has sections universal-lenses/project-focus/doc-currency.
	cfg := "prefix: example\nskills: []\nagents: [code-reviewer]\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"agents/code-reviewer.yaml": "sections:\n  bogus:\n    drop: true\n",
	})
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected error for unknown agent section override 'bogus'")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention 'bogus', got: %v", err)
	}
}

func TestSyncRendersAgentFromMap(t *testing.T) {
	root := scaffold(t, "prefix: myproject\nagents: [code-reviewer]\nskills: []\n")
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	agentPath := filepath.Join(root, ".claude/agents/code-reviewer.md")
	b, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("agent file not written: %v", err)
	}
	if !strings.Contains(string(b), "myproject") {
		t.Errorf("agent file should contain prefix 'myproject', got:\n%s", b)
	}
}

// TestSyncErrorsOnUnresolvedValueToken verifies the publication-safety net:
// Sync errors when rendered output contains the literal unresolved-value token.
// Since ADR-0045 every shipped var interpolation degrades gracefully, so the
// trigger here is content that carries the token itself (the ADR-0011/ADR-0014
// gotcha: prose containing the literal token trips the guard).
func TestSyncErrorsOnUnresolvedValueToken(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nvars: {}\nskills: [tdd]\nagents: []\n",
		map[string]string{
			"skills/tdd.yaml": "data:\n  testSurfaces:\n    - {name: \"<no value>\", kind: k, location: l}\n",
		})
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	err = p.Sync()
	if err == nil {
		t.Fatal("expected Sync to return an error on an unresolved-value token, got nil")
	}
	if !strings.Contains(err.Error(), "<no value>") {
		t.Errorf("error should mention \"<no value>\", got: %v", err)
	}
}

func TestSyncRendersAgentsDoc(t *testing.T) {
	t.Run("always-on by default", func(t *testing.T) {
		root := scaffold(t, "prefix: example\nvars:\n  testCmd: go test ./...\n  gateCmd: make gate\nskills: []\nagents: []\n")
		p, err := Open(root)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if err := p.Sync(); err != nil {
			t.Fatalf("Sync: %v", err)
		}
		b, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil {
			t.Fatalf("AGENTS.md not written: %v", err)
		}
		if !strings.Contains(string(b), "example") {
			t.Errorf("AGENTS.md should contain prefix 'example', got:\n%s", b)
		}
	})

	t.Run("a local agents-doc sidecar suppresses it", func(t *testing.T) {
		root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\n", map[string]string{
			"agents-doc.yaml": "local: true\n",
		})
		p, err := Open(root)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if err := p.Sync(); err != nil {
			t.Fatalf("Sync: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
			t.Errorf("AGENTS.md should not exist when agents-doc is local")
		}
	})
}

// TestSyncPrunesEmptySkillDir verifies that after a skill is removed from config
// and Sync runs again, both the SKILL.md file and its now-empty parent directory
// are removed.
func TestSyncPrunesEmptySkillDir(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	skillDir := filepath.Join(root, ".claude/skills/example-tdd")
	if _, err := os.Stat(skillDir); err != nil {
		t.Fatalf("skill dir should exist after first sync: %v", err)
	}

	noTDD := strings.Replace(sampleYAML, "skills:\n  - tdd\n", "skills: []\n", 1)
	if err := os.WriteFile(configPath(root), []byte(noTDD), 0o644); err != nil {
		t.Fatalf("rewrite config: %v", err)
	}
	p2, err := Open(root)
	if err != nil {
		t.Fatalf("Open after removing tdd skill: %v", err)
	}
	if err := p2.Sync(); err != nil {
		t.Fatalf("second Sync: %v", err)
	}

	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); !os.IsNotExist(err) {
		t.Errorf("SKILL.md should have been pruned")
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Errorf("skill directory %s should have been pruned when empty", skillDir)
	}
}

// invariant: rendering/project-output-plan:layout-derivation
func TestLayoutDerivesFromDocsDir(t *testing.T) {
	p := &Project{Cfg: &config.Config{DocsDir: "documentation", Docs: []string{"architecture"}}}
	l := p.layout()
	if l.DocsDir != "documentation" || l.ADRDir != "documentation/decisions" ||
		l.IndexMd != "documentation/decisions/INDEX.md" || l.PlansDir != "documentation/plans" {
		t.Errorf("layout = %+v", l)
	}
	// invariant: rendering/project-output-plan:domains-dir-given
	if l.DomainsDir != "documentation/domains" {
		t.Errorf("domainsDir = %q", l.DomainsDir)
	}
	// invariant: rendering/project-output-plan:layout-docs-enabled-only
	wantDocs := map[string]string{
		"architecture": "documentation/architecture.md",
	}
	if !reflect.DeepEqual(l.Docs, wantDocs) {
		t.Errorf("Docs = %v, want %v", l.Docs, wantDocs)
	}
	// templateMap reproduces the historical .layout map by literal value (ConfigHash
	// stability). The fixed directory keys are hand-built; the mandatory-singleton
	// keys derive from the catalog (ADR-0061) - assert each one's exact value so a
	// wrong derivation is caught, not just a present key.
	tm := l.templateMap()
	wantTM := map[string]string{
		"docsDir":          "documentation",
		"adrDir":           "documentation/decisions",
		"indexMd":          "documentation/decisions/INDEX.md",
		"plansDir":         "documentation/plans",
		"domainsDir":       "documentation/domains",
		"adrReadme":        "documentation/decisions/README.md",
		"adrTemplate":      "documentation/decisions/template.md",
		"plansReadme":      "documentation/plans/README.md",
		"plansTemplate":    "documentation/plans/template.md",
		"workflowRef":      "documentation/workflow.md",
		"docStandard":      "documentation/doc-standard.md",
		"agentsMdStandard": "documentation/agents-md-standard.md",
		"workingWithAwf":   "documentation/working-with-awf.md",
	}
	for k, want := range wantTM {
		if tm[k] != want {
			t.Errorf("templateMap[%q] = %v, want %q", k, tm[k], want)
		}
	}
	if got, ok := tm["docs"].(map[string]any); !ok || got["architecture"] != "documentation/architecture.md" {
		t.Errorf("templateMap[docs] = %v", tm["docs"])
	}
	// 5 fixed dir keys + docs + 9 mandatory-singleton keys = 15 (agents-doc has
	// no TemplateKey and is excluded; the generated config reference is
	// layout-exposed like its hash-checked siblings).
	if len(tm) != 15 {
		t.Errorf("templateMap has %d keys, want 15", len(tm))
	}
	if got := p.docOutPath("architecture"); got != "documentation/architecture.md" {
		t.Errorf("docOutPath = %q", got)
	}
}

// A doc-gated skill always renders while enabled: the ADR-0013 render-time
// suppression is gone (ADR-0081 Decision 7) - the doc-less state is refused
// at Open instead (TestOpenRefusesUnclosedEnabledSet), so enabled means
// rendered even when the doc is dropped post-Open.
func TestRenderAllRendersEnabledDocGatedSkill(t *testing.T) {
	cfg := "prefix: example\nskills: [roadmap-graduation]\ndocs: [roadmap]\nagents: []\n"
	root := scaffoldFiles(t, cfg, map[string]string{"agents-doc.yaml": "local: true\n"})
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	const rel = ".claude/skills/example-roadmap-graduation/SKILL.md"
	rendered := func() bool {
		files, err := p.RenderAll()
		if err != nil {
			t.Fatalf("RenderAll: %v", err)
		}
		for _, f := range files {
			if f.Path == rel {
				return true
			}
		}
		return false
	}
	if !rendered() {
		t.Error("roadmap-graduation should render when the roadmap doc is enabled")
	}
	// The doc-less state (unreachable through Open) no longer silently
	// suppresses: the publication-safety net fails the render loudly.
	p.Cfg.Docs = nil
	if _, err := p.RenderAll(); err == nil || !strings.Contains(err.Error(), "<no value>") {
		t.Errorf("fabricated doc-less state should fail the publication-safety net, got %v", err)
	}
}

// invariant: rendering/project-output-plan:sync-always-writes-active-md
// invariant: rendering/project-output-plan:check-active-md-stale
func TestSyncGeneratesActiveMDAndCheckDetectsStaleness(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\n")
	adrDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	adrBody := testsupport.ADR("Accepted", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
		testsupport.WithTitle("0001: First"), testsupport.WithBody("## Context\nx\n## Decision\n\n1. x.\n"))
	testsupport.WriteFile(t, filepath.Join(adrDir, "0001-first.md"), adrBody)

	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	index, err := os.ReadFile(filepath.Join(adrDir, "INDEX.md"))
	if err != nil {
		t.Fatalf("INDEX.md not generated: %v", err)
	}
	// The Accepted ADR renders under the In flight status section.
	inflightPos := strings.Index(string(index), "## In flight")
	entryPos := strings.Index(string(index), "ADR-0001: First")
	if inflightPos < 0 || !strings.Contains(string(index), "## History") {
		t.Errorf("INDEX.md missing status sections:\n%s", index)
	}
	if entryPos < 0 || entryPos < inflightPos {
		t.Errorf("INDEX.md missing the ADR entry under In flight:\n%s", index)
	}
	if drift, err := p.Check(); err != nil || len(drift) != 0 {
		t.Fatalf("expected clean check after sync, got drift=%#v err=%v", drift, err)
	}

	// Change frontmatter status (Accepted In flight -> Implemented History); the
	// on-disk INDEX.md is now stale.
	adr2 := strings.Replace(adrBody, "status: Accepted", "status: Implemented", 1)
	testsupport.WriteFile(t, filepath.Join(adrDir, "0001-first.md"), adr2)
	drift, err := p.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	found := false
	for _, d := range drift {
		if strings.HasSuffix(d.Path, "decisions/INDEX.md") && d.Kind == "stale" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected stale drift for INDEX.md, got %#v", drift)
	}
}

// invariant: rendering/project-output-plan:sync-always-writes-active-md
func TestSyncRendersPlaceholderIndexMDWithoutADRs(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\n")
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "docs", "decisions", "INDEX.md"))
	if err != nil {
		t.Fatalf("expected a placeholder INDEX.md when no ADRs exist: %v", err)
	}
	if !strings.Contains(string(got), "No decisions recorded yet") {
		t.Errorf("expected placeholder index, got:\n%s", got)
	}
	if drift, err := p.Check(); err != nil || len(drift) != 0 {
		t.Errorf("expected clean check with no ADRs, got drift=%#v err=%v", drift, err)
	}
}

// invariant: rendering/project-output-plan:check-invalid-frontmatter
func TestCheckDetectsInvalidFrontmatter(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(root)
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	skillPath := ".claude/skills/example-tdd/SKILL.md"
	broken := "---\nname: \"\"\ndescription: \"\"\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(root, skillPath), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	// Re-point the lock OutputHash to the edited content so the file is "in sync"
	// by hash and the frontmatter check is what fires.
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	e := lock.Files[skillPath]
	e.OutputHash = manifest.Hash([]byte(broken))
	lock.Files[skillPath] = e
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range drift {
		if d.Path == skillPath && d.Kind == "invalid-frontmatter" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid-frontmatter drift for %s, got %#v", skillPath, drift)
	}
}

// invariant: rendering/project-output-plan:adr-system-singletons-rendered
// invariant: rendering/project-output-plan:plain-singleton-via-renderkind
// invariant: rendering/project-output-plan:working-with-awf-mandatory
func TestAdrSingletonsRenderedAndSuppressible(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"docs/decisions/README.md":   false,
		"docs/decisions/template.md": false,
		"docs/plans/README.md":       false,
		"docs/workflow.md":           false,
		"docs/doc-standard.md":       false,
		"docs/agents-md-standard.md": false,
		"docs/working-with-awf.md":   false,
	}
	for _, f := range files {
		if _, ok := want[f.Path]; ok {
			want[f.Path] = true
		}
	}
	for path, seen := range want {
		if !seen {
			t.Errorf("%s not rendered", path)
		}
	}
	// local: true suppresses the README singleton.
	if err := os.WriteFile(filepath.Join(root, ".awf", "adr-readme.yaml"), []byte("local: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// local: true also suppresses a newly-mandatory singleton, matching the other four (ADR-0043
	// Decision item 1: "not togglable" keeps the local: true escape hatch).
	if err := os.WriteFile(filepath.Join(root, ".awf", "workflow.yaml"), []byte("local: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "working-with-awf.yaml"), []byte("local: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files2, err := p2.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files2 {
		if f.Path == "docs/decisions/README.md" {
			t.Error("README should be suppressed by local: true")
		}
		if f.Path == "docs/workflow.md" {
			t.Error("workflow.md should be suppressed by local: true")
		}
		if f.Path == "docs/working-with-awf.md" {
			t.Error("working-with-awf.md should be suppressed by local: true")
		}
	}
}

func TestSyncReportBacksUpForeignIndexNotManaged(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	lay := p.layout()
	// Plant a foreign ADR index with hand content before the first sync (no lock yet),
	// so its path is absent from the prior lock and therefore foreign.
	foreign := filepath.Join(root, lay.IndexMd)
	if err := os.MkdirAll(filepath.Dir(foreign), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreign, []byte("hand index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	backups, _, _, err := p.InitializeReport(InitAuthority{InitializedWithVersion: Version})
	if err != nil {
		t.Fatalf("InitializeReport: %v", err)
	}
	var got *Backup
	for i := range backups {
		if backups[i].Path == lay.IndexMd {
			got = &backups[i]
		}
	}
	// invariant: rendering/project-output-plan:sync-backs-up-foreign
	if got == nil {
		t.Fatalf("foreign INDEX.md not backed up; backups=%#v", backups)
	}
	if !got.Index {
		t.Errorf("INDEX.md backup must be flagged Index=true")
	}
	if b, _ := os.ReadFile(filepath.Join(root, got.Bak)); string(b) != "hand index\n" {
		t.Errorf("backup = %q, want original hand content", b)
	}
	// A path recorded in the prior lock is awf-managed: a second sync backs up
	// nothing and prunes nothing.
	again, _, pruned, err := p.SyncReport()
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Errorf("re-sync of awf-managed output must not back up, got %#v", again)
	}
	if len(pruned) != 0 {
		t.Errorf("re-sync of awf-managed output must not prune, got %v", pruned)
	}
}

// The generated indexes carry RegenChecked=true (drift checked by regeneration,
// not the frozen OutputHash); an ordinary rendered file carries false. This is the
// single source of truth that replaced the hardcoded index-path literals.
func TestRegenCheckedAttribute(t *testing.T) {
	root := scaffold(t, domainCfg)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// invariant: rendering/project-output-plan:regeneration-checked-attribute
	amd, err := p.generateIndexMD()
	if err != nil {
		t.Fatal(err)
	}
	if !amd.RegenChecked {
		t.Errorf("INDEX.md must be regeneration-checked")
	}
	dds, err := p.generateDomainDocs()
	if err != nil {
		t.Fatal(err)
	}
	if len(dds) == 0 {
		t.Fatal("fixture must declare at least one domain")
	}
	for _, dd := range dds {
		if !dd.RegenChecked {
			t.Errorf("domain doc %s must be regeneration-checked", dd.Path)
		}
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	cref, ok, err := p.generateConfigReference(slices.Concat(files, dds))
	if err != nil {
		t.Fatal(err)
	}
	if !ok || !cref.RegenChecked {
		t.Errorf("config reference must be regeneration-checked (ok=%v)", ok)
	}
	// Ordinary planned writes are frozen-OutputHash-checked; generated plan
	// nodes are explicitly regeneration-checked.
	if len(files) == 0 {
		t.Fatal("RenderAll produced no files")
	}
	for _, f := range files {
		if f.Policy.Regenerate != f.RegenChecked {
			t.Errorf("plan policy and RegenChecked disagree for %s", f.Path)
		}
	}
}

// invariant: rendering/templates:document-map-lists-mandatory-docs
func TestAgentsDocDocumentMapListsMandatorySingletonsUnconditionally(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ndocs: []\n")
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md not written: %v", err)
	}
	got := string(b)
	// Iterate the catalog's DocumentMap entries (default docsDir "docs") so a new
	// mandatory document-map doc cannot silently stop being covered (ADR-0061).
	mapped := 0
	for name, e := range catalog.Standard.Docs {
		if !e.DocumentMap {
			continue
		}
		mapped++
		// Assert the whole rendered line - title, link, and catalog desc - so a
		// mandatory doc is cited with its data-driven title/desc, not just linked.
		line := fmt.Sprintf("- **%s:** [docs/%s](docs/%s), %s", e.Title, e.Path, e.Path, e.Desc)
		if !strings.Contains(got, line) {
			t.Errorf("Document map should unconditionally cite %q (%s; docs: array is empty):\n%s", line, name, got)
		}
	}
	if mapped != 5 {
		t.Errorf("expected 5 DocumentMap entries, iterated %d", mapped)
	}
}

// A reviewing skill enabled without its dispatched agent fails project open -
// the error names both sides and the fix. The fixture carries reviewing-impl's
// skill closure so the agent edge is the failing one (ADR-0050, generalized by
// ADR-0081's closure validation).
// invariant: rendering/project-output-plan:reviewing-skill-agent-pairing
func TestOpenRejectsPairedSkillWithoutAgent(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: [reviewing-impl, executing-plans, retrospective, subagent-driven-development]\nagents: []\n")
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected pairing error for reviewing-impl without code-reviewer")
	}
	want := `skill "reviewing-impl" requires agent "code-reviewer"; add it to agents: in .awf/config.yaml (or run ` + "`awf upgrade`" + ` after a binary upgrade), or remove the skill`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestOpenAllowsPairedSkillWithAgent(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: [reviewing-impl, executing-plans, retrospective, subagent-driven-development]\nagents: [code-reviewer]\n")
	if _, err := Open(root); err != nil {
		t.Fatalf("paired skill with its agent must open cleanly, got: %v", err)
	}
}

// Every enabled, non-local artifact's direct catalog requirements must be
// enabled - a violation fails open with a repair hint (ADR-0081 Decision 3).
// invariant: rendering/catalog-and-targets:enabled-set-closed
func TestOpenRefusesUnclosedEnabledSet(t *testing.T) {
	cases := []struct {
		name, cfg, wantSub string
	}{
		{"missing skill requirement",
			"prefix: example\nskills: [brainstorming]\nagents: []\n",
			`skill "brainstorming" requires skill "exploring"`},
		{"missing doc requirement",
			"prefix: example\nskills: [roadmap-graduation]\nagents: []\n",
			`skill "roadmap-graduation" requires doc "roadmap"`},
		{"agent's skill requirement",
			"prefix: example\nskills: []\nagents: [plan-reviewer]\n",
			`agent "plan-reviewer" requires skill "reviewing-plan-resync"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Open(scaffold(t, tc.cfg))
			if err == nil {
				t.Fatal("expected closure-validation error")
			}
			if !strings.Contains(err.Error(), tc.wantSub) || !strings.Contains(err.Error(), "awf upgrade") {
				t.Errorf("error = %q, want it to contain %q and the awf upgrade hint", err.Error(), tc.wantSub)
			}
		})
	}
	// A local sidecar exempts the artifact from the closure check.
	root := scaffoldFiles(t, "prefix: example\nskills: [brainstorming]\nagents: []\n",
		map[string]string{"skills/brainstorming.yaml": "local: true\n"})
	p, err := Open(root)
	if err != nil {
		t.Fatalf("local-sidecar artifact must skip closure validation, got: %v", err)
	}
	// An unknown node kind is never enabled (defensive default arm).
	if p.nodeEnabled(catalog.Node{Kind: "bogus", Name: "x"}) {
		t.Error("unknown node kind must report not enabled")
	}
}

func TestOpenAllowsLocalPairedSkillWithoutAgent(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: [reviewing-impl]\nagents: []\n",
		map[string]string{"skills/reviewing-impl.yaml": "local: true\n"})
	if _, err := Open(root); err != nil {
		t.Fatalf("local skill sidecar must skip the pairing check, got: %v", err)
	}
}

func TestSyncRecordsTopicOutputsInManifest(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root, "contracts", "Contracts", "paths: [\"internal/**\"]\n")
	p, _ := Open(root)
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lock.Files["docs/topics/rendering/contracts.md"]; !ok {
		t.Fatal("topic output missing from manifest")
	}
}
