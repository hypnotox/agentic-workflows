package project

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
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
// invariant: tooling/cli:pitfall-scaffold (TestPitfallScaffoldCLIContract)
func TestPitfallScaffoldCLIContract(t *testing.T) {
	t.Run("creation-presentation-and-no-render", TestNewPitfallScaffoldContract)
	t.Run("exclusive-race-and-retry", TestNewPitfallExclusiveRaceRefusesThenRetryReallocates)
	t.Run("load-and-directory-errors", func(t *testing.T) {
		t.Run("malformed corpus", func(t *testing.T) {
			root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n")
			dir := filepath.Join(root, ".awf/docs/pitfalls")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "bad.md"), []byte("malformed"), 0o644); err != nil {
				t.Fatal(err)
			}
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.NewPitfall("New"); err == nil || !strings.Contains(err.Error(), "missing frontmatter") {
				t.Fatalf("load error = %v", err)
			}
		})
		t.Run("source directory is a file", func(t *testing.T) {
			root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n")
			dir := filepath.Join(root, ".awf/docs/pitfalls")
			if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dir, []byte("not a directory"), 0o644); err != nil {
				t.Fatal(err)
			}
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.NewPitfall("New"); err == nil {
				t.Fatal("non-directory source root accepted")
			}
		})
		t.Run("mkdir failure", func(t *testing.T) {
			root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n")
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			mkdirErr := errors.New("mkdir failed")
			if _, err := p.newPitfallWith("New", func(string, os.FileMode) error { return mkdirErr }, createPitfallExclusive); !errors.Is(err, mkdirErr) {
				t.Fatalf("mkdir error = %v", err)
			}
		})
	})
	t.Run("refusals-and-suffix-gap", func(t *testing.T) {
		root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n")
		dir := filepath.Join(root, ".awf/docs/pitfalls")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, title := range map[string]string{"hazard.md": "Other", "hazard-2.md": "Another", "hazard-4.md": "Fourth"} {
			source := "---\ntitle: " + title + "\n---\nbody\n"
			if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		p, err := Open(testContext(t), root)
		if err != nil {
			t.Fatal(err)
		}
		for _, title := range []string{"", "index", "日本語"} {
			if _, err := p.NewPitfall(title); err == nil {
				t.Fatalf("title %q accepted", title)
			}
		}
		if _, err := p.NewPitfall("Hazard"); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "hazard-3.md")); err != nil {
			t.Fatalf("first suffix gap not selected: %v", err)
		}
	})
}

func TestNewPitfallScaffoldContract(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	generated := filepath.Join(root, "docs", "pitfalls.md")
	if _, err := os.Stat(generated); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected generated fixture: %v", err)
	}
	document, err := p.NewPitfall("Unicode + punctuation: 日本語")
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := presentation.Render(&output, document); err != nil {
		t.Fatal(err)
	}
	const relative = ".awf/docs/pitfalls/unicode-punctuation.md"
	if output.String() != "status: pitfall created\nauthored path: "+relative+"\n" {
		t.Fatalf("presentation = %q", output.String())
	}
	const want = "---\ntitle: 'Unicode + punctuation: 日本語'\n---\nDescribe the durable hazard, its consequence, and the safer practice.\n"
	got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil || string(got) != want {
		t.Fatalf("source = %q, %v", got, err)
	}
	if _, err := os.Stat(generated); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("NewPitfall rendered generated output: %v", err)
	}
	if _, err := p.NewPitfall(" unicode\t+ PUNCTUATION: 日本語 "); err == nil || !strings.Contains(err.Error(), "duplicates") {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestNewPitfallExclusiveRaceRefusesThenRetryReallocates(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	raced := false
	create := func(path string, source []byte) error {
		if !raced {
			raced = true
			const competing = "---\ntitle: Competing writer\n---\nbody\n"
			if err := os.WriteFile(path, []byte(competing), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return createPitfallExclusive(path, source)
	}
	if _, err := p.newPitfallWith("Race", os.MkdirAll, create); !errors.Is(err, os.ErrExist) {
		t.Fatalf("race error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf/docs/pitfalls/race-2.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("race silently advanced suffix: %v", err)
	}
	if _, err := p.newPitfallWith("Race", os.MkdirAll, create); err != nil {
		t.Fatalf("ordinary retry: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf/docs/pitfalls/race-2.md")); err != nil {
		t.Fatalf("retry did not recompute suffix: %v", err)
	}
}

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
	for _, retired := range []string{"skills:", "agents:", "docs:", "targets:", "docsDir:"} {
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
