package migrate

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCommitPolicyMigrationPreservesAbsentConfigBytes(t *testing.T) {
	src := []byte("prefix: example\nintegrationBranch: main\nskills: []\nagents: []\n")
	root := t.TempDir()
	configPath := filepath.Join(root, ".awf", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, src, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyCommitPolicy(root, io.Discard); err != nil {
		t.Fatalf("applyCommitPolicy: %v", err)
	}
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, src) {
		t.Fatalf("absent-policy config changed:\n%s", got)
	}
	found := false
	for _, migration := range registry {
		if migration.Name == "commit-policy" {
			found = migration.To == Current()
		}
	}
	if !found {
		t.Fatalf("commit-policy migration is not the current generation: %v", registry)
	}
}
