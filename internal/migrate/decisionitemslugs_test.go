package migrate

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestDecisionItemSlugsMigrationPreservesAuthoredBytes(t *testing.T) {
	root := t.TempDir()
	fixtures := map[string][]byte{
		".awf/config.yaml":             []byte("prefix: fixture\nskills: []\nagents: []\n"),
		"docs/decisions/0001-v1.md":    []byte("---\nformat: current-state-v1\nstatus: Proposed\ndate: 2026-01-01\n---\n# ADR-0001: V1\n\n## Decision\n\n1. Historical V1.\n"),
		"docs/decisions/0002-v2.md":    []byte("---\nformat: current-state-v2\nstatus: Proposed\ndate: 2026-01-02\n---\n# ADR-0002: V2\n\n## Decision\n\n1. Historical V2.\n"),
		"docs/decisions/v3-pending.md": []byte("---\nformat: current-state-v3\nslug: v3-pending\nstatus: Proposed\ndate: 2026-01-03\n---\n# ADR-v3-pending: V3\n\n## Decision\n\n1. Historical V3.\n"),
	}
	for path, content := range fixtures {
		testsupport.WriteFile(t, filepath.Join(root, path), string(content))
	}
	stampLockAt(t, filepath.Join(root, ".awf", "awf.lock"), 39)
	ordinary := []byte("ordinary authored bytes\n")
	testsupport.WriteFile(t, filepath.Join(root, "notes.txt"), string(ordinary))

	var out Changes
	applied, _, err := Upgrade(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	wantApplied := []string{registry[len(registry)-6].Name, registry[len(registry)-5].Name, registry[len(registry)-4].Name, registry[len(registry)-3].Name, registry[len(registry)-2].Name, registry[len(registry)-1].Name}
	if !reflect.DeepEqual(applied, wantApplied) || out.Len() != 0 {
		t.Fatalf("upgrade = %v, output %q; want %v", applied, out.String(), wantApplied)
	}
	for path, want := range fixtures {
		if path == ".awf/config.yaml" {
			want = []byte("prefix: fixture\n")
		}
		got, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s after upgrade = %q, %v; want %q", path, got, err, want)
		}
	}
	got, err := os.ReadFile(filepath.Join(root, "notes.txt"))
	if err != nil || !bytes.Equal(got, ordinary) {
		t.Fatalf("ordinary bytes after upgrade = %q, %v", got, err)
	}
	if generation, err := Generation(root); err != nil || generation != Current() {
		t.Fatalf("generation = %d, %v; want Current()=%d", generation, err, Current())
	}
	if err := applyDecisionItemSlugs(testContext(t), root, &Changes{}); err != nil {
		t.Fatal(err)
	}
}
