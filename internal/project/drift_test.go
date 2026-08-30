package project

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// configHashOf re-opens the project and returns the per-target ConfigHash of the
// rendered file at rel.
// invariant: rendering/project-output-plan:output-policy-explicit (TestOutputPolicyRoutesMisleadingPathsEndToEnd)
func TestOutputPolicyRoutesMisleadingPathsEndToEnd(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}

	// The .ts suffix is irrelevant: the declared Markdown policy selects
	// frontmatter validation, link scanning, and skill-reference scanning.
	frontmatter := RenderedFile{Path: "misleading.ts", Content: "not frontmatter\n", Policy: OutputPolicy{ValidateFrontmatter: true}, Encoder: MarkdownAgentDialect}
	testsupport.WriteFile(t, filepath.Join(root, frontmatter.Path), frontmatter.Content)
	lock := &manifest.Lock{Files: map[string]manifest.Entry{"misleading.ts": {OutputHash: manifest.Hash([]byte(frontmatter.Content))}}}
	if drift := checkLockedDrift(renderInputsForTest(p).residentRoots(), lock, map[string]RenderedFile{frontmatter.Path: frontmatter}, nil); len(drift) != 1 || drift[0].Kind != "invalid-frontmatter" {
		t.Fatalf("frontmatter policy drift = %#v", drift)
	}
	link := RenderedFile{Path: "misleading.toml", Content: "[missing](no/such/file.md)", Policy: OutputPolicy{ScanReferences: true}}
	if drift := checkDeadRefs(renderInputsForTest(p), []RenderedFile{link}); len(drift) != 1 || drift[0].Kind != "dead-reference" {
		t.Fatalf("link policy drift = %#v", drift)
	}
	skill := RenderedFile{Path: "misleading.ts", Content: "example-debugging", Policy: OutputPolicy{ScanSkillReferences: true}}
	if drift := checkDeadSkillRefs(renderInputsForTest(p), []RenderedFile{skill}, map[string]bool{}); len(drift) != 1 || drift[0].Kind != "dead-skill-reference" {
		t.Fatalf("skill-reference policy drift = %#v", drift)
	}

	// Conversely a Markdown-looking path remains unscanned when its declared
	// policy says it is plain output.
	plain := RenderedFile{Path: "misleading.md", Content: "[missing](no/such/file.md) example-debugging"}
	if drift := checkDeadRefs(renderInputsForTest(p), []RenderedFile{plain}); len(drift) != 0 {
		t.Fatalf("unscanned link drift = %#v", drift)
	}
	if drift := checkDeadSkillRefs(renderInputsForTest(p), []RenderedFile{plain}, map[string]bool{}); len(drift) != 0 {
		t.Fatalf("unscanned skill drift = %#v", drift)
	}

	// Regeneration likewise follows policy, not a generated template ID or
	// conventional skill location.
	testsupport.WriteFile(t, filepath.Join(root, "misleading/SKILL.md"), "old\n")
	regen := RenderedFile{Path: "misleading/SKILL.md", Content: "new\n", Policy: OutputPolicy{Regenerate: true}}
	lock = &manifest.Lock{Files: map[string]manifest.Entry{regen.Path: {}}}
	if drift := checkLockedDrift(renderInputsForTest(p).residentRoots(), lock, map[string]RenderedFile{regen.Path: regen}, nil); len(drift) != 1 || drift[0].Kind != "stale" {
		t.Fatalf("regeneration policy drift = %#v", drift)
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
			root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if tt.observed != nil {
				testsupport.WriteFile(t, filepath.Join(root, path), *tt.observed)
			}
			lock := &manifest.Lock{Files: map[string]manifest.Entry{path: tt.entry}}
			got := checkLockedDrift(renderInputsForTest(p).residentRoots(), lock, map[string]RenderedFile{path: tt.file}, nil)
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
	files, err := renderAll(p)
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
	const unrelated = ".claude/skills/example-debugging/SKILL.md"
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\n")
	before := configHashOf(t, root, unrelated)
	if after := configHashOf(t, root, unrelated); after != before {
		t.Fatal("fixed topic fan-out budget changed unrelated skill guidance")
	}
}

// invariant: rendering/sync-and-drift:drift-source-set (TestPerTargetDriftProjection)
func TestPerTargetDriftProjection(t *testing.T) {
	const planning = ".claude/skills/example-planning/SKILL.md"
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\n", map[string]string{
		"skills/parts/planning/shape.md": "ORIGINAL SHAPE\n",
	})
	before := configHashOf(t, root, planning)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/parts/planning/shape.md"), "NEW SHAPE\n")
	if after := configHashOf(t, root, planning); after == before {
		t.Fatal("consumed part did not change target hash")
	}
}

func sprintfVars(_ string) string {
	return "vars:\n  gateCmd: make gate\n"
}

func TestSyncPruneRefusesEscapingLockPathBeforeMutation(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
	victim := filepath.Join(root, "..", "victim.txt")
	testsupport.WriteFile(t, victim, "keep me\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
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
	lockBefore, err := os.ReadFile(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(root, "AGENTS.md")
	agentsBefore, err := os.ReadFile(agents)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p2); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("sync error = %v, want corrupt-lock refusal", err)
	}
	for path, want := range map[string][]byte{victim: []byte("keep me\n"), agents: agentsBefore, lockFile(root): lockBefore} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s after refusal = %q, %v; want %q", path, got, err, want)
		}
	}
}

// Singleton convention parts (.awf/parts/<kind>/<section>.md) are subject to
// the same orphan scan as per-artifact parts: a typo'd section or an unknown
// kind must be flagged instead of silently never rendering.
func TestCheckFlagsOrphanedSingletonParts(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n", map[string]string{
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
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	drift, err := checkProject(p, testContext(t))
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

// invariant: config/migrations-and-locks:schema-version-lock (TestSyncStampsSchemaVersion)
func TestSyncStampsSchemaVersion(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
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

func chainClosureConfig(scope string) string {
	return "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\naudit:\n  allowedScopes:\n    - " + scope + "\n"
}

// Editing audit.allowedScopes reflags exactly the artifacts whose assembled
// templates reference .commitScopes; non-referencing artifacts stay in sync,
// and the rendered prose quotes the configured scopes (ADR-0051).
// invariant: rendering/sync-and-drift:scopes-in-confighash (TestScopesEditReflagsPlaceholderPart)
func TestScopesEditReflagsPlaceholderPart(t *testing.T) {
	cfg := func(meaning string) string {
		return "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\n" +
			"audit:\n  allowedScopes:\n    - {name: adr, meaning: " + meaning + "}\n"
	}
	root := scaffoldFiles(t, cfg("ADR docs"), map[string]string{
		"parts/workflow/commit-discipline.md": "## Commit discipline\n\n{{=awf:commitScopeTable}}\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteAwfConfig(t, root, cfg("ADR markdown documents")) // scope edit, part untouched
	p2, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := checkProject(p2, testContext(t))
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
	if _, _, _, err := syncReportProject(p); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("want refusal with hint, got %v", err)
	}
	after, err := os.ReadFile(agents)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("SyncReport wrote despite refusing (err %v)", err)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md.awf-bak")); err == nil {
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
	if _, err := checkProject(p, testContext(t)); err == nil || strings.Contains(err.Error(), "no lock") || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("corrupt lock misreported: %v", err)
	}
	if err := os.Remove(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if _, err := checkProject(p, testContext(t)); err == nil || !strings.Contains(err.Error(), "no lock (run awf render)") {
		t.Fatalf("missing lock lost its message: %v", err)
	}
}

func TestUninstallSplitsMissingVsCorrupt(t *testing.T) {
	root := scaffold(t, sampleYAML)
	syncClean(t, root)
	corruptProjectLock(t, root)
	if _, err := uninstallProject(t, root); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("corrupt lock must refuse uninstall with the hint, got %v", err)
	}
	if err := os.Remove(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if _, err := uninstallProject(t, root); err == nil || !strings.Contains(err.Error(), "nothing to uninstall") {
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
	if _, _, err := auditProject(p, testContext(t), "HEAD", "HEAD"); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
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
		return "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\n" +
			"audit:\n  allowedScopes:\n    - {name: adr, meaning: " + meaning + "}\n"
	}
	root := scaffoldFiles(t, cfg("ADR docs"), map[string]string{
		"parts/workflow/commit-discipline.md": "## Commit discipline\n\n<!-- awf:comment demo of {{=awf:commitScopeTable}} -->\nplain text\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteAwfConfig(t, root, cfg("ADR markdown documents")) // scope edit, part untouched
	p2, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := checkProject(p2, testContext(t))
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
	writeProjectTopic(t, root)
	p, _ := Open(testContext(t), root)
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/metadata/rendering/contracts.yaml"), "title: Changed\nsummary: Current Contracts contracts.\npaths: [\"internal/**\"]\n")
	drift, err := checkProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if !hasDrift(drift, "docs/topics/rendering/contracts.md", "stale") {
		t.Fatalf("metadata drift = %#v", drift)
	}
}

func TestCheckCleanAfterSync(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	drift, err := checkProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 0 {
		t.Errorf("expected clean, got drift: %#v", drift)
	}
}

func TestCheckDetectsHandEdit(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	_ = syncProject(p)
	skill := filepath.Join(root, ".claude/skills/example-debugging/SKILL.md")
	_ = os.WriteFile(skill, []byte("hand edited\n"), 0o644)
	drift, _ := checkProject(p, testContext(t))
	if len(drift) == 0 || drift[0].Kind != "hand-edited" {
		t.Errorf("expected hand-edited drift, got %#v", drift)
	}
}

func TestCheckStaleTakesPrecedence(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	skillPath := ".claude/skills/example-debugging/SKILL.md"
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
	drift, err := checkProject(p, testContext(t))
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

// invariant: rendering/sync-and-drift:sync-always-writes-active-md (TestSyncGeneratesActiveMDAndCheckDetectsStaleness)
// invariant: rendering/sync-and-drift:check-active-md-stale (TestSyncGeneratesActiveMDAndCheckDetectsStaleness)
func TestSyncGeneratesActiveMDAndCheckDetectsStaleness(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
	adrDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	adrBody := testsupport.ADR("Accepted", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
		testsupport.WithTitle("0001: First"), testsupport.WithBody("## Context\nx\n## Decision\n\n1. x.\n"))
	testsupport.WriteFile(t, filepath.Join(adrDir, "0001-first.md"), adrBody)

	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := syncProject(p); err != nil {
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
	if drift, err := checkProject(p, testContext(t)); err != nil || len(drift) != 0 {
		t.Fatalf("expected clean check after sync, got drift=%#v err=%v", drift, err)
	}

	// Change frontmatter status (Accepted In flight -> Implemented History); the
	// on-disk INDEX.md is now stale.
	adr2 := strings.Replace(adrBody, "status: Accepted", "status: Implemented", 1)
	testsupport.WriteFile(t, filepath.Join(adrDir, "0001-first.md"), adr2)
	drift, err := checkProject(p, testContext(t))
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

// invariant: rendering/sync-and-drift:check-invalid-frontmatter (TestCheckDetectsInvalidFrontmatter)
func TestCheckDetectsInvalidFrontmatter(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	const skillPath = ".claude/skills/example-debugging/SKILL.md"
	broken := "---\nname: \"\"\ndescription: \"\"\n---\nbody\n"
	// Fresh planned bytes, the locked hash, and observed bytes all agree, so
	// frontmatter validation is the first applicable finding.
	testsupport.WriteFile(t, filepath.Join(root, skillPath), broken)
	file := RenderedFile{Path: skillPath, Content: broken, Policy: OutputPolicy{ValidateFrontmatter: true}, Encoder: MarkdownAgentDialect}
	lock := &manifest.Lock{Files: map[string]manifest.Entry{
		skillPath: {OutputHash: manifest.Hash([]byte(broken))},
	}}
	findings := checkLockedFiles(renderInputsForTest(p).residentRoots(), lock, map[string]RenderedFile{skillPath: file}, nil)
	if len(findings) != 1 || findings[0].Property != propertyReproducibility {
		t.Fatalf("invalid-frontmatter semantic finding = %#v, want reproducibility", findings)
	}
	drift := checkLockedDrift(renderInputsForTest(p).residentRoots(), lock, map[string]RenderedFile{skillPath: file}, nil)
	want := []manifest.Drift{{Path: skillPath, Kind: "invalid-frontmatter", Detail: "frontmatter name is empty"}}
	if !slices.Equal(drift, want) {
		t.Errorf("invalid-frontmatter drift = %#v, want %#v", drift, want)
	}
}

// Generated indexes carry RegenChecked=true and use regeneration drift checks,
// while ordinary rendered files carry false and use frozen OutputHash checks.
// The attribute is the single source for selecting the applicable check.
func TestRegenCheckedAttribute(t *testing.T) {
	root := scaffold(t, domainCfg)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	// invariant: rendering/sync-and-drift:regeneration-checked-attribute (TestRegenCheckedAttribute)
	amd := generateIndexMD(renderInputsForTest(p), mustDeriveCorpus(t, p))
	if !amd.RegenChecked {
		t.Errorf("INDEX.md must be regeneration-checked")
	}
	dds, err := generateDomainDocs(renderInputsForTest(p), mustDeriveTopics(t, p), mustDeriveSkills(t, p))
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
	files, err := renderAll(p)
	if err != nil {
		t.Fatal(err)
	}
	cref, ok, err := generateConfigReference(renderInputsForTest(p), slices.Concat(files, dds), mustDeriveSkills(t, p))
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
			t.Errorf("plan policy and RegenChecked disagree for %s: policy=%v regenChecked=%v", f.Path, f.Policy.Regenerate, f.RegenChecked)
		}
	}
}
