package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestNewADRErrors(t *testing.T) {
	root := gitScaffold(t, defaultFixtureBranch)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newADRProject(p, testContext(t), "Missing Lock"); err == nil {
		t.Fatal("expected missing lock error")
	}
}

// On the integration branch the scaffold allocates a number; off it - another
// branch, a detached HEAD, or a tree with no repository at all - it writes the
// slug-identified pending form (ADR-0202 item 5).
func TestOnIntegrationBranchDegradesOnBranchProbeFailure(t *testing.T) {
	root := gitScaffold(t, defaultFixtureBranch)
	state, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	repo := testRepo(state)
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("invalid head\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if onIntegrationBranch(root, testConfig(state), repo, testContext(t)) {
		t.Fatal("failed branch probe identified the integration branch")
	}
}

func TestNewADRIsBranchAware(t *testing.T) {
	for _, tc := range []struct {
		name     string
		wantBase string
		setup    func(t *testing.T) string
	}{
		{
			name: "on the integration branch numbers", wantBase: "0001-branch-aware-title.md",
			setup: func(t *testing.T) string { return gitScaffold(t, defaultFixtureBranch) },
		},
		{
			name: "another branch is pending", wantBase: "branch-aware-title.md",
			setup: func(t *testing.T) string { return gitScaffold(t, "effort/side") },
		},
		{
			name: "detached HEAD is pending", wantBase: "branch-aware-title.md",
			setup: func(t *testing.T) string {
				root := gitScaffold(t, defaultFixtureBranch)
				repo := gitfixture.At(root)
				gitfixture.NativeCheckout(t, repo, gitfixture.NativeRevParse(t, repo, "HEAD"))
				return root
			},
		},
		{
			name: "no repository is pending", wantBase: "branch-aware-title.md",
			setup: func(t *testing.T) string { return scaffold(t, gitSampleYAML) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.setup(t)
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if err := syncProject(p); err != nil {
				t.Fatal(err)
			}
			decisions := filepath.Join(root, "docs", "decisions")
			if err := os.MkdirAll(decisions, 0o755); err != nil {
				t.Fatal(err)
			}
			template := "---\nformat: current-state-v4\nstatus: Proposed\ndate: YYYY-MM-DD\n---\n# ADR-NNNN: Title\n\n## Status history\n\n- YYYY-MM-DD: Proposed\n"
			if err := os.WriteFile(filepath.Join(decisions, "template.md"), []byte(template), 0o644); err != nil {
				t.Fatal(err)
			}
			path, err := newADRProject(p, testContext(t), "Branch Aware Title")
			if err != nil {
				t.Fatalf("NewADR: %v", err)
			}
			if got := filepath.Base(path); got != tc.wantBase {
				t.Fatalf("scaffolded %q, want %q", got, tc.wantBase)
			}
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), "slug: branch-aware-title\n") {
				t.Errorf("scaffold missing the retained slug key:\n%s", body)
			}
		})
	}
}
