package contextq

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// TestOutsideContextPathReadsBothPathSpaces documents that an absolute request
// is refused as outside the repository in either path space. The classifier
// receives a slash-normalized path but asked only filepath.IsAbs, which on
// Windows answers false for "/etc/passwd" and let it fall through to
// pathNotFound.
//
// This is documentation, not a regression guard: on Unix path.IsAbs and
// filepath.IsAbs agree on every input below, so the table stays green against
// the pre-fix predicate too. Only a Windows run distinguishes them, and giving
// the predicate an injectable path-space seam purely to simulate that would be
// the test-only production indirection direct-injection-first rejects. It
// therefore carries no invariant marker.
func TestOutsideContextPathReadsBothPathSpaces(t *testing.T) {
	for _, p := range []string{"/etc/passwd", "/", "..", "../x", "../../x"} {
		if !outsideContextPath(p) {
			t.Errorf("outsideContextPath(%q) = false, want outside", p)
		}
	}
	for _, p := range []string{"a", "a/b", ".awf/config.yaml", "..a", "a/../b"} {
		if outsideContextPath(p) {
			t.Errorf("outsideContextPath(%q) = true, want inside", p)
		}
	}
}

// invariant: tooling/context-and-topic:context-path-attribution (TestContextRequestCensusGroupingAndClassification)
// invariant: tooling/context-and-topic:context-path-classification (TestContextRequestCensusGroupingAndClassification)
func TestContextRequestCensusGroupingAndClassification(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{{Path: "owned/a.go", Mode: snapshot.Regular}, {Path: "owned/b.go", Mode: snapshot.Regular}, {Path: "owned/c.go", Mode: snapshot.Regular}, {Path: "owned/d.go", Mode: snapshot.Regular}, {Path: "ignored/x", Mode: snapshot.Regular}, {Path: "nested/.awf/config.yaml", Mode: snapshot.Regular}, {Path: "nested/x", Mode: snapshot.Regular}, {Path: "link", Mode: snapshot.Symlink, Bytes: []byte("../x")}, {Path: "inside-link", Mode: snapshot.Symlink, Bytes: []byte("owned/a.go")}, {Path: "absolute-link", Mode: snapshot.Symlink, Bytes: []byte("/x")}, {Path: "unowned", Mode: snapshot.Regular}})
	if err != nil {
		t.Fatal(err)
	}
	covered := contextPathImpact{Classification: pathCovered, Domains: []domainRef{}, Topics: []contextPathTopic{}, Relationships: contextRelationships{State: []string{"d/t:kept"}, Touches: []string{}, Proofs: []string{}}, Provenance: []contextProvenance{}, Warnings: []contextWarning{}}
	set := contextPathSet{tree: tree, nested: []string{"nested"}, outputs: map[string]bool{"generated": true}, ignores: []string{"ignored/**"}, domainPaths: map[string][]string{"d": {"owned/**"}}, impacts: map[string]contextPathImpact{}}
	for _, f := range tree.List() {
		class, nested, target := classifyContextPath(f.Path, set)
		impact := covered
		impact.Classification = class
		impact.NestedRoot = nested
		impact.TargetInsideRepository = target
		set.impacts[f.Path] = impact
	}
	for _, p := range []string{"link", "inside-link", "absolute-link"} {
		impact := set.impacts[p]
		impact.Relationships = contextRelationships{State: []string{"d/t:excluded-symlink"}, Touches: []string{}, Proofs: []string{}}
		set.impacts[p] = impact
	}
	nestedImpact := set.impacts["nested/.awf/config.yaml"]
	nestedImpact.Relationships = contextRelationships{State: []string{"d/t:excluded-nested"}, Touches: []string{}, Proofs: []string{}}
	set.impacts["nested/.awf/config.yaml"] = nestedImpact
	ignoredImpact := set.impacts["ignored/x"]
	ignoredImpact.Relationships = contextRelationships{State: []string{"d/t:excluded"}, Touches: []string{}, Proofs: []string{}}
	set.impacts["ignored/x"] = ignoredImpact
	requests := buildContextRequests([]string{"", "owned", "owned/a.go", "owned/a.go", "  ", "ignored", "missing"}, set, ContextOptions{Selection: SelectionExplicit})
	if len(requests) != 5 || requests[0].Argument != "owned" || requests[2].Argument != "owned/a.go" {
		t.Fatalf("requests=%#v", requests)
	}
	dir := requests[0].Directory
	rootRequest := buildContextRequests([]string{"."}, set, ContextOptions{Selection: SelectionExplicit})[0]
	if rootRequest.Directory == nil || len(rootRequest.Directory.Excluded) != 3 {
		t.Fatalf("root census=%#v", rootRequest)
	}
	if !reflect.DeepEqual(rootRequest.Directory.Relationships, covered.Relationships) {
		t.Fatalf("root relationships include a boundary=%#v", rootRequest.Directory.Relationships)
	}
	if dir == nil || dir.Included != 4 || len(dir.Groups) != 1 || dir.Groups[0].Count != 4 || dir.Groups[0].Members == nil || len(dir.Groups[0].Members) != 0 {
		t.Fatalf("directory=%#v", dir)
	}
	if !reflect.DeepEqual(dir.Relationships, covered.Relationships) {
		t.Fatalf("directory relationships=%#v want=%#v", dir.Relationships, covered.Relationships)
	}
	ignoredDir := requests[3].Directory
	if ignoredDir == nil || len(ignoredDir.Relationships.State) != 0 {
		t.Fatalf("excluded descendant relationships=%#v", ignoredDir)
	}
	set.impacts["owned/a.go"] = covered
	git := buildContextRequests([]string{"owned/a.go"}, set, ContextOptions{Selection: SelectionStaged})
	if git[0].Kind != requestGitSelected || git[0].Exact == nil {
		t.Fatalf("git=%#v", git)
	}
	cases := map[string]pathClassification{"../x": pathOutsideRepository, "nested/x": pathNestedAdopter, "link": pathSymlink, "ignored/x": pathContextIgnored, "missing": pathNotFound, "owned/a.go": pathCovered, "generated": pathGeneratedOutput, "unowned": pathEligibleUnowned, "inside-link": pathSymlink, "absolute-link": pathSymlink}
	for p, want := range cases {
		got, _, _ := classifyContextPath(p, set)
		if got != want {
			t.Errorf("%s=%s", p, got)
		}
	}
}

func TestContextRelationshipsCollectDeduplicateAndUnion(t *testing.T) {
	sites := map[string][]topic.MarkerSite{"x.go": {
		{Kind: topic.ProofMarker, ClaimID: "d/t:tested-z"},
		{Kind: topic.StateMarker, ClaimID: "d/t:order-z"},
		{Kind: topic.TouchesMarker, ClaimID: "d/t:stable-z"},
		{Kind: topic.ProofMarker, ClaimID: "d/t:tested-a"},
		{Kind: topic.StateMarker, ClaimID: "d/t:order-a"},
		{Kind: topic.TouchesMarker, ClaimID: "d/t:stable-a"},
		{Kind: topic.ProofMarker, ClaimID: "d/t:tested-z"},
		{Kind: topic.StateMarker, ClaimID: "d/t:order-z"},
		{Kind: topic.TouchesMarker, ClaimID: "d/t:stable-z"},
	}}
	got := contextRelationshipsForPath(sites, "x.go")
	want := contextRelationships{State: []string{"d/t:order-a", "d/t:order-z"}, Touches: []string{"d/t:stable-a", "d/t:stable-z"}, Proofs: []string{"d/t:tested-a", "d/t:tested-z"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("relationships=%#v want=%#v", got, want)
	}
	other := contextRelationships{State: []string{"d/t:another"}, Touches: []string{"d/t:stable-z"}, Proofs: []string{"d/t:tested-z"}}
	gotBefore := contextRelationships{State: slices.Clone(got.State), Touches: slices.Clone(got.Touches), Proofs: slices.Clone(got.Proofs)}
	otherBefore := contextRelationships{State: slices.Clone(other.State), Touches: slices.Clone(other.Touches), Proofs: slices.Clone(other.Proofs)}
	union := unionContextRelationships(got, other)
	if !reflect.DeepEqual(got, gotBefore) || !reflect.DeepEqual(other, otherBefore) || !reflect.DeepEqual(union.State, []string{"d/t:another", "d/t:order-a", "d/t:order-z"}) {
		t.Fatalf("union=%#v first=%#v second=%#v", union, got, other)
	}
	empty := contextRelationshipsForPath(sites, "absent.go")
	if empty.State == nil || empty.Touches == nil || empty.Proofs == nil {
		t.Fatalf("nil empty slices=%#v", empty)
	}
}

func TestContextFacetsAndGroupKey(t *testing.T) {
	facets, err := ParseContextFacets([]string{"pending", "relationships", "invariants", "all-rules", "pending"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(facets, []ContextFacet{"relationships", "invariants", FacetAllRules, FacetPending}) {
		t.Fatal(facets)
	}
	full, err := ParseContextFacets([]string{"pending"}, true)
	wantFull := []ContextFacet{"relationships", "invariants", FacetAllRules, FacetEvidence, FacetSelectors, FacetReferences, FacetPending, FacetArtifacts}
	if err != nil || !reflect.DeepEqual(full, wantFull) {
		t.Fatalf("full=%v want=%v err=%v", full, wantFull, err)
	}
	if _, err := ParseContextFacets([]string{"unknown"}, false); err == nil {
		t.Fatal("unknown accepted")
	}
	a := contextPathImpact{Classification: pathCovered, Provenance: []contextProvenance{}, Domains: []domainRef{}, Topics: []contextPathTopic{}, Relationships: emptyContextRelationships(), Warnings: []contextWarning{}}
	inside := true
	a.TargetInsideRepository = &inside
	a.Provenance = []contextProvenance{{Role: "template", Identity: "x", Sources: []artifactLink{{Path: "s", Label: "source"}}, Outputs: []artifactLink{{Path: "o", Label: "output"}}, Navigation: []artifactLink{{Path: "n", Label: "navigation"}}}}
	a.Domains = []domainRef{{Name: "d", CurrentState: "docs/d.md"}}
	a.Topics = []contextPathTopic{{ID: "d/t"}}
	a.Relationships = contextRelationships{State: []string{"d/t:r"}, Touches: []string{}, Proofs: []string{"d/t:i"}}
	a.ADR = &adrArtifactContext{Number: "0001", Status: "Implementing", Operations: []adrOperationContext{{Operation: "add", Claim: "d/t:r", Progress: "remaining", ClaimState: "not-yet-current"}}}
	_ = contextGroupKey(a, nil)
	b := a
	b.Warnings = []contextWarning{warningGlobLiteral}
	if contextGroupKey(a, nil) == contextGroupKey(b, nil) {
		t.Fatal("warning omitted from key")
	}
	if !strings.Contains(string(warningEligibleUnowned), "no domain") {
		t.Fatal("warning contract")
	}
}

// invariant: tooling/context-and-topic:context-full-authority-packet (TestContextDirectoryGroupingUsesOnlyVisibleProjection)
func TestContextDirectoryGroupingUsesOnlyVisibleProjection(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{{Path: "dir/a.go", Mode: snapshot.Regular}, {Path: "dir/b.go", Mode: snapshot.Regular}})
	if err != nil {
		t.Fatal(err)
	}
	makeImpact := func(claim, source string) contextPathImpact {
		return contextPathImpact{
			Classification: pathCovered,
			Provenance:     []contextProvenance{{Role: "template", Identity: "shared", Sources: []artifactLink{{Path: source, Label: "source"}}, Outputs: []artifactLink{}, Navigation: []artifactLink{}}},
			Domains:        []domainRef{{Name: "d", CurrentState: "docs/domains/d.md"}},
			Topics:         []contextPathTopic{{ID: "d/t"}},
			Relationships:  contextRelationships{State: []string{claim}, Touches: []string{}, Proofs: []string{}},
			Warnings:       []contextWarning{},
		}
	}
	set := contextPathSet{tree: tree, impacts: map[string]contextPathImpact{
		"dir/a.go": makeImpact("d/t:a", "source-a"),
		"dir/b.go": makeImpact("d/t:b", "source-b"),
	}}
	cases := []struct {
		name   string
		facets []ContextFacet
		groups int
	}{
		{name: "bare", groups: 1},
		{name: "artifacts", facets: []ContextFacet{FacetArtifacts}, groups: 2},
		{name: "relationships", facets: []ContextFacet{"relationships"}, groups: 1},
		{name: "invariants", facets: []ContextFacet{"invariants"}, groups: 1},
		{name: "all-rules", facets: []ContextFacet{FacetAllRules}, groups: 1},
		{name: "evidence", facets: []ContextFacet{FacetEvidence}, groups: 1},
		{name: "references", facets: []ContextFacet{FacetReferences}, groups: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := buildContextRequests([]string{"dir"}, set, ContextOptions{Selection: SelectionExplicit, Facets: tc.facets})[0]
			if got := len(request.Directory.Groups); got != tc.groups {
				t.Fatalf("groups=%d want=%d: %#v", got, tc.groups, request.Directory.Groups)
			}
		})
	}
}
