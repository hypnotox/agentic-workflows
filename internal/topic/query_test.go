package topic

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func loadedQueryFixture(t *testing.T) (Corpus, adr.Corpus) {
	t.Helper()
	root, _, adrs := corpusFixture(t)
	cfg, err := config.Parse(filepath.Join(root, ".awf"), []byte(`prefix: test
integrationBranch: main
domains: [alpha, beta]
currentState:
  sources:
    - globs: ["internal/**", "pkg/**"]
      marker: "//"
  testGlobs: ["internal/**/*_test.go"]
`))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	writeTopic(t, root, "alpha", "contracts", "title: Contracts\nsummary: Current contracts.\npaths: [\"internal/**\"]\n", `Intro.

## Claims

### `+"`rule: order`"+`
Deterministic order.
Origin: ADR-0001
Revised-by: ADR-0002
References: beta/global:shared

### `+"`invariant: stable`"+`
Stable output.
Origin: ADR-0001
Backing: unbacked
Verify: compare snapshots.
`)
	writeTopic(t, root, "beta", "global", "title: Global\nsummary: Global contracts.\napplies: global\n", rulePart("shared", "0001", "alpha/contracts:stable"))
	testsupport.WriteFile(t, filepath.Join(root, "internal/schedule.go"), "package schedule\n// touches-state: alpha/contracts:order - scheduler entry point\n")
	testsupport.WriteFile(t, filepath.Join(root, "internal/stable_test.go"), "package schedule\n// touches-state: alpha/contracts:stable - snapshot boundary\n")
	testsupport.WriteFile(t, filepath.Join(root, "pkg/global.go"), "package global\n// state: beta/global:shared\n")
	corpus, err := LoadCorpus(root, cfg, adrs)
	if err != nil {
		t.Fatal(err)
	}
	return corpus, adrs
}

func TestQueryDefaultTopicAndClaim(t *testing.T) {
	corpus, adrs := loadedQueryFixture(t)
	topicResult, err := Query(corpus, adrs, "alpha/contracts", QueryOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if topicResult.Kind != "topic" || topicResult.Title != "Contracts" || topicResult.Summary != "Current contracts." || len(topicResult.Claims) != 2 {
		t.Fatalf("topic result = %#v", topicResult)
	}
	if topicResult.Claims[0].Type != Rule || topicResult.Claims[0].Backing != ExplicitNoBacking || topicResult.Claims[1].Backing != Unbacked || topicResult.Claims[1].Verify != "compare snapshots." {
		t.Fatalf("claims = %#v", topicResult.Claims)
	}
	if topicResult.History != nil || topicResult.References != nil || topicResult.Coverage != nil {
		t.Fatalf("default leaked detail = %#v", topicResult)
	}
	claimResult, err := Query(corpus, adrs, "alpha/contracts:stable", QueryOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if claimResult.Kind != "claim" || claimResult.ID != "alpha/contracts:stable" || claimResult.Title != "" || len(claimResult.Claims) != 1 {
		t.Fatalf("claim result = %#v", claimResult)
	}
}

func TestQueryIndependentDetailsAndCombination(t *testing.T) {
	corpus, adrs := loadedQueryFixture(t)
	cases := []struct {
		name                          string
		opts                          QueryOptions
		history, references, coverage bool
	}{
		{"history", QueryOptions{History: true}, true, false, false},
		{"references", QueryOptions{References: true}, false, true, false},
		{"coverage", QueryOptions{Coverage: true}, false, false, true},
		{"combined", QueryOptions{History: true, References: true, Coverage: true}, true, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Query(corpus, adrs, "alpha/contracts", tc.opts, nil)
			if err != nil {
				t.Fatal(err)
			}
			if (got.History != nil) != tc.history || (got.References != nil) != tc.references || (got.Coverage != nil) != tc.coverage {
				t.Fatalf("detail union = %#v", got)
			}
		})
	}
	currentPaths := []string{"internal/schedule.go", "internal/stable_test.go", "pkg/global.go"}
	combined, _ := Query(corpus, adrs, "alpha/contracts", QueryOptions{History: true, References: true, Coverage: true}, currentPaths)
	if len(combined.History) != 2 || combined.History[0].Origin.Number != "0001" || len(combined.History[0].RevisedBy) != 1 || combined.History[0].RevisedBy[0].Number != "0002" {
		t.Fatalf("history = %#v", combined.History)
	}
	if got := combined.References[0].Outgoing; !reflect.DeepEqual(got, []string{"beta/global:shared"}) {
		t.Fatalf("outgoing = %v", got)
	}
	if got := combined.References[0].Incoming; len(got) != 0 {
		t.Fatalf("query traversed references: %v", got)
	}
	if a := combined.Coverage.Applicability; a.DeclaredGlobal || !reflect.DeepEqual(a.TopicPaths, []string{"internal/**"}) || len(a.ApplicablePaths) != 2 || len(a.OwnedPaths) != 2 || len(a.MarkerSites) != 2 {
		t.Fatalf("coverage = %#v", combined.Coverage)
	}
	claim, err := Query(corpus, adrs, "alpha/contracts:stable", QueryOptions{Coverage: true}, currentPaths)
	if err != nil {
		t.Fatal(err)
	}
	if a := claim.Coverage.Applicability; a.DeclaredGlobal || !reflect.DeepEqual(a.TopicPaths, []string{"internal/**"}) || len(a.MarkerSites) != 1 || a.MarkerSites[0].ClaimID != "alpha/contracts:stable" {
		t.Fatalf("claim coverage included sibling markers or lost topic scope = %#v", claim.Coverage)
	}
	global, err := Query(corpus, adrs, "beta/global", QueryOptions{Coverage: true, References: true}, currentPaths)
	if err != nil {
		t.Fatal(err)
	}
	if a := global.Coverage.Applicability; !a.DeclaredGlobal || len(a.TopicPaths) != 0 || a.MarkerSites[0].Path != "pkg/global.go" {
		t.Fatalf("global coverage = %#v", global.Coverage)
	}
	if got := global.References[0].Incoming; !reflect.DeepEqual(got, []string{"alpha/contracts:order"}) {
		t.Fatalf("sorted direct incoming = %v", got)
	}
}

func TestQueryHistoricalOnlyRemovedClaim(t *testing.T) {
	corpus, existing := loadedQueryFixture(t)
	claimID := "alpha/contracts:removed"
	adrs := mustCorpus(append(withoutRecord(existing.All(), "0003"),
		adr.ADR{Number: "0003", Title: "ADR-0003: Add removed claim", Status: "Implemented", Format: adr.CurrentStateV1,
			Operations: []adr.Operation{{Verb: adr.OpAdd, ID: claimID}}, History: []adr.StatusEntry{{Status: "Implemented"}}},
		adr.ADR{Number: "0004", Title: "ADR-0004: Remove old claim", Status: "Implemented", Format: adr.CurrentStateV1,
			Operations: []adr.Operation{{Verb: adr.OpRemove, ID: claimID}}, History: []adr.StatusEntry{{Status: "Implemented"}}},
	))

	if _, err := Query(corpus, adrs, claimID, QueryOptions{}, nil); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("default removed-claim query = %v", err)
	}
	got, err := Query(corpus, adrs, claimID, QueryOptions{History: true, References: true, Coverage: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "claim" || got.ID != claimID || !got.HistoricalOnly || got.Claims == nil || len(got.Claims) != 0 {
		t.Fatalf("historical-only identity = %#v", got)
	}
	if len(got.History) != 1 || got.History[0].Origin.Number != "0003" || got.History[0].RemovedBy == nil || got.History[0].RemovedBy.Number != "0004" || len(got.History[0].RevisedBy) != 0 {
		t.Fatalf("historical-only operations = %#v", got.History)
	}
	if got.References != nil || got.Coverage != nil || got.Title != "" || got.Summary != "" {
		t.Fatalf("historical-only query fabricated active detail = %#v", got)
	}
	if _, err := Query(corpus, adrs, "alpha/contracts:unknown", QueryOptions{History: true}, nil); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unknown historical query = %v", err)
	}
}

func TestQueryActiveOperationHistoryAndIncompleteFallback(t *testing.T) {
	corpus, existing := loadedQueryFixture(t)
	claimID := "alpha/contracts:stable"
	record := func(number string, verb adr.OpVerb) adr.ADR {
		return adr.ADR{Number: number, Title: "ADR-" + number + ": Operation " + number, Status: "Implemented", Format: adr.CurrentStateV1,
			Operations: []adr.Operation{{Verb: verb, ID: claimID}}, History: []adr.StatusEntry{{Status: "Implemented"}}}
	}
	operations := mustCorpus(append(withoutRecord(existing.All(), "0003"), record("0003", adr.OpAdd), record("0004", adr.OpUpdate)))
	got, err := Query(corpus, operations, claimID, QueryOptions{History: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.History) != 1 || got.History[0].Origin.Number != "0003" || len(got.History[0].RevisedBy) != 1 || got.History[0].RevisedBy[0].Number != "0004" {
		t.Fatalf("active operation history = %#v", got.History)
	}

	incremental := adr.ADR{Number: "0004", Title: "ADR-0004: Incremental update", Status: "Implementing", Format: adr.CurrentStateV2,
		Operations: []adr.Operation{{Verb: adr.OpUpdate, ID: claimID}, {Verb: adr.OpAdd, ID: "alpha/contracts:later"}},
		History: []adr.HistoryEvent{
			{Kind: adr.HistoryStatus, Status: "Proposed"},
			{Kind: adr.HistoryStatus, Status: "Implementing"},
			{Kind: adr.HistoryApplied, Operations: []adr.Operation{{Verb: adr.OpUpdate, ID: claimID}}},
		},
	}
	operations = mustCorpus(append(withoutRecord(existing.All(), "0003"), record("0003", adr.OpAdd), incremental))
	got, err = Query(corpus, operations, claimID, QueryOptions{History: true}, nil)
	if err != nil || len(got.History) != 1 || got.History[0].RevisedBy[0].Status != "Implementing" {
		t.Fatalf("immediate incremental operation history = %#v, err=%v", got.History, err)
	}

	incomplete := mustCorpus(append(append([]adr.ADR{}, existing.All()...), record("0004", adr.OpUpdate)))
	got, err = Query(corpus, incomplete, claimID, QueryOptions{History: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.History) != 1 || got.History[0].Origin.Number != "0001" {
		t.Fatalf("incomplete operations did not fall back to claim-based origin provenance: %#v", got.History)
	}
}

func TestQuerySelectorsMissingAndStableJSON(t *testing.T) {
	for _, selector := range []string{"", "alpha", "Alpha/topic", "alpha/topic:", "alpha/topic:bad:more"} {
		if _, _, err := ParseSelector(selector); err == nil {
			t.Errorf("ParseSelector(%q) accepted", selector)
		}
	}
	if _, err := Query(Corpus{}, adr.Corpus{}, "bad", QueryOptions{}, nil); err == nil || !strings.Contains(err.Error(), "invalid topic selector") {
		t.Fatalf("Query malformed selector = %v", err)
	}
	if topicID, claimID, err := ParseSelector("alpha/contracts"); err != nil || topicID != "alpha/contracts" || claimID != "" {
		t.Fatalf("topic selector = %q %q %v", topicID, claimID, err)
	}
	if topicID, claimID, err := ParseSelector("alpha/contracts:stable"); err != nil || topicID != "alpha/contracts" || claimID != "alpha/contracts:stable" {
		t.Fatalf("claim selector = %q %q %v", topicID, claimID, err)
	}
	corpus, adrs := loadedQueryFixture(t)
	for _, selector := range []string{"alpha/missing", "alpha/contracts:missing"} {
		if _, err := Query(corpus, adrs, selector, QueryOptions{History: true}, nil); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("Query(%q) = %v", selector, err)
		}
	}
	result, err := Query(corpus, adrs, "alpha/contracts", QueryOptions{History: true, References: true, Coverage: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	one, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	two, _ := json.Marshal(result)
	if !reflect.DeepEqual(one, two) || !strings.Contains(string(one), `"claimId"`) || !strings.Contains(string(one), `"backing":"none"`) || !strings.Contains(string(one), `"applicablePaths":[]`) || !strings.Contains(string(one), `"ownedPaths":[]`) || strings.Contains(string(one), `"Origin"`) {
		t.Fatalf("unstable or semantically incomplete JSON: %s / %s", one, two)
	}
}

// withoutRecord drops the fixture record a case replaces, so the corpus keeps
// one file per identity now that a duplicate is a hard error.
func withoutRecord(all []adr.ADR, number string) []adr.ADR {
	out := make([]adr.ADR, 0, len(all))
	for _, a := range all {
		if a.Number != number {
			out = append(out, a)
		}
	}
	return out
}

// A claim may name a pending record as its Origin, and the fallback provenance
// path must resolve it by identity: a number-keyed lookup resolves nothing for a
// slug and renders an empty ADR entry in its place.
func TestQueryResolvesAPendingSlugOrigin(t *testing.T) {
	root, cfg, _ := corpusFixture(t)
	writeTopic(t, root, "alpha", "contracts", "title: Contracts\nsummary: Current contracts.\npaths: [\"internal/**\"]\n",
		rulePart("order", "keep-the-corpus", ""))
	add := adr.Operation{Verb: adr.OpAdd, ID: "alpha/contracts:order"}
	pending := adr.ADR{Slug: "keep-the-corpus", Title: "ADR-keep-the-corpus: Keep the corpus", Status: "Implementing",
		Format: adr.CurrentStateV3, Operations: []adr.Operation{add, {Verb: adr.OpUpdate, ID: "alpha/contracts:order"}},
		History: []adr.HistoryEvent{
			{Kind: adr.HistoryStatus, Status: "Proposed"},
			{Kind: adr.HistoryStatus, Status: "Implementing"},
			{Kind: adr.HistoryApplied, Operations: []adr.Operation{add}},
		}}
	adrs := mustCorpus([]adr.ADR{pending})
	corpus, err := LoadCorpus(root, cfg, adrs)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Query(corpus, adrs, "alpha/contracts:order", QueryOptions{History: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.History) != 1 || got.History[0].Origin == nil ||
		got.History[0].Origin.Number != "keep-the-corpus" || got.History[0].Origin.Title != "Keep the corpus" ||
		got.History[0].Origin.Status != "Implementing" {
		t.Fatalf("pending origin provenance = %#v", got.History)
	}
}
