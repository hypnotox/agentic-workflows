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

func TestCoverageForTopic(t *testing.T) {
	markers := MarkerIndex{sites: map[string][]MarkerSite{"d/t:c": {{Path: "z", Line: 2, ClaimID: "d/t:c"}, {Path: "a", Line: 1, ClaimID: "d/t:c"}}}}
	topic := Topic{ID: TopicID{"d", "t"}, Metadata: Metadata{Paths: []string{"internal/**"}}, Claims: []Claim{{ID: "d/t:c"}}}
	c := CoverageForTopic(topic, []string{"internal/pkg/**"}, markers)
	if !c.HasClaims || !c.SatisfiesScopedCoverage || len(c.EffectiveSelectors) != 1 || c.MarkerSites[0].Path != "a" {
		t.Fatalf("%#v", c)
	}
	topic.Claims = nil
	c = CoverageForTopic(topic, []string{"internal/**"}, markers)
	if c.SatisfiesScopedCoverage {
		t.Fatalf("empty %#v", c)
	}
	topic.Metadata = Metadata{Applies: "global"}
	c = CoverageForTopic(topic, nil, markers)
	if !c.DeclaredGlobal || c.SatisfiesScopedCoverage {
		t.Fatalf("global %#v", c)
	}
	topic.Metadata = Metadata{Paths: []string{"x/**"}}
	topic.Claims = []Claim{{ID: "d/t:c"}}
	c = CoverageForTopic(topic, nil, markers)
	if c.SatisfiesScopedCoverage {
		t.Fatalf("unowned %#v", c)
	}
}
