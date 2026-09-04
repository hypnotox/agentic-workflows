package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func readRepositoryFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRootDir(t), filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func TestSelfHostedRemotePolicyDocumentation(t *testing.T) {
	workflow := readRepositoryFile(t, "docs/workflow.md")
	for _, want := range []string{
		"optional client-side preflight",
		"These checks do not gate remote updates by themselves.",
		"ruleset `main` (ID `18766557`) is the final remote control for publishing `main`",
		"requires signed commits and blocks non-fast-forward updates and deletion, but has no required-status rule",
		"`CI / gate` is therefore definitive post-push assurance rather than a pre-update requirement on `main`",
		"`release tags` ruleset (ID `21631403`) requires only `CI / gate` before `refs/tags/v*` can be created or updated",
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
		"requires `CI / gate` and the retired `CI / release-config`",
	} {
		if strings.Contains(workflow, stale) {
			t.Errorf("self-hosted workflow retains stale remote-policy claim %q", stale)
		}
	}

	releasing := readRepositoryFile(t, "docs/releasing.md")
	for _, want := range []string{
		"Local hooks are optional preflight.",
		"wait for that commit's `CI / gate` check to succeed",
		"live GitHub `release tags` ruleset requires successful `CI / gate`",
		"live tag ruleset requires a successful `CI / gate` conclusion for the exact tag SHA before accepting the tag",
		"credential-bearing publish job runs only after both native jobs succeed",
		"Production candidate construction and archive validation belong to the read-only release preparation job",
		"Ordinary local Go tests use synthetic archive fixtures and a locally built Linux/amd64 candidate without invoking GoReleaser.",
	} {
		if !strings.Contains(releasing, want) {
			t.Errorf("self-hosted release guidance missing remote-policy contract %q", want)
		}
	}

	testingGuide := readRepositoryFile(t, "docs/testing.md")
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

func TestActiveAuthorityExcludesRetiredExecutableSurfaces(t *testing.T) {
	root := repoRootDir(t)
	for _, path := range []string{
		".nvmrc",
		".pi/extensions/awf-subagents",
		"templates/pi/awf-subagents",
		"tools/pi-extension-test",
	} {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Errorf("retired executable surface %s remains: %v", path, err)
		}
	}

	for _, path := range []string{
		".github/workflows/ci.yml",
		".github/workflows/release.yml",
		"test-selection.json",
		"x",
	} {
		text := readRepositoryFile(t, path)
		for _, retired := range []string{
			"pi-runtime",
			"pi_runtime",
			"./x pi-test",
			"tools/pi-extension-test/",
			".pi/extensions/awf-subagents/",
		} {
			if strings.Contains(text, retired) {
				t.Errorf("executable authority %s retains %q", path, retired)
			}
		}
	}
}

func TestCanonicalTestingLaneAuthority(t *testing.T) {
	var policy struct {
		Lanes []struct {
			Name string `json:"name"`
		} `json:"lanes"`
	}
	if err := json.Unmarshal([]byte(readRepositoryFile(t, "test-selection.json")), &policy); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(policy.Lanes))
	for _, lane := range policy.Lanes {
		got = append(got, lane.Name)
	}
	sort.Strings(got)
	want := []string{"go", "platform-sensitive", "release-archive", "render-template"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selection policy lanes = %v, want %v", got, want)
	}

	workflow := readRepositoryFile(t, ".github/workflows/ci.yml")
	workflowInventory := `[.lanes[].name] == ["` + strings.Join(want, `", "`) + `"]`
	if !strings.Contains(workflow, workflowInventory) {
		t.Errorf("CI workflow does not enforce selector lane inventory %q", workflowInventory)
	}

	pairs := []struct {
		source    string
		generated string
	}{
		{source: ".awf/parts/workflow/ci.md", generated: "docs/workflow.md"},
		{source: ".awf/topics/parts/tooling/quality-gates/current-state.md", generated: "docs/topics/tooling/quality-gates.md"},
		{source: ".awf/docs/parts/releasing/content.md", generated: "docs/releasing.md"},
		{source: ".awf/docs/parts/testing/gate.md", generated: "docs/testing.md"},
		{source: ".awf/docs/parts/testing/tiers.md", generated: "docs/testing.md"},
	}
	for _, pair := range pairs {
		source := strings.TrimSpace(readRepositoryFile(t, pair.source))
		generated := readRepositoryFile(t, pair.generated)
		if !strings.Contains(generated, source) {
			t.Errorf("generated authority %s does not contain authored source %s verbatim", pair.generated, pair.source)
		}
	}

	gate := readRepositoryFile(t, ".awf/docs/parts/testing/gate.md")
	for _, lane := range want {
		if !strings.Contains(gate, "`"+lane+"`") {
			t.Errorf("testing gate authority omits selector lane %q", lane)
		}
	}
	if lanes := markdownLaneTable(readRepositoryFile(t, ".awf/docs/parts/testing/tiers.md")); !reflect.DeepEqual(lanes, want) {
		t.Errorf("authored testing lane table = %v, want %v", lanes, want)
	}
	generatedTiers := markdownSection(readRepositoryFile(t, "docs/testing.md"), "## Tiers and lanes")
	if lanes := markdownLaneTable(generatedTiers); !reflect.DeepEqual(lanes, want) {
		t.Errorf("generated testing lane table = %v, want %v", lanes, want)
	}
}

func markdownSection(text, heading string) string {
	start := strings.Index(text, heading)
	if start < 0 {
		return ""
	}
	section := text[start+len(heading):]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	return section
}

func markdownLaneTable(text string) []string {
	var lanes []string
	for _, line := range strings.Split(text, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 4 {
			continue
		}
		name := strings.Trim(strings.TrimSpace(cells[1]), "`")
		if name == "" || name == "Lane" || strings.Trim(name, "-") == "" {
			continue
		}
		lanes = append(lanes, name)
	}
	return lanes
}

func TestReadmeScopesHarnessPackagesAsOptional(t *testing.T) {
	readme := readRepositoryFile(t, "README.md")
	for _, optionalHarnessInstall := range []string{
		"For Claude Code harness use, optionally install `agentic-skills` to add its generic skills and roles:",
		"For Pi harness use, optionally install [`pi-tools`](https://github.com/hypnotox/pi-tools) first, then `agentic-skills`:",
	} {
		if !strings.Contains(readme, optionalHarnessInstall) {
			t.Errorf("README does not scope harness package installation to optional harness use: %q", optionalHarnessInstall)
		}
	}
	if !strings.Contains(readme, "binary remains offline and functional when those optional operator-managed capabilities are absent.") {
		t.Error("README does not preserve AWF's package-independent binary contract")
	}
	for _, stale := range []string{
		"For Claude Code, install `agentic-skills` before initializing or upgrading AWF:",
		"For Pi, install [`pi-tools`](https://github.com/hypnotox/pi-tools) first, then `agentic-skills`:",
	} {
		if strings.Contains(readme, stale) {
			t.Errorf("README retains stale prerequisite wording %q", stale)
		}
	}
}

func TestReleaseProvidesRestrictedRootlessExtraction(t *testing.T) {
	release := readRepositoryFile(t, ".github/workflows/release.yml")
	for _, want := range []string{
		"kernel.apparmor_restrict_unprivileged_userns",
		"unshare --user --map-root-user true",
		"go run ./cmd/releasecheck --verify-archives dist",
	} {
		if !strings.Contains(release, want) {
			t.Errorf("release verification does not provide %q", want)
		}
	}

	ci := readRepositoryFile(t, ".github/workflows/ci.yml")
	for _, stale := range []string{"release-config", "goreleaser/goreleaser-action", "go run ./cmd/releasecheck --verify-archives"} {
		if strings.Contains(ci, stale) {
			t.Errorf("CI retains release-only assurance %q", stale)
		}
	}
}
