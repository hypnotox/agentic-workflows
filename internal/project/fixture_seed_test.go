package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// invariant: tooling/test-infrastructure:immutable-fixture-seeds (TestGitScaffoldSeedClonesAreIsolated)
func TestGitScaffoldSeedClonesAreIsolated(t *testing.T) {
	first := gitScaffold(t, defaultFixtureBranch)
	digest := gitScaffoldSeed.Digest()
	second := gitScaffold(t, defaultFixtureBranch)
	firstHead := gitfixture.NativeRevParse(t, gitfixture.At(first), "HEAD")
	secondHead := gitfixture.NativeRevParse(t, gitfixture.At(second), "HEAD")
	if firstHead != secondHead {
		t.Fatalf("cloned HEAD identity differs: first=%s second=%s", firstHead, secondHead)
	}
	if err := os.WriteFile(filepath.Join(first, "README.md"), []byte("mutated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(second, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "base\n" {
		t.Fatalf("second clone changed through first: %q", body)
	}
	if gitScaffoldSeed.Digest() != digest {
		t.Fatal("project Git seed digest changed after clone mutation")
	}
}
