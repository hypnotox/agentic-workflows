package project

import (
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// projectInvocationTopic renders one applicable topic exactly once for the
// whole invocation. selectingPaths is the sorted set of effective paths the
// topic applies to; in the concise projection the marker-selected direct-claim
// union renders with full detail (sites filtered to selectingPaths), while the
// full projection leaves DirectClaims empty so each claim's detail renders
// exactly once, under Full (ADR-0147).
func projectInvocationTopic(t topic.Topic, corpus topic.Corpus, selectingPaths []string, currentPaths []string, pending []PendingChange, projection ContextProjection) InvocationTopicContext {
	a := topic.ApplicabilityForTopic(t, corpus.DomainPaths[t.ID.Domain], corpus.Markers, currentPaths)
	out := InvocationTopicContext{
		ID: t.ID.String(), Title: t.Metadata.Title, Summary: t.Metadata.Summary,
		Applicability: TopicApplicabilityBrief{
			DomainPaths: a.DomainPaths, TopicPaths: a.TopicPaths,
			DeclaredGlobal: a.DeclaredGlobal, MatchedPathCount: len(a.MatchedPaths),
		},
		ClaimIDs: []string{}, DirectClaims: []ClaimDetail{},
		TopicCommand: "awf topic " + t.ID.String(),
	}
	out.CoverageCommand = out.TopicCommand + " --coverage"
	for _, claim := range t.Claims {
		out.ClaimIDs = append(out.ClaimIDs, claim.ID)
	}
	slices.Sort(out.ClaimIDs)
	if projection == ContextFull {
		full := &FullTopicContext{Claims: []ClaimDetail{}, Pending: []PendingChange{}}
		for _, claim := range t.Claims {
			full.Claims = append(full.Claims, contextClaimDetail(claim, corpus, "", true))
		}
		slices.SortFunc(full.Claims, func(a, b ClaimDetail) int { return strings.Compare(a.ID, b.ID) })
		full.Pending = append(full.Pending, pending...)
		out.Full = full
		return out
	}
	selecting := map[string]bool{}
	for _, p := range selectingPaths {
		selecting[p] = true
	}
	for _, claim := range t.Claims {
		detail := ClaimDetail{
			ID: claim.ID, Type: string(claim.Type), Prose: claim.Prose, Backing: string(claim.Backing), Verify: claim.Verify,
			Sites: []topic.MarkerSite{}, References: ClaimReferences{Incoming: []string{}, Outgoing: []string{}},
		}
		for _, site := range corpus.Markers.ForClaim(claim.ID) {
			if selecting[site.Path] {
				detail.Sites = append(detail.Sites, site)
			}
		}
		if len(detail.Sites) > 0 {
			out.DirectClaims = append(out.DirectClaims, detail)
		}
	}
	slices.SortFunc(out.DirectClaims, func(a, b ClaimDetail) int { return strings.Compare(a.ID, b.ID) })
	out.OmittedDetailCount = len(out.ClaimIDs) - len(out.DirectClaims)
	return out
}

// pathTopicRef attributes t to one effective path with the path's own directly
// marker-selected claim IDs (ADR-0147).
func pathTopicRef(t topic.Topic, corpus topic.Corpus, path string) PathTopicRef {
	ref := PathTopicRef{ID: t.ID.String(), DirectClaimIDs: []string{}}
	for _, claim := range t.Claims {
		for _, site := range corpus.Markers.ForClaim(claim.ID) {
			if site.Path == path {
				ref.DirectClaimIDs = append(ref.DirectClaimIDs, claim.ID)
				break
			}
		}
	}
	slices.Sort(ref.DirectClaimIDs)
	return ref
}

func contextClaimDetail(claim topic.Claim, corpus topic.Corpus, exactPath string, includeReferences bool) ClaimDetail {
	detail := ClaimDetail{
		ID: claim.ID, Type: string(claim.Type), Prose: claim.Prose, Backing: string(claim.Backing), Verify: claim.Verify,
		Sites: []topic.MarkerSite{}, References: ClaimReferences{Incoming: []string{}, Outgoing: []string{}},
	}
	for _, site := range corpus.Markers.ForClaim(claim.ID) {
		if exactPath == "" || site.Path == exactPath {
			detail.Sites = append(detail.Sites, site)
		}
	}
	if includeReferences {
		detail.References.Incoming = nonNilStrings(corpus.Incoming(claim.ID))
		detail.References.Outgoing = nonNilStrings(corpus.Outgoing(claim.ID))
	}
	return detail
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	out := slices.Clone(in)
	slices.Sort(out)
	return slices.Compact(out)
}

func explicitContextPath(requests []ContextRequest, path string) bool {
	for _, request := range requests {
		if request.Status == RequestLiteral && request.Query == path && len(request.EffectivePaths) == 1 && request.EffectivePaths[0] == path {
			return true
		}
	}
	return false
}

func claimStateForOperation(operation string, claimID string, progress string, corpus topic.Corpus, history *topic.ClaimHistory) string {
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

func trimADRTitle(number, title string) string {
	return strings.TrimPrefix(title, "ADR-"+number+": ")
}
