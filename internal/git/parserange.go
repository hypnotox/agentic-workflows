package git

import (
	"errors"
	"fmt"
	"strings"
)

// ParseRange resolves a range argument to an explicit base and head revision.
// An argument containing ".." is a two-sided range; otherwise it is a base and
// head defaults to HEAD, which callers opt into via allowBareBase (ADR-0127
// Decision 5). Git forbids ".." inside a ref name, so the discrimination is
// unambiguous. Rejects an empty side, a three-dot range, a multi-".." input,
// and a "-"-prefixed side: the first three would reach git as a bogus revision
// and the last as an option-like argument. Dots inside a revision (v0.10.0) are
// legal, since git forbids "."-leading, ".."-containing, and "-"-leading refs.
func ParseRange(arg string, allowBareBase bool) (base, head string, err error) {
	if arg == "" {
		// errors.New, not fmt.Errorf: perfsprint rejects a constant-string
		// fmt.Errorf and the gate would fail.
		return "", "", errors.New("range must not be empty")
	}
	if !strings.Contains(arg, "..") {
		if !allowBareBase {
			return "", "", fmt.Errorf("range %q must be <a>..<b>", arg)
		}
		if strings.HasPrefix(arg, "-") {
			return "", "", fmt.Errorf("range %q must not start with a dash", arg)
		}
		return arg, "HEAD", nil
	}
	base, head, _ = strings.Cut(arg, "..")
	if base == "" || head == "" {
		return "", "", fmt.Errorf("range %q must be <a>..<b>", arg)
	}
	if strings.HasPrefix(head, ".") || strings.Contains(head, "..") {
		return "", "", fmt.Errorf("range %q must use exactly two dots", arg)
	}
	if strings.HasPrefix(base, "-") || strings.HasPrefix(head, "-") {
		return "", "", fmt.Errorf("range %q must not start with a dash", arg)
	}
	return base, head, nil
}
