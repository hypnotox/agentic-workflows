package migrate

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invariant: config/migrations-and-locks:selection-keys-dropped (TestDropSelectionRemovesKeysAndSidecarLocal)
// invariant: config/migrations-and-locks:sidecar-local-field-dropped (TestDropSelectionRemovesKeysAndSidecarLocal)
func TestDropSelectionRemovesKeysAndSidecarLocal(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf/config.yaml"), "prefix: example\nskills: [tdd]\nagents: [reviewer]\ndocs: [testing]\ntargets: [claude]\ndocsDir: docs\n")
	for _, kind := range catalog.SingletonKinds() {
		testsupport.WriteFile(t, filepath.Join(root, ".awf", kind+".yaml"), "# singleton\nlocal: false\ndata: {}\n")
	}
	paths := map[string]string{
		"skills/project-local.yaml":             "# skill comment\ndata:\n  x: y\nlocal: false\n",
		"agents/project-local.yaml":             "local: false\n# agent comment\ndata: {}\n",
		"docs/project-local.yaml":               "local: false\ndata: {}\n",
		"docs/nested/project-local.yaml":        "# nested comment\nlocal: false\ndata: {}\n",
		"docs/nested/more/project-local-2.yaml": "data: {}\nlocal: false\n",
	}
	for rel, body := range paths {
		testsupport.WriteFile(t, filepath.Join(root, ".awf", rel), body)
	}
	// These are outside the frozen artifact-sidecar surface. Invalid YAML makes
	// accidental parsing observable, while the valid fixtures catch mutation.
	for rel, body := range map[string]string{
		"random.yaml":                                 "local: [\n",
		"domains/project-local.yaml":                  "local: [\n",
		"topics/project-local.yaml":                   "local: [\n",
		"efforts/project-local.yaml":                  "local: [\n",
		"worktrees/project-local.yaml":                "local: [\n",
		"parts/project-local.yaml":                    "local: [\n",
		"docs/not_valid.yaml":                         "local: [\n",
		"docs/nested-checkout/.git":                   "gitdir: elsewhere\n",
		"docs/nested-checkout/inside.yaml":            "local: [\n",
		"docs/nested-checkout/project-local.yaml":     "local: [\n",
		"docs/nested-checkout-dir/.git/HEAD":          "ref: refs/heads/main\n",
		"docs/nested-checkout-dir/project-local.yaml": "local: [\n",
		"docs/project-local.md.yaml":                  "local: [\n",
		"skills/not_valid.yaml":                       "local: [\n",
		"agents/not_valid.yaml":                       "local: [\n",
	} {
		testsupport.WriteFile(t, filepath.Join(root, ".awf", rel), body)
	}
	modePath := filepath.Join(root, ".awf/skills/project-local.yaml")
	if err := os.Chmod(modePath, 0o600); err != nil {
		t.Fatal(err)
	}
	var changes Changes
	if err := applyDropSelection(root, &changes); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, ".awf/config.yaml")); err != nil || string(got) != "prefix: example\n" {
		t.Fatalf("config = %q, %v", got, err)
	}
	for rel, before := range paths {
		got, err := os.ReadFile(filepath.Join(root, ".awf", rel))
		if err != nil {
			t.Fatal(err)
		}
		want := strings.Replace(before, "local: false\n", "", 1)
		// RemoveKey owns YAML comments attached to the removed mapping key.
		want = strings.Replace(want, "# nested comment\n", "", 1)
		if string(got) != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	if info, err := os.Stat(modePath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, %v; want 0600", info.Mode(), err)
	}
	for rel := range map[string]string{
		"random.yaml": "", "domains/project-local.yaml": "", "topics/project-local.yaml": "", "efforts/project-local.yaml": "", "worktrees/project-local.yaml": "", "parts/project-local.yaml": "", "docs/not_valid.yaml": "", "docs/nested-checkout/inside.yaml": "", "docs/nested-checkout-dir/project-local.yaml": "", "docs/project-local.md.yaml": "", "skills/not_valid.yaml": "", "agents/not_valid.yaml": "",
	} {
		got, err := os.ReadFile(filepath.Join(root, ".awf", rel))
		if err != nil || !strings.Contains(string(got), "local:") {
			t.Errorf("unrelated %s changed or unreadable: %q, %v", rel, got, err)
		}
	}
	var wantChanges strings.Builder
	wantChanges.WriteString("drop-selection: removed skills\ndrop-selection: removed agents\ndrop-selection: removed docs\ndrop-selection: removed targets\ndrop-selection: removed docsDir\n")
	for _, kind := range catalog.SingletonKinds() {
		wantChanges.WriteString("drop-selection: removed local from " + filepath.ToSlash(filepath.Join(root, ".awf", kind+".yaml")) + "\n")
	}
	for _, rel := range []string{"skills/project-local.yaml", "agents/project-local.yaml", "docs/nested/more/project-local-2.yaml", "docs/nested/project-local.yaml", "docs/project-local.yaml"} {
		wantChanges.WriteString("drop-selection: removed local from " + filepath.ToSlash(filepath.Join(root, ".awf", rel)) + "\n")
	}
	if got, want := changes.String(), wantChanges.String(); got != want {
		t.Fatalf("changes = %q, want %q", got, want)
	}
}

func TestDropSelectionRefusesLocalArtifactBeforeWriting(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, ".awf/config.yaml")
	src := "prefix: example\nskills: [a-first, z-last]\n"
	testsupport.WriteFile(t, configPath, src)
	first := filepath.Join(root, ".awf/skills/a-first.yaml")
	rejecting := filepath.Join(root, ".awf/skills/z-last.yaml")
	testsupport.WriteFile(t, first, "local: false\ndata: {}\n")
	testsupport.WriteFile(t, rejecting, "local: true\n")
	if err := os.Chmod(first, 0o600); err != nil {
		t.Fatal(err)
	}
	var changes Changes
	err := applyDropSelection(root, &changes)
	if err == nil || !strings.Contains(err.Error(), "local: true") || !strings.Contains(err.Error(), "skills/z-last.yaml") {
		t.Fatalf("error = %v", err)
	}
	if changes.String() != "" {
		t.Fatalf("refusal announced unapplied changes: %q", changes.String())
	}
	if got, readErr := os.ReadFile(configPath); readErr != nil || string(got) != src {
		t.Fatalf("config changed before refusal: %q, %v", got, readErr)
	}
	if got, readErr := os.ReadFile(first); readErr != nil || string(got) != "local: false\ndata: {}\n" {
		t.Fatalf("earlier sidecar changed before refusal: %q, %v", got, readErr)
	}
	info, statErr := os.Stat(first)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("earlier sidecar mode = %v, want 0600", info.Mode())
	}
	if got, readErr := os.ReadFile(rejecting); readErr != nil || string(got) != "local: true\n" {
		t.Fatalf("rejecting sidecar changed before refusal: %q, %v", got, readErr)
	}
}

func TestConfigForCurrentSchemaStripsSelectionKeys(t *testing.T) {
	src := []byte("prefix: example\nskills: []\nagents: []\ndocs: []\ntargets: [claude]\ndocsDir: docs\n")
	for generation := 1; generation <= Current(); generation++ {
		got, err := ConfigForCurrentSchema(src, generation)
		if err != nil {
			t.Fatalf("generation %d: %v", generation, err)
		}
		if strings.Contains(string(got), "skills:") || strings.Contains(string(got), "agents:") || strings.Contains(string(got), "docs:") || strings.Contains(string(got), "targets:") || strings.Contains(string(got), "docsDir:") {
			t.Fatalf("generation %d retained a retired key: %q", generation, got)
		}
		root := t.TempDir()
		testsupport.WriteFile(t, config.ConfigPath(root), string(got))
		if _, err := config.Load(config.RootDir(root)); err != nil {
			t.Fatalf("generation %d result does not strict-decode: %v", generation, err)
		}
	}
}

func TestDropSelectionPropagatesOperationFailures(t *testing.T) {
	failure := errors.New("injected failure")
	rootWithSidecar := func(t *testing.T, body string) string {
		t.Helper()
		root := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/tdd.yaml"), body)
		return root
	}
	for _, test := range []struct {
		name   string
		change func(*dropSelectionOperation)
	}{
		{"read directory", func(o *dropSelectionOperation) {
			o.readDir = func(string) ([]os.DirEntry, error) { return nil, failure }
		}},
		{"read", func(o *dropSelectionOperation) { o.readFile = func(string) ([]byte, error) { return nil, failure } }},
		{"stat", func(o *dropSelectionOperation) {
			original := o.stat
			o.stat = func(path string) (fs.FileInfo, error) {
				if strings.HasSuffix(path, "skills/tdd.yaml") {
					return nil, failure
				}
				return original(path)
			}
		}},
		{"remove", func(o *dropSelectionOperation) {
			o.removeKey = func([]byte, string) ([]byte, error) { return nil, failure }
		}},
		{"write", func(o *dropSelectionOperation) {
			o.writeAtomic = func(string, []byte, os.FileMode) error { return failure }
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			op := productionDropSelectionOperation()
			test.change(&op)
			err := dropSidecarLocalForTest(rootWithSidecar(t, "local: false\n"), &Changes{}, op)
			if !errors.Is(err, failure) || !strings.Contains(err.Error(), ".awf/") {
				t.Fatalf("error = %v", err)
			}
		})
	}
	t.Run("parse", func(t *testing.T) {
		err := dropSidecarLocalForTest(rootWithSidecar(t, "local: [\n"), &Changes{}, productionDropSelectionOperation())
		if err == nil || !strings.Contains(err.Error(), "skills/tdd.yaml") {
			t.Fatalf("error = %v", err)
		}
	})
}

func dropSidecarLocalForTest(root string, out *Changes, operation dropSelectionOperation) error {
	edits, err := preflightSidecarLocal(root, operation)
	if err != nil {
		return err
	}
	return writeSidecarLocal(edits, out, operation)
}

func TestDropSelectionCoversConfigurationAndEnumerationEdges(t *testing.T) {
	failure := errors.New("injected failure")
	root := t.TempDir()
	testsupport.WriteFile(t, config.ConfigPath(root), "skills: [tdd]\n")
	t.Run("config remove", func(t *testing.T) {
		op := productionDropSelectionOperation()
		op.removeKey = func([]byte, string) ([]byte, error) { return nil, failure }
		if err := applyDropSelectionWith(root, &Changes{}, op); !errors.Is(err, failure) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("config write", func(t *testing.T) {
		op := productionDropSelectionOperation()
		op.configEditor.writeAtomic = func(string, []byte) error { return failure }
		if err := applyDropSelectionWith(root, &Changes{}, op); !errors.Is(err, failure) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("singleton stat", func(t *testing.T) {
		op := productionDropSelectionOperation()
		op.stat = func(string) (fs.FileInfo, error) { return nil, failure }
		if _, err := selectionSidecarPaths(root, op); !errors.Is(err, failure) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("walk and checkout stat", func(t *testing.T) {
		op := productionDropSelectionOperation()
		op.walkDir = func(string, fs.WalkDirFunc) error { return failure }
		if _, err := selectionSidecarPaths(root, op); !errors.Is(err, failure) {
			t.Fatalf("walk error = %v", err)
		}
		op = productionDropSelectionOperation()
		op.walkDir = func(path string, visit fs.WalkDirFunc) error {
			return visit(path, fakeDirEntry{name: "docs", dir: true}, nil)
		}
		original := op.stat
		op.stat = func(path string) (fs.FileInfo, error) {
			if strings.HasSuffix(path, ".git") {
				return nil, failure
			}
			return original(path)
		}
		if _, err := selectionSidecarPaths(root, op); !errors.Is(err, failure) {
			t.Fatalf("checkout stat error = %v", err)
		}
	})
	if validHistoricalDocName("---") {
		t.Fatal("punctuation-only historical doc name accepted")
	}
	op := productionDropSelectionOperation()
	if err := writeSidecarLocal([]selectionSidecarEdit{{path: "unchanged", source: "unchanged", bytes: []byte("data: {}\n")}}, &Changes{}, op); err != nil {
		t.Fatal(err)
	}
}

type fakeDirEntry struct {
	name string
	dir  bool
}

func (e fakeDirEntry) Name() string { return e.name }
func (e fakeDirEntry) IsDir() bool  { return e.dir }
func (e fakeDirEntry) Type() fs.FileMode {
	if e.dir {
		return fs.ModeDir
	}
	return 0
}
func (e fakeDirEntry) Info() (fs.FileInfo, error) { return nil, errors.New("not used") }

func TestLoadForMigrationRejectsMalformedHistoricalYAML(t *testing.T) {
	for _, source := range []string{"invariants: [\n", "prefix: example\nunknownCurrentKey: true\n"} {
		root := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(root, ".awf/config.yaml"), source)
		if _, err := loadForMigration(root); err == nil {
			t.Fatalf("invalid migration config accepted: %q", source)
		}
	}
}

func TestHistoricalConfigErrors(t *testing.T) {
	if _, err := loadHistoricalConfig(t.TempDir(), []byte("skills: [\n")); err == nil {
		t.Fatal("malformed historical config accepted")
	}
	root := t.TempDir()
	cfg, err := loadHistoricalConfig(root, []byte("prefix: example\n"))
	if err != nil {
		t.Fatal(err)
	}
	sidecar := filepath.Join(root, ".awf", "skills", "tdd.yaml")
	if err := os.MkdirAll(filepath.Dir(sidecar), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("tdd.yaml", sidecar); err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.Sidecar("skills", "tdd"); err == nil {
		t.Fatal("historical sidecar read error was not propagated")
	}
	if err := os.Remove(sidecar); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, sidecar, "local: [\n")
	if _, err := cfg.Sidecar("skills", "tdd"); err == nil {
		t.Fatal("malformed historical sidecar accepted")
	}
}

func TestDropSelectionRegisteredAtGeneration39(t *testing.T) {
	var found bool
	for _, migration := range registry {
		if migration.To == 39 && migration.Name == "drop-selection" {
			found = true
		}
	}
	if !found {
		t.Fatal("drop-selection migration is not registered at generation 39")
	}
	applied, _, err := upgradeLegacyForTest(context.Background(), t.TempDir())
	if err != nil || len(applied) != 0 {
		t.Fatalf("empty upgrade = %v, %v", applied, err)
	}
}
