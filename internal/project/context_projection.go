package project

import (
	"slices"
	"strings"
	"unicode"

	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func projectTopicImpact(t topic.Topic, corpus topic.Corpus, directSources map[string][]ContextRelationshipSource, currentPaths []string, pending []PendingChange, facets []ContextFacet) TopicImpact {
	out := TopicImpact{ID: t.ID.String(), Title: t.Metadata.Title, Summary: t.Metadata.Summary, Direct: []ContextClaimImpact{}, Invariants: []ContextClaimImpact{}, Additional: []ContextClaimImpact{}, Referenced: []ContextClaimImpact{}, Pending: contextPending(pending, slices.Contains(facets, FacetPending))}
	for _, claim := range t.Claims {
		if claim.Type == topic.Invariant {
			out.Counts.Invariants++
		} else {
			out.Counts.Rules++
		}
	}
	visible := map[string]bool{}
	for _, claim := range t.Claims {
		category := ""
		switch {
		case len(directSources[claim.ID]) > 0:
			category = "direct"
		case claim.Type == topic.Invariant && slices.Contains(facets, FacetInvariants):
			category = "invariant"
		case claim.Type != topic.Invariant && slices.Contains(facets, FacetAllRules):
			category = "additional"
		}
		if category == "" {
			continue
		}
		impact := projectClaimImpact(claim, corpus, facets)
		if category == "direct" {
			impact.Sources = cloneContextRelationshipSources(directSources[claim.ID])
		}
		visible[claim.ID] = true
		switch category {
		case "direct":
			out.Direct = append(out.Direct, impact)
		case "invariant":
			out.Invariants = append(out.Invariants, impact)
		case "additional":
			out.Additional = append(out.Additional, impact)
		}
	}
	if slices.Contains(facets, FacetSelectors) {
		a := topic.ApplicabilityForTopic(t, corpus.DomainPaths[t.ID.Domain], corpus.Markers, currentPaths)
		out.Selectors = &ContextSelectorImpact{DomainPaths: nonNilStrings(a.DomainPaths), TopicPaths: nonNilStrings(a.TopicPaths), DeclaredGlobal: a.DeclaredGlobal}
	}
	if slices.Contains(facets, FacetReferences) {
		byID := map[string]topic.Claim{}
		for _, candidate := range corpus.All() {
			for _, claim := range candidate.Claims {
				byID[claim.ID] = claim
			}
		}
		refs := map[string]bool{}
		applyEdges := func(items []ContextClaimImpact) []ContextClaimImpact {
			for i := range items {
				items[i].Incoming = nonNilStrings(corpus.Incoming(items[i].ID))
				items[i].Outgoing = nonNilStrings(corpus.Outgoing(items[i].ID))
				for _, id := range append(slices.Clone(items[i].Incoming), items[i].Outgoing...) {
					if !visible[id] {
						refs[id] = true
					}
				}
			}
			return items
		}
		out.Direct = applyEdges(out.Direct)
		out.Invariants = applyEdges(out.Invariants)
		out.Additional = applyEdges(out.Additional)
		refIDs := make([]string, 0, len(refs))
		for id := range refs {
			refIDs = append(refIDs, id)
		}
		slices.Sort(refIDs)
		for _, id := range refIDs {
			if claim, ok := byID[id]; ok {
				out.Referenced = append(out.Referenced, projectClaimImpact(claim, corpus, facets))
			}
		}
	}
	sortClaims := func(items []ContextClaimImpact) {
		slices.SortFunc(items, func(a, b ContextClaimImpact) int { return strings.Compare(a.ID, b.ID) })
	}
	sortClaims(out.Direct)
	sortClaims(out.Invariants)
	sortClaims(out.Additional)
	sortClaims(out.Referenced)
	return out
}

func projectClaimImpact(claim topic.Claim, corpus topic.Corpus, facets []ContextFacet) ContextClaimImpact {
	out := ContextClaimImpact{ID: claim.ID, Type: string(claim.Type), Summary: claimSummary(claim), Sources: []ContextRelationshipSource{}, Evidence: []ContextEvidence{}, Incoming: []string{}, Outgoing: []string{}}
	if slices.Contains(facets, FacetEvidence) {
		out.Backing, out.Verify = string(claim.Backing), claim.Verify
		out.Evidence = contextEvidenceForClaim(claim.ID, corpus)
	}
	return out
}

func cloneContextRelationshipSources(in []ContextRelationshipSource) []ContextRelationshipSource {
	out := make([]ContextRelationshipSource, 0, len(in))
	for _, source := range in {
		out = append(out, ContextRelationshipSource{RequestIndex: source.RequestIndex, Kinds: slices.Clone(source.Kinds)})
	}
	return out
}

func contextEvidenceForClaim(claimID string, corpus topic.Corpus) []ContextEvidence {
	out := []ContextEvidence{}
	for _, kind := range []topic.MarkerKind{topic.StateMarker, topic.TouchesMarker, topic.ProofMarker} {
		sites := []topic.MarkerSite{}
		for _, site := range corpus.Markers.ForClaim(claimID) {
			if site.Kind == kind {
				sites = append(sites, site)
			}
		}
		if len(sites) == 0 {
			continue
		}
		slices.SortFunc(sites, func(a, b topic.MarkerSite) int {
			if a.Path != b.Path {
				return strings.Compare(a.Path, b.Path)
			}
			return a.Line - b.Line
		})
		e := ContextEvidence{Kind: string(kind), Count: len(sites), Sites: []topic.MarkerSite{}}
		if len(sites) <= 3 {
			e.Sites = sites
		}
		out = append(out, e)
	}
	return out
}

func claimSummary(claim topic.Claim) string {
	if claim.Summary != "" {
		return claim.Summary
	}
	paragraph := strings.Split(strings.ReplaceAll(claim.Prose, "\r\n", "\n"), "\n\n")[0]
	folded := strings.Join(strings.Fields(paragraph), " ")
	runes := []rune(folded)
	if len(runes) <= 160 {
		return folded
	}
	cut := 157
	for i := cut; i > 0; i-- {
		if unicode.IsSpace(runes[i-1]) {
			cut = i - 1
			break
		}
	}
	return string(runes[:cut]) + "..."
}

func contextPending(changes []PendingChange, expanded bool) ContextPendingImpact {
	out := ContextPendingImpact{OperationCount: len(changes), ADRs: []string{}, Operations: []PendingChange{}}
	seen := map[string]bool{}
	for _, change := range changes {
		if !seen[change.ADR] {
			seen[change.ADR] = true
			out.ADRs = append(out.ADRs, change.ADR)
		}
	}
	slices.Sort(out.ADRs)
	if len(out.ADRs) > 3 {
		out.AdditionalADRCount = len(out.ADRs) - 3
		out.ADRs = out.ADRs[:3]
	}
	if expanded {
		out.Operations = append(out.Operations, changes...)
	}
	return out
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	out := slices.Clone(in)
	slices.Sort(out)
	return slices.Compact(out)
}

func claimStateForOperation(operation, claimID, progress string, corpus topic.Corpus, history *topic.ClaimHistory) string {
	if _, ok := corpus.ByClaimID(claimID); ok {
		return "active-current"
	}
	if history != nil && history.RemovedBy != nil {
		return "historically-removed"
	}
	if operation == "remove" && progress == "applied" {
		return "historically-removed"
	}
	return "not-yet-current"
}
func trimADRTitle(number, title string) string { return strings.TrimPrefix(title, "ADR-"+number+": ") }
