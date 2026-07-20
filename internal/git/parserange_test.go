package git

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestParseRangeTable pins the two accepted shapes and every rejection.
// invariant: tooling/audit-and-snapshots:git-range-rejects-malformed
func TestParseRangeTable(t *testing.T) {
	cases := []struct {
		arg          string
		bare         bool
		base, head   string
		wantErr      bool
		errSubstring string
	}{
		// Bare base: accepted only when the caller opts in.
		{arg: "HEAD", bare: true, base: "HEAD", head: "HEAD"},
		{arg: "v0.16.0", bare: true, base: "v0.16.0", head: "HEAD"},
		{arg: "HEAD", bare: false, wantErr: true, errSubstring: "must be <a>..<b>"},
		// Two-sided ranges parse the same either way.
		{arg: "a..b", bare: true, base: "a", head: "b"},
		{arg: "a..b", bare: false, base: "a", head: "b"},
		{arg: "v0.10.0..HEAD", bare: false, base: "v0.10.0", head: "HEAD"},
		// Empty input.
		{arg: "", bare: true, wantErr: true, errSubstring: "must not be empty"},
		{arg: "", bare: false, wantErr: true, errSubstring: "must not be empty"},
		// Three-dot and multi-"..": Cut would mangle both into a bogus rev.
		{arg: "a...b", bare: false, wantErr: true, errSubstring: "exactly two dots"},
		{arg: "a..b..c", bare: false, wantErr: true, errSubstring: "exactly two dots"},
		// Empty sides.
		{arg: "..b", bare: false, wantErr: true, errSubstring: "must be <a>..<b>"},
		{arg: "a..", bare: false, wantErr: true, errSubstring: "must be <a>..<b>"},
		// Dash-prefixed sides would reach git as option-like arguments.
		{arg: "-x", bare: true, wantErr: true, errSubstring: "must not start with a dash"},
		{arg: "-x", bare: false, wantErr: true, errSubstring: "must be <a>..<b>"},
		{arg: "-a..b", bare: false, wantErr: true, errSubstring: "must not start with a dash"},
		{arg: "a..-b", bare: false, wantErr: true, errSubstring: "must not start with a dash"},
	}
	for _, c := range cases {
		base, head, err := ParseRange(c.arg, c.bare)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseRange(%q, %v) = %q, %q, nil; want an error", c.arg, c.bare, base, head)
				continue
			}
			if !strings.Contains(err.Error(), c.errSubstring) {
				t.Errorf("ParseRange(%q, %v) error = %v; want it to contain %q", c.arg, c.bare, err, c.errSubstring)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRange(%q, %v) unexpected error: %v", c.arg, c.bare, err)
			continue
		}
		if base != c.base || head != c.head {
			t.Errorf("ParseRange(%q, %v) = %q, %q; want %q, %q", c.arg, c.bare, base, head, c.base, c.head)
		}
	}
}

// TestParseRangeIsTheOnlyRangeParser fails if a second range parser reappears
// anywhere in the module. internal/git is skipped because ParseRange itself
// splits on ".."; the repo-walk boundary (hidden trees, nested checkouts,
// test files) is owned by testsupport.WalkRepoSources.
// invariant: tooling/audit-and-snapshots:git-range-parser-single-definition
func TestParseRangeIsTheOnlyRangeParser(t *testing.T) {
	root := testsupport.RepoRoot(t)
	var offenders []string
	testsupport.WalkRepoSources(t, root, func(rel string, body []byte) {
		if strings.HasPrefix(rel, "internal/git/") || strings.HasPrefix(rel, "examples/") {
			return
		}
		for _, line := range strings.Split(string(body), "\n") {
			if strings.Contains(line, "strings.Cut(") && strings.Contains(line, `".."`) {
				offenders = append(offenders, rel)
				break
			}
		}
	})
	if len(offenders) > 0 {
		t.Errorf("range parsing must live only in internal/git.ParseRange; found a second parser in: %v", offenders)
	}
}
