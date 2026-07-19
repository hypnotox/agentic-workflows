package topic

import "testing"

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
