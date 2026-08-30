package project

import (
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// scaffold writes a .awf/config.yaml tree under a fresh temp root.
func scaffold(t *testing.T, configYAML string) string {
	return scaffoldFiles(t, configYAML, nil)
}

// testLayout returns a complete .layout template map - every key
// Layout.templateMap produces, with canonical docs/-rooted values - so a test
// that renders a template directly doesn't need to hand-pick which keys that
// template happens to reference today. A future Layout field addition only
// needs updating here, not at every hand-built fixture across the package.
func testLayout() map[string]any {
	return map[string]any{
		"docsDir":                "docs",
		"adrDir":                 "docs/decisions",
		"indexMd":                "docs/decisions/INDEX.md",
		"adrReadme":              "docs/decisions/README.md",
		"adrTemplate":            "docs/decisions/template.md",
		"docs":                   map[string]any{},
		"workflowRef":            "docs/workflow.md",
		"docStandard":            "docs/doc-standard.md",
		"agentsMdStandard":       "docs/agents-md-standard.md",
		"workingWithAwf":         "docs/working-with-awf.md",
		"maintainableCodeDesign": "docs/maintainable-code-design.md",
		"configReference":        "docs/config-reference.md",
		"domainsDir":             "docs/domains",
	}
}

// scaffoldFiles writes config.yaml plus optional sidecar/part files keyed by path
// relative to .awf/ (e.g. "skills/tdd.yaml", "skills/parts/x/y.md").
func scaffoldFiles(t *testing.T, configYAML string, files map[string]string) string {
	t.Helper()
	return scaffoldFilesRaw(t, withTestGateCmd(withTestProfile(configYAML)), files)
}

func withTestProfile(configYAML string) string { return configYAML }

func scaffoldFilesRaw(t *testing.T, configYAML string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, configYAML)
	for rel, body := range files {
		testsupport.WriteFile(t, filepath.Join(root, ".awf", rel), body)
	}
	return root
}

func withTestGateCmd(configYAML string) string {
	if strings.Contains(configYAML, "gateCmd:") {
		return configYAML
	}
	lines := strings.Split(strings.TrimSuffix(configYAML, "\n"), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "vars:") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, "vars:"))
		switch {
		case rest == "" || rest == "{}":
			lines[i] = "vars:"
			lines = slices.Insert(lines, i+1, "  gateCmd: test-gate")
		case strings.HasPrefix(rest, "{") && strings.HasSuffix(rest, "}"):
			inside := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(rest, "{"), "}"))
			if inside != "" {
				inside += ", "
			}
			lines[i] = "vars: {" + inside + "gateCmd: test-gate}"
		}
		return strings.Join(lines, "\n") + "\n"
	}
	return strings.Join(lines, "\n") + "\nvars:\n  gateCmd: test-gate\n"
}

// gitScaffold writes gitSampleYAML into a fresh git-backed root whose checkout
// sits on branch. A test that exercises branch-aware behaviour needs a real
// repository, because the branch is read through the git seam rather than
// injected (ADR-0193 keeps branch detection in one home).
var (
	gitScaffoldSeedOnce sync.Once
	gitScaffoldSeed     testsupport.TreeSeed
	gitScaffoldSeedErr  error
)

func gitScaffold(t *testing.T, branch string) string {
	t.Helper()
	gitScaffoldSeedOnce.Do(func() {
		repo := gitfixture.InitRepo(t)
		root := repo.Root()
		gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
		testsupport.WriteAwfConfig(t, root, gitSampleYAML)
		gitScaffoldSeed, gitScaffoldSeedErr = testsupport.CaptureTree(root)
	})
	if gitScaffoldSeedErr != nil {
		t.Fatalf("prepare project Git seed: %v", gitScaffoldSeedErr)
	}
	root := filepath.Join(t.TempDir(), "project")
	if err := gitScaffoldSeed.Clone(root); err != nil {
		t.Fatalf("clone project Git seed: %v", err)
	}
	if branch != defaultFixtureBranch {
		repo := gitfixture.At(root)
		gitfixture.NativeBranch(t, repo, branch)
		gitfixture.NativeCheckout(t, repo, branch)
	}
	return root
}

// defaultFixtureBranch is the branch a go-git fixture repository starts on.
const defaultFixtureBranch = "master"

// gitSampleYAML is sampleYAML with the integration branch pointed at the
// fixture default, so a plain git-backed scaffold is "on" the integration
// branch without any extra checkout.
const gitSampleYAML = `prefix: example
integrationBranch: master
vars:
  testCmd: go test ./...
  gateCmd: make gate
`

// lockFile is the relocated lock path under the tree.
func lockFile(root string) string {
	return filepath.Join(root, ".awf", "awf.lock")
}

// configPath is the tree config file path.
func configPath(root string) string {
	return filepath.Join(root, ".awf", "config.yaml")
}

const sampleYAML = `prefix: example
integrationBranch: main
vars:
  testCmd: go test ./...
  gateCmd: make gate
`
