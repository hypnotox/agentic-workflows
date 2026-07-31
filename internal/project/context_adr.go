package project

import (
	"path"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

type ADROperationDetail struct {
	Current  *ContextClaimImpact
	History  *topic.ClaimHistory
	Evidence []ContextEvidence
}
type ADROperationContext struct {
	Operation, Claim, Topic, Progress string
	ClaimState                        string
	Detail                            *ADROperationDetail
}
type ADRArtifactContext struct {
	Number, Title, Status, Mutability, AuthorityRole string
	Operations                                       []ADROperationContext
}

func projectADRArtifact(filePath, decisionsDir string, adrs adr.Corpus, topics topic.Corpus, facets []ContextFacet) *ADRArtifactContext {
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
	if record.IsContentAmendable() {
		mutability = "mutable"
	}
	out := &ADRArtifactContext{Number: record.Number, Title: trimADRTitle(record.Number, record.Title), Status: record.Status, Mutability: mutability, AuthorityRole: "pending intent or decision history; not current authority", Operations: []ADROperationContext{}}
	if !slices.Contains(facets, FacetPending) {
		return out
	}
	progress, _, err := adrs.OperationProgress(record.Number)
	if err != nil {
		return out
	}
	states := map[adr.Operation]string{}
	if record.IsProposed() {
		for _, op := range record.Operations {
			states[op] = "proposed"
		}
	} else {
		for _, op := range progress.Remaining {
			states[op] = "remaining"
		}
		for _, op := range progress.Canceled {
			states[op] = "canceled"
		}
		for _, op := range progress.Applied {
			states[op.Operation] = "applied"
		}
	}
	for _, op := range record.Operations {
		s := states[op]
		var history *topic.ClaimHistory
		if query, e := topic.Query(topics, adrs, op.ID, topic.QueryOptions{History: true}, nil); e == nil && len(query.History) == 1 {
			copy := query.History[0]
			history = &copy
		}
		entry := ADROperationContext{Operation: string(op.Verb), Claim: op.ID, Topic: topicOfClaim(op.ID), Progress: s, ClaimState: claimStateForOperation(string(op.Verb), op.ID, s, topics, history)}
		if slices.Contains(facets, FacetEvidence) {
			detail := &ADROperationDetail{History: history, Evidence: []ContextEvidence{}}
			if claim, active := topics.ByClaimID(op.ID); active {
				impact := projectClaimImpact(claim, topics, facets)
				detail.Current = &impact
				detail.Evidence = impact.Evidence
			} else {
				detail.Evidence = contextEvidenceForClaim(op.ID, topics)
			}
			entry.Detail = detail
		}
		out.Operations = append(out.Operations, entry)
	}
	return out
}
