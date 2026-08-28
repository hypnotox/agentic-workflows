package git

import (
	"runtime"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestRangeTouchedPathsPreservesInvalidUTF8(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires a byte-preserving filesystem")
	}
	fixture := gitfixture.InitRepo(t)
	odd := " invalid-\xff "
	base := gitfixture.Commit(t, fixture, "base", map[string]string{odd: "base\n"})
	head := gitfixture.Commit(t, fixture, "modify", map[string]string{odd: "changed\n"})
	repo := walkRepo(t, fixture.Root())
	paths, err := repo.RangeTouchedPaths(testContext(t), base, head)
	if err != nil || len(paths) != 1 || paths[0] != odd {
		t.Fatalf("invalid UTF-8 path = %#v, %v", paths, err)
	}
}

func TestRangeTouchedPathsAccumulatesRestoredPaths(t *testing.T) {
	fixture := gitfixture.InitRepo(t)
	odd := " odd ` "
	base := gitfixture.Commit(t, fixture, "base", map[string]string{"restored.txt": "base\n", odd: "base\n"})
	gitfixture.Commit(t, fixture, "modify", map[string]string{"restored.txt": "changed\n", odd: "changed\n", "transient.txt": "present\n"})
	head := gitfixture.Commit(t, fixture, "restore", map[string]string{"restored.txt": "base\n", odd: "base\n"}, "transient.txt")
	repo := walkRepo(t, fixture.Root())
	paths, err := repo.RangeTouchedPaths(testContext(t), base, head)
	if err != nil || strings.Join(paths, ",") != odd+",restored.txt,transient.txt" {
		t.Fatalf("multi-commit accumulated paths = %#v, %v", paths, err)
	}
	if _, err := repo.RangeTouchedPaths(testContext(t), "missing", head); err == nil {
		t.Fatal("missing range base accepted")
	}

	gitfixture.CheckoutNewBranch(t, fixture, "feature", head)
	feature := gitfixture.Commit(t, fixture, "feature", map[string]string{"feature.txt": "feature\n"})
	gitfixture.NativeCheckout(t, fixture, "master")
	main := gitfixture.Commit(t, fixture, "main", map[string]string{"main.txt": "main\n"})
	gitfixture.Stage(t, fixture, map[string]string{"feature.txt": "feature\n", "resolution-only.txt": "merge result\n"})
	merge := gitfixture.Merge(t, fixture, "merge", main, feature)
	paths, err = repo.RangeTouchedPaths(testContext(t), main, merge)
	if err != nil || strings.Join(paths, ",") != "feature.txt,resolution-only.txt" {
		t.Fatalf("merge-resolution accumulated paths = %#v, %v", paths, err)
	}
}
