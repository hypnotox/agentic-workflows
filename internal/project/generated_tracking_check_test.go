package project

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// invariant: rendering/sync-and-drift:generated-artifacts-tracked (TestCheckReportRequiresGeneratedArtifactsInIndex)
func TestCheckReportRequiresGeneratedArtifactsInIndex(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	testsupport.WriteAwfConfig(t, root, withTestGateCmd("prefix: example\nintegrationBranch: main\nvars: {}\n"))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	excludes := filepath.Join(home, "global-ignore")
	if err := os.WriteFile(excludes, []byte("AGENTS.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[core]\n\texcludesfile = "+excludes+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	working, err := testRepo(p).WorkingPaths(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(working, "AGENTS.md") {
		t.Fatalf("global ignore did not hide untracked generated output: %v", working)
	}
	report, err := checkReportProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(report.Drift, func(finding manifest.Drift) bool {
		return finding.Path == "AGENTS.md" && finding.Kind == "untracked"
	}) {
		t.Fatalf("globally ignored generated output was not reported: %#v", report.Drift)
	}

	gitfixture.AddAll(t, repo)
	report, err = checkReportProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(report.Drift, func(finding manifest.Drift) bool {
		return finding.Path == "AGENTS.md" && finding.Kind == "untracked"
	}) {
		t.Fatalf("tracked ignored generated output reported untracked: %#v", report.Drift)
	}
	gitfixture.StageRemoval(t, repo, "AGENTS.md", ".awf/awf.lock")

	report, err = checkReportProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, finding := range report.Drift {
		if finding.Kind == "untracked" {
			got = append(got, finding.Path)
		}
		if (finding.Path == "AGENTS.md" || finding.Path == ".awf/awf.lock") && finding.Kind == "missing" {
			t.Fatalf("missing duplicates untracked root cause: %#v", report.Drift)
		}
	}
	if joined := strings.Join(got, ","); joined != ".awf/awf.lock,AGENTS.md" {
		t.Fatalf("untracked findings = %q", joined)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "index"), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := checkReportProject(p, testContext(t)); err == nil {
		t.Fatal("corrupt tracking index accepted")
	}
}

func TestCheckLockedFilesSuppressesMissingForUntrackedOutputs(t *testing.T) {
	root := t.TempDir()
	p := testStateWith(testState(&config.Config{}), root, resident.NewRoots(root, root), false, catalog.Standard, catalog.Standard, nil)
	rendered := map[string]RenderedFile{
		"regen.md":  {Path: "regen.md", Content: "regen", Policy: OutputPolicy{Regenerate: true}},
		"normal.md": {Path: "normal.md", Content: "normal"},
	}
	lock := &manifest.Lock{Files: map[string]manifest.Entry{
		"regen.md":  {},
		"normal.md": {OutputHash: manifest.Hash([]byte("normal"))},
	}}
	tracking := []manifest.Drift{{Path: "regen.md", Kind: "untracked"}, {Path: "normal.md", Kind: "untracked"}}
	if got := checkLockedDrift(renderInputsForTest(p).residentRoots(), lock, rendered, tracking); len(got) != 0 {
		t.Fatalf("untracked missing files = %#v", got)
	}
	for path := range rendered {
		if err := os.Mkdir(filepath.Join(root, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := checkLockedDrift(renderInputsForTest(p).residentRoots(), lock, rendered, tracking)
	if len(got) != 2 || !slices.ContainsFunc(got, func(finding manifest.Drift) bool {
		return finding.Path == "regen.md" && finding.Kind == "missing"
	}) || !slices.ContainsFunc(got, func(finding manifest.Drift) bool {
		return finding.Path == "normal.md" && finding.Kind == "missing"
	}) {
		t.Fatalf("non-not-exist read errors suppressed as untracked root causes: %#v", got)
	}
}

// invariant: rendering/sync-and-drift:generated-artifacts-tracked (TestCheckGeneratedTrackingNoGitAndNestedResidentExclusion)
func TestCheckGeneratedTrackingNoGitAndNestedResidentExclusion(t *testing.T) {
	t.Run("no Git", func(t *testing.T) {
		root := scaffold(t, withTestGateCmd("prefix: example\nintegrationBranch: main\nvars: {}\n"))
		p, err := Open(testContext(t), root)
		if err != nil {
			t.Fatal(err)
		}
		if err := syncProject(p); err != nil {
			t.Fatal(err)
		}
		report, err := checkReportProject(p, testContext(t))
		if err != nil {
			t.Fatal(err)
		}
		const unavailable = "generated-artifact tracking is unavailable outside a Git repository"
		count := 0
		for _, item := range report.Result.Information() {
			if item.Evidence.Detail == unavailable {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("no-Git aggregate tracking information count = %d, want 1", count)
		}
	})
	t.Run("nested resident output", func(t *testing.T) {
		fixture := gitfixture.InitRepo(t)
		root := filepath.Join(fixture.Root(), "nested")
		testsupport.WriteAwfConfig(t, root, withTestGateCmd("prefix: example\nintegrationBranch: main\nvars: {}\n"))
		p, err := Open(testContext(t), root)
		if err != nil {
			t.Fatal(err)
		}
		if !p.nested() {
			t.Fatal("Loader.Open did not preserve the containing-repository prefix")
		}
		if err := syncProject(p); err != nil {
			t.Fatal(err)
		}
		gitfixture.AddAll(t, fixture)
		report, err := checkReportProject(p, testContext(t))
		if err != nil {
			t.Fatal(err)
		}
		for _, finding := range report.Drift {
			if finding.Kind == "untracked" && resident.IsResidentPath(finding.Path) {
				t.Fatalf("nested resident output reported untracked: %#v", report.Drift)
			}
		}
	})
}
