package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// The happy path proves the sidecar-paths plumbing end to end (ADR-0077): a
// domain with sidecar `paths` reaches the domain-code-staleness rule, and a
// configured domain without `paths` stays inert.
func TestAuditBuildsDomainPathsFromSidecars(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, ".awf", "domains"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := gitfixture.Commit(t, repo, dir, "base", map[string]string{
		".awf/config.yaml":          "prefix: example\nskills: []\nagents: []\ndomains:\n  - tooling\n  - rendering\n",
		".awf/domains/tooling.yaml": "paths:\n  - cmd/**\n",
		"base.txt":                  "x\n",
	})
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := wt.Checkout(&git.CheckoutOptions{Hash: base, Branch: plumbing.NewBranchReferenceName("feature"), Create: true}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitfixture.Commit(t, repo, dir, "feat: churn tooling territory", map[string]string{
		"cmd/x.go": "package main\n",
	})
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	findings, _, err := p.Audit(base.String(), "HEAD")
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	var hits []string
	for _, f := range findings {
		if f.Rule == "domain-code-staleness" {
			hits = append(hits, f.Detail)
		}
	}
	if len(hits) != 1 || !strings.Contains(hits[0], `"tooling"`) {
		t.Errorf("want exactly one tooling warning (rendering has no paths), got %v", hits)
	}
}

func TestAuditRejectsMalformedDomainPaths(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ndomains:\n  - tooling\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "domains", "tooling.yaml"), "paths:\n  - '['\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.Audit("HEAD", "HEAD"); err == nil || !strings.Contains(err.Error(), `domain "tooling" paths`) {
		t.Fatalf("want malformed-pattern error naming the domain, got %v", err)
	}
}

func TestAuditPropagatesDomainSidecarReadError(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ndomains:\n  - tooling\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// Created after Open: the ADR-0086 open-time validation reads domain
	// sidecars too, so a pre-existing unreadable sidecar would fail there -
	// this test pins Audit's own read-error propagation.
	if err := os.MkdirAll(filepath.Join(root, ".awf", "domains", "tooling.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.Audit("HEAD", "HEAD"); err == nil || !strings.Contains(err.Error(), "sidecar") {
		t.Fatalf("want sidecar read error, got %v", err)
	}
}
