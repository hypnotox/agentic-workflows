package migrate

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invariant: config/migrations-and-locks:selection-keys-dropped (TestDropSelectionRemovesKeysAndSidecarLocal)
// invariant: config/migrations-and-locks:sidecar-local-field-dropped (TestDropSelectionRemovesKeysAndSidecarLocal)
func TestDropSelectionRemovesKeysAndSidecarLocal(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf/config.yaml"), "prefix: example\nskills: [tdd]\nagents: [reviewer]\ndocs: [testing]\ntargets: [claude]\ndocsDir: docs\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/tdd.yaml"), "data:\n  x: y\nlocal: false\n")
	var changes Changes
	if err := applyDropSelection(root, &changes); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, ".awf/config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "prefix: example\n" {
		t.Fatalf("config = %q", got)
	}
	sidecar, err := os.ReadFile(filepath.Join(root, ".awf/skills/tdd.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sidecar), "local:") {
		t.Fatalf("sidecar retains local: %q", sidecar)
	}
	if !strings.Contains(changes.String(), "drop-selection: removed skills") || !strings.Contains(changes.String(), "drop-selection: removed local") {
		t.Fatalf("changes = %q", changes.String())
	}
}

func TestDropSelectionRefusesLocalArtifactBeforeWriting(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, ".awf/config.yaml")
	src := "prefix: example\nskills: [tdd]\n"
	testsupport.WriteFile(t, configPath, src)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/tdd.yaml"), "local: true\n")
	var changes Changes
	err := applyDropSelection(root, &changes)
	if err == nil || !strings.Contains(err.Error(), "local: true") || !strings.Contains(err.Error(), "skills/tdd.yaml") {
		t.Fatalf("error = %v", err)
	}
	got, readErr := os.ReadFile(filepath.Join(root, ".awf/skills/tdd.yaml"))
	if readErr != nil || string(got) != "local: true\n" {
		t.Fatalf("sidecar changed before refusal: %q, %v", got, readErr)
	}
}

// invariant: config/migrations-and-locks:retired-keys-forward-ported (TestConfigForCurrentSchemaStripsSelectionKeys)
func TestConfigForCurrentSchemaStripsSelectionKeys(t *testing.T) {
	got, err := ConfigForCurrentSchema([]byte("prefix: example\nskills: []\nagents: []\ndocs: []\ntargets: [claude]\ndocsDir: docs\n"), Current())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "prefix: example\n" {
		t.Fatalf("forward-port = %q", got)
	}
}

func TestDropSelectionPropagatesOperationFailures(t *testing.T) {
	failure := errors.New("injected failure")
	rootWithConfig := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(root, ".awf/config.yaml"), "prefix: example\nskills: [tdd]\n")
		return root
	}
	t.Run("config mutation", func(t *testing.T) {
		operation := productionDropSelectionOperation()
		operation.removeKey = func([]byte, string) ([]byte, error) { return nil, failure }
		if err := applyDropSelectionWith(rootWithConfig(t), &Changes{}, operation); !errors.Is(err, failure) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("config write", func(t *testing.T) {
		operation := productionDropSelectionOperation()
		operation.configEditor.writeAtomic = func(string, []byte) error { return failure }
		if err := applyDropSelectionWith(rootWithConfig(t), &Changes{}, operation); !errors.Is(err, failure) {
			t.Fatalf("error = %v", err)
		}
	})

	rootWithSidecar := func(t *testing.T, body string) string {
		t.Helper()
		root := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/tdd.yaml"), body)
		return root
	}
	t.Run("absent tree", func(t *testing.T) {
		operation := productionDropSelectionOperation()
		operation.walkDir = func(string, fs.WalkDirFunc) error { return fs.ErrNotExist }
		if err := dropSidecarLocalWith(t.TempDir(), &Changes{}, operation); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("walk callback", func(t *testing.T) {
		operation := productionDropSelectionOperation()
		operation.walkDir = func(_ string, visit fs.WalkDirFunc) error { return visit("bad", nil, failure) }
		if err := dropSidecarLocalWith(t.TempDir(), &Changes{}, operation); !errors.Is(err, failure) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("walk", func(t *testing.T) {
		operation := productionDropSelectionOperation()
		operation.walkDir = func(string, fs.WalkDirFunc) error { return failure }
		if err := dropSidecarLocalWith(t.TempDir(), &Changes{}, operation); !errors.Is(err, failure) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("preflight read", func(t *testing.T) {
		operation := productionDropSelectionOperation()
		operation.readFile = func(string) ([]byte, error) { return nil, failure }
		if err := dropSidecarLocalWith(rootWithSidecar(t, "local: false\n"), &Changes{}, operation); !errors.Is(err, failure) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("preflight yaml", func(t *testing.T) {
		if err := dropSidecarLocalWith(rootWithSidecar(t, "local: [\n"), &Changes{}, productionDropSelectionOperation()); err == nil {
			t.Fatal("malformed sidecar accepted")
		}
	})
	t.Run("second read", func(t *testing.T) {
		operation := productionDropSelectionOperation()
		calls := 0
		operation.readFile = func(string) ([]byte, error) {
			calls++
			if calls == 2 {
				return nil, failure
			}
			return []byte("local: false\n"), nil
		}
		if err := dropSidecarLocalWith(rootWithSidecar(t, "local: false\n"), &Changes{}, operation); !errors.Is(err, failure) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("remove", func(t *testing.T) {
		operation := productionDropSelectionOperation()
		calls := 0
		operation.removeKey = func(src []byte, _ string) ([]byte, error) {
			calls++
			return nil, failure
		}
		if err := dropSidecarLocalWith(rootWithSidecar(t, "local: false\n"), &Changes{}, operation); !errors.Is(err, failure) || calls != 1 {
			t.Fatalf("error = %v, calls = %d", err, calls)
		}
	})
	t.Run("unchanged", func(t *testing.T) {
		if err := dropSidecarLocalWith(rootWithSidecar(t, "data: {}\n"), &Changes{}, productionDropSelectionOperation()); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("write", func(t *testing.T) {
		operation := productionDropSelectionOperation()
		operation.writeAtomic = func(string, []byte) error { return failure }
		if err := dropSidecarLocalWith(rootWithSidecar(t, "local: false\n"), &Changes{}, operation); !errors.Is(err, failure) {
			t.Fatalf("error = %v", err)
		}
	})
}

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
	if Current() != 39 {
		t.Fatalf("Current() = %d, want 39", Current())
	}
	applied, _, err := Upgrade(context.Background(), t.TempDir())
	if err != nil || len(applied) != 0 {
		t.Fatalf("empty upgrade = %v, %v", applied, err)
	}
}
