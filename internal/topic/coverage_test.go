package topic

import (
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/severity"
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

// invariant: invariants/topics-and-markers:coverage-evaluation-selects-checks (TestEvaluateCoverage)
func TestEvaluateCoverage(t *testing.T) {
	c := coverageCorpus()
	paths := []string{"internal/app/y.go", "internal/lib/x.go", "bare/z.go", "shared/a.go", "README.md"}

	// internal/lib/x.go is both uncovered (only claimless topics) and over the
	// fan-out budget, so its two findings exercise the kind tie-break in the sort.
	got := EvaluateCoverage(c, paths, CoveragePolicy{Coverage: true, Fanout: true, MaxTopicsPerPath: 1})
	want := []CoverageFinding{
		{Path: "bare/z.go", Domain: "bare", Kind: Uncovered, Severity: severity.Error},
		{Path: "internal/app/y.go", Kind: Fanout, Severity: severity.Warn, Topics: 2},
		{Path: "internal/lib/x.go", Kind: Fanout, Severity: severity.Warn, Topics: 2},
		{Path: "internal/lib/x.go", Domain: "core", Kind: Uncovered, Severity: severity.Error},
		{Path: "shared/a.go", Domain: "d1", Kind: Uncovered, Severity: severity.Error},
		{Path: "shared/a.go", Domain: "d2", Kind: Uncovered, Severity: severity.Error},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("full policy:\n got %#v\nwant %#v", got, want)
	}

	// Requesting fan-out only produces no Uncovered finding, even though four
	// paths are uncovered: an unrequested check does not run.
	got = EvaluateCoverage(c, paths, CoveragePolicy{Coverage: false, Fanout: true, MaxTopicsPerPath: 1})
	want = []CoverageFinding{
		{Path: "internal/app/y.go", Kind: Fanout, Severity: severity.Warn, Topics: 2},
		{Path: "internal/lib/x.go", Kind: Fanout, Severity: severity.Warn, Topics: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fan-out only:\n got %#v\nwant %#v", got, want)
	}

	// Requesting coverage only produces no Fanout finding, even though two paths
	// exceed the budget of 1.
	got = EvaluateCoverage(c, paths, CoveragePolicy{Coverage: true, Fanout: false, MaxTopicsPerPath: 1})
	want = []CoverageFinding{
		{Path: "bare/z.go", Domain: "bare", Kind: Uncovered, Severity: severity.Error},
		{Path: "internal/lib/x.go", Domain: "core", Kind: Uncovered, Severity: severity.Error},
		{Path: "shared/a.go", Domain: "d1", Kind: Uncovered, Severity: severity.Error},
		{Path: "shared/a.go", Domain: "d2", Kind: Uncovered, Severity: severity.Error},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("coverage only:\n got %#v\nwant %#v", got, want)
	}

	// A generous budget leaves no fan-out finding; requesting neither check
	// yields nothing at all.
	if got := EvaluateCoverage(c, []string{"internal/app/y.go"}, CoveragePolicy{Coverage: true, Fanout: true, MaxTopicsPerPath: 8}); len(got) != 0 {
		t.Fatalf("generous budget: %#v", got)
	}
	if got := EvaluateCoverage(c, paths, CoveragePolicy{Coverage: false, Fanout: false, MaxTopicsPerPath: 1}); len(got) != 0 {
		t.Fatalf("neither check requested: %#v", got)
	}
}

// invariant: tooling/context-and-topic:context-applicability-navigation (TestApplicabilityForTopic)
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
