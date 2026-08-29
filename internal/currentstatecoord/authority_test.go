package currentstatecoord

import (
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// The focused census retains contextq's topmost-directory semantics without
// importing its context result model: owned file descendants stop a collapse,
// while no owned descendant permits root collapse.
func TestAuthorityCensusTopmostCollapseAndIndependentExclusions(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: "owned/live.go", Mode: snapshot.Regular},
		{Path: "free/a.txt", Mode: snapshot.Regular},
		{Path: "free/b.txt", Mode: snapshot.Regular},
		{Path: "also-free/a.txt", Mode: snapshot.Regular},
		{Path: "generated/out.txt", Mode: snapshot.Regular},
		{Path: ".awf/efforts/current/memory.md", Mode: snapshot.Regular},
		{Path: "nested/.awf/config.yaml", Mode: snapshot.Regular},
		{Path: "nested/hidden.txt", Mode: snapshot.Regular},
	})
	if err != nil {
		t.Fatal(err)
	}
	paths := authorityCensusPaths(tree.List(), map[string]bool{"generated/out.txt": true})
	wantPaths := []string{"also-free/a.txt", "free/a.txt", "free/b.txt", "owned/live.go"}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("census paths = %v, want %v", paths, wantPaths)
	}
	if got, want := collapseUnowned([]string{"also-free/a.txt", "free/a.txt", "free/b.txt"}, map[string]bool{"owned/live.go": true}), []string{"also-free/", "free/"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("collapsed = %v, want %v", got, want)
	}
	if got, want := collapseUnowned([]string{"free/a.txt", "other/b.txt"}, nil), []string{"."}; !reflect.DeepEqual(got, want) {
		t.Fatalf("root collapsed = %v, want %v", got, want)
	}
}
