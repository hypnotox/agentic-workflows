package plan

import "time"

// SetNowForTest overrides the now seam for a test and returns the previous
// value, so the caller can restore it. It lives in an in-package _test.go file
// (package plan) so the external plan_test package can reach it without the seam
// shipping in the production binary (mirrors adr.SetNowForTest, ADR-0063).
func SetNowForTest(fn func() time.Time) (prev func() time.Time) {
	prev = now
	now = fn
	return prev
}
