package project

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// configHashOf re-opens the project and returns the per-target ConfigHash of the
// rendered file at rel.
func configHashOf(t *testing.T, root, rel string) string {
	t.Helper()
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	for _, f := range files {
		if f.Path == rel {
			return f.ConfigHash
		}
	}
	t.Fatalf("no rendered file %s", rel)
	return ""
}

// invariant: drift-source-set
func TestPerTargetDriftProjection(t *testing.T) {
	const (
		A = ".claude/skills/example-tdd/SKILL.md"
		B = ".claude/skills/example-bugfix/SKILL.md"
	)
	cfg := func(pitfalls string) string {
		return "prefix: example\n" + sprintfVars(pitfalls) + "skills:\n  - tdd\n  - bugfix\nagents: []\n"
	}
	root := scaffoldFiles(t, cfg(""), map[string]string{
		"skills/tdd.yaml":           "data:\n  testSurfaces:\n    - {name: One, location: a, kind: b}\n",
		"skills/bugfix.yaml":        "data:\n  k: v\n",
		"skills/parts/tdd/notes.md": "ORIGINAL NOTES\n",
	})

	a0 := configHashOf(t, root, A)
	b0 := configHashOf(t, root, B)

	// (1) Editing target A's sidecar changes A's hash but not B's.
	testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/tdd.yaml"), "data:\n  testSurfaces:\n    - {name: Changed, location: x, kind: y}\n")
	a1 := configHashOf(t, root, A)
	b1 := configHashOf(t, root, B)
	if a1 == a0 {
		t.Error("editing A's sidecar should change A's ConfigHash")
	}
	if b1 != b0 {
		t.Error("editing A's sidecar must not change B's ConfigHash")
	}

	// (2) Editing a part A consumes changes A.
	testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/parts/tdd/notes.md"), "NEW NOTES BODY\n")
	a2 := configHashOf(t, root, A)
	if a2 == a1 {
		t.Error("editing a part A consumes should change A's ConfigHash")
	}

	// (3) An unrelated vars edit (a var A does not reference) does not change A's hash.
	testsupport.WriteFile(t, filepath.Join(root, ".awf/config.yaml"), cfg("now-set"))
	a3 := configHashOf(t, root, A)
	if a3 != a2 {
		t.Errorf("a var A does not reference (pitfallsDoc) must not change A's ConfigHash:\n%s\n%s", a2, a3)
	}

	// (4) A sidecar/part for a target absent from the enable list is an orphan.
	orphRoot := scaffoldFiles(t, cfg(""), map[string]string{
		"skills/tdd.yaml":                 "data:\n  k: v\n",
		"skills/bugfix.yaml":              "data:\n  k: v\n",
		"skills/debugging.yaml":           "data:\n  k: v\n", // debugging not enabled
		"skills/parts/orphan-target/x.md": "stray\n",         // orphan-target not enabled
	})
	p, err := Open(orphRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	wantOrphans := map[string]bool{
		".awf/skills/debugging.yaml":      false,
		".awf/skills/parts/orphan-target": false,
	}
	for _, d := range drift {
		if d.Kind == "orphaned" {
			if _, ok := wantOrphans[d.Path]; ok {
				wantOrphans[d.Path] = true
			}
		}
	}
	for path, seen := range wantOrphans {
		if !seen {
			t.Errorf("expected orphan drift for %s, got %#v", path, drift)
		}
	}
}

// An artifact enabled after the last sync has a rendered output but no lock
// entry; Check must flag it instead of passing clean while the file is absent
// from disk.
func TestCheckFlagsEnabledButUnsyncedArtifact(t *testing.T) {
	cfg := func(agents string) string {
		return "prefix: example\nskills:\n  - tdd\nagents:" + agents + "\n"
	}
	root := scaffold(t, cfg(" []"))
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	// Enable an agent by hand-editing config — the documented flow — without syncing.
	testsupport.WriteAwfConfig(t, root, cfg("\n  - code-reviewer"))
	p2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p2.Check()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drift {
		if d.Path == ".claude/agents/code-reviewer.md" && d.Kind == "unsynced" {
			return
		}
	}
	t.Errorf("enabled-but-unsynced agent not flagged; drift = %#v", drift)
}

// The sync prune walks old lock entries no longer produced; an entry escaping
// the repo root must be skipped, not deleted out-of-tree (and the ancestor
// walk must terminate).
func TestSyncPruneSkipsEscapingLockPaths(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\n")
	victim := filepath.Join(root, "..", "victim.txt")
	testsupport.WriteFile(t, victim, "keep me\n")
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
	lock.Files["../victim.txt"] = manifest.Entry{}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	p2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p2.Sync(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Errorf("prune deleted the out-of-tree file: %v", err)
	}
}

// Singleton convention parts (.awf/parts/<kind>/<section>.md) are subject to
// the same orphan scan as per-artifact parts: a typo'd section or an unknown
// kind must be flagged instead of silently never rendering.
func TestCheckFlagsOrphanedSingletonParts(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\n", map[string]string{
		"parts/workflow/typo-section.md": "stray\n",
		"parts/nonsense/x.md":            "stray\n",
		"parts/workflow/principles.md":   "## Principles\n\nLegit override.\n",
		"parts/loose.md":                 "not a kind dir\n",
		"parts/workflow/notes.txt":       "not a part file\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	orphaned := map[string]bool{}
	for _, d := range drift {
		if d.Kind == "orphaned" {
			orphaned[d.Path] = true
		}
	}
	for _, want := range []string{".awf/parts/workflow/typo-section.md", ".awf/parts/nonsense"} {
		if !orphaned[want] {
			t.Errorf("expected orphan drift for %s; drift = %#v", want, drift)
		}
	}
	if orphaned[".awf/parts/workflow/principles.md"] {
		t.Error("declared-section singleton part wrongly flagged as orphan")
	}
}

func sprintfVars(pitfalls string) string {
	return "vars:\n  testCmd: \"\"\n  gateCmd: \"\"\n  gateCmdFull: \"\"\n  workflowDoc: \"\"\n  docCurrencyTargets: \"\"\n  pitfallsDoc: \"" + pitfalls + "\"\n"
}

// invariant: schema-version-lock
func TestSyncStampsSchemaVersion(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\n")
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
	if lock.SchemaVersion != migrate.Current() {
		t.Errorf("lock SchemaVersion = %d, want %d (current schema)", lock.SchemaVersion, migrate.Current())
	}
	if lock.AWFVersion != Version {
		t.Errorf("AWFVersion = %q, want %q (independent tool version)", lock.AWFVersion, Version)
	}
}

// chainClosureConfig derives the chain-unit enabled set from the catalog:
// the Chain-flagged skills, their transitive RequiresSkills closure, and the
// RequiresAgent agents of every skill in that combined set (ADR-0080
// Decision 5) — never a hand list.
func chainClosureConfig(scope string) string {
	set := map[string]bool{}
	var add func(name string)
	add = func(name string) {
		if set[name] {
			return
		}
		set[name] = true
		for _, r := range catalog.Standard.Skills[name].RequiresSkills {
			add(r)
		}
	}
	for name, spec := range catalog.Standard.Skills {
		if spec.Chain {
			add(name)
		}
	}
	agents := map[string]bool{}
	skills := make([]string, 0, len(set))
	for name := range set {
		skills = append(skills, name)
		if a := catalog.Standard.Skills[name].RequiresAgent; a != "" {
			agents[a] = true
		}
	}
	sort.Strings(skills)
	agentList := make([]string, 0, len(agents))
	for a := range agents {
		agentList = append(agentList, a)
	}
	sort.Strings(agentList)
	var b strings.Builder
	b.WriteString("prefix: example\nvars: {}\nskills:\n")
	for _, s := range skills {
		b.WriteString("  - " + s + "\n")
	}
	b.WriteString("agents:\n")
	for _, a := range agentList {
		b.WriteString("  - " + a + "\n")
	}
	b.WriteString("audit:\n  allowedScopes:\n    - " + scope + "\n")
	return b.String()
}

// Editing audit.allowedScopes reflags exactly the artifacts whose assembled
// templates reference .commitScopes; non-referencing artifacts stay in sync,
// and the rendered prose quotes the configured scopes (ADR-0051).
// invariant: scopes-in-confighash
func TestScopesEditReflagsReferencingArtifacts(t *testing.T) {
	root := scaffold(t, chainClosureConfig("awf"))
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	rendered, err := os.ReadFile(filepath.Join(root, ".claude/skills/example-reviewing-adr/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rendered), "using a Conventional-Commits scope from `awf`") {
		t.Errorf("rendered prose does not quote audit.allowedScopes:\n%s", rendered)
	}
	testsupport.WriteAwfConfig(t, root, chainClosureConfig("core"))
	p2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p2.Check()
	if err != nil {
		t.Fatal(err)
	}
	flagged := map[string]bool{}
	for _, d := range drift {
		if d.Kind != "stale" {
			t.Errorf("unexpected drift kind %q on %s", d.Kind, d.Path)
		}
		flagged[d.Path] = true
	}
	if !flagged[".claude/skills/example-reviewing-adr/SKILL.md"] {
		t.Errorf("scopes edit did not reflag the referencing skill; drift = %v", drift)
	}
	if flagged[".claude/skills/example-brainstorming/SKILL.md"] {
		t.Error("scopes edit reflagged the non-referencing brainstorming skill")
	}
}

// Editing audit.allowedScopes reflags an artifact whose raw convention part uses
// a {{=awf:commitScope*}} placeholder — the config-hash folds scope data via the
// part-body scan, not the template-source scan — while a non-referencing
// artifact stays in sync (ADR-0057).
// invariant: part-scopes-in-confighash
func TestScopesEditReflagsPlaceholderPart(t *testing.T) {
	cfg := func(meaning string) string {
		return "prefix: example\nvars: {}\nskills: []\nagents: []\n" +
			"audit:\n  allowedScopes:\n    - {name: adr, meaning: " + meaning + "}\n"
	}
	root := scaffoldFiles(t, cfg("ADR docs"), map[string]string{
		"parts/workflow/commit-discipline.md": "## Commit discipline\n\n{{=awf:commitScopeTable}}\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteAwfConfig(t, root, cfg("ADR markdown documents")) // scope edit, part untouched
	p2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p2.Check()
	if err != nil {
		t.Fatal(err)
	}
	flagged := map[string]bool{}
	for _, d := range drift {
		flagged[d.Path] = true
	}
	if !flagged["docs/workflow.md"] {
		t.Errorf("scopes edit did not reflag the placeholder-using part artifact; drift = %v", drift)
	}
	// The ADR readme references no scopes in template or part — it must not reflag.
	if flagged["docs/decisions/README.md"] {
		t.Error("scopes edit reflagged a non-referencing artifact (docs/decisions/README.md)")
	}
}

func corruptProjectLock(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(lockFile(root), []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// invariant: corrupt-lock-refuses
func TestSyncReportRefusesCorruptLockBeforeWriting(t *testing.T) {
	root := scaffold(t, sampleYAML)
	syncClean(t, root)
	agents := filepath.Join(root, "AGENTS.md")
	before, err := os.ReadFile(agents)
	if err != nil {
		t.Fatal(err)
	}
	corruptProjectLock(t, root)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.SyncReport(); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("want refusal with hint, got %v", err)
	}
	after, err := os.ReadFile(agents)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("SyncReport wrote despite refusing (err %v)", err)
	}
	if fileExists(filepath.Join(root, "AGENTS.md.awf-bak")) {
		t.Fatal("backup created despite refusal")
	}
}

func TestCheckSplitsMissingVsCorrupt(t *testing.T) {
	root := scaffold(t, sampleYAML)
	syncClean(t, root)
	corruptProjectLock(t, root)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Check(); err == nil || strings.Contains(err.Error(), "no lock") || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("corrupt lock misreported: %v", err)
	}
	if err := os.Remove(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Check(); err == nil || !strings.Contains(err.Error(), "no lock (run awf sync)") {
		t.Fatalf("missing lock lost its message: %v", err)
	}
}

func TestUninstallSplitsMissingVsCorrupt(t *testing.T) {
	root := scaffold(t, sampleYAML)
	syncClean(t, root)
	corruptProjectLock(t, root)
	if _, err := Uninstall(root); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("corrupt lock must refuse uninstall with the hint, got %v", err)
	}
	if err := os.Remove(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(root); err == nil || !strings.Contains(err.Error(), "nothing to uninstall") {
		t.Fatalf("missing lock lost its message: %v", err)
	}
}

func TestAuditAndCollisionsRefuseCorruptLock(t *testing.T) {
	root := scaffold(t, sampleYAML)
	syncClean(t, root)
	corruptProjectLock(t, root)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Audit(""); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("Audit: %v", err)
	}
	if _, err := CollisionsAt(root, []string{"AGENTS.md"}); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("CollisionsAt: %v", err)
	}
}
