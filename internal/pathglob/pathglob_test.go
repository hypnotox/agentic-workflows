package pathglob

import "testing"

// The table is the spec for ADR-0077's anchored dialect: no basename mode,
// `**/` is the only any-depth form, slashed patterns anchor at the repo root.
func TestMatchAnchored(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"*.go", "a.go", true},
		{"*.go", "cmd/a.go", false}, // anchored: no basename fallback
		{"**/*.go", "a.go", true},   // `**/` matches zero directories too
		{"**/*.go", "cmd/x/a.go", true},
		{"cmd/**", "cmd/awf/main.go", true},
		{"cmd/**", "internal/audit/audit.go", false},
		{"internal/audit/*.go", "internal/audit/audit.go", true},
		{"internal/audit/*.go", "internal/audit/sub/x.go", false},
		{"go.mod", "go.mod", true},
		{"go.mod", "sub/go.mod", false},
		{"**/go.mod", "sub/go.mod", true},
	}
	for _, c := range cases {
		// invariant: config/validation:pathglob-anchored
		if got := Match(c.pattern, c.path); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestMatchMalformedPatternMatchesNothing(t *testing.T) {
	if Match("[", "a.go") {
		t.Error("malformed pattern must match nothing")
	}
}

func TestValidate(t *testing.T) {
	if err := Validate("**/*.go"); err != nil {
		t.Errorf("valid pattern rejected: %v", err)
	}
	if err := Validate("["); err == nil {
		t.Error("expected malformed pattern to be rejected")
	}
}
