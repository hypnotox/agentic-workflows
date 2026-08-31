package effort

import (
	"errors"
	"fmt"
)

// DiagnosticFact is one domain-owned observable fact in an effort refusal.
// Literal marks values whose whitespace must be preserved by presentation.
type DiagnosticFact struct {
	Label   string
	Value   string
	Literal bool
}

// DiagnosticInfo carries effort failure semantics without depending on a CLI
// presentation representation.
type DiagnosticInfo struct {
	Condition string
	State     string
	Cause     string
	Changed   []DiagnosticFact
	Actions   []RecoveryAction
}

// DiagnosticFor extracts modeled effort failure semantics while preserving the
// original error and mechanism identities for callers.
func DiagnosticFor(err error) (DiagnosticInfo, bool) {
	// Select the outer partial transition before inspecting its typed mechanism
	// cause; the transition's observed resident and durability facts are the
	// actionable outcome.
	var partial *PartialFinishError
	if errors.As(err, &partial) {
		facts := []DiagnosticFact{
			{Label: "active resident", Value: yesNo(partial.Result.State == FinishStateActive)},
			{Label: "finishing reservation", Value: yesNo(partial.Result.State == FinishStateReserved)},
			{Label: "archived resident", Value: yesNo(partial.Result.State == FinishStateArchived)},
			{Label: "archive parent sync available", Value: yesNo(partial.Result.DestinationSyncAvailable)},
			{Label: "archive parent synced", Value: yesNo(partial.Result.DestinationSynced)},
			{Label: "efforts parent sync available", Value: yesNo(partial.Result.SourceSyncAvailable)},
			{Label: "efforts parent synced", Value: yesNo(partial.Result.SourceSynced)},
		}
		if partial.Result.ArchivePath != "" {
			facts = append(facts, DiagnosticFact{Label: "archive", Value: partial.Result.ArchivePath, Literal: true})
		}
		return DiagnosticInfo{
			Condition: "effort finish was interrupted",
			State:     "operation",
			Cause:     partial.Cause.Error(),
			Changed:   facts,
			Actions:   append([]RecoveryAction(nil), partial.Actions...),
		}, true
	}
	var managed *managedTopologyError
	if errors.As(err, &managed) {
		return DiagnosticInfo{
			Condition: "managed topology remains",
			State:     "topology",
			Changed: []DiagnosticFact{
				{Label: "active resident", Value: "no"},
				{Label: "managed topology", Value: "no"},
			},
			Actions: append([]RecoveryAction(nil), managed.actions...),
		}, true
	}
	var refusal *refusalError
	if errors.As(err, &refusal) {
		return DiagnosticInfo{
			Condition: refusal.condition,
			State:     refusal.state,
			Cause:     refusal.cause,
			Changed:   []DiagnosticFact{{Label: "bytes", Value: "no"}},
			Actions:   append([]RecoveryAction(nil), refusal.actions...),
		}, true
	}
	var corrupt *CorruptError
	if errors.As(err, &corrupt) {
		return DiagnosticInfo{
			Condition: "effort resident is unusable",
			State:     "resident",
			Cause:     fmt.Sprintf("%s: %v", corrupt.Path, corrupt.Err),
			Changed:   []DiagnosticFact{{Label: "bytes", Value: "no"}},
			Actions:   []RecoveryAction{{Text: "preserve the resident and inspect it for manual cleanup"}},
		}, true
	}
	return DiagnosticInfo{}, false
}
