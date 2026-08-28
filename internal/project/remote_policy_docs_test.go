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
		"Production snapshot construction and portability validation belong to the `CI / release-config` job.",
		"Ordinary local Go tests use synthetic archive fixtures and never invoke GoReleaser.",
	} {
		if !strings.Contains(releasing, want) {
			t.Errorf("self-hosted release guidance missing remote-policy contract %q", want)
		}
	}

	testingGuide := read("docs/testing.md")
	for _, want := range []string{
		"Codecov is informational",
		"exact coverage policy is enforced by `./x gate full`",
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

func TestHostedGatesProvideRestrictedRootlessExtraction(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRootDir(t), ".github/workflows/ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"kernel.apparmor_restrict_unprivileged_userns",
		"unshare --user --map-root-user true",
		"go run ./cmd/releasecheck --verify-archives dist",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("CI release-config does not provide %q", want)
		}
	}

	release, err := os.ReadFile(filepath.Join(repoRootDir(t), ".github/workflows/release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, stale := range []string{"Gate (full release assurance)", "Drift check (rendered output matches config)", "unshare --user --map-root-user true"} {
		if strings.Contains(string(release), stale) {
			t.Errorf("release repeats CI-owned assurance %q", stale)
		}
	}
}
