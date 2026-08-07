package project

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// configHashOf re-opens the project and returns the per-target ConfigHash of the
// rendered file at rel.
// invariant: rendering/project-output-plan:output-policy-explicit (TestOutputPolicyRoutesMisleadingPathsEndToEnd)
func TestOutputPolicyRoutesMisleadingPathsEndToEnd(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nskills: [tdd]\nagents: []\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}

	// The .ts suffix is irrelevant: the declared Markdown policy selects
	// frontmatter validation, link scanning, and skill-reference scanning.
	frontmatter := RenderedFile{Path: "misleading.ts", Content: "not frontmatter\n", Policy: OutputPolicy{ValidateFrontmatter: true}, Encoder: MarkdownAgentDialect}
	testsupport.WriteFile(t, filepath.Join(root, frontmatter.Path), frontmatter.Content)
	lock := &manifest.Lock{Files: map[string]manifest.Entry{"misleading.ts": {OutputHash: manifest.Hash([]byte(frontmatter.Content))}}}
	if drift := p.checkLockedFiles(lock, map[string]RenderedFile{frontmatter.Path: frontmatter}); len(drift) != 1 || drift[0].Kind != "invalid-frontmatter" {
		t.Fatalf("frontmatter policy drift = %#v", drift)
	}
	link := RenderedFile{Path: "misleading.toml", Content: "[missing](no/such/file.md)", Policy: OutputPolicy{ScanReferences: true}}
	if drift := p.checkDeadRefs([]RenderedFile{link}); len(drift) != 1 || drift[0].Kind != "dead-reference" {
		t.Fatalf("link policy drift = %#v", drift)
	}
	skill := RenderedFile{Path: "misleading.ts", Content: "example-tdd", Policy: OutputPolicy{ScanSkillReferences: true}}
	if drift := p.checkDeadSkillRefs([]RenderedFile{skill}, map[string]bool{}); len(drift) != 1 || drift[0].Kind != "dead-skill-reference" {
		t.Fatalf("skill-reference policy drift = %#v", drift)
	}

	// Conversely a Markdown-looking path remains unscanned when its declared
	// policy says it is plain output.
	plain := RenderedFile{Path: "misleading.md", Content: "[missing](no/such/file.md) example-tdd"}
	if drift := p.checkDeadRefs([]RenderedFile{plain}); len(drift) != 0 {
		t.Fatalf("unscanned link drift = %#v", drift)
	}
	if drift := p.checkDeadSkillRefs([]RenderedFile{plain}, map[string]bool{}); len(drift) != 0 {
		t.Fatalf("unscanned skill drift = %#v", drift)
	}

	// Regeneration and local validation likewise follow policy, not a generated
	// template ID or conventional skill location.
	testsupport.WriteFile(t, filepath.Join(root, "misleading/SKILL.md"), "old\n")
	regen := RenderedFile{Path: "misleading/SKILL.md", Content: "new\n", Policy: OutputPolicy{Regenerate: true}}
	lock = &manifest.Lock{Files: map[string]manifest.Entry{regen.Path: {}}}
	if drift := p.checkLockedFiles(lock, map[string]RenderedFile{regen.Path: regen}); len(drift) != 1 || drift[0].Kind != "stale" {
		t.Fatalf("regeneration policy drift = %#v", drift)
	}
	reservation := &OutputPlan{Nodes: []OutputNode{{Path: "local.unknown", Reservation: true, Policy: OutputPolicy{LocalValidation: true, ValidateFrontmatter: true}, Recipe: OutputRecipe{Encoder: MarkdownAgentDialect}}}}
	var localErr error
	p.localReservations(reservation, func(_ string, err error) { localErr = err })
	if localErr == nil || !strings.Contains(localErr.Error(), "absent") {
		t.Fatalf("local validation policy error = %v", localErr)
	}
}

// invariant: rendering/sync-and-drift:ordinary-render-freshness (TestCheckLockedFilesClassifiesOrdinaryFreshnessBeforeObservation)
func TestCheckLockedFilesClassifiesOrdinaryFreshnessBeforeObservation(t *testing.T) {
	const path = "ordinary-output"
	text := func(value string) *string { return &value }
	tests := []struct {
		name     string
		observed *string
		file     RenderedFile
		entry    manifest.Entry
		want     []manifest.Drift
	}{
		{
			name:     "binary-derived stale precedes hand edit",
			observed: text("hand edit"),
			file:     RenderedFile{Path: path, Content: "fresh render", TemplateHash: "template", ConfigHash: "config"},
			entry:    manifest.Entry{TemplateHash: "template", ConfigHash: "config", OutputHash: manifest.Hash([]byte("locked output"))},
			want:     []manifest.Drift{{Path: path, Kind: "stale", Detail: "rendered output out of date; run awf render"}},
		},
		{
			name:  "binary-derived stale precedes missing",
			file:  RenderedFile{Path: path, Content: "fresh render", TemplateHash: "template", ConfigHash: "config"},
			entry: manifest.Entry{TemplateHash: "template", ConfigHash: "config", OutputHash: manifest.Hash([]byte("locked output"))},
			want:  []manifest.Drift{{Path: path, Kind: "stale", Detail: "rendered output out of date; run awf render"}},
		},
		{
			name:     "hand edited",
			observed: text("hand edit"),
			file:     RenderedFile{Path: path, Content: "locked output", TemplateHash: "template", ConfigHash: "config"},
			entry:    manifest.Entry{TemplateHash: "template", ConfigHash: "config", OutputHash: manifest.Hash([]byte("locked output"))},
			want:     []manifest.Drift{{Path: path, Kind: "hand-edited", Detail: "on-disk output differs from lock; run awf render to discard the edit, or move it into a .awf convention part to keep it"}},
		},
		{
			name:     "clean",
			observed: text("locked output"),
			file:     RenderedFile{Path: path, Content: "locked output", TemplateHash: "template", ConfigHash: "config"},
			entry:    manifest.Entry{TemplateHash: "template", ConfigHash: "config", OutputHash: manifest.Hash([]byte("locked output"))},
		},
		{
			name:  "missing after fresh match",
			file:  RenderedFile{Path: path, Content: "locked output", TemplateHash: "template", ConfigHash: "config"},
			entry: manifest.Entry{TemplateHash: "template", ConfigHash: "config", OutputHash: manifest.Hash([]byte("locked output"))},
			want:  []manifest.Drift{{Path: path, Kind: "missing", Detail: "file absent; run awf render"}},
		},
		{
			name:     "regenerated policy retained",
			observed: text("old"),
			file:     RenderedFile{Path: path, Content: "new", Policy: OutputPolicy{Regenerate: true}},
			entry:    manifest.Entry{},
			want:     []manifest.Drift{{Path: path, Kind: "stale", Detail: "generated output out of date; run awf render"}},
		},
		{
			name:     "in-place policy retained",
			observed: text("old"),
			file:     RenderedFile{Path: path, Content: "new", TemplateID: "template", Policy: OutputPolicy{Regenerate: true}},
			entry:    manifest.Entry{},
			want:     []manifest.Drift{{Path: path, Kind: "hand-edited", Detail: "on-disk output differs from the regenerated file; run awf render to restore awf-owned regions"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := scaffold(t, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\n")
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if tt.observed != nil {
				testsupport.WriteFile(t, filepath.Join(root, path), *tt.observed)
			}
			lock := &manifest.Lock{Files: map[string]manifest.Entry{path: tt.entry}}
			got := p.checkLockedFiles(lock, map[string]RenderedFile{path: tt.file})
			if !slices.Equal(got, tt.want) {
				t.Fatalf("drift = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func configHashOf(t *testing.T, root, rel string) string {
	t.Helper()
	p, err := Open(testContext(t), root)
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

func TestRetiredTopicMaximumDoesNotAffectProjection(t *testing.T) {
	const unrelated = ".claude/skills/example-tdd/SKILL.md"
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\nskills: [tdd]\nagents: []\n")
	before := configHashOf(t, root, unrelated)
	if after := configHashOf(t, root, unrelated); after != before {
		t.Fatal("fixed topic fan-out budget changed unrelated skill guidance")
	}
}

// invariant: rendering/sync-and-drift:drift-source-set (TestPerTargetDriftProjection)
func TestPerTargetDriftProjection(t *testing.T) {
	const (
		A = ".claude/skills/example-tdd/SKILL.md"
		B = ".claude/skills/example-bugfix/SKILL.md"
	)
	cfg := func(pitfalls string) string {
		return "prefix: example\nintegrationBranch: main\n" + sprintfVars(pitfalls) + "skills:\n  - tdd\n  - bugfix\nagents: []\n"
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
	p, err := Open(testContext(t), orphRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check(testContext(t))
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
func TestCheckIgnoresUnsyncedSelectionChanges(t *testing.T) {
	cfg := func(agents string) string {
		return "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\nskills:\n  - tdd\nagents:" + agents + "\n"
	}
	root := scaffold(t, cfg(" []"))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteAwfConfig(t, root, cfg("\n  - code-reviewer"))
	p, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drift {
		if d.Path == ".claude/agents/code-reviewer.md" && d.Kind == "unsynced" {
			t.Fatalf("selection change produced unsynced drift: %#v", drift)
		}
	}
}

// The sync prune walks old lock entries no longer produced; an entry escaping
// the repo root must be skipped, not deleted out-of-tree (and the ancestor
// walk must terminate).
func TestSyncPruneSkipsEscapingLockPaths(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\n")
	victim := filepath.Join(root, "..", "victim.txt")
	testsupport.WriteFile(t, victim, "keep me\n")
	p, err := Open(testContext(t), root)
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
	p2, err := Open(testContext(t), root)
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
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\n", map[string]string{
		"parts/workflow/typo-section.md": "stray\n",
		"parts/nonsense/x.md":            "stray\n",
		"parts/workflow/principles.md":   "## Principles\n\nLegit override.\n",
		"parts/loose.md":                 "not a kind dir\n",
		"parts/workflow/notes.txt":       "not a part file\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check(testContext(t))
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
	return "vars:\n  testCmd: \"\"\n  gateCmd: make gate\n  gateCmdFull: \"\"\n  workflowDoc: \"\"\n  pitfallsDoc: \"" + pitfalls + "\"\n"
}

// invariant: config/migrations-and-locks:schema-version-lock (TestSyncStampsSchemaVersion)
func TestSyncStampsSchemaVersion(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\n")
	p, err := Open(testContext(t), root)
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

// chainClosureConfig derives the chain-unit enabled set from the catalog via
// the production forward walk: the Chain-flagged skills' full closure (ADR-0080
// Decision 5, walk unified by ADR-0081) - never a hand list.
func chainClosureConfig(scope string) string {
	var seeds []catalog.Node
	for name, spec := range catalog.Standard.Skills {
		if spec.Profile.Kind == catalog.WorkflowChain {
			seeds = append(seeds, catalog.Node{Kind: "skill", Name: name})
		}
	}
	sort.Slice(seeds, func(i, j int) bool { return seeds[i].Name < seeds[j].Name })
	var skills, agentList []string
	for _, n := range catalog.Closure(catalog.Standard, seeds) {
		switch n.Kind {
		case "skill":
			skills = append(skills, n.Name)
		case "agent":
			agentList = append(agentList, n.Name)
		}
	}
	sort.Strings(skills)
	sort.Strings(agentList)
	var b strings.Builder
	b.WriteString("prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\nskills:\n")
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
// invariant: rendering/sync-and-drift:scopes-in-confighash (TestScopesEditReflagsReferencingArtifacts)
func TestScopesEditReflagsReferencingArtifacts(t *testing.T) {
	root := scaffold(t, chainClosureConfig("awf"))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	// The guide template is the remaining .commitScopes consumer (ADR-0197
	// removed the reviewing skills' restatement), so the scopes fold is
	// proved on AGENTS.md: the edit below must reflag it and nothing else.
	testsupport.WriteAwfConfig(t, root, chainClosureConfig("core"))
	p2, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p2.Check(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	flagged := map[string]bool{}
	for _, d := range drift {
		if d.Kind == "dead-skill-reference" {
			continue
		}
		if d.Kind != "stale" {
			t.Errorf("unexpected drift kind %q on %s", d.Kind, d.Path)
		}
		flagged[d.Path] = true
	}
	if !flagged["AGENTS.md"] {
		t.Errorf("scopes edit did not reflag the referencing guide; drift = %v", drift)
	}
	if flagged[".claude/skills/example-brainstorming/SKILL.md"] {
		t.Error("scopes edit reflagged the non-referencing brainstorming skill")
	}
}

// Editing the skills enable array leaves the neutral guide fresh because native
// target frontmatter owns the catalog, while a skill template that reads .skills
// receives the configured set in its config hash and is therefore stale.
// invariant: rendering/sync-and-drift:skills-set-in-confighash (TestSkillsEditLeavesNeutralGuideFreshAndReflagsSkill)
func TestSkillsEditLeavesNeutralGuideFreshAndReflagsSkill(t *testing.T) {
	cfg := func(skills string) string {
		return "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\nskills:" + skills + "\nagents: []\n"
	}
	root := scaffoldFiles(t, cfg("\n  - debugging\n  - bugfix"), map[string]string{
		"skills/parts/debugging/debugging-surfaces.md": "Configured skills: {{ range .skills }}{{ . }} {{ end }}\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteAwfConfig(t, root, cfg("\n  - debugging"))
	p, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	flagged := map[string]bool{}
	for _, finding := range drift {
		if finding.Kind == "stale" {
			flagged[finding.Path] = true
		}
	}
	if flagged["AGENTS.md"] {
		t.Errorf("editing enabled skills staled neutral AGENTS.md: %v", drift)
	}
	if !flagged[".claude/skills/example-debugging/SKILL.md"] {
		t.Errorf("editing enabled skills did not stale the .skills-consuming skill: %v", drift)
	}
}

// Editing audit.allowedScopes reflags an artifact whose raw convention part uses
// a {{=awf:commitScope*}} placeholder - the config-hash folds scope data via the
// part-body scan, not the template-source scan - while a non-referencing
// artifact stays in sync (ADR-0057).
// invariant: rendering/sync-and-drift:part-scopes-in-confighash (TestScopesEditReflagsPlaceholderPart)
func TestScopesEditReflagsPlaceholderPart(t *testing.T) {
	cfg := func(meaning string) string {
		return "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\nskills: []\nagents: []\n" +
			"audit:\n  allowedScopes:\n    - {name: adr, meaning: " + meaning + "}\n"
	}
	root := scaffoldFiles(t, cfg("ADR docs"), map[string]string{
		"parts/workflow/commit-discipline.md": "## Commit discipline\n\n{{=awf:commitScopeTable}}\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteAwfConfig(t, root, cfg("ADR markdown documents")) // scope edit, part untouched
	p2, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p2.Check(testContext(t))
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
	// The ADR readme references no scopes in template or part - it must not reflag.
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

// invariant: config/migrations-and-locks:corrupt-lock-refuses (TestSyncReportRefusesCorruptLockBeforeWriting)
func TestSyncReportRefusesCorruptLockBeforeWriting(t *testing.T) {
	root := scaffold(t, sampleYAML)
	syncClean(t, root)
	agents := filepath.Join(root, "AGENTS.md")
	before, err := os.ReadFile(agents)
	if err != nil {
		t.Fatal(err)
	}
	corruptProjectLock(t, root)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.SyncReport(testContext(t)); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
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
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Check(testContext(t)); err == nil || strings.Contains(err.Error(), "no lock") || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("corrupt lock misreported: %v", err)
	}
	if err := os.Remove(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Check(testContext(t)); err == nil || !strings.Contains(err.Error(), "no lock (run awf render)") {
		t.Fatalf("missing lock lost its message: %v", err)
	}
}

func TestUninstallSplitsMissingVsCorrupt(t *testing.T) {
	root := scaffold(t, sampleYAML)
	syncClean(t, root)
	corruptProjectLock(t, root)
	if _, err := resident.Uninstall(testContext(t), root); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("corrupt lock must refuse uninstall with the hint, got %v", err)
	}
	if err := os.Remove(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if _, err := resident.Uninstall(testContext(t), root); err == nil || !strings.Contains(err.Error(), "nothing to uninstall") {
		t.Fatalf("missing lock lost its message: %v", err)
	}
}

func TestAuditAndCollisionsRefuseCorruptLock(t *testing.T) {
	root := scaffold(t, sampleYAML)
	syncClean(t, root)
	corruptProjectLock(t, root)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.Audit(testContext(t), "HEAD", "HEAD"); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("Audit: %v", err)
	}
	if _, err := resident.CollisionsAt(root, []string{"AGENTS.md"}); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("CollisionsAt: %v", err)
	}
}

// A {{=awf:commitScope*}} placeholder appearing only inside an authoring
// comment never renders, so it must not fold audit.allowedScopes into the
// artifact's ConfigHash: a scopes edit leaves the artifact in sync
// (ADR-0121 Decision 2, the stripped-detector counterpart of
// TestScopesEditReflagsPlaceholderPart).
func TestCommentWrappedScopePlaceholderDoesNotFold(t *testing.T) {
	cfg := func(meaning string) string {
		return "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\nskills: []\nagents: []\n" +
			"audit:\n  allowedScopes:\n    - {name: adr, meaning: " + meaning + "}\n"
	}
	root := scaffoldFiles(t, cfg("ADR docs"), map[string]string{
		"parts/workflow/commit-discipline.md": "## Commit discipline\n\n<!-- awf:comment demo of {{=awf:commitScopeTable}} -->\nplain text\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteAwfConfig(t, root, cfg("ADR markdown documents")) // scope edit, part untouched
	p2, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p2.Check(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drift {
		if d.Path == "docs/workflow.md" {
			t.Errorf("comment-wrapped placeholder must not fold scopes into the hash; drift = %v", drift)
		}
	}
}

func TestTopicMetadataAndPartBothDriveDrift(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root, "contracts", "Contracts", "paths: [\"internal/**\"]\n")
	p, _ := Open(testContext(t), root)
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/metadata/rendering/contracts.yaml"), "title: Changed\nsummary: Current Contracts contracts.\npaths: [\"internal/**\"]\n")
	drift, err := p.Check(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if !hasDrift(drift, "docs/topics/rendering/contracts.md", "stale") {
		t.Fatalf("metadata drift = %#v", drift)
	}
}
