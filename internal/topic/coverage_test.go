package topic

import (
	"fmt"
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
		{ID: TopicID{"core", "empty3"}, Metadata: Metadata{Paths: []string{"internal/lib/**"}}},
		{ID: TopicID{"core", "empty4"}, Metadata: Metadata{Paths: []string{"internal/lib/**"}}},
		{ID: TopicID{"core", "empty5"}, Metadata: Metadata{Paths: []string{"internal/lib/**"}}},
		{ID: TopicID{"core", "empty6"}, Metadata: Metadata{Paths: []string{"internal/lib/**"}}},
		{ID: TopicID{"core", "empty7"}, Metadata: Metadata{Paths: []string{"internal/lib/**"}}},
		{ID: TopicID{"core", "empty8"}, Metadata: Metadata{Paths: []string{"internal/lib/**"}}},
		{ID: TopicID{"core", "empty9"}, Metadata: Metadata{Paths: []string{"internal/lib/**"}}},
		{ID: TopicID{"core", "glob"}, Metadata: Metadata{Applies: "global"}},
		{ID: TopicID{"core", "global-owner"}, Metadata: Metadata{Applies: "global", Paths: []string{"internal/global/**"}}, Claims: []Claim{{ID: "core/global-owner:a"}}},
		{ID: TopicID{"overlap", "extra"}, Metadata: Metadata{Paths: []string{"internal/app/**"}}, Claims: []Claim{{ID: "overlap/extra:a"}}},
	}
	return c
}

// invariant: invariants/topics-and-markers:coverage-evaluation-selects-checks (TestEvaluateCoverage)
func TestEvaluateCoverage(t *testing.T) {
	c := coverageCorpus()
	paths := []string{"internal/app/y.go", "internal/lib/x.go", "bare/z.go", "shared/a.go", "README.md"}

	// internal/lib/x.go is both uncovered (only claimless topics) and over the
	// fixed fan-out budget, so its two findings exercise the kind tie-break in the sort.
	got := EvaluateCoverage(c, paths, CoveragePolicy{Coverage: true, Fanout: true})
	want := []CoverageFinding{
		{Path: "bare/z.go", Domain: "bare", Kind: Uncovered, Severity: severity.Error},
		{Path: "internal/lib/x.go", Kind: Fanout, Severity: severity.Warn, Topics: 9},
		{Path: "internal/lib/x.go", Domain: "core", Kind: Uncovered, Severity: severity.Error},
		{Path: "shared/a.go", Domain: "d1", Kind: Uncovered, Severity: severity.Error},
		{Path: "shared/a.go", Domain: "d2", Kind: Uncovered, Severity: severity.Error},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("full policy:\n got %#v\nwant %#v", got, want)
	}

	// Requesting fan-out only produces no Uncovered finding, even though four
	// paths are uncovered: an unrequested check does not run.
	got = EvaluateCoverage(c, paths, CoveragePolicy{Coverage: false, Fanout: true})
	want = []CoverageFinding{{Path: "internal/lib/x.go", Kind: Fanout, Severity: severity.Warn, Topics: 9}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fan-out only:\n got %#v\nwant %#v", got, want)
	}

	// Requesting coverage only produces no Fanout finding.
	got = EvaluateCoverage(c, paths, CoveragePolicy{Coverage: true, Fanout: false})
	want = []CoverageFinding{
		{Path: "bare/z.go", Domain: "bare", Kind: Uncovered, Severity: severity.Error},
		{Path: "internal/lib/x.go", Domain: "core", Kind: Uncovered, Severity: severity.Error},
		{Path: "shared/a.go", Domain: "d1", Kind: Uncovered, Severity: severity.Error},
		{Path: "shared/a.go", Domain: "d2", Kind: Uncovered, Severity: severity.Error},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("coverage only:\n got %#v\nwant %#v", got, want)
	}

	// Requesting neither check yields nothing at all.
	if got := EvaluateCoverage(c, paths, CoveragePolicy{Coverage: false, Fanout: false}); len(got) != 0 {
		t.Fatalf("neither check requested: %#v", got)
	}
}

// invariant: invariants/topics-and-markers:fan-out-budget-fixed (TestEvaluateCoverageFanoutBoundary)
func TestEvaluateCoverageFanoutBoundary(t *testing.T) {
	corpus := func(count int) Corpus {
		c := Corpus{DomainPaths: map[string][]string{"core": {"internal/**"}}}
		for i := range count {
			path := "internal/**"
			if i%2 == 0 {
				path = "internal/app/**"
			}
			c.all = append(c.all, Topic{
				ID:       TopicID{"core", fmt.Sprintf("topic-%d", i)},
				Metadata: Metadata{Paths: []string{path}},
				Claims:   []Claim{{ID: fmt.Sprintf("core/topic-%d:claim", i)}},
			})
		}
		return c
	}
	policy := CoveragePolicy{Fanout: true}
	if got := EvaluateCoverage(corpus(8), []string{"internal/app/x.go"}, policy); len(got) != 0 {
		t.Fatalf("eight matching topics exceeded the fixed budget: %#v", got)
	}
	got := EvaluateCoverage(corpus(9), []string{"internal/app/x.go"}, policy)
	want := []CoverageFinding{{Path: "internal/app/x.go", Kind: Fanout, Severity: severity.Warn, Topics: 9}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("nine matching topics:\n got %#v\nwant %#v", got, want)
	}
}

// invariant: tooling/context-and-topic:context-applicability-navigation (TestApplicabilityForTopic)
func TestApplicabilityForTopic(t *testing.T) {
	markers := MarkerIndex{sites: map[string][]MarkerSite{"d/t:c": {{Path: "z", Line: 2, ClaimID: "d/t:c"}, {Path: "a", Line: 1, ClaimID: "d/t:c"}}}}
	topic := Topic{ID: TopicID{"d", "t"}, Metadata: Metadata{Paths: []string{"internal/**"}}, Claims: []Claim{{ID: "d/t:c"}}}
	a := ApplicabilityForTopic(topic, []string{"internal/pkg/**"}, markers, []string{"other.go", "internal/pkg/a.go"})
	if !reflect.DeepEqual(a.DomainPaths, []string{"internal/pkg/**"}) || !reflect.DeepEqual(a.TopicPaths, []string{"internal/**"}) || !reflect.DeepEqual(a.ApplicablePaths, []string{"internal/pkg/a.go"}) || !reflect.DeepEqual(a.OwnedPaths, []string{"internal/pkg/a.go"}) || a.MarkerSites[0].Path != "a" {
		t.Fatalf("%#v", a)
	}
	topic.Metadata = Metadata{Applies: "global"}
	a = ApplicabilityForTopic(topic, []string{"internal/**"}, markers, []string{"internal/a.go", "other.go"})
	if !a.DeclaredGlobal || !reflect.DeepEqual(a.TopicPaths, []string{}) || !reflect.DeepEqual(a.ApplicablePaths, []string{"internal/a.go", "other.go"}) || len(a.OwnedPaths) != 0 {
		t.Fatalf("global %#v", a)
	}
}

// invariant: invariants/topics-and-markers:global-topic-path-ownership (TestGlobalTopicPathOwnership)
func TestGlobalTopicPathOwnership(t *testing.T) {
	metadata := []byte("title: Global owner\nsummary: Applies everywhere and owns a bounded path.\napplies: global\npaths: [\"internal/owned/**\"]\n")
	if _, _, err := ParseMetadata(".awf/topics/metadata", ".awf/topics/metadata/core/global-owner.yaml", metadata); err != nil {
		t.Fatalf("combined metadata: %v", err)
	}
	corpus := Corpus{DomainPaths: map[string][]string{"core": {"internal/**"}, "other": {"other/**"}}}
	owner := Topic{ID: TopicID{"core", "global-owner"}, Metadata: Metadata{Applies: "global", Paths: []string{"internal/owned/**"}}, Claims: []Claim{{ID: "core/global-owner:claim"}}}
	corpus.all = []Topic{owner}
	applicability := ApplicabilityForTopic(owner, corpus.DomainPaths["core"], MarkerIndex{sites: map[string][]MarkerSite{}}, []string{"internal/owned/a.go", "internal/elsewhere.go", "other/a.go"})
	if !reflect.DeepEqual(applicability.ApplicablePaths, []string{"internal/elsewhere.go", "internal/owned/a.go", "other/a.go"}) || !reflect.DeepEqual(applicability.OwnedPaths, []string{"internal/owned/a.go"}) {
		t.Fatalf("applicability = %#v", applicability)
	}
	if got := EvaluateCoverage(corpus, []string{"internal/owned/a.go", "other/a.go"}, CoveragePolicy{Coverage: true}); len(got) != 1 || got[0].Path != "other/a.go" || got[0].Domain != "other" {
		t.Fatalf("coverage = %#v", got)
	}
	claimless := owner
	claimless.ID = TopicID{"core", "claimless-owner"}
	claimless.Claims = nil
	corpus.all = append(corpus.all, claimless)
	for i := range maxTopicsPerPath - 1 {
		t := claimless
		t.ID.Slug = fmt.Sprintf("claimless-%d", i)
		corpus.all = append(corpus.all, t)
	}
	got := EvaluateCoverage(corpus, []string{"internal/owned/a.go", "internal/elsewhere.go"}, CoveragePolicy{Fanout: true})
	want := []CoverageFinding{{Path: "internal/owned/a.go", Kind: Fanout, Severity: severity.Warn, Topics: maxTopicsPerPath + 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("claimless global fan-out = %#v, want %#v", got, want)
	}
}
