package project

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filepublication"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
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
	t.Run("committed-cleanup-outcome", TestNewPitfallCommittedCleanupOutcomeIsActionableAndDoesNotAdvance)
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
		t.Run("nested source and injected read or walk failures", func(t *testing.T) {
			root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n")
			dir := filepath.Join(root, ".awf/docs/pitfalls/nested")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "bad.md"), []byte("---\ntitle: Bad\n---\nbody\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			tree := openPitfallScaffoldTree(t, root)
			if _, err := p.newPitfallWith("New", tree); err == nil || !strings.Contains(err.Error(), "direct child") {
				t.Fatalf("nested source error = %v", err)
			}
			if err := os.RemoveAll(filepath.Join(root, ".awf/docs/pitfalls")); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(root, ".awf/docs/pitfalls"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".awf/docs/pitfalls/a.md"), []byte("---\ntitle: A\n---\nbody\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			readErr := errors.New("read failed")
			if _, err := p.newPitfallWith("New", &faultPitfallFilesystem{pitfallScaffoldFilesystem: tree, readErr: readErr}); !errors.Is(err, readErr) {
				t.Fatalf("read error = %v", err)
			}
			walkErr := errors.New("walk failed")
			if _, err := p.newPitfallWith("New", &faultPitfallFilesystem{pitfallScaffoldFilesystem: tree, walkErr: walkErr}); !errors.Is(err, walkErr) {
				t.Fatalf("walk error = %v", err)
			}
		})
		t.Run("mkdir failure", func(t *testing.T) {
			root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n")
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			tree := openPitfallScaffoldTree(t, root)
			mkdirErr := errors.New("mkdir failed")
			faults := &faultPitfallFilesystem{pitfallScaffoldFilesystem: tree, mkdirErr: mkdirErr}
			if _, err := p.newPitfallWith("New", faults); !errors.Is(err, mkdirErr) {
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

// invariant: tooling/cli:pitfall-scaffold (TestNewPitfallPublicationFailureLeavesDestinationAbsent)
func TestNewPitfallPublicationFailureLeavesDestinationAbsent(t *testing.T) {
	if _, err := (&Project{Root: filepath.Join(t.TempDir(), "missing")}).NewPitfall("Unopened"); err == nil {
		t.Fatal("missing project root opened for pitfall scaffold")
	}
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	tree := openPitfallScaffoldTree(t, root)
	publishErr := errors.New("publication preparation failed")
	faults := &faultPitfallFilesystem{
		pitfallScaffoldFilesystem: tree,
		publish:                   func(string, []byte, os.FileMode) error { return publishErr },
	}
	if _, err := p.newPitfallWith("Unpublished", faults); !errors.Is(err, publishErr) {
		t.Fatalf("publication error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf/docs/pitfalls/unpublished.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed publication left selected destination: %v", err)
	}
}

func TestNewPitfallCommittedCleanupOutcomeIsActionableAndDoesNotAdvance(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	tree := openPitfallScaffoldTree(t, root)
	cleanupErr := errors.New("persistent cleanup failure")
	const sourcePath = ".awf/docs/pitfalls/committed.md"
	const residuePath = ".awf/docs/pitfalls/.filepublication-injected.tmp"
	faults := &faultPitfallFilesystem{pitfallScaffoldFilesystem: tree}
	faults.publish = func(path string, source []byte, mode os.FileMode) error {
		if err := tree.Publish(path, source, mode); err != nil {
			return err
		}
		if err := tree.Publish(residuePath, []byte("temporary"), 0o600); err != nil {
			return err
		}
		return &filepublication.CommittedCleanupError{DestinationPath: path, ResiduePath: residuePath, Cause: cleanupErr}
	}
	_, err = p.newPitfallWith("Committed", faults)
	var outcome *PitfallScaffoldCleanupError
	var committed *filepublication.CommittedCleanupError
	if !errors.As(err, &outcome) || !errors.As(err, &committed) || !errors.Is(err, cleanupErr) {
		t.Fatalf("committed scaffold error = %v", err)
	}
	if outcome.SourcePath != sourcePath || outcome.ResiduePath != residuePath || !strings.Contains(outcome.Error(), cleanupErr.Error()) {
		t.Fatalf("committed scaffold outcome = %#v (%v)", outcome, outcome)
	}
	raw, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(sourcePath)))
	info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(sourcePath)))
	if readErr != nil || statErr != nil || !strings.Contains(string(raw), "title: Committed") || info.Mode().Perm() != 0o644 {
		t.Fatalf("committed source = %q, %v, %v", raw, info, errors.Join(readErr, statErr))
	}
	diagnostic, err := outcome.Diagnostic()
	if err != nil {
		t.Fatal(err)
	}
	document, err := diagnostic.Document()
	if err != nil {
		t.Fatal(err)
	}
	var rendered bytes.Buffer
	if err := presentation.Render(&rendered, document); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"condition: pitfall source " + sourcePath + " is created", "authored source: yes", "cleanup residue: yes", "step 1: inspect the created authored source " + sourcePath, "step 2: remove publication cleanup residue " + residuePath} {
		if !strings.Contains(rendered.String(), want) {
			t.Fatalf("actionable committed diagnostic missing %q:\n%s", want, rendered.String())
		}
	}
	invalidOutcome := *outcome
	invalidOutcome.ResiduePath = "bad\nresidue"
	if _, err := invalidOutcome.Diagnostic(); err == nil {
		t.Fatal("line-breaking cleanup action accepted")
	}
	if _, retryErr := p.newPitfallWith("Committed", tree); retryErr == nil || !strings.Contains(retryErr.Error(), residuePath) {
		t.Fatalf("ordinary retry did not report cleanup residue: %v", retryErr)
	}
	if err := tree.Remove(residuePath); err != nil {
		t.Fatal(err)
	}
	if _, retryErr := p.newPitfallWith("Committed", tree); retryErr == nil || !strings.Contains(retryErr.Error(), "duplicates") {
		t.Fatalf("post-cleanup retry did not recognize committed authored identity: %v", retryErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".awf/docs/pitfalls/committed-2.md")); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("ordinary retry silently advanced suffix: %v", statErr)
	}
}

// invariant: tooling/cli:pitfall-scaffold (TestNewPitfallRootConfinement)
func TestNewPitfallRootConfinement(t *testing.T) {
	for _, tc := range []struct {
		name    string
		arrange func(*testing.T, string, string)
	}{
		{
			name: "escaping docs symlink",
			arrange: func(t *testing.T, root, outside string) {
				if err := os.RemoveAll(filepath.Join(root, ".awf/docs")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(root, ".awf/docs")); err != nil {
					t.Skipf("symlink unsupported: %v", err)
				}
			},
		},
		{
			name: "escaping pitfalls symlink",
			arrange: func(t *testing.T, root, outside string) {
				if err := os.MkdirAll(filepath.Join(root, ".awf/docs"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(root, ".awf/docs/pitfalls")); err != nil {
					t.Skipf("symlink unsupported: %v", err)
				}
			},
		},
		{
			name: "non-regular source root",
			arrange: func(t *testing.T, root, _ string) {
				if err := os.MkdirAll(filepath.Join(root, ".awf/docs"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".awf/docs/pitfalls"), []byte("not a directory"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n")
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			outside := t.TempDir()
			tc.arrange(t, root, outside)
			if _, err := p.NewPitfall("Escaping"); err == nil {
				t.Fatal("unsafe source root accepted")
			}
			entries, err := os.ReadDir(outside)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("external target mutated: %v", entries)
			}
		})
	}
}

// invariant: tooling/cli:pitfall-scaffold (TestNewPitfallScaffoldContract)
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
	path := filepath.Join(root, filepath.FromSlash(relative))
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("source = %q, %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("source mode = %v, %v", info, err)
	}
	if _, err := os.Stat(generated); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("NewPitfall rendered generated output: %v", err)
	}
	if _, err := p.NewPitfall(" unicode\t+ PUNCTUATION: 日本語 "); err == nil || !strings.Contains(err.Error(), "duplicates") {
		t.Fatalf("duplicate error = %v", err)
	}
}

// invariant: tooling/cli:pitfall-scaffold (TestNewPitfallExclusiveRaceRefusesThenRetryReallocates)
func TestNewPitfallExclusiveRaceRefusesThenRetryReallocates(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	tree := openPitfallScaffoldTree(t, root)
	raced := false
	faults := &faultPitfallFilesystem{pitfallScaffoldFilesystem: tree}
	faults.publish = func(path string, source []byte, mode os.FileMode) error {
		if !raced {
			raced = true
			const competing = "---\ntitle: Competing writer\n---\nbody\n"
			if err := tree.Publish(path, []byte(competing), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return tree.Publish(path, source, mode)
	}
	if _, err := p.newPitfallWith("Race", faults); !errors.Is(err, os.ErrExist) {
		t.Fatalf("race error = %v", err)
	}
	winner := filepath.Join(root, ".awf/docs/pitfalls/race.md")
	if got, err := os.ReadFile(winner); err != nil || !strings.Contains(string(got), "Competing writer") {
		t.Fatalf("race changed winner bytes = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf/docs/pitfalls/race-2.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("race silently advanced suffix: %v", err)
	}
	if _, err := p.newPitfallWith("Race", faults); err != nil {
		t.Fatalf("ordinary retry: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf/docs/pitfalls/race-2.md")); err != nil {
		t.Fatalf("retry did not recompute suffix: %v", err)
	}
}

type faultPitfallFilesystem struct {
	pitfallScaffoldFilesystem
	mkdirErr error
	readErr  error
	walkErr  error
	publish  func(string, []byte, os.FileMode) error
}

func (f *faultPitfallFilesystem) MkdirAll(path string, mode os.FileMode) error {
	if f.mkdirErr != nil {
		return f.mkdirErr
	}
	return f.pitfallScaffoldFilesystem.MkdirAll(path, mode)
}

func (f *faultPitfallFilesystem) Read(path string) ([]byte, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	return f.pitfallScaffoldFilesystem.Read(path)
}

func (f *faultPitfallFilesystem) Walk(path string, visit func(string, fs.FileInfo) (bool, error)) error {
	if f.walkErr != nil {
		return f.walkErr
	}
	return f.pitfallScaffoldFilesystem.Walk(path, visit)
}

func (f *faultPitfallFilesystem) Publish(path string, source []byte, mode os.FileMode) error {
	if f.publish != nil {
		return f.publish(path, source, mode)
	}
	return f.pitfallScaffoldFilesystem.Publish(path, source, mode)
}

func openPitfallScaffoldTree(t *testing.T, root string) *filesystem.Handle {
	t.Helper()
	tree, err := filesystem.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tree.Close(); err != nil {
			t.Error(err)
		}
	})
	return tree
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
