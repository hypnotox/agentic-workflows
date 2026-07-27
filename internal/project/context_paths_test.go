package project

import (
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// invariant: tooling/context-and-topic:context-path-attribution
// invariant: tooling/context-and-topic:context-path-classification
func TestContextRequestCensusGroupingAndClassification(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{{Path: "owned/a.go", Mode: snapshot.Regular}, {Path: "owned/b.go", Mode: snapshot.Regular}, {Path: "owned/c.go", Mode: snapshot.Regular}, {Path: "owned/d.go", Mode: snapshot.Regular}, {Path: "ignored/x", Mode: snapshot.Regular}, {Path: "nested/.awf/config.yaml", Mode: snapshot.Regular}, {Path: "nested/x", Mode: snapshot.Regular}, {Path: "link", Mode: snapshot.Symlink, Bytes: []byte("../x")}, {Path: "inside-link", Mode: snapshot.Symlink, Bytes: []byte("owned/a.go")}, {Path: "absolute-link", Mode: snapshot.Symlink, Bytes: []byte("/x")}, {Path: "unowned", Mode: snapshot.Regular}})
	if err != nil {
		t.Fatal(err)
	}
	covered := ContextPathImpact{Classification: PathCovered, Domains: []DomainRef{}, Topics: []ContextPathTopic{}, DirectRuleIDs: []string{}, InvariantIDs: []string{}, ProofIDs: []string{}, Provenance: []ContextProvenance{}, Warnings: []ContextWarning{}}
	set := contextPathSet{tree: tree, nested: []string{"nested"}, outputs: map[string]bool{"generated": true}, ignores: []string{"ignored/**"}, domainPaths: map[string][]string{"d": {"owned/**"}}, impacts: map[string]ContextPathImpact{}}
	for _, f := range tree.List() {
		class, nested, target := classifyContextPath(f.Path, set)
		impact := covered
		impact.Classification = class
		impact.NestedRoot = nested
		impact.TargetInsideRepository = target
		set.impacts[f.Path] = impact
	}
	requests := buildContextRequests([]string{"", "owned", "owned/a.go", "owned/a.go", "  ", "ignored", "missing"}, set, ContextOptions{Selection: SelectionExplicit})
	if len(requests) != 5 || requests[0].Argument != "owned" || requests[2].Argument != "owned/a.go" {
		t.Fatalf("requests=%#v", requests)
	}
	dir := requests[0].Directory
	rootRequest := buildContextRequests([]string{"."}, set, ContextOptions{Selection: SelectionExplicit})[0]
	if rootRequest.Directory == nil || len(rootRequest.Directory.Excluded) != 3 {
		t.Fatalf("root census=%#v", rootRequest)
	}
	if dir == nil || dir.Included != 4 || len(dir.Groups) != 1 || dir.Groups[0].Count != 4 || dir.Groups[0].Members == nil || len(dir.Groups[0].Members) != 0 {
		t.Fatalf("directory=%#v", dir)
	}
	set.impacts["owned/a.go"] = covered
	git := buildContextRequests([]string{"owned/a.go"}, set, ContextOptions{Selection: SelectionStaged})
	if git[0].Kind != RequestGitSelected || git[0].Exact == nil {
		t.Fatalf("git=%#v", git)
	}
	cases := map[string]PathClassification{"../x": PathOutsideRepository, "nested/x": PathNestedAdopter, "link": PathSymlink, "ignored/x": PathContextIgnored, "missing": PathNotFound, "owned/a.go": PathCovered, "generated": PathGeneratedOutput, "unowned": PathEligibleUnowned, "inside-link": PathSymlink, "absolute-link": PathSymlink}
	for p, want := range cases {
		got, _, _ := classifyContextPath(p, set)
		if got != want {
			t.Errorf("%s=%s", p, got)
		}
	}
}

func TestContextFacetsAndGroupKey(t *testing.T) {
	facets, err := ParseContextFacets([]string{"pending", "all-rules", "pending"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(facets, []ContextFacet{FacetAllRules, FacetPending}) {
		t.Fatal(facets)
	}
	full, err := ParseContextFacets([]string{"pending"}, true)
	if err != nil || len(full) != 6 {
		t.Fatalf("full=%v err=%v", full, err)
	}
	if _, err := ParseContextFacets([]string{"unknown"}, false); err == nil {
		t.Fatal("unknown accepted")
	}
	a := ContextPathImpact{Classification: PathCovered, Provenance: []ContextProvenance{}, Domains: []DomainRef{}, Topics: []ContextPathTopic{}, DirectRuleIDs: []string{}, InvariantIDs: []string{}, ProofIDs: []string{}, Warnings: []ContextWarning{}}
	inside := true
	a.TargetInsideRepository = &inside
	a.Provenance = []ContextProvenance{{Role: "template", Identity: "x", Sources: []ArtifactLink{{Path: "s", Label: "source"}}, Outputs: []ArtifactLink{{Path: "o", Label: "output"}}, Navigation: []ArtifactLink{{Path: "n", Label: "navigation"}}}}
	a.Domains = []DomainRef{{Name: "d", CurrentState: "docs/d.md"}}
	a.Topics = []ContextPathTopic{{ID: "d/t", DirectClaimIDs: []string{"d/t:r"}}}
	a.DirectRuleIDs = []string{"d/t:r"}
	a.InvariantIDs = []string{"d/t:i"}
	a.ProofIDs = []string{"d/t:i"}
	a.ADR = &ADRArtifactContext{Number: "0001", Status: "Implementing", Operations: []ADROperationContext{{Operation: "add", Claim: "d/t:r", Progress: "remaining", ClaimState: "not-yet-current", StateSequence: 2}}}
	_ = contextGroupKey(a)
	b := a
	b.Warnings = []ContextWarning{WarningGlobLiteral}
	if contextGroupKey(a) == contextGroupKey(b) {
		t.Fatal("warning omitted from key")
	}
	if !strings.Contains(string(WarningEligibleUnowned), "no domain") {
		t.Fatal("warning contract")
	}
}
