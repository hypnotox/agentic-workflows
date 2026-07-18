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
)

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

// IsProposed reports whether the ADR's body is still mutable.
func (a ADR) IsProposed() bool { return a.Status == statusProposed }

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
