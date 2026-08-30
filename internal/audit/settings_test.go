package audit

import (
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// invariant: tooling/audit-and-snapshots:audit-thresholds-fixed (TestFixedAuditPolicy)
func TestFixedAuditPolicy(t *testing.T) {
	if got, want := subjectMaxLength, 72; got != want {
		t.Errorf("subjectMaxLength = %d, want %d", got, want)
	}
	if got, want := defaultAllowedTypes(), []string{"build", "chore", "ci", "docs", "feat", "fix", "perf", "refactor", "revert", "style", "test"}; !reflect.DeepEqual(got, want) {
		t.Errorf("allowed types = %v, want %v", got, want)
	}
}

func TestResolvePreservesOnlyScopes(t *testing.T) {
	scopes := []config.ScopeSpec{{Name: "awf"}}
	if got := Resolve(scopes).AllowedScopes; !reflect.DeepEqual(got, scopes) {
		t.Errorf("AllowedScopes = %v, want %v", got, scopes)
	}
	if got := Resolve(nil).AllowedScopes; got != nil {
		t.Errorf("AllowedScopes = %v, want nil", got)
	}
}
