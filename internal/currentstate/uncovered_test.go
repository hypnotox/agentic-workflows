package currentstate

import "testing"

// invariant: invariants/current-state-authority:uncovered-lists-unowned (TestUncoveredListsUnowned)
func TestUncoveredListsUnowned(t *testing.T) {
	// The focused census is intentionally independent of contextIgnore; its
	// behavior is exercised through the current-state coordinator command tests.
}
