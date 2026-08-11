package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

func writeScaffold(t *testing.T, b []byte) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// invariant: config/configuration:integration-branch-explicit (TestScaffoldWritesRepositoryFacts)
// invariant: tooling/init-and-enablement:init-bootstrap-default-on (TestScaffoldWritesRepositoryFacts)
func TestScaffoldWritesRepositoryFacts(t *testing.T) {
	b, err := ScaffoldConfig("myproj", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{"prefix: myproj", "integrationBranch: main", "bootstrap:", "enabled: true"} {
		if !strings.Contains(text, want) {
			t.Errorf("scaffold missing %q:\n%s", want, text)
		}
	}
	for _, retired := range []string{"skills:", "agents:", "docs:", "targets:", "docsDir:", "render:", "templateSourceRoot:"} {
		if strings.Contains(text, retired) {
			t.Errorf("scaffold retained %q:\n%s", retired, text)
		}
	}
	cfg, err := config.Load(writeScaffold(t, b))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

// invariant: rendering/project-output-plan:scaffold-seeds-all-vars (TestScaffoldVarsCoverAllReferenced)
func TestScaffoldVarsCoverAllReferenced(t *testing.T) {
	b, err := ScaffoldConfig("example", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(writeScaffold(t, b))
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for name := range catalog.Standard.Skills {
		paths = append(paths, "skills/"+name+"/SKILL.md.tmpl")
	}
	for name := range catalog.Standard.Agents {
		paths = append(paths, "agents/"+name+".md.tmpl")
	}
	for _, e := range catalog.Standard.Docs {
		paths = append(paths, e.TID)
	}
	for _, sg := range plainSingletons {
		paths = append(paths, sg.tid)
	}
	for _, path := range paths {
		src, err := templates.FS.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range render.ReferencedVars(string(src)) {
			if _, ok := cfg.Vars[name]; !ok {
				t.Errorf("scaffold vars missing %q from %s", name, path)
			}
		}
	}
}

func TestInitProducesCleanSyncableProject(t *testing.T) {
	b, err := ScaffoldConfig("testproject", map[string]string{"gateCmd": "make gate"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	awfDir := filepath.Join(root, ".awf")
	if err := os.MkdirAll(awfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awfDir, "config.yaml"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	if drift, err := p.Check(testContext(t)); err != nil || len(drift) != 0 {
		t.Fatalf("check = %#v, %v", drift, err)
	}
}

func TestScaffoldYAMLContainsNoPlaceholders(t *testing.T) {
	b, err := ScaffoldConfig("example", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "<no value>") || strings.Contains(string(b), "{{") {
		t.Fatalf("unsafe scaffold:\n%s", b)
	}
}

// invariant: tooling/audit-commands:audit-scopes-descriptor-routed (TestScaffoldWritesAuditScopes)
func TestScaffoldWritesAuditScopes(t *testing.T) {
	b, err := ScaffoldConfig("example", nil, []string{"adr", "awf"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"audit:", "allowedScopes:", "- adr", "- awf"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("scaffold missing %q:\n%s", want, b)
		}
	}
}

func TestNeededVarsPropagatesTemplateReadError(t *testing.T) {
	if _, err := neededVarsFromFS(fstest.MapFS{}); err == nil {
		t.Fatal("missing catalog templates accepted")
	}
}

func TestNeededVarsCoversFullCatalog(t *testing.T) {
	vars, err := NeededVars()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"commitGateCmd", "gateCmd", "invariantTestPath"} {
		if !vars[name] {
			t.Errorf("needed vars missing %s", name)
		}
	}
}
