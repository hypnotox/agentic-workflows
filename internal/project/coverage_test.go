package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// corruptSidecar overwrites a sidecar (relative to .awf) with YAML that
// the strict decoder rejects (unknown field), so a fresh Sidecar read fails.
func corruptSidecar(t *testing.T, root, rel string) {
	t.Helper()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", rel), "bogusUnknownField: true\n")
}

// --- Open error paths ---

func TestOpenMissingConfigFails(t *testing.T) {
	// A bare temp dir with no .awf/config.yaml: config.Load fails.
	_, err := Open(t.TempDir())
	if err == nil {
		t.Fatal("expected Open to fail with no config.yaml")
	}
}

func TestOpenRejectsEmptyPrefix(t *testing.T) {
	root := scaffold(t, "prefix: \"\"\nskills: []\nagents: []\n")
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected Open to fail validation on empty prefix")
	}
	if !strings.Contains(err.Error(), "prefix") {
		t.Errorf("error should mention prefix, got: %v", err)
	}
}

func TestOpenRejectsMalformedSkillSidecar(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: [tdd]\nagents: []\n", map[string]string{
		"skills/tdd.yaml": "bogusUnknownField: true\n",
	})
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected Open to fail on a malformed skill sidecar")
	}
}

func TestOpenRejectsMalformedAgentsDocSidecar(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\n", map[string]string{
		"agents-doc.yaml": "bogusUnknownField: true\n",
	})
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected Open to fail on a malformed agents-doc sidecar")
	}
}

func TestOpenRejectsUnknownAgentsDocSection(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\n", map[string]string{
		"agents-doc.yaml": "sections:\n  not-a-real-section:\n    drop: true\n",
	})
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected Open to reject an undeclared agents-doc section")
	}
	if !strings.Contains(err.Error(), "not-a-real-section") {
		t.Errorf("error should mention the offending section, got: %v", err)
	}
}

func TestOpenRejectsMalformedAdrReadmeSidecar(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\n", map[string]string{
		"adr-readme.yaml": "bogusUnknownField: true\n",
	})
	if _, err := Open(root); err == nil {
		t.Fatal("expected Open to fail on a malformed adr-readme sidecar")
	}
}

func TestOpenRejectsUnknownAdrReadmeSection(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\n", map[string]string{
		"adr-readme.yaml": "sections:\n  not-a-real-section:\n    drop: true\n",
	})
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected Open to reject an undeclared adr-readme section")
	}
	if !strings.Contains(err.Error(), "not-a-real-section") {
		t.Errorf("error should mention the offending section, got: %v", err)
	}
}

// --- validateFrontmatter direct cases ---

func TestValidateFrontmatter(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantErr string
	}{
		{"malformed yaml", "---\nname: [unterminated\n---\nbody\n", ""},
		{"missing frontmatter", "no frontmatter at all\n", "missing frontmatter"},
		{"empty name", "---\nname: \"\"\ndescription: d\n---\n", "name is empty"},
		{"empty description", "---\nname: n\ndescription: \"\"\n---\n", "description is empty"},
		{"valid", "---\nname: n\ndescription: d\n---\nbody\n", "ok"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateFrontmatter([]byte(tc.content))
			if tc.wantErr == "ok" {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error for %q", tc.name)
			}
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q should contain %q", err, tc.wantErr)
			}
		})
	}
}

// --- localOutPaths direct cases ---

func TestLocalOutPaths(t *testing.T) {
	// One path per enabled target; neutral kinds yield nil.
	p := &Project{Cfg: &config.Config{Prefix: "ex"}, Targets: []Target{claudeTarget, cursorTarget}}
	if got := p.localOutPaths("skills", "foo"); len(got) != 2 ||
		got[0] != ".claude/skills/ex-foo/SKILL.md" || got[1] != ".cursor/skills/ex-foo/SKILL.md" {
		t.Errorf("skills localOutPaths = %q", got)
	}
	if got := p.localOutPaths("agents", "bar"); len(got) != 2 ||
		got[0] != ".claude/agents/bar.md" || got[1] != ".cursor/agents/bar.md" {
		t.Errorf("agents localOutPaths = %q", got)
	}
	if got := p.localOutPaths("docs", "x"); got != nil {
		t.Errorf("neutral kind localOutPaths should be nil, got %q", got)
	}
}

// --- declaredSections direct cases ---

func TestDeclaredSections(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.declaredSections("skills", "tdd"); len(got) == 0 {
		t.Error("expected tdd to declare sections")
	}
	if got := p.declaredSections("agents", "code-reviewer"); len(got) == 0 {
		t.Error("expected code-reviewer to declare sections")
	}
	if got := p.declaredSections("docs", "architecture"); len(got) == 0 {
		t.Error("expected architecture to declare sections")
	}
	if got := p.declaredSections("bogus-kind", "x"); got != nil {
		t.Errorf("unknown kind should yield nil, got %v", got)
	}
}

// --- RenderAll: local agent skip + malformed-sidecar error branches ---

func TestRenderAllSkipsLocalAgent(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: [my-local-agent]\n", map[string]string{
		"agents/my-local-agent.yaml": "local: true\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	for _, f := range files {
		if strings.Contains(f.Path, "my-local-agent") {
			t.Errorf("local agent must not be rendered: %#v", f)
		}
	}
}

func TestRenderAllSurfacesMalformedSidecars(t *testing.T) {
	cases := []struct {
		name       string
		cfg        string
		corruptRel string
	}{
		{"skills", "prefix: example\nskills: [tdd]\nagents: []\n", "skills/tdd.yaml"},
		{"agents", "prefix: example\nskills: []\nagents: [code-reviewer]\n", "agents/code-reviewer.yaml"},
		{"docs", "prefix: example\nskills: []\nagents: []\ndocs: [architecture]\n", "docs/architecture.yaml"},
		{"agents-doc", "prefix: example\nskills: []\nagents: []\n", "agents-doc.yaml"},
		{"adr-readme", "prefix: example\nskills: []\nagents: []\n", "adr-readme.yaml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, tc.cfg)
			p, err := Open(root)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			// Corrupt the sidecar after a clean open so RenderAll re-reads it.
			corruptSidecar(t, root, tc.corruptRel)
			if _, err := p.RenderAll(); err == nil {
				t.Fatalf("expected RenderAll to surface the malformed %s sidecar", tc.name)
			}
		})
	}
}

// --- RenderAll/renderTarget: render-time failures via missing/broken parts ---

// A convention part path that is a directory makes os.ReadFile fail with a
// non-ErrNotExist error, exercising planSections' read-error arm. The arm is
// target-agnostic, so one case covers it for all kinds.
func TestRenderAllAssembleErrorOnUnreadablePart(t *testing.T) {
	// Each kind's RenderAll loop has its own error-propagation arm; cover agent,
	// doc, and the agents-doc singleton.
	cases := []struct {
		name, cfg, partDir string
	}{
		{"agent", "prefix: example\nskills: []\nagents: [code-reviewer]\n", ".awf/agents/parts/code-reviewer/doc-currency.md"},
		{"doc", "prefix: example\nskills: []\nagents: []\ndocs: [architecture]\n", ".awf/docs/parts/architecture/overview.md"},
		{"agents-doc", "prefix: example\nskills: []\nagents: []\n", ".awf/parts/agents-doc/identity.md"},
		{"adr-readme", "prefix: example\nskills: []\nagents: []\n", ".awf/parts/adr-readme/intro.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, tc.cfg)
			if err := os.MkdirAll(filepath.Join(root, tc.partDir), 0o755); err != nil {
				t.Fatal(err)
			}
			p, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.RenderAll(); err == nil {
				t.Fatalf("expected RenderAll to fail reading an unreadable %s convention part", tc.name)
			}
		})
	}
}

// Note: a convention part containing template-shaped text no longer makes
// RenderAll fail — parts are raw input (ADR-0034), rendered verbatim. The
// render.Execute error branches are unit-tested directly in internal/render.

// --- renderTarget: template-read error (direct) ---

func TestRenderTargetMissingTemplate(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	sc := config.Sidecar{}
	if _, err := p.renderTarget("skills", "ghost", "skills/ghost/SKILL.md.tmpl", nil, sc, p.data(sc), ".claude/skills/example-ghost/SKILL.md"); err == nil {
		t.Fatal("expected renderTarget to fail reading a nonexistent template")
	}
}

// --- artifactConfigHash: unreadable part (direct) ---

func TestArtifactConfigHashUnreadablePart(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.artifactConfigHash("body", config.Sidecar{}, []string{filepath.Join(root, "does", "not", "exist.md")}); err == nil {
		t.Fatal("expected artifactConfigHash to fail reading a missing part file")
	}
}

// --- resolvedDocs: malformed docs sidecar (direct) ---

func TestResolvedDocsSurfacesMalformedSidecar(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ndocs: [architecture]\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	corruptSidecar(t, root, "docs/architecture.yaml")
	if _, err := p.resolvedDocs(); err == nil {
		t.Fatal("expected resolvedDocs to surface a malformed docs sidecar")
	}
}

// --- checkLocalFrontmatter: malformed sidecar (direct) ---

func TestCheckLocalFrontmatterSurfacesMalformedSidecar(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: [my-local]\nagents: []\n", map[string]string{
		"skills/my-local.yaml": "local: true\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	corruptSidecar(t, root, "skills/my-local.yaml")
	if err := p.checkLocalFrontmatter(func(string, error) {}); err == nil {
		t.Fatal("expected checkLocalFrontmatter to surface a malformed sidecar")
	}
}

// --- localTargetPaths: malformed sidecar (direct) ---

func TestLocalTargetPathsSurfacesMalformedSidecar(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: [my-local]\nagents: []\n", map[string]string{
		"skills/my-local.yaml": "local: true\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	corruptSidecar(t, root, "skills/my-local.yaml")
	if _, err := p.localTargetPaths(); err == nil {
		t.Fatal("expected localTargetPaths to surface a malformed sidecar")
	}
}

// --- Sync: ADR-index generation failure ---

func TestSyncFailsOnMalformedADR(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0001-bad.md"),
		"---\nstatus: [unterminated\n---\n# ADR-0001: Bad\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err == nil {
		t.Fatal("expected Sync to fail generating the ADR index from a malformed ADR")
	}
}

// --- Sync: MkdirAll and WriteFile IO errors via path squatting ---

func TestSyncMkdirAllErrorWhenParentIsFile(t *testing.T) {
	root := scaffold(t, sampleYAML)
	// A regular file squatting on .claude/skills makes MkdirAll of the skill's
	// output directory fail for every user (incl. root).
	testsupport.WriteFile(t, filepath.Join(root, ".claude", "skills"), "i am a file, not a dir\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err == nil {
		t.Fatal("expected Sync to fail when the output parent path is a file")
	}
}

func TestSyncWriteFileErrorWhenOutputIsDir(t *testing.T) {
	root := scaffold(t, sampleYAML)
	// A directory squatting on the SKILL.md output path makes WriteFile fail.
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills", "example-tdd", "SKILL.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err == nil {
		t.Fatal("expected Sync to fail when the output path is a directory")
	}
}

// --- CheckInvariants ---

func TestCheckInvariantsReportsUnbacked(t *testing.T) {
	cfg := "prefix: example\nskills: []\nagents: []\n" +
		"invariants:\n  sources:\n    - globs: [\"*.go\"]\n      marker: \"//\"\n"
	root := scaffold(t, cfg)
	adrBody := testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
		testsupport.WithTitle("0001: First"), testsupport.WithBody("## Invariants\n- `invariant: my-slug`\n## Context\nx\n"))
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0001-first.md"), adrBody)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	findings, _, err := p.CheckInvariants()
	if err != nil {
		t.Fatalf("CheckInvariants: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.Slug == "my-slug" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an unbacked finding for my-slug, got %#v", findings)
	}
}

// --- Check error branches ---

func TestCheckFailsWithoutLock(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Check(); err == nil {
		t.Fatal("expected Check to fail when no lock exists")
	}
}

func TestCheckSurfacesRenderError(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	// Corrupt a sidecar so the post-lock RenderAll inside Check fails.
	corruptSidecar(t, root, "skills/tdd.yaml")
	if _, err := p.Check(); err == nil {
		t.Fatal("expected Check to surface the RenderAll error")
	}
}

func TestCheckFlagsFileWhereKindDirBelongs(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	// A regular file where the .awf/domains sidecar dir would be. The old orphan
	// scan surfaced this as a ReadDir error; the closed-tree sweep (ADR-0086)
	// reports it as unclaimed drift instead — the file is simply not claimable.
	if err := os.WriteFile(filepath.Join(root, ".awf", "domains"), []byte("not a dir\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drift {
		if d.Path == ".awf/domains" && d.Kind == "orphaned" {
			return
		}
	}
	t.Fatalf("expected unclaimed drift for the .awf/domains file, got %#v", drift)
}

func TestCheckReportsLockEntryNoLongerProduced(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	// Drop tdd from config but keep the lock (no re-sync): its lock entry is now
	// orphaned because RenderAll no longer produces it.
	noTDD := strings.Replace(sampleYAML, "skills:\n  - tdd\n", "skills: []\n", 1)
	if err := os.WriteFile(configPath(root), []byte(noTDD), 0o644); err != nil {
		t.Fatal(err)
	}
	p2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p2.Check()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range drift {
		if d.Path == ".claude/skills/example-tdd/SKILL.md" && d.Kind == "orphaned" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected orphaned drift for the unproduced lock entry, got %#v", drift)
	}
}

func TestCheckReportsMissingRenderedFile(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	// Delete an in-sync rendered file so Check's on-disk read reports it missing.
	if err := os.Remove(filepath.Join(root, ".claude", "skills", "example-tdd", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range drift {
		if d.Path == ".claude/skills/example-tdd/SKILL.md" && d.Kind == "missing" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing drift for the deleted file, got %#v", drift)
	}
}

func TestCheckReportsLocalFrontmatterDrift(t *testing.T) {
	cfg := "prefix: example\nskills: [my-local]\nagents: []\n"
	root := scaffoldFiles(t, cfg, map[string]string{"skills/my-local.yaml": "local: true\n"})
	writeLocalSkill(t, root, ".claude/skills/example-my-local/SKILL.md")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	// Delete the local file: Check's local-frontmatter pass appends drift.
	if err := os.Remove(filepath.Join(root, ".claude", "skills", "example-my-local", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range drift {
		if d.Path == ".claude/skills/example-my-local/SKILL.md" && d.Kind == "invalid-frontmatter" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid-frontmatter drift for the absent local file, got %#v", drift)
	}
}

func TestCheckFailsOnMalformedADRIndex(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	// Introduce a malformed ADR; Check regenerates the index and fails.
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0001-bad.md"),
		"---\nstatus: [unterminated\n---\n# ADR-0001: Bad\n")
	if _, err := p.Check(); err == nil {
		t.Fatal("expected Check to fail regenerating the index from a malformed ADR")
	}
}

func TestCheckReportsMissingActiveMD(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\n")
	adrBody := testsupport.ADR("Accepted", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
		testsupport.WithTitle("0001: First"), testsupport.WithBody("## Context\nx\n"))
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0001-first.md"), adrBody)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	// Delete the generated ACTIVE.md: Check reports it missing.
	if err := os.Remove(filepath.Join(root, "docs", "decisions", "ACTIVE.md")); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range drift {
		if strings.HasSuffix(d.Path, "decisions/ACTIVE.md") && d.Kind == "missing" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing drift for ACTIVE.md, got %#v", drift)
	}
}

// --- scaffold: collectVars read error (direct) ---

func TestCollectVarsReadError(t *testing.T) {
	if err := collectVars(fstest.MapFS{}, "missing.tmpl", map[string]bool{}); err == nil {
		t.Fatal("expected collectVars to fail reading a nonexistent template")
	}
}

// invariant: dead-reference-gated
func TestCheckDetectsDeadReference(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\n", map[string]string{
		"parts/agents-doc/identity.md": "See [missing](no/such/file.md).\n",
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
	found := false
	for _, d := range drift {
		if d.Kind == "dead-reference" && d.Detail == "no/such/file.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected dead-reference drift, got %#v", drift)
	}
}

// A leading-/ target is repo-root-relative (not joined under the linking
// file's directory), and a target escaping the repo root is dead by
// definition — never validated against host paths outside the repo.
func TestCheckDeadRefsAbsoluteAndEscapingTargets(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\n", map[string]string{
		// agents-doc renders at the repo root; workflow doc at docs/workflow.md.
		// /docs/workflow.md from inside docs/ must resolve to the repo root copy,
		// not docs/docs/workflow.md.
		"parts/doc-standard/principles.md": "See [w](/docs/workflow.md) and [out](../../outside.md).\n",
	})
	testsupport.WriteFile(t, filepath.Join(root, "..", "outside.md"), "outside\n")
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
	dead := map[string]bool{}
	for _, d := range drift {
		if d.Kind == "dead-reference" {
			dead[d.Detail] = true
		}
	}
	if dead["/docs/workflow.md"] {
		t.Errorf("root-relative target to an existing file flagged dead: %#v", drift)
	}
	if !dead["../../outside.md"] {
		t.Errorf("root-escaping target not flagged dead (stat'd outside the repo): %#v", drift)
	}
}

func TestIsManagedMarkdownExcludesBootstrap(t *testing.T) {
	if isManagedMarkdown("bootstrap/awf-bootstrap.sh.tmpl") {
		t.Error("awf-bootstrap.sh template must not be scanned for dead references")
	}
	if !isManagedMarkdown("docs/architecture.md.tmpl") {
		t.Error("a managed doc template must remain in the dead-reference scan")
	}
	if isManagedMarkdown(memoryTID) {
		t.Error("the memory gitignore template must not be scanned for dead references")
	}
}

// --- NewADR ---

func TestProjectNewADR(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// docs/decisions/template.md is a rendered singleton (ADR-0021) — it only
	// exists on disk after a sync, unlike CheckInvariants/Audit above which read
	// hand-written fixture ADRs directly.
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	path, err := p.NewADR("My Plan Title")
	if err != nil {
		t.Fatalf("NewADR: %v", err)
	}
	want := filepath.Join(root, "docs", "decisions", "0001-my-plan-title.md")
	if path != want {
		t.Errorf("NewADR path = %q, want %q", path, want)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("created file not found: %v", err)
	}
}
