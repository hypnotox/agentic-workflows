package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// invariant: tooling/test-infrastructure:immutable-fixture-seeds (TestScaffoldProjectSeedClonesAreCleanAndIsolated)
func TestScaffoldProjectSeedClonesAreCleanAndIsolated(t *testing.T) {
	first := scaffoldProject(t)
	digest := scaffoldSeed.Digest()
	second := scaffoldProject(t)
	firstStatus, err := gitfixture.NativeRun(gitfixture.At(first), "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	secondStatus, err := gitfixture.NativeRun(gitfixture.At(second), "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if firstStatus == "" || firstStatus != secondStatus {
		t.Fatalf("cloned Git index state differs: first=%q second=%q", firstStatus, secondStatus)
	}
	if firstHead, secondHead := gitfixture.NativeRevParse(t, gitfixture.At(first), "HEAD"), gitfixture.NativeRevParse(t, gitfixture.At(second), "HEAD"); firstHead != secondHead {
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
	if scaffoldSeed.Digest() != digest {
		t.Fatal("scaffold seed digest changed after clone mutation")
	}
}
