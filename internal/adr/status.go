package adr

import "strings"

// Status literals live here and nowhere else (ADR-0130 item 3). Every consumer
// asks a predicate rather than comparing a string, which is what stops the
// three-way "is live" and five-way "is superseded" divergences from recurring.
const (
	statusAccepted    = "Accepted"
	statusImplemented = "Implemented"
	statusProposed    = "Proposed"
	statusSuperseded  = "Superseded"
	// statusAbandoned is the current-state-v1 terminal state for a decision
	// that stops before implementation (ADR-0135 item 1). It never appears in a
	// legacy-format ADR.
	statusAbandoned = "Abandoned"
)

// IsAbandoned reports the current-state-v1 terminal Abandoned state.
func (a ADR) IsAbandoned() bool { return a.Status == statusAbandoned }

// v1Statuses is the closed current-state-v1 status enum (ADR-0135 item 1);
// Superseded is deliberately absent.
var v1Statuses = map[string]bool{
	statusProposed:    true,
	statusAccepted:    true,
	statusImplemented: true,
	statusAbandoned:   true,
}

// v1Transitions maps each non-terminal status to the statuses it may become
// (ADR-0135 item 1). Implemented and Abandoned are terminal and absent as keys.
var v1Transitions = map[string]map[string]bool{
	statusProposed: {statusAccepted: true, statusImplemented: true, statusAbandoned: true},
	statusAccepted: {statusImplemented: true, statusAbandoned: true},
}

// v1StatusKnown reports whether s is a legal current-state-v1 status.
func v1StatusKnown(s string) bool { return v1Statuses[s] }

// v1TransitionLegal reports whether from -> to is one of the five legal
// current-state-v1 edges. A same-status pair and any edge out of a terminal
// state are illegal.
func v1TransitionLegal(from, to string) bool { return v1Transitions[from][to] }

// TransitionLegal reports whether from -> to is one of the five legal
// current-state-v1 status edges (ADR-0135 item 1). It is the exported seam the
// snapshot-diff transition check takes to validate an ADR status change observed
// across a before/after pair, without re-encoding the lifecycle matrix.
func TransitionLegal(from, to string) bool { return v1TransitionLegal(from, to) }

// IsLive reports whether the ADR's decisions are current guidance.
func (a ADR) IsLive() bool {
	return a.Status == statusAccepted || a.Status == statusImplemented
}

// IsSuperseded reports whether the ADR has been retired. The prefix test
// tolerates the pre-generation-12 suffixed form as well as the bare status
// ADR-0128 item 4 moves to.
func (a ADR) IsSuperseded() bool { return strings.HasPrefix(a.Status, statusSuperseded) }

// IsImplemented reports whether the ADR's decisions have shipped. Invariant
// backing and token retirement are both gated on this.
func (a ADR) IsImplemented() bool { return a.Status == statusImplemented }

// IsLegacyShipped reports whether a legacy decision shipped, including the
// historical Superseded state. Migration inventory uses this broader predicate;
// normal legacy authority continues to use its existing predicates.
func (a ADR) IsLegacyShipped() bool {
	return a.Status == statusImplemented || a.Status == statusSuperseded
}

// IsProposed reports whether the ADR's body is still mutable.
func (a ADR) IsProposed() bool { return a.Status == statusProposed }

// IsAccepted reports the current-state-v1 Accepted state: the decision is
// normative only for executing its pending State changes, which never override
// the topic claims describing current reality (ADR-0135).
func (a ADR) IsAccepted() bool { return a.Status == statusAccepted }

// ReachedAccepted reports whether the ADR's history entered Accepted, including
// a later transition to a terminal state.
func (a ADR) ReachedAccepted() bool {
	for _, entry := range a.History {
		if entry.Status == statusAccepted {
			return true
		}
	}
	return false
}

// IsInflight reports a legacy decision that must be resolved before bridge
// attestation.
func (a ADR) IsInflight() bool { return a.Status == statusProposed || a.Status == statusAccepted }

// HasSameStatus reports exact status equality without exporting literal
// comparisons to migration consumers.
func (a ADR) HasSameStatus(other ADR) bool { return a.Status == other.Status }

// HasStatus reports whether the record carries a frontmatter status at all.
// The audit distinguishes an ADR with no status from one with a real status,
// and that tri-state is what the bytes seam carries (ADR-0130 item 3).
func (a ADR) HasStatus() bool { return a.Status != "" }

// Bucket is the ACTIVE.md section an ADR belongs to. Every superseded ADR folds
// into one group regardless of the successor its status names.
func (a ADR) Bucket() string {
	if a.IsSuperseded() {
		return statusSuperseded
	}
	return a.Status
}
