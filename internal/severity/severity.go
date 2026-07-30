// Package severity holds the one finding rank awf reports. It owns no other
// concern: internal/audit and internal/topic both consume it and import each
// other in neither direction, so housing the rank in either would make an
// unrelated sibling depend on it purely to borrow a type (ADR-0183 item 4).
package severity

// Rank is how bad a produced finding is. There are exactly two, and Error is
// the zero value so an accidentally-defaulted finding is reported rather than
// silently downgraded. There is deliberately no suppressing value: a caller
// that does not want a finding class does not request it (ADR-0183 item 2).
type Rank int

const (
	// Error makes a consuming command exit nonzero.
	Error Rank = iota
	// Warn reports the finding without changing the exit code.
	Warn
)

// String renders the rank exactly as it is spelled everywhere awf reports it.
func (r Rank) String() string {
	if r == Warn {
		return "warn"
	}
	return "error"
}
