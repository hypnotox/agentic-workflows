package migrate

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestDecisionItemSlugsMigrationPreservesAuthoredBytes(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "prefix: fixture\nskills: []\nagents: []\n")
	stampLockAt(t, filepath.Join(root, ".awf", "awf.lock"), 32)
	fixtures := map[string][]byte{
		"docs/decisions/0001-v1.md":    []byte("---\nformat: current-state-v1\nstatus: Proposed\ndate: 2026-01-01\n---\n# ADR-0001: V1\n\n## Decision\n\n1. Historical V1.\n"),
		"docs/decisions/0002-v2.md":    []byte("---\nformat: current-state-v2\nstatus: Proposed\ndate: 2026-01-02\n---\n# ADR-0002: V2\n\n## Decision\n\n1. Historical V2.\n"),
		"docs/decisions/v3-pending.md": []byte("---\nformat: current-state-v3\nslug: v3-pending\nstatus: Proposed\ndate: 2026-01-03\n---\n# ADR-v3-pending: V3\n\n## Decision\n\n1. Historical V3.\n"),
	}
	for path, content := range fixtures {
		testsupport.WriteFile(t, filepath.Join(root, path), string(content))
	}
	ordinary := []byte("ordinary authored bytes\n")
	testsupport.WriteFile(t, filepath.Join(root, "notes.txt"), string(ordinary))

	var out bytes.Buffer
	applied, err := Upgrade(testContext(t), root, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(applied, []string{"decision-item-slugs"}) || out.Len() != 0 {
		t.Fatalf("upgrade = %v, output %q", applied, out.String())
	}
	for path, want := range fixtures {
		got, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s after upgrade = %q, %v; want %q", path, got, err, want)
		}
	}
	got, err := os.ReadFile(filepath.Join(root, "notes.txt"))
	if err != nil || !bytes.Equal(got, ordinary) {
		t.Fatalf("ordinary bytes after upgrade = %q, %v", got, err)
	}
	if generation, err := Generation(root); err != nil || generation != 33 {
		t.Fatalf("generation = %d, %v; want 33", generation, err)
	}
	if err := applyDecisionItemSlugs(testContext(t), root, io.Discard); err != nil {
		t.Fatal(err)
	}
}
