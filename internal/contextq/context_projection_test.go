package contextq

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// invariant: tooling/context-and-topic:context-summary-projection
// invariant: tooling/context-and-topic:context-concise-projection
func TestClaimSummaryProjection(t *testing.T) {
	parse := func(summary, prose string) topic.Claim {
		t.Helper()
		part := "Intro.\n\n## Claims\n\n### `rule: x`\n" + prose + "\n" + summary + "Origin: ADR-0001\n"
		parsed, err := topic.ParsePart(topic.TopicID{Domain: "alpha", Slug: "one"}, "part", []byte(part))
		if err != nil {
			t.Fatal(err)
		}
		return parsed.Claims[0]
	}
	declared := strings.Repeat("é", 160)
	if got := claimSummary(parse("Summary: "+declared+"\n", "ignored fallback")); got != declared {
		t.Fatalf("declared summary=%q", got)
	}
	cases := []struct {
		name  string
		prose string
		want  string
	}{
		{"fold first paragraph", "First\nline.\n\nSecond paragraph.", "First line."},
		{"ASCII 160 unchanged", strings.Repeat("a", 160), strings.Repeat("a", 160)},
		{"ASCII 161 hard cut", strings.Repeat("a", 161), strings.Repeat("a", 157) + "..."},
		{"Unicode 161 hard cut by code point", strings.Repeat("é", 161), strings.Repeat("é", 157) + "..."},
		{"truncate at word boundary", strings.Repeat("word ", 40), strings.TrimSpace(strings.Repeat("word ", 31)) + "..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := claimSummary(parse("", tc.prose))
			if got != tc.want || len([]rune(got)) > 160 {
				t.Errorf("runes=%d got=%q want=%q", len([]rune(got)), got, tc.want)
			}
		})
	}
}

func TestProjectionHelpers(t *testing.T) {
	changes := []pendingChange{{ADR: "0004"}, {ADR: "0001"}, {ADR: "0002"}, {ADR: "0003"}}
	bounded := contextPending(changes, false)
	if bounded.OperationCount != 4 || bounded.AdditionalADRCount != 1 || len(bounded.Operations) != 0 {
		t.Fatal(bounded)
	}
	expanded := contextPending(changes, true)
	if len(expanded.Operations) != 4 {
		t.Fatal(expanded)
	}
	var corpus topic.Corpus
	if got := claimStateForOperation("remove", "d/t:x", "applied", corpus, nil); got != "historically-removed" {
		t.Fatal(got)
	}
	history := &topic.ClaimHistory{RemovedBy: &topic.ADRHistory{Number: "1"}}
	if got := claimStateForOperation("add", "d/t:x", "applied", corpus, history); got != "historically-removed" {
		t.Fatal(got)
	}
	if got := claimStateForOperation("add", "d/t:x", "remaining", corpus, nil); got != "not-yet-current" {
		t.Fatal(got)
	}
}

// invariant: tooling/context-and-topic:context-concise-projection
// invariant: tooling/context-and-topic:context-path-attribution
func TestContextDirectProjectionDeduplicatesMixedRequests(t *testing.T) {
	files := ctxFiles()
	files["internal/foo/x_test.go"] = "package foo\n// state: alpha/one:order\n// touches-state: alpha/one:stable - exercised here\n// touches-state: alpha/one:stable - exercised here\n// invariant: alpha/one:tested\n// invariant: alpha/one:tested\n"
	q := queryFor(t, ctxRepo(t, ctxConfig, files))
	res := q.ContextForOptions([]string{"internal/foo", "internal/foo/x_test.go"}, ContextOptions{Selection: SelectionExplicit})
	if len(res.Requests) != 2 || res.Requests[0].Index != 1 || res.Requests[1].Index != 2 || res.Requests[0].Directory == nil || res.Requests[1].Exact == nil {
		t.Fatalf("requests=%#v", res.Requests)
	}
	impact := res.Requests[1].Exact.Context
	wantRelationships := contextRelationships{State: []string{"alpha/one:order"}, Touches: []string{"alpha/one:stable"}, Proofs: []string{"alpha/one:tested"}}
	if !reflect.DeepEqual(impact.Relationships, wantRelationships) {
		t.Fatalf("relationships=%#v want=%#v", impact.Relationships, wantRelationships)
	}
	if !reflect.DeepEqual(res.Requests[0].Directory.Relationships, wantRelationships) {
		t.Fatalf("directory relationships=%#v want=%#v", res.Requests[0].Directory.Relationships, wantRelationships)
	}
	var alpha topicImpact
	for _, impact := range res.Topics {
		if impact.ID == "alpha/one" {
			alpha = impact
		}
	}
	if len(alpha.Direct) != 3 {
		t.Fatalf("direct claims=%#v", alpha.Direct)
	}
	if got := []string{alpha.Direct[0].ID, alpha.Direct[1].ID, alpha.Direct[2].ID}; !slices.Equal(got, []string{"alpha/one:order", "alpha/one:stable", "alpha/one:tested"}) {
		t.Fatalf("globally deduplicated direct claims=%v", got)
	}
	wantKinds := map[string][]string{"alpha/one:order": {"State"}, "alpha/one:stable": {"Touches"}, "alpha/one:tested": {"Proofs"}}
	for _, claim := range alpha.Direct {
		if len(claim.Sources) != 1 || claim.Sources[0].RequestIndex != 2 || !slices.Equal(claim.Sources[0].Kinds, wantKinds[claim.ID]) {
			t.Fatalf("claim sources=%#v", claim)
		}
	}
	xResult := q.ContextForOptions([]string{"internal/foo/x.go"}, ContextOptions{Selection: SelectionExplicit})
	xImpact := xResult.Requests[0].Exact.Context
	if len(xImpact.Relationships.Proofs) != 0 {
		t.Fatalf("actual proof filtering=%#v", xImpact.Relationships)
	}
}

// invariant: tooling/context-and-topic:context-concise-projection
// invariant: tooling/context-and-topic:context-full-authority-packet
func TestContextRequestTiersAndAuthorityExpansion(t *testing.T) {
	files := ctxFiles()
	files[".awf/topics/parts/alpha/one/current-state.md"] = "Intro.\n\n## Claims\n\n### `rule: order`\nOrder.\nOrigin: ADR-0001\nReferences: core/g:everywhere\n\n### `rule: extra`\nExtra.\nOrigin: ADR-0001\n\n### `invariant: tested`\nTested.\nOrigin: ADR-0001\nBacking: test\n\n### `invariant: stable`\nStable.\nOrigin: ADR-0001\nBacking: unbacked\nVerify: inspect.\n"
	files[".awf/topics/metadata/alpha/two.yaml"] = "title: Two\nsummary: The second topic.\npaths:\n  - internal/foo/**\n"
	files[".awf/topics/parts/alpha/two/current-state.md"] = "Intro.\n\n## Claims\n\n### `rule: second`\nSecond.\nOrigin: ADR-0001\nReferences: core/g:everywhere\n"
	files[".awf/topics/parts/core/g/current-state.md"] = "Intro.\n\n## Claims\n\n### `invariant: everywhere`\nGlobal invariant.\nOrigin: ADR-0001\nBacking: unbacked\nVerify: inspect global state.\n"
	files["internal/evidence/core.go"] = "package evidence\n// state: core/g:everywhere\n"
	q := queryFor(t, ctxRepo(t, ctxConfig, files))
	find := func(res ContextResult, id string) topicImpact {
		t.Helper()
		for _, impact := range res.Topics {
			if impact.ID == id {
				return impact
			}
		}
		t.Fatalf("missing topic %s in %#v", id, res.Topics)
		return topicImpact{}
	}
	ids := func(claims []contextClaimImpact) []string {
		out := []string{}
		for _, claim := range claims {
			out = append(out, claim.ID)
		}
		return out
	}
	for _, selection := range []ContextSelection{SelectionExplicit, SelectionStaged, SelectionRange} {
		res := q.ContextForOptions([]string{"internal/foo/x.go"}, ContextOptions{Selection: selection, Range: "a..b"})
		if got := res.Requests[0].Exact.Context.Relationships; !reflect.DeepEqual(got, (contextRelationships{State: []string{"alpha/one:order"}, Touches: []string{}, Proofs: []string{}})) {
			t.Fatalf("%s relationships=%#v", selection, got)
		}
		alpha := find(res, "alpha/one")
		if got := ids(alpha.Direct); !reflect.DeepEqual(got, []string{"alpha/one:order"}) {
			t.Fatalf("%s direct=%v", selection, got)
		}
		if alpha.Counts != (contextAuthorityCounts{Invariants: 2, Rules: 2}) {
			t.Fatalf("%s counts=%#v", selection, alpha.Counts)
		}
	}
	mixed := q.ContextForOptions([]string{"internal/foo", "internal/foo/y.go"}, ContextOptions{Selection: SelectionExplicit})
	if got := ids(find(mixed, "alpha/one").Direct); len(got) != 0 {
		t.Fatalf("bare mixed promoted directory relationships: %v", got)
	}
	withRelationships := q.ContextForOptions([]string{"internal/foo", "internal/foo/y.go"}, ContextOptions{Selection: SelectionExplicit, Facets: []ContextFacet{"relationships", FacetEvidence, FacetReferences}})
	alpha := find(withRelationships, "alpha/one")
	if got := ids(alpha.Direct); !reflect.DeepEqual(got, []string{"alpha/one:order", "alpha/one:tested"}) {
		t.Fatalf("expanded direct=%v", got)
	}
	if len(alpha.Direct[0].Evidence) == 0 || !reflect.DeepEqual(ids(alpha.Referenced), []string{"core/g:everywhere"}) || len(alpha.Referenced[0].Evidence) != 0 || alpha.Referenced[0].Backing != "" || alpha.Referenced[0].Verify != "" {
		t.Fatalf("enriched direct=%#v referenced=%#v", alpha.Direct, alpha.Referenced)
	}
	globalDedup := q.ContextForOptions([]string{"internal/foo/y.go"}, ContextOptions{Selection: SelectionExplicit, Facets: []ContextFacet{FacetAllRules, FacetReferences}})
	referencedEverywhere := 0
	for _, impact := range globalDedup.Topics {
		for _, claim := range impact.Referenced {
			if claim.ID == "core/g:everywhere" {
				referencedEverywhere++
			}
		}
	}
	if referencedEverywhere != 1 {
		t.Fatalf("globally deduplicated referenced target count=%d: %#v", referencedEverywhere, globalDedup.Topics)
	}
	invariants := q.ContextForOptions([]string{"internal/foo/y.go"}, ContextOptions{Selection: SelectionExplicit, Facets: []ContextFacet{"invariants"}})
	if got := ids(find(invariants, "alpha/one").Invariants); !reflect.DeepEqual(got, []string{"alpha/one:stable", "alpha/one:tested"}) {
		t.Fatalf("invariants=%v", got)
	}
	rules := q.ContextForOptions([]string{"internal/foo/y.go"}, ContextOptions{Selection: SelectionExplicit, Facets: []ContextFacet{FacetAllRules}})
	if got := ids(find(rules, "alpha/one").Additional); !reflect.DeepEqual(got, []string{"alpha/one:extra", "alpha/one:order"}) {
		t.Fatalf("rules=%v", got)
	}
	for _, facet := range []ContextFacet{FacetEvidence, FacetReferences} {
		res := q.ContextForOptions([]string{"internal/foo/y.go"}, ContextOptions{Selection: SelectionExplicit, Facets: []ContextFacet{facet}})
		impact := find(res, "alpha/one")
		if len(impact.Direct)+len(impact.Invariants)+len(impact.Additional)+len(impact.Referenced) != 0 {
			t.Fatalf("%s revealed hidden claims: %#v", facet, impact)
		}
	}
}

func TestContextFacetProjectionAndClosestCategory(t *testing.T) {
	files := ctxFiles()
	files["internal/foo/a_test.go"] = "package foo\n// invariant: alpha/one:tested\n// invariant: alpha/one:tested\n"
	files["internal/foo/b_test.go"] = "package foo\n// invariant: alpha/one:tested\n"
	files["internal/foo/c_test.go"] = "package foo\n// invariant: alpha/one:tested\n"
	files[".awf/topics/parts/alpha/one/current-state.md"] = "Intro.\n\n## Claims\n\n### `rule: order`\nOrder prose.\nSummary: Order summary.\nOrigin: ADR-0001\nReferences: core/g:everywhere\n\n### `invariant: tested`\nTests protect output.\nOrigin: ADR-0001\nBacking: test\n\n### `invariant: stable`\nOutput is stable.\nOrigin: ADR-0001\nBacking: unbacked\nVerify: by hand.\n"
	p := ctxRepo(t, ctxConfig, files)
	state, err := p.ContextState(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	q := New(state)
	if got := claimStateForOperation("add", "alpha/one:order", "applied", state.Loaded.Topics, nil); got != "active-current" {
		t.Fatal(got)
	}
	facets, _ := ParseContextFacets([]string{"invariants", "all-rules", "evidence", "selectors", "references", "pending"}, false)
	res := q.ContextForOptions([]string{"internal/foo/x.go", "internal/foo/y_test.go"}, ContextOptions{Selection: SelectionExplicit, Facets: facets})
	var alpha topicImpact
	for _, impact := range res.Topics {
		if impact.ID == "alpha/one" {
			alpha = impact
		}
	}
	if len(alpha.Direct) != 2 || len(alpha.Invariants) != 1 || len(alpha.Additional) != 0 || alpha.Selectors == nil {
		t.Fatalf("alpha=%#v", alpha)
	}
	if alpha.Direct[0].Summary != "Order summary." || len(alpha.Direct[1].Evidence) == 0 {
		t.Fatalf("direct=%#v", alpha.Direct)
	}
	if len(alpha.Direct[0].Outgoing) != 1 || alpha.Direct[0].Outgoing[0] != "core/g:everywhere" {
		t.Fatalf("refs=%#v", alpha.Direct[0])
	}
	if len(alpha.Referenced) != 0 {
		t.Fatalf("globally visible claim repeated as referenced=%#v", alpha.Referenced)
	}
}
