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
