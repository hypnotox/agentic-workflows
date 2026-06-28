package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// corruptSidecar overwrites a sidecar (relative to .awf) with YAML that
// the strict decoder rejects (unknown field), so a fresh Sidecar read fails.
func corruptSidecar(t *testing.T, root, rel string) {
	t.Helper()
	writeFileAt(t, root, filepath.Join(".awf", rel), "bogusUnknownField: true\n")
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
	root := scaffold(t, "prefix: \"\"\nskills: []\nagents: []\nhooks: []\n")
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected Open to fail validation on empty prefix")
	}
	if !strings.Contains(err.Error(), "prefix") {
		t.Errorf("error should mention prefix, got: %v", err)
	}
}

func TestOpenRejectsMalformedSkillSidecar(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: [tdd]\nagents: []\nhooks: []\n", map[string]string{
		"skills/tdd.yaml": "bogusUnknownField: true\n",
	})
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected Open to fail on a malformed skill sidecar")
	}
}

func TestOpenRejectsMalformedAgentsDocSidecar(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\nhooks: []\n", map[string]string{
		"agents-doc.yaml": "bogusUnknownField: true\n",
	})
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected Open to fail on a malformed agents-doc sidecar")
	}
}

func TestOpenRejectsUnknownAgentsDocSection(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\nhooks: []\n", map[string]string{
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
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\nhooks: []\n", map[string]string{
		"adr-readme.yaml": "bogusUnknownField: true\n",
	})
	if _, err := Open(root); err == nil {
		t.Fatal("expected Open to fail on a malformed adr-readme sidecar")
	}
}

func TestOpenRejectsUnknownAdrReadmeSection(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\nhooks: []\n", map[string]string{
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

// --- localOutPath direct cases ---

func TestLocalOutPath(t *testing.T) {
	p := &Project{Cfg: &config.Config{Prefix: "ex"}, Target: claudeTarget}
	if got := p.localOutPath("skills", "foo"); got != ".claude/skills/ex-foo/SKILL.md" {
		t.Errorf("skills localOutPath = %q", got)
	}
	if got := p.localOutPath("agents", "bar"); got != ".claude/agents/bar.md" {
		t.Errorf("agents localOutPath = %q", got)
	}
	if got := p.localOutPath("docs", "x"); got != "" {
		t.Errorf("unknown kind localOutPath should be empty, got %q", got)
	}
}

// --- declaredSections direct cases ---

func TestDeclaredSections(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\nhooks: []\n")
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
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: [my-local-agent]\nhooks: []\n", map[string]string{
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
		{"skills", "prefix: example\nskills: [tdd]\nagents: []\nhooks: []\n", "skills/tdd.yaml"},
		{"agents", "prefix: example\nskills: []\nagents: [code-reviewer]\nhooks: []\n", "agents/code-reviewer.yaml"},
		{"docs", "prefix: example\nskills: []\nagents: []\nhooks: []\ndocs: [architecture]\n", "docs/architecture.yaml"},
		{"agents-doc", "prefix: example\nskills: []\nagents: []\nhooks: []\n", "agents-doc.yaml"},
		{"adr-readme", "prefix: example\nskills: []\nagents: []\nhooks: []\n", "adr-readme.yaml"},
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

// A hook body with no trailing newline exercises injectBanner's fallback arm
// (real hook templates always carry a shebang + newline, so Sync never hits it).
func TestInjectBannerHookNoNewline(t *testing.T) {
	got := injectBanner("#!/usr/bin/env bash", "hooks/pre-commit")
	if !strings.Contains(got, "# GENERATED by awf") {
		t.Errorf("hook no-newline fallback missing banner: %q", got)
	}
}

// A convention part path that is a directory makes os.ReadFile fail with a
// non-ErrNotExist error, exercising planSections' read-error arm. The arm is
// target-agnostic, so one case covers it for all kinds.
func TestRenderAllAssembleErrorOnUnreadablePart(t *testing.T) {
	// Each kind's RenderAll loop has its own error-propagation arm; cover agent,
	// doc, and the agents-doc singleton.
	cases := []struct {
		name, cfg, partDir string
	}{
		{"agent", "prefix: example\nskills: []\nagents: [code-reviewer]\nhooks: []\n", ".awf/agents/parts/code-reviewer/doc-currency.md"},
		{"doc", "prefix: example\nskills: []\nagents: []\nhooks: []\ndocs: [architecture]\n", ".awf/docs/parts/architecture/overview.md"},
		{"agents-doc", "prefix: example\nskills: []\nagents: []\nhooks: []\n", ".awf/parts/agents-doc/identity.md"},
		{"adr-readme", "prefix: example\nskills: []\nagents: []\nhooks: []\n", ".awf/parts/adr-readme/intro.md"},
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

func TestRenderAllSkillExecuteErrorOnBrokenPart(t *testing.T) {
	cfg := "prefix: example\nvars:\n  testCmd: t\n  gateCmd: g\nskills: [tdd]\nagents: []\nhooks: []\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		// A convention part injecting an unterminated template action; Assemble
		// succeeds (text substitution) but Execute's parse fails.
		"skills/parts/tdd/notes.md": "{{ .broken syntax\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.RenderAll(); err == nil {
		t.Fatal("expected RenderAll to fail executing a template with a broken convention part")
	}
}

// --- renderTarget: template-read error (direct) ---

func TestRenderTargetMissingTemplate(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\nhooks: []\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	sc := config.Sidecar{}
	if _, err := p.renderTarget("skills", "ghost", "skills/ghost/SKILL.md.tmpl", nil, sc, p.data(sc), ".claude/skills/example-ghost/SKILL.md"); err == nil {
		t.Fatal("expected renderTarget to fail reading a nonexistent template")
	}
}

// --- targetConfigHash: unreadable part (direct) ---

func TestTargetConfigHashUnreadablePart(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\nhooks: []\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.targetConfigHash("body", config.Sidecar{}, []string{filepath.Join(root, "does", "not", "exist.md")}); err == nil {
		t.Fatal("expected targetConfigHash to fail reading a missing part file")
	}
}

// --- resolvedDocs: malformed docs sidecar (direct) ---

func TestResolvedDocsSurfacesMalformedSidecar(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\nhooks: []\ndocs: [architecture]\n")
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
	root := scaffoldFiles(t, "prefix: example\nskills: [my-local]\nagents: []\nhooks: []\n", map[string]string{
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

// --- Sync: ADR-index generation failure ---

func TestSyncFailsOnMalformedADR(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\nhooks: []\n")
	writeFileAt(t, root, filepath.Join("docs", "decisions", "0001-bad.md"),
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
	writeFileAt(t, root, filepath.Join(".claude", "skills"), "i am a file, not a dir\n")
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
	cfg := "prefix: example\nskills: []\nagents: []\nhooks: []\n" +
		"invariants:\n  sources:\n    - globs: [\"*.go\"]\n      marker: \"//\"\n"
	root := scaffold(t, cfg)
	adrBody := "---\nstatus: Implemented\ndate: 2026-06-25\ntags: [x]\n---\n" +
		"# ADR-0001: First\n## Invariants\n- `inv: my-slug`\n## Context\nx\n"
	writeFileAt(t, root, filepath.Join("docs", "decisions", "0001-first.md"), adrBody)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := p.CheckInvariants()
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

// --- orphans: non-dir in parts root, and non-md/subdir inside a section dir ---

func TestOrphansSkipsNonDirAndNonMarkdown(t *testing.T) {
	cfg := "prefix: example\n" + debuggingVars + "skills: [debugging]\nagents: []\nhooks: []\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		// A stray non-directory entry directly under skills/parts/.
		"skills/parts/stray-file.txt": "not a target dir\n",
		// Inside an enabled target's parts dir: a non-.md file is skipped...
		"skills/parts/debugging/notes.txt": "ignored\n",
		// ...and a valid declared-section part stays clean.
		"skills/parts/debugging/debugging-surfaces.md": "## Surfaces\n\nbody\n",
	})
	// A subdirectory inside the section parts dir is skipped too.
	writeFileAt(t, root, filepath.Join(".awf", "skills", "parts", "debugging", "sub", "x.md"), "nested\n")

	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.orphans()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drift {
		if strings.Contains(d.Path, "stray-file.txt") || strings.Contains(d.Path, "notes.txt") || strings.Contains(d.Path, "/sub") {
			t.Errorf("non-target/non-markdown entries must not be flagged as orphans: %#v", d)
		}
	}
}

// TestOrphansSurfacesReadDirFault asserts orphans() returns a non-absent
// ReadDir fault rather than silently treating it as "kind branch absent".
func TestOrphansSurfacesReadDirFault(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// Put a regular file where the .awf/skills directory would be, after Open, so
	// orphans()' os.ReadDir returns a non-ErrNotExist error (ENOTDIR).
	if err := os.WriteFile(filepath.Join(root, ".awf", "skills"), []byte("not a dir\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.orphans(); err == nil {
		t.Fatal("expected a ReadDir fault to surface, got nil")
	}
}

// TestOrphansSurfacesPartsReadDirFault covers the parts-dir arm: the kind dir is
// readable but its parts/ path is a regular file, so the second os.ReadDir faults.
func TestOrphansSurfacesPartsReadDirFault(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".awf", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "skills", "parts"), []byte("not a dir\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.orphans(); err == nil {
		t.Fatal("expected a parts-dir ReadDir fault to surface, got nil")
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

func TestCheckSurfacesOrphanScanError(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	// Place a regular file where the .awf/domains sidecar dir would be. No domains
	// are enabled, so RenderAll skips it and the fault first surfaces in orphans().
	if err := os.WriteFile(filepath.Join(root, ".awf", "domains"), []byte("not a dir\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Check(); err == nil {
		t.Fatal("expected Check to surface the orphan-scan error")
	}
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
	cfg := "prefix: example\nskills: [my-local]\nagents: []\nhooks: []\n"
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
	writeFileAt(t, root, filepath.Join("docs", "decisions", "0001-bad.md"),
		"---\nstatus: [unterminated\n---\n# ADR-0001: Bad\n")
	if _, err := p.Check(); err == nil {
		t.Fatal("expected Check to fail regenerating the index from a malformed ADR")
	}
}

func TestCheckReportsMissingActiveMD(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\nhooks: []\n")
	adrBody := "---\nstatus: Accepted\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-0001: First\n## Context\nx\n"
	writeFileAt(t, root, filepath.Join("docs", "decisions", "0001-first.md"), adrBody)
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
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\nhooks: []\n", map[string]string{
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
