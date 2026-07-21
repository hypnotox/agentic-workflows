package topic

import (
	"reflect"
	"testing"
)

// coverageCorpus builds a corpus exercising every EvaluateCoverage branch:
//   - core owns internal/**; core/rules covers internal/app/**, core/empty and
//     core/empty2 are claimless internal/lib/** topics (uncovered yet counted
//     for fan-out), core/glob is global (never satisfies scoped coverage).
//   - overlap owns internal/app/** and covers it, so it shares that path.
//   - bare owns bare/** with no topics.
//   - d1 and d2 both own shared/** with no covering topic (two owners, one path).
func coverageCorpus() Corpus {
	c := Corpus{DomainPaths: map[string][]string{
		"core":    {"internal/**"},
		"overlap": {"internal/app/**"},
		"bare":    {"bare/**"},
		"d1":      {"shared/**"},
		"d2":      {"shared/**"},
	}}
	c.all = []Topic{
		{ID: TopicID{"core", "rules"}, Metadata: Metadata{Paths: []string{"internal/app/**"}}, Claims: []Claim{{ID: "core/rules:a"}}},
		{ID: TopicID{"core", "empty"}, Metadata: Metadata{Paths: []string{"internal/lib/**"}}},
		{ID: TopicID{"core", "empty2"}, Metadata: Metadata{Paths: []string{"internal/lib/**"}}},
		{ID: TopicID{"core", "glob"}, Metadata: Metadata{Applies: "global"}},
		{ID: TopicID{"overlap", "extra"}, Metadata: Metadata{Paths: []string{"internal/app/**"}}, Claims: []Claim{{ID: "overlap/extra:a"}}},
	}
	return c
}

func TestEvaluateCoverage(t *testing.T) {
	c := coverageCorpus()
	paths := []string{"internal/app/y.go", "internal/lib/x.go", "bare/z.go", "shared/a.go", "README.md"}

	// internal/lib/x.go is both uncovered (only claimless topics) and over the
	// fan-out budget, so its two findings exercise the kind tie-break in the sort.
	got := EvaluateCoverage(c, paths, CoveragePolicy{Coverage: CoverageError, Fanout: CoverageWarn, MaxTopicsPerPath: 1})
	want := []CoverageFinding{
		{Path: "bare/z.go", Domain: "bare", Kind: Uncovered, Severity: CoverageError},
		{Path: "internal/app/y.go", Kind: Fanout, Severity: CoverageWarn, Topics: 2},
		{Path: "internal/lib/x.go", Kind: Fanout, Severity: CoverageWarn, Topics: 2},
		{Path: "internal/lib/x.go", Domain: "core", Kind: Uncovered, Severity: CoverageError},
		{Path: "shared/a.go", Domain: "d1", Kind: Uncovered, Severity: CoverageError},
		{Path: "shared/a.go", Domain: "d2", Kind: Uncovered, Severity: CoverageError},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("full policy:\n got %#v\nwant %#v", got, want)
	}

	// Coverage off suppresses every Uncovered finding; fan-out survives.
	got = EvaluateCoverage(c, paths, CoveragePolicy{Coverage: CoverageOff, Fanout: CoverageWarn, MaxTopicsPerPath: 1})
	want = []CoverageFinding{
		{Path: "internal/app/y.go", Kind: Fanout, Severity: CoverageWarn, Topics: 2},
		{Path: "internal/lib/x.go", Kind: Fanout, Severity: CoverageWarn, Topics: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("coverage off:\n got %#v\nwant %#v", got, want)
	}

	// Fan-out off suppresses the fan-out finding; Uncovered survives.
	got = EvaluateCoverage(c, paths, CoveragePolicy{Coverage: CoverageError, Fanout: CoverageOff, MaxTopicsPerPath: 1})
	want = []CoverageFinding{
		{Path: "bare/z.go", Domain: "bare", Kind: Uncovered, Severity: CoverageError},
		{Path: "internal/lib/x.go", Domain: "core", Kind: Uncovered, Severity: CoverageError},
		{Path: "shared/a.go", Domain: "d1", Kind: Uncovered, Severity: CoverageError},
		{Path: "shared/a.go", Domain: "d2", Kind: Uncovered, Severity: CoverageError},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fan-out off:\n got %#v\nwant %#v", got, want)
	}

	// A generous budget leaves no fan-out finding; both off yields nothing.
	if got := EvaluateCoverage(c, []string{"internal/app/y.go"}, CoveragePolicy{Coverage: CoverageError, Fanout: CoverageWarn, MaxTopicsPerPath: 8}); len(got) != 0 {
		t.Fatalf("generous budget: %#v", got)
	}
	if got := EvaluateCoverage(c, paths, CoveragePolicy{Coverage: CoverageOff, Fanout: CoverageOff, MaxTopicsPerPath: 1}); len(got) != 0 {
		t.Fatalf("both off: %#v", got)
	}
}

func TestClaimBudgetNotes(t *testing.T) {
	claims := func(n int) []Claim {
		out := make([]Claim, n)
		for i := range out {
			out[i].ID = "x"
		}
		return out
	}
	c := Corpus{all: []Topic{
		{ID: TopicID{"zeta", "large"}, Claims: claims(3)},
		{ID: TopicID{"alpha", "equal"}, Claims: claims(2)},
		{ID: TopicID{"alpha", "large"}, Claims: claims(4)},
	}}
	got := ClaimBudgetNotes(c, 2)
	want := []string{
		"topic alpha/large has 4 claims, above maxClaimsPerTopic limit 2; consider splitting .awf/topics/metadata/alpha/large.yaml and .awf/topics/parts/alpha/large/current-state.md",
		"topic zeta/large has 3 claims, above maxClaimsPerTopic limit 2; consider splitting .awf/topics/metadata/zeta/large.yaml and .awf/topics/parts/zeta/large/current-state.md",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("notes = %#v, want %#v", got, want)
	}
	if got := ClaimBudgetNotes(c, 4); len(got) != 0 {
		t.Fatalf("equal or below threshold notes = %#v", got)
	}
}

// invariant: tooling/cli:context-applicability-navigation
func TestApplicabilityForTopic(t *testing.T) {
	markers := MarkerIndex{sites: map[string][]MarkerSite{"d/t:c": {{Path: "z", Line: 2, ClaimID: "d/t:c"}, {Path: "a", Line: 1, ClaimID: "d/t:c"}}}}
	topic := Topic{ID: TopicID{"d", "t"}, Metadata: Metadata{Paths: []string{"internal/**"}}, Claims: []Claim{{ID: "d/t:c"}}}
	a := ApplicabilityForTopic(topic, []string{"internal/pkg/**"}, markers, []string{"other.go", "internal/pkg/a.go"})
	if !reflect.DeepEqual(a.DomainPaths, []string{"internal/pkg/**"}) || !reflect.DeepEqual(a.TopicPaths, []string{"internal/**"}) || !reflect.DeepEqual(a.MatchedPaths, []string{"internal/pkg/a.go"}) || a.MarkerSites[0].Path != "a" {
		t.Fatalf("%#v", a)
	}
	topic.Metadata = Metadata{Applies: "global"}
	a = ApplicabilityForTopic(topic, []string{"internal/**"}, markers, []string{"internal/a.go", "other.go"})
	if !a.DeclaredGlobal || !reflect.DeepEqual(a.TopicPaths, []string{}) || !reflect.DeepEqual(a.MatchedPaths, []string{"internal/a.go"}) {
		t.Fatalf("global %#v", a)
	}
}
