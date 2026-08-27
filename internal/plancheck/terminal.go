package plancheck

import (
	"bytes"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/plan"
)

// TerminalTransition validates selected before/after plan evidence. An
// Implemented plan is byte-frozen history. A Proposed-to-Implemented
// transition must reconcile the complete selected implementation path set and
// material route deviations in the plan-owned parsed Notes model.
func TerminalTransition(before, after []plan.Plan, selected map[string][]string) error {
	old := make(map[string]plan.Plan, len(before))
	for _, p := range before {
		old[p.Filename] = p
	}
	for _, next := range after {
		prior, exists := old[next.Filename]
		if !exists {
			continue
		}
		if prior.IsImplemented() && !bytes.Equal(prior.Source, next.Source) {
			return fmt.Errorf("%s: Implemented plan is frozen history", next.Path)
		}
		if !prior.IsProposed() || !next.IsImplemented() {
			continue
		}
		reconciliation := next.TerminalReconciliation
		if reconciliation == nil {
			return fmt.Errorf("%s: terminal transition lacks parsed reconciliation", next.Path)
		}
		changed, ok := selected[next.Filename]
		if !ok || len(changed) == 0 {
			return fmt.Errorf("%s: terminal transition has unavailable touched-path evidence", next.Path)
		}
		if !samePathSet(changed, reconciliation.TouchedPaths) {
			return fmt.Errorf("%s: terminal reconciliation does not equal selected touched paths", next.Path)
		}
	}
	return nil
}

func samePathSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	values := make(map[string]bool, len(a))
	for _, value := range a {
		values[value] = true
	}
	if len(values) != len(a) {
		return false
	}
	for _, value := range b {
		if !values[value] {
			return false
		}
	}
	return true
}
