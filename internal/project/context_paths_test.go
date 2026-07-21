package project

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// invariant: tooling/cli:context-path-attribution
// invariant: tooling/cli:context-path-classification
func TestContextPathRequestsAndClassification(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: "owned/a.go", Mode: snapshot.Regular, Bytes: []byte("a")},
		{Path: "ignored/x.go", Mode: snapshot.Regular, Bytes: []byte("x")},
		{Path: "nested/.awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("x")},
		{Path: "nested/x.go", Mode: snapshot.Regular, Bytes: []byte("x")},
		{Path: "link", Mode: snapshot.Symlink, Bytes: []byte("../outside")},
		{Path: "inside-link", Mode: snapshot.Symlink, Bytes: []byte("owned/a.go")},
		{Path: "absolute-link", Mode: snapshot.Symlink, Bytes: []byte("/outside")},
		{Path: "empty/ignored.go", Mode: snapshot.Regular, Bytes: []byte("x")},
	})
	if err != nil {
		t.Fatal(err)
	}
	set := contextPathSet{tree: tree, eligible: []string{"owned/a.go"}, nested: []string{"nested"}, outputs: map[string]bool{"planned.md": true, "ignored/generated.md": true}, ignores: []string{"ignored/**", "empty/**"}, domainPaths: map[string][]string{"d": {"owned/**", "ignored/**"}}}
	reqs, paths := buildContextRequests([]string{"", "owned", "owned/a.go", "owned/a.go", "../escape", "planned.md", "empty"}, false, set)
	var owned ContextRequest
	for _, r := range reqs {
		if r.Query == "owned" {
			owned = r
		}
	}
	if len(reqs) != 5 || owned.Status != RequestDirectoryExpanded || !reflect.DeepEqual(paths["owned/a.go"], []string{"owned", "owned/a.go"}) {
		t.Fatalf("requests=%#v attribution=%#v", reqs, paths)
	}
	cases := map[string]PathClassification{"../escape": PathOutsideRepository, "nested/x.go": PathNestedAdopter, "planned.md": PathGeneratedOutput, "ignored/generated.md": PathGeneratedOutput, "link": PathSymlink, "inside-link": PathSymlink, "absolute-link": PathSymlink, "ignored/x.go": PathContextIgnored, "missing": PathNotFound, "owned/a.go": PathCovered}
	for p, want := range cases {
		got, _, inside := classifyContextPath(p, set)
		if got != want {
			t.Errorf("%s=%s want %s", p, got, want)
		}
		if p == "link" && (inside == nil || *inside) {
			t.Errorf("escaping link inside=%v", inside)
		}
		if p == "absolute-link" && (inside == nil || *inside) {
			t.Errorf("absolute link inside=%v", inside)
		}
		if p == "inside-link" && (inside == nil || !*inside) {
			t.Errorf("inside link inside=%v", inside)
		}
	}
	gitReqs, _ := buildContextRequests([]string{"owned/a.go"}, true, set)
	if gitReqs[0].Status != RequestGitSelected {
		t.Fatalf("git request=%#v", gitReqs)
	}
	var empty ContextRequest
	for _, r := range reqs {
		if r.Query == "empty" {
			empty = r
		}
	}
	if empty.Status != RequestDirectoryEmpty || len(empty.EffectivePaths) != 0 {
		t.Fatalf("empty=%#v", empty)
	}
	b, err := json.Marshal(ContextResult{Projection: ContextConcise, Requests: []ContextRequest{}, Paths: []ContextPath{}})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "{\"projection\":\"concise\",\"requests\":[],\"paths\":[]}" {
		t.Fatalf("non-null JSON=%s", b)
	}
}
