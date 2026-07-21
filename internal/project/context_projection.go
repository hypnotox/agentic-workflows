package project

import (
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func projectPathTopic(t topic.Topic, corpus topic.Corpus, path string, currentPaths []string, pending []PendingChange, projection ContextProjection) PathTopicContext {
	directIDs := map[string]bool{}
	for _, claim := range t.Claims {
		for _, site := range corpus.Markers.ForClaim(claim.ID) {
			if site.Path == path {
				directIDs[claim.ID] = true
			}
		}
	}
	out := PathTopicContext{
		ID: t.ID.String(), Title: t.Metadata.Title, Summary: t.Metadata.Summary,
		Applicability: topic.ApplicabilityForTopic(t, corpus.DomainPaths[t.ID.Domain], corpus.Markers, currentPaths),
		DirectClaims:  []ClaimDetail{}, TopicCommand: "awf topic " + t.ID.String(),
	}
	for _, claim := range t.Claims {
		if directIDs[claim.ID] {
			out.DirectClaims = append(out.DirectClaims, contextClaimDetail(claim, corpus, path, false))
		}
	}
	slices.SortFunc(out.DirectClaims, func(a, b ClaimDetail) int { return strings.Compare(a.ID, b.ID) })
	out.OmittedClaimCount = len(t.Claims) - len(out.DirectClaims)
	if projection == ContextFull {
		full := &FullTopicContext{Claims: []ClaimDetail{}, Pending: []PendingChange{}}
		for _, claim := range t.Claims {
			full.Claims = append(full.Claims, contextClaimDetail(claim, corpus, "", true))
		}
		slices.SortFunc(full.Claims, func(a, b ClaimDetail) int { return strings.Compare(a.ID, b.ID) })
		for _, change := range pending {
			if topicOfClaim(change.Claim) == t.ID.String() {
				full.Pending = append(full.Pending, change)
			}
		}
		out.Full = full
	}
	return out
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
