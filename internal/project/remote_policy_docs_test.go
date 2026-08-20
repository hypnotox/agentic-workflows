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
		"These checks do not gate remote updates.",
		"ruleset `main` (ID `18766557`)",
		"with no bypass actors, it requires signed commits and blocks non-fast-forward updates and deletion",
		"does not require CI status checks before accepting an update",
		"provide post-push detection and a backstop for bypassed hooks",
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("self-hosted workflow missing remote-policy contract %q", want)
		}
	}
	for _, stale := range []string{
		"GitHub branch protection is the final publication boundary.",
		"CI is the enforcement backstop",
	} {
		if strings.Contains(workflow, stale) {
			t.Errorf("self-hosted workflow retains stale remote-policy claim %q", stale)
		}
	}

	releasing := read("docs/releasing.md")
	for _, want := range []string{
		"Local hooks are optional preflight.",
		"wait for that commit's `CI` workflow run to succeed",
		"No verified protected-tag ruleset gates tag creation.",
		"`.github/workflows/release.yml` is the final artifact-publication gate",
		"`./x gate && ./x check` before",
	} {
		if !strings.Contains(releasing, want) {
			t.Errorf("self-hosted release guidance missing remote-policy contract %q", want)
		}
	}
}
