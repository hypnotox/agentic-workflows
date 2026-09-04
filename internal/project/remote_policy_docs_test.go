package project

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var standalonePiToken = regexp.MustCompile(`(?i)\bpi\b`)

func retainsRetiredRuntimeAssurance(text string) bool {
	if standalonePiToken.MatchString(text) {
		return true
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(text), " "))
	return containsAny(normalized, []string{"typescript", "javascript", "agent runtime", "agent-runtime", "harness runtime", "harness"}) &&
		containsAny(normalized, []string{"lane", "test", "suite", "smoke", "assurance", "behavior", " ci ", "hosted"})
}

func presentsHarnessPackagePrerequisite(text string) bool {
	for _, paragraph := range strings.Split(text, "\n\n") {
		normalized := strings.ToLower(strings.Join(strings.Fields(paragraph), " "))
		if !containsAny(normalized, []string{"agentic-skills", "pi-tools"}) {
			continue
		}
		namesBinaryOperation := strings.Contains(normalized, " init") || strings.Contains(normalized, "initializ") || strings.Contains(normalized, "upgrad")
		namesPrerequisite := containsAny(normalized, []string{
			"before",
			"must",
			"requir",
			"prerequisite",
			"depends on",
			"dependent on",
			"need",
			"necessary",
			"only after",
			"prior to",
		})
		if namesBinaryOperation && namesPrerequisite {
			return true
		}
	}
	return false
}

func containsAny(text string, fragments []string) bool {
	for _, fragment := range fragments {
		if strings.Contains(text, fragment) {
			return true
		}
	}
	return false
}

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

	releasing := read("docs/releasing.md")
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

func TestActiveAuthorityExcludesRetiredWorkflowSurfaces(t *testing.T) {
	root := repoRootDir(t)
	ciAndReleaseAuthorityPaths := []string{
		".awf/parts/workflow/ci.md",
		".awf/topics/parts/tooling/quality-gates/current-state.md",
		".awf/docs/parts/releasing/content.md",
		"docs/workflow.md",
		"docs/topics/tooling/quality-gates.md",
		"docs/releasing.md",
	}
	paths := []string{
		".awf/parts/workflow/ci.md",
		".awf/topics/parts/tooling/quality-gates/current-state.md",
		".awf/topics/parts/code-design/test-design/current-state.md",
		".awf/topics/parts/code-design/package-composition/current-state.md",
		".awf/domains/parts/rendering/current-state.md",
		".awf/docs/parts/releasing/content.md",
		".awf/docs/parts/testing/tiers.md",
		".awf/agents-doc.yaml",
		"docs/workflow.md",
		"docs/topics/tooling/quality-gates.md",
		"docs/topics/code-design/test-design.md",
		"docs/topics/code-design/package-composition.md",
		"docs/domains/rendering.md",
		"docs/releasing.md",
		"docs/testing.md",
		"AGENTS.md",
	}
	banned := []string{
		"typed Pi",
		"TypeScript lane",
		"Maintainable Code Design guide",
		"Pi host lane",
		"interactive Pi smoke",
		"applicable Pi",
		"Go and Pi behavior",
		"Go and Pi test suites",
	}
	for _, path := range paths {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(body)
		for _, phrase := range banned {
			if strings.Contains(text, phrase) {
				t.Errorf("active authority %s retains retired workflow surface %q", path, phrase)
			}
		}
	}

	const closedLaneInventory = "The closed CI lane inventory is `go`, `platform-sensitive`, `release-archive`, and `render-template`."
	for _, path := range ciAndReleaseAuthorityPaths {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if retainsRetiredRuntimeAssurance(string(body)) {
			t.Errorf("CI and release authority %s retains a retired harness/runtime assurance claim", path)
		}
		if !strings.Contains(string(body), closedLaneInventory) {
			t.Errorf("CI and release authority %s does not name the exact closed lane inventory %q", path, closedLaneInventory)
		}
	}

	contracts := []struct {
		name  string
		text  string
		paths []string
	}{
		{
			name: "quality-gate lane selection",
			text: "select applicable lanes from one typed JSON v2 result",
			paths: []string{
				".awf/topics/parts/tooling/quality-gates/current-state.md",
				"docs/topics/tooling/quality-gates.md",
			},
		},
		{
			name: "workflow lane selection",
			text: "consumes one typed JSON v2 selection",
			paths: []string{
				".awf/parts/workflow/ci.md",
				"docs/workflow.md",
			},
		},
		{
			name: "release local test scope",
			text: "skips the complete Go test suite locally while versioncheck and every static gate still run",
			paths: []string{
				".awf/docs/parts/releasing/content.md",
				"docs/releasing.md",
			},
		},
	}
	for _, contract := range contracts {
		for _, path := range contract.paths {
			body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			if !strings.Contains(string(body), contract.text) {
				t.Errorf("%s authority %s does not contain %q", contract.name, path, contract.text)
			}
		}
	}

	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	const workflowInventoryContract = `[.lanes[].name] == ["go", "platform-sensitive", "release-archive", "render-template"]`
	if !strings.Contains(string(workflow), workflowInventoryContract) {
		t.Errorf("CI selection consumer does not enforce the lane inventory paired with active authority: %s", workflowInventoryContract)
	}
}

func TestRetiredRuntimeAssuranceDetection(t *testing.T) {
	for _, test := range []struct {
		name string
		text string
		want bool
	}{
		{name: "pi lane", text: "Select the applicable Pi validation lane.", want: true},
		{name: "typescript assurance", text: "Run the TypeScript harness assurance lane.", want: true},
		{name: "javascript smoke", text: "Hosted JavaScript agent-runtime smoke tests run here.", want: true},
		{name: "closed inventory", text: "The closed CI lane inventory is go, platform-sensitive, release-archive, and render-template.", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := retainsRetiredRuntimeAssurance(test.text); got != test.want {
				t.Fatalf("retainsRetiredRuntimeAssurance(%q) = %v, want %v", test.text, got, test.want)
			}
		})
	}
}

func TestReadmeScopesHarnessPackagesAsOptional(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join(repoRootDir(t), "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, optionalHarnessInstall := range []string{
		"For Claude Code harness use, optionally install `agentic-skills` to add its generic skills and roles:",
		"For Pi harness use, optionally install [`pi-tools`](https://github.com/hypnotox/pi-tools) first, then `agentic-skills`:",
	} {
		if !strings.Contains(string(readme), optionalHarnessInstall) {
			t.Errorf("README does not scope harness package installation to optional harness use: %q", optionalHarnessInstall)
		}
	}
	const offlineWithoutHarnessPackages = "binary remains offline and functional when those optional operator-managed capabilities are absent."
	if !strings.Contains(string(readme), offlineWithoutHarnessPackages) {
		t.Errorf("README does not preserve AWF's package-independent binary contract: %q", offlineWithoutHarnessPackages)
	}

	if presentsHarnessPackagePrerequisite(string(readme)) {
		t.Error("README presents an operator-managed harness package as an AWF init or upgrade prerequisite")
	}
}

func TestHarnessPackagePrerequisiteDetection(t *testing.T) {
	for _, test := range []struct {
		name string
		text string
		want bool
	}{
		{name: "before initialization", text: "Install agentic-skills before initializing AWF.", want: true},
		{name: "depends on", text: "AWF initialization depends on agentic-skills.", want: true},
		{name: "only after", text: "Upgrade AWF only after installing pi-tools.", want: true},
		{name: "needed", text: "agentic-skills is needed to initialize AWF.", want: true},
		{name: "prior to", text: "Install pi-tools prior to an AWF upgrade.", want: true},
		{name: "optional", text: "For Pi harness use, optionally install pi-tools and agentic-skills.", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := presentsHarnessPackagePrerequisite(test.text); got != test.want {
				t.Fatalf("presentsHarnessPackagePrerequisite(%q) = %v, want %v", test.text, got, test.want)
			}
		})
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
