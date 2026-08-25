package migrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
			_, _, err := Upgrade(context.Background(), root)
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
	applied, _, err := Upgrade(context.Background(), root)
	if err != nil || len(applied) != 0 {
		t.Fatalf("schema 46: applied=%v err=%v", applied, err)
	}
	original := registry
	registry = append(registry, Migration{To: LiveSchemaFloor + 1, Name: "future", Apply: func(context.Context, string, *Changes) error { return nil }})
	t.Cleanup(func() { registry = original })
	applied, _, err = Upgrade(context.Background(), root)
	if err != nil || !slices.Equal(applied, []string{"future"}) {
		t.Fatalf("future seam: applied=%v err=%v", applied, err)
	}
}

func TestAheadSchemaRefuses(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, LiveSchemaFloor+1)
	_, _, err := Upgrade(context.Background(), root)
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
	registry = append(registry, Migration{To: LiveSchemaFloor + 1, Name: "future", Apply: func(context.Context, string, *Changes) error { return failure }})
	t.Cleanup(func() { registry = original })

	err := CheckLiveGeneration(LiveSchemaFloor)
	var required *UpgradeRequiredError
	if !errors.As(err, &required) || !strings.Contains(required.Error(), "requires migration") {
		t.Fatalf("CheckLiveGeneration() error = %v, want upgrade requirement", err)
	}
	root := t.TempDir()
	writeLock(t, root, LiveSchemaFloor)
	_, _, err = Upgrade(context.Background(), root)
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), `migration "future"`) {
		t.Fatalf("Upgrade() error = %v, want named migration failure", err)
	}
}
