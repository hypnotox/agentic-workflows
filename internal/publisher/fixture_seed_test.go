package publisher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitializedSampleSeedClonesAreIsolated(t *testing.T) {
	first, _ := initializedSampleProject(t)
	digest := initializedSampleSeed.Digest()
	second, _ := initializedSampleProject(t)
	if err := os.WriteFile(filepath.Join(first, "AGENTS.md"), []byte("mutated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(second, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) == "mutated\n" {
		t.Fatal("second publisher clone changed through first")
	}
	if initializedSampleSeed.Digest() != digest {
		t.Fatal("publisher sample seed digest changed after clone mutation")
	}
}
