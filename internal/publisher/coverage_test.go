package publisher

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
	_, err := loadTestSession(testContext(t), t.TempDir())
	if err == nil {
		t.Fatal("expected Open to fail with no config.yaml")
	}
}

func TestOpenRejectsEmptyPrefix(t *testing.T) {
	root := scaffold(t, "prefix: \"\"\n")
	_, err := loadTestSession(testContext(t), root)
	if err == nil {
		t.Fatal("expected Open to fail validation on empty prefix")
	}
	if !strings.Contains(err.Error(), "prefix") {
		t.Errorf("error should mention prefix, got: %v", err)
	}
}

func TestOpenRejectsMalformedSkillSidecar(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n", map[string]string{
		"skills/awf-maintenance.yaml": "bogusUnknownField: true\n",
	})
	_, err := loadTestSession(testContext(t), root)
	if err == nil {
		t.Fatal("expected Open to fail on a malformed skill sidecar")
	}
}

func TestOpenRejectsMalformedAgentsDocSidecar(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n", map[string]string{
		"agents-doc.yaml": "bogusUnknownField: true\n",
	})
	_, err := loadTestSession(testContext(t), root)
	if err == nil {
		t.Fatal("expected Open to fail on a malformed agents-doc sidecar")
	}
}

func TestOpenRejectsUnknownAgentsDocSection(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n", map[string]string{
		"agents-doc.yaml": "sections:\n  not-a-real-section:\n    drop: true\n",
	})
	_, err := loadTestSession(testContext(t), root)
	if err == nil {
		t.Fatal("expected Open to reject an undeclared agents-doc section")
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

// --- declaredSections direct cases ---

func TestDeclaredSections(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if got := declaredSections(renderInputsForTest(p), "skills", "awf-maintenance"); len(got) == 0 {
		t.Error("expected awf-maintenance to declare sections")
	}
	if got := declaredSections(renderInputsForTest(p), "docs", "architecture"); len(got) == 0 {
		t.Error("expected architecture to declare sections")
	}
	if got := declaredSections(renderInputsForTest(p), "bogus-kind", "x"); got != nil {
		t.Errorf("unknown kind should yield nil, got %v", got)
	}
}

// --- RenderAll malformed-sidecar error branches ---

func TestRenderAllSurfacesMalformedSidecars(t *testing.T) {
	cases := []struct {
		name       string
		cfg        string
		corruptRel string
	}{
		{"skills", "prefix: example\nintegrationBranch: main\n", "skills/awf-effort.yaml"},
		{"docs", "prefix: example\nintegrationBranch: main\n", "docs/architecture.yaml"},
		{"agents-doc", "prefix: example\nintegrationBranch: main\n", "agents-doc.yaml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, tc.cfg)
			p, err := loadTestSession(testContext(t), root)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			// Corrupt the sidecar after a clean open so RenderAll re-reads it.
			corruptSidecar(t, root, tc.corruptRel)
			if _, err := renderAll(p); err == nil {
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
		{"doc", "prefix: example\nintegrationBranch: main\n", ".awf/docs/parts/architecture/overview.md"},
		{"agents-doc", "prefix: example\nintegrationBranch: main\n", ".awf/parts/agents-doc/identity.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, tc.cfg)
			if err := os.MkdirAll(filepath.Join(root, tc.partDir), 0o755); err != nil {
				t.Fatal(err)
			}
			p, err := loadTestSession(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := renderAll(p); err == nil {
				t.Fatalf("expected RenderAll to fail reading an unreadable %s convention part", tc.name)
			}
		})
	}
}

// Note: a convention part containing template-shaped text no longer makes
// RenderAll fail - parts are raw input (ADR-0034), rendered verbatim. The
// render.Execute error branches are unit-tested directly in internal/render.

// --- renderTarget: template-read error (direct) ---

func TestRenderTargetMissingTemplate(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	sc := config.Sidecar{}
	if _, err := renderTarget(renderInputsForTest(p), "skills", "ghost", "skills/ghost/SKILL.md.tmpl", nil, sc, projectData(renderInputsForTest(p), sc), ".claude/skills/example-ghost/SKILL.md"); err == nil {
		t.Fatal("expected renderTarget to fail reading a nonexistent template")
	}
}

// --- artifactConfigHash: unreadable part (direct) ---

func TestArtifactConfigHashUnreadablePart(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := artifactConfigHash(renderInputsForTest(p), "body", config.Sidecar{}, []string{filepath.Join(root, "does", "not", "exist.md")}); err == nil {
		t.Fatal("expected artifactConfigHash to fail reading a missing part file")
	}
}

// --- resolvedDocs: malformed docs sidecar (direct) ---

// --- Sync: MkdirAll and WriteFile IO errors via path squatting ---

func TestSyncMkdirAllErrorWhenParentIsFile(t *testing.T) {
	root := scaffold(t, sampleYAML)
	// A regular file squatting on .claude/skills makes MkdirAll of the skill's
	// output directory fail for every user (incl. root).
	testsupport.WriteFile(t, filepath.Join(root, ".claude", "skills"), "i am a file, not a dir\n")
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err == nil {
		t.Fatal("expected Sync to fail when the output parent path is a file")
	}
}

func TestSyncWriteFileErrorWhenOutputIsDir(t *testing.T) {
	root := scaffold(t, sampleYAML)
	// A directory squatting on the SKILL.md output path makes WriteFile fail.
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills", "awf-maintenance", "SKILL.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err == nil {
		t.Fatal("expected Sync to fail when the output path is a directory")
	}
}

// --- Check error branches ---

func TestCheckFailsWithoutLock(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := checkProject(p, testContext(t)); err == nil {
		t.Fatal("expected Check to fail when no lock exists")
	}
}

func TestCheckSurfacesRenderError(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	// Corrupt a sidecar so the post-lock RenderAll inside Check fails.
	corruptSidecar(t, root, "skills/awf-maintenance.yaml")
	if _, err := checkProject(p, testContext(t)); err == nil {
		t.Fatal("expected Check to surface the RenderAll error")
	}
}

func TestCheckFlagsFileWhereKindDirBelongs(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	// A regular file where the .awf/domains sidecar dir would be. The old orphan
	// scan surfaced this as a ReadDir error; the closed-tree sweep (ADR-0086)
	// reports it as unclaimed drift instead - the file is simply not claimable.
	if err := os.WriteFile(filepath.Join(root, ".awf", "domains"), []byte("not a dir\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drift, err := checkProject(p, testContext(t))
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

// invariant: rendering/sync-and-drift:target-prune-ancestors (TestSyncPrunesRemovedTargetTree)
func TestSyncPrunesRemovedTargetTree(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	// Remove a real rendered target surface from the next output plan. Only a
	// catalog-path removal proves pruning; selection edits do not remove paths.
	if err := os.WriteFile(filepath.Join(root, ".claude", "unrelated.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p = setTestTargets(p, nil)
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(root, ".claude", "skills", "awf-maintenance", "SKILL.md"),
		filepath.Join(root, ".claude", "skills"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("removed target path %s remains: %v", path, err)
		}
	}
	if got, err := os.ReadFile(filepath.Join(root, ".claude", "unrelated.txt")); err != nil || string(got) != "keep\n" {
		t.Errorf("unrelated target ancestor content = %q, %v; want preserved", got, err)
	}
}

func TestCheckReportsMissingRenderedFile(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	// Delete an in-sync rendered file so Check's on-disk read reports it missing.
	if err := os.Remove(filepath.Join(root, ".claude", "skills", "awf-maintenance", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	drift, err := checkProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range drift {
		if d.Path == ".claude/skills/awf-maintenance/SKILL.md" && d.Kind == "missing" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing drift for the deleted file, got %#v", drift)
	}
}

// --- scaffold: collectVars read error (direct) ---

func TestCollectVarsReadError(t *testing.T) {
	if err := collectVars(fstest.MapFS{}, "missing.tmpl", map[string]bool{}); err == nil {
		t.Fatal("expected collectVars to fail reading a nonexistent template")
	}
}

// invariant: rendering/render-engine:dead-reference-gated (TestCheckDetectsDeadReference)
func TestCheckDetectsDeadReference(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n", map[string]string{
		"parts/agents-doc/identity.md": "See [missing](no/such/file.md).\n",
	})
	p, err := loadTestSession(testContext(t), root)
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
// definition - never validated against host paths outside the repo.
func TestCheckDeadRefsAbsoluteAndEscapingTargets(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n", map[string]string{
		// agents-doc renders at the repo root; workflow doc at docs/workflow.md.
		// /docs/workflow.md from inside docs/ must resolve to the repo root copy,
		// not docs/docs/workflow.md.
		"parts/doc-standard/principles.md": "See [w](/docs/workflow.md) and [out](../../outside.md).\n",
	})
	testsupport.WriteFile(t, filepath.Join(root, "..", "outside.md"), "outside\n")
	p, err := loadTestSession(testContext(t), root)
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
