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
		{Path: "unowned.txt", Mode: snapshot.Regular, Bytes: []byte("x")},
		{Path: "ignored-link", Mode: snapshot.Symlink, Bytes: []byte("owned/a.go")},
		{Path: "generated-link", Mode: snapshot.Symlink, Bytes: []byte("owned/a.go")},
	})
	if err != nil {
		t.Fatal(err)
	}
	set := contextPathSet{tree: tree, eligible: []string{"owned/a.go", "unowned.txt"}, nested: []string{"nested"}, outputs: map[string]bool{"../escape": true, "nested/x.go": true, "planned.md": true, "ignored/generated.md": true, "generated-link": true}, ignores: []string{"ignored/**", "empty/**", "ignored-link"}, domainPaths: map[string][]string{"d": {"owned/**", "ignored/**"}}}
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
	cases := map[string]PathClassification{"../escape": PathOutsideRepository, "nested/x.go": PathNestedAdopter, "planned.md": PathGeneratedOutput, "ignored/generated.md": PathGeneratedOutput, "generated-link": PathGeneratedOutput, "link": PathSymlink, "inside-link": PathSymlink, "absolute-link": PathSymlink, "ignored-link": PathSymlink, "ignored/x.go": PathContextIgnored, "ignored/missing.go": PathContextIgnored, "missing": PathNotFound, "owned/a.go": PathCovered, "unowned.txt": PathEligibleUnowned}
	for p, want := range cases {
		got, _, inside := classifyContextPath(p, set)
		if got != want {
			t.Errorf("%s=%s want %s", p, got, want)
		}
		if p == "nested/x.go" {
			_, nestedRoot, _ := classifyContextPath(p, set)
			if nestedRoot != "nested/.awf/config.yaml" {
				t.Errorf("nested root = %q", nestedRoot)
			}
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
	b, err := json.Marshal(ContextResult{Projection: ContextConcise, Requests: []ContextRequest{}, Topics: []InvocationTopicContext{}, Paths: []ContextPath{}})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "{\"projection\":\"concise\",\"requests\":[],\"topics\":[],\"paths\":[]}" {
		t.Fatalf("non-null JSON=%s", b)
	}
	nestedJSON, err := json.Marshal(ContextPath{Path: "nested/x.go", Requests: []string{"nested/x.go"}, Classification: PathNestedAdopter, NestedRoot: "nested/.awf/config.yaml", Domains: []DomainRef{}, Topics: []PathTopicRef{}, Artifacts: []ArtifactRecord{}})
	if err != nil {
		t.Fatal(err)
	}
	if string(nestedJSON) != "{\"path\":\"nested/x.go\",\"requests\":[\"nested/x.go\"],\"classification\":\"nested-adopter\",\"nestedRoot\":\"nested/.awf/config.yaml\",\"domains\":[],\"topics\":[],\"artifacts\":[]}" {
		t.Fatalf("nested JSON = %s", nestedJSON)
	}
}
