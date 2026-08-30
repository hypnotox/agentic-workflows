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
		"ruleset `main` (ID `18766557`) is the final remote control for publishing `main`",
		"requires signed commits and blocks non-fast-forward updates and deletion, but has no required-status rule",
		"`CI / gate` is therefore definitive post-push assurance rather than a pre-update requirement on `main`",
		"`release tags` ruleset (ID `21631403`) requires `CI / gate` and the retired `CI / release-config`",
		"before `refs/tags/v*` can be created or updated",
		"One aggregate `CI / gate` result is the definitive hosted verdict.",
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("self-hosted workflow missing remote-policy contract %q", want)
		}
	}
	for _, stale := range []string{
		"GitHub branch protection is the final publication boundary.",
		"CI is the enforcement backstop",
		"currently requires the GitHub Actions checks `CI / gate` and the retired `CI / release-config`",
		"release tags` ruleset (ID `21631403`) currently requires the same app-bound checks",
	} {
		if strings.Contains(workflow, stale) {
			t.Errorf("self-hosted workflow retains stale remote-policy claim %q", stale)
		}
	}

	releasing := read("docs/releasing.md")
	for _, want := range []string{
		"Local hooks are optional preflight.",
		"wait for that commit's `CI / gate` check to succeed",
		"live ruleset still requires the retired `CI / release-config` status",
		"repository owner must update the remote requirement",
		"needs-bound credential-bearing GoReleaser job",
		"Production snapshot construction and portability validation belong to the read-only release verification job.",
		"Ordinary local Go tests use synthetic archive fixtures and never invoke GoReleaser.",
	} {
		if !strings.Contains(releasing, want) {
			t.Errorf("self-hosted release guidance missing remote-policy contract %q", want)
		}
	}

	testingGuide := read("docs/testing.md")
	for _, want := range []string{
		"Coverage percentages may be reported by external services but are informational.",
		"aggregate `CI / gate` job is the definitive repository verdict",
	} {
		if !strings.Contains(testingGuide, want) {
			t.Errorf("self-hosted testing guidance missing verification contract %q", want)
		}
	}
	if strings.Contains(testingGuide, "the gate blocks merges") {
		t.Error("self-hosted testing guidance retains stale local merge-gating claim")
	}
}

func TestReleaseProvidesRestrictedRootlessExtraction(t *testing.T) {
	release, err := os.ReadFile(filepath.Join(repoRootDir(t), ".github/workflows/release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"kernel.apparmor_restrict_unprivileged_userns",
		"unshare --user --map-root-user true",
		"go run ./cmd/releasecheck --verify-archives dist",
	} {
		if !strings.Contains(string(release), want) {
			t.Errorf("release verification does not provide %q", want)
		}
	}

	ci, err := os.ReadFile(filepath.Join(repoRootDir(t), ".github/workflows/ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, stale := range []string{"release-config", "goreleaser/goreleaser-action", "go run ./cmd/releasecheck --verify-archives"} {
		if strings.Contains(string(ci), stale) {
			t.Errorf("CI retains release-only assurance %q", stale)
		}
	}
}
