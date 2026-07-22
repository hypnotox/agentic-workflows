package project

import (
	"path"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func projectADRArtifact(filePath, decisionsDir string, adrs adr.Corpus, topics topic.Corpus, projection ContextProjection) *ADRArtifactContext {
	prefix := strings.TrimRight(decisionsDir, "/") + "/"
	if !strings.HasPrefix(filePath, prefix) {
		return nil
	}
	base := path.Base(filePath)
	match := adr.FilenameRe.FindStringSubmatch(base)
	if match == nil {
		return nil
	}
	record, ok := adrs.ByNumber(match[1])
	if !ok || record.Filename != base {
		return nil
	}
	mutability := "frozen"
	if record.IsProposed() {
		mutability = "mutable"
	}
	role := "decision history, never current authority"
	if record.IsProposed() {
		role = "pending intent, never current authority"
	}
	out := &ADRArtifactContext{
		Number: record.Number, Title: trimADRTitle(record.Number, record.Title), Status: record.Status,
		Mutability: mutability, AuthorityRole: role, Operations: []ADROperationContext{},
	}
	progress, _, err := adrs.OperationProgress(record.Number)
	if err != nil {
		return out
	}
	type progressEntry struct {
		name     string
		sequence int
	}
	states := map[adr.Operation]progressEntry{}
	if record.IsProposed() {
		for _, operation := range record.Operations {
			states[operation] = progressEntry{name: "proposed"}
		}
	} else {
		for _, operation := range progress.Remaining {
			states[operation] = progressEntry{name: "remaining"}
		}
		for _, operation := range progress.Canceled {
			states[operation] = progressEntry{name: "canceled"}
		}
		for _, operation := range progress.Applied {
			states[operation.Operation] = progressEntry{name: "applied", sequence: operation.Sequence}
		}
	}
	for _, operation := range record.Operations {
		state := states[operation]
		var history *topic.ClaimHistory
		if query, queryErr := topic.Query(topics, adrs, operation.ID, topic.QueryOptions{History: true}, nil); queryErr == nil && len(query.History) == 1 {
			copy := query.History[0]
			history = &copy
		}
		entry := ADROperationContext{
			Operation: string(operation.Verb), Claim: operation.ID, Topic: topicOfClaim(operation.ID),
			Progress: state.name, StateSequence: state.sequence,
			ClaimState: claimStateForOperation(string(operation.Verb), operation.ID, state.name, topics, history),
		}
		if projection == ContextFull {
			detail := &ADROperationDetail{History: history, MarkerSites: []topic.MarkerSite{}}
			if claim, active := topics.ByClaimID(operation.ID); active {
				current := contextClaimDetail(claim, topics, nil, false)
				detail.Current = &current
				detail.MarkerSites = append(detail.MarkerSites, current.Sites...)
			} else {
				detail.MarkerSites = nonNilMarkerSites(topics.Markers.ForClaim(operation.ID))
			}
			entry.Detail = detail
		}
		out.Operations = append(out.Operations, entry)
	}
	return out
}

func nonNilMarkerSites(in []topic.MarkerSite) []topic.MarkerSite {
	if in == nil {
		return []topic.MarkerSite{}
	}
	return append([]topic.MarkerSite(nil), in...)
}
