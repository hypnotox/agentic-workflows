package topic

import "slices"

type EffectiveSelector struct{ DomainPath, TopicPath string }
type TopicCoverage struct {
	DeclaredGlobal          bool
	DeclaredPaths           []string
	EffectiveSelectors      []EffectiveSelector
	HasClaims               bool
	SatisfiesScopedCoverage bool
	MarkerSites             []MarkerSite
}

func CoverageForTopic(t Topic, domainPaths []string, markers MarkerIndex) TopicCoverage {
	out := TopicCoverage{DeclaredGlobal: t.Metadata.Applies == "global", DeclaredPaths: slices.Clone(t.Metadata.Paths), HasClaims: len(t.Claims) > 0}
	if !out.DeclaredGlobal {
		for _, d := range domainPaths {
			for _, p := range t.Metadata.Paths {
				out.EffectiveSelectors = append(out.EffectiveSelectors, EffectiveSelector{DomainPath: d, TopicPath: p})
			}
		}
		out.SatisfiesScopedCoverage = out.HasClaims && len(out.EffectiveSelectors) > 0
	}
	claimIDs := map[string]bool{}
	for _, cl := range t.Claims {
		claimIDs[cl.ID] = true
	}
	for _, site := range markers.All() {
		if claimIDs[site.ClaimID] {
			out.MarkerSites = append(out.MarkerSites, site)
		}
	}
	return out
}
