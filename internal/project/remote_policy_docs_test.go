package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelfHostedRemotePolicyDocumentation(t *testing.T) {
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(repoRootDir(t), path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return string(body)
	}

	workflow := read("docs/workflow.md")
	for _, want := range []string{
		"optional client-side preflight",
		"These checks do not gate remote updates by themselves.",
		"ruleset `main` (ID `18766557`)",
		"with no bypass actors, it requires signed commits, blocks non-fast-forward updates and deletion",
		"requires the GitHub Actions checks `CI / gate` and `CI / release-config` on the exact candidate revision",
		"`release tags` ruleset (ID `21631403`)",
		"before `refs/tags/v*` can be created or updated",
		"provide post-push detection and a backstop for bypassed hooks",
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("self-hosted workflow missing remote-policy contract %q", want)
		}
	}
	for _, stale := range []string{
		"GitHub branch protection is the final publication boundary.",
		"CI is the enforcement backstop",
		"does not require CI status checks before accepting an update",
	} {
		if strings.Contains(workflow, stale) {
			t.Errorf("self-hosted workflow retains stale remote-policy claim %q", stale)
		}
	}

	releasing := read("docs/releasing.md")
	for _, want := range []string{
		"Local hooks are optional preflight.",
		"wait for that commit's `CI / gate` and `CI / release-config` checks to succeed",
		"live GitHub `release tags` ruleset covers `refs/tags/v*` without bypass actors",
		"requires successful `CI / gate` and `CI / release-config` checks",
		"needs-bound credential-bearing GoReleaser job",
		"`./x gate && ./x check` before",
	} {
		if !strings.Contains(releasing, want) {
			t.Errorf("self-hosted release guidance missing remote-policy contract %q", want)
		}
	}

	testingGuide := read("docs/testing.md")
	for _, want := range []string{
		"Codecov is informational",
		"exact coverage policy is enforced by `./x gate`",
		"hosted `CI / gate` check required for protected `main` and release tags",
	} {
		if !strings.Contains(testingGuide, want) {
			t.Errorf("self-hosted testing guidance missing remote-policy contract %q", want)
		}
	}
	if strings.Contains(testingGuide, "the gate blocks merges") {
		t.Error("self-hosted testing guidance retains stale local merge-gating claim")
	}
}
