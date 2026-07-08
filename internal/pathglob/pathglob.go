// Package pathglob is awf's single glob dialect (ADR-0077): anchored full-path
// doublestar matching against slash-separated repo-relative paths. There is
// deliberately no basename mode — `*.go` matches only top-level .go files;
// any-depth is written `**/*.go`. Leaf package: imports nothing from awf.
package pathglob

import (
	"fmt"

	"github.com/bmatcuk/doublestar/v4"
)

// Validate rejects a malformed doublestar pattern.
func Validate(pattern string) error {
	if !doublestar.ValidatePattern(pattern) {
		return fmt.Errorf("glob %q is malformed", pattern)
	}
	return nil
}

// Match reports whether the slash-separated repo-relative path matches the
// anchored pattern. A malformed pattern matches nothing — Validate at config
// load / audit-input building keeps that branch cold in practice.
// invariant: pathglob-anchored
func Match(pattern, relPath string) bool {
	ok, err := doublestar.Match(pattern, relPath)
	return err == nil && ok
}
