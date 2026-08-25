package migrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func writeLock(t *testing.T, root string, schema int) {
	t.Helper()
	testsupport.WriteFile(t, config.ConfigPath(root), "prefix: test\n")
	if err := (&manifest.Lock{AWFVersion: "0.39.2", SchemaVersion: schema, Files: map[string]manifest.Entry{}}).Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
}
func snapshot(t *testing.T, root string) map[string][]byte {
	t.Helper()
	got := map[string][]byte{}
	if err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			b, readErr := os.ReadFile(p)
			if readErr != nil {
				return readErr
			}
			rel, relErr := filepath.Rel(root, p)
			if relErr != nil {
				return relErr
			}
			got[rel] = b
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return got
}
func assertSnapshot(t *testing.T, root string, want map[string][]byte) {
	t.Helper()
	got := snapshot(t, root)
	if len(got) != len(want) {
		t.Fatalf("file count changed: %#v", got)
	}
	for p, b := range want {
		if !slices.Equal(got[p], b) {
			t.Fatalf("%s changed", p)
		}
	}
}

// invariant: config/migrations-and-locks:upgrade-gate (TestUnsupportedSourcesRefuseWithoutMutation)
func TestUnsupportedSourcesRefuseWithoutMutation(t *testing.T) {
	cases := []struct {
		name, path string
		schema     int
	}{
		{"legacy single file", ".claude/awf.yaml", 0}, {"retired tree", ".claude/awf/config.yaml", 0},
		{"schema below floor", ".awf/config.yaml", LiveSchemaFloor - 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.schema == 0 {
				testsupport.WriteFile(t, filepath.Join(root, tc.path), "malformed: [")
			} else {
				writeLock(t, root, tc.schema)
			}
			before := snapshot(t, root)
			_, _, _, err := Build(context.Background(), root)
			if !errors.Is(err, manifest.ErrUnsupportedLiveSource) {
				t.Fatalf("Upgrade error=%v", err)
			}
			assertSnapshot(t, root, before)
		})
	}
}

// invariant: config/migrations-and-locks:migration-ordering (TestSchema46IsNoopAndFutureMigrationIsOrdered)
func TestSchema46IsNoopAndFutureMigrationIsOrdered(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, LiveSchemaFloor)
	applied, _, _, err := Build(context.Background(), root)
	if err != nil || len(applied) != 0 {
		t.Fatalf("schema 46: applied=%v err=%v", applied, err)
	}
	original := registry
	var calls []string
	registry = append(registry,
		Migration{To: LiveSchemaFloor + 1, Name: "first future", Build: func(context.Context, string, *Changes) ([]FileMutation, error) {
			calls = append(calls, "first")
			return []FileMutation{{Path: ".awf/future.yaml", Content: []byte("future\n"), Mode: 0o600}}, nil
		}},
		Migration{To: LiveSchemaFloor + 2, Name: "second future", Build: func(context.Context, string, *Changes) ([]FileMutation, error) {
			calls = append(calls, "second")
			return []FileMutation{{Path: ".awf/retired.yaml", Remove: true}}, nil
		}},
	)
	t.Cleanup(func() { registry = original })
	applied, _, mutations, err := Build(context.Background(), root)
	if err != nil || !slices.Equal(applied, []string{"first future", "second future"}) || !slices.Equal(calls, []string{"first", "second"}) {
		t.Fatalf("future seam: applied=%v calls=%v err=%v", applied, calls, err)
	}
	if len(mutations) != 2 || mutations[0].Path != ".awf/future.yaml" || mutations[1].Path != ".awf/retired.yaml" || !mutations[1].Remove {
		t.Fatalf("future mutations = %#v", mutations)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".awf", "future.yaml")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("Build mutated future file: %v", statErr)
	}
}

func TestAheadSchemaRefuses(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, LiveSchemaFloor+1)
	_, _, _, err := Build(context.Background(), root)
	if !errors.Is(err, manifest.ErrUnsupportedLiveSource) {
		t.Fatalf("error=%v", err)
	}
}

func TestSupportedMigrationClassifierAndFailure(t *testing.T) {
	if got := gateStateFor(LiveSchemaFloor+1, LiveSchemaFloor, []int{LiveSchemaFloor}); got != "ahead" {
		t.Fatalf("ahead state = %q", got)
	}
	if got := gateStateFor(LiveSchemaFloor, LiveSchemaFloor, []int{LiveSchemaFloor}); got != "ok" {
		t.Fatalf("current state = %q", got)
	}
	if got := gateStateFor(LiveSchemaFloor, LiveSchemaFloor+1, []int{LiveSchemaFloor, LiveSchemaFloor + 1}); got != "gate" {
		t.Fatalf("migration state = %q", got)
	}
	if got := gateStateFor(LiveSchemaFloor, LiveSchemaFloor+1, []int{LiveSchemaFloor}); got != "autobump" {
		t.Fatalf("autobump state = %q", got)
	}

	original := registry
	failure := errors.New("future migration failed")
	registry = append(registry, Migration{To: LiveSchemaFloor + 1, Name: "future", Build: func(context.Context, string, *Changes) ([]FileMutation, error) { return nil, failure }})
	t.Cleanup(func() { registry = original })

	err := CheckLiveGeneration(LiveSchemaFloor)
	var required *UpgradeRequiredError
	if !errors.As(err, &required) || !strings.Contains(required.Error(), "requires migration") {
		t.Fatalf("CheckLiveGeneration() error = %v, want upgrade requirement", err)
	}
	root := t.TempDir()
	writeLock(t, root, LiveSchemaFloor)
	_, _, _, err = Build(context.Background(), root)
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), `migration "future"`) {
		t.Fatalf("Upgrade() error = %v, want named migration failure", err)
	}
}

func TestBuildRejectsInvalidMigrationRegistry(t *testing.T) {
	original := registry
	t.Cleanup(func() { registry = original })
	for _, tc := range []struct {
		name string
		set  []Migration
		want string
	}{
		{"empty", nil, "begin at supported floor"},
		{"wrong floor", []Migration{{To: LiveSchemaFloor + 1}}, "begin at supported floor"},
		{"unordered", append(append([]Migration{}, original...), Migration{To: LiveSchemaFloor + 2}, Migration{To: LiveSchemaFloor + 1}), "strictly ascending"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry = tc.set
			if _, _, _, err := Build(context.Background(), t.TempDir()); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid registry error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestGenerationPreservesControlPathStatFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		path func(string) string
	}{
		{"current config", config.ConfigPath},
		{"retired layout", func(root string) string { return filepath.Join(root, ".claude", "awf.yaml") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := tc.path(root)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Base(path), path); err != nil {
				t.Fatal(err)
			}
			if _, err := Generation(root); !errors.Is(err, syscall.ELOOP) {
				t.Fatalf("Generation error = %v, want stat loop", err)
			}
		})
	}
}
