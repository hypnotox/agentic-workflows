package git

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestRangeTouchedPathsAccumulatesRestoredPaths(t *testing.T) {
	fixture := gitfixture.InitRepo(t)
	odd := " odd `\xff "
	base := gitfixture.Commit(t, fixture, "base", map[string]string{"restored.txt": "base\n", odd: "base\n"})
	gitfixture.Commit(t, fixture, "modify", map[string]string{"restored.txt": "changed\n", odd: "changed\n"})
	head := gitfixture.Commit(t, fixture, "restore", map[string]string{"restored.txt": "base\n", odd: "base\n"})
	repo := walkRepo(t, fixture.Root())
	paths, err := repo.RangeTouchedPaths(testContext(t), base, head)
	if err != nil || len(paths) != 2 || paths[0] != odd || paths[1] != "restored.txt" {
		t.Fatalf("accumulated paths = %#v, %v", paths, err)
	}
	if _, err := repo.RangeTouchedPaths(testContext(t), "missing", head); err == nil {
		t.Fatal("missing range base accepted")
	}

	gitfixture.CheckoutNewBranch(t, fixture, "feature", head)
	feature := gitfixture.Commit(t, fixture, "feature", map[string]string{"feature.txt": "feature\n"})
	gitfixture.NativeCheckout(t, fixture, "master")
	gitfixture.Stage(t, fixture, map[string]string{"feature.txt": "feature\n"})
	merge := gitfixture.Merge(t, fixture, "merge", head, feature)
	paths, err = repo.RangeTouchedPaths(testContext(t), head, merge)
	if err != nil || strings.Join(paths, ",") != "feature.txt" {
		t.Fatalf("merge accumulated paths = %#v, %v", paths, err)
	}
}
