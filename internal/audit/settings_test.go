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

func TestHistoricalSettingsUsesCompleteFixedScopeVocabulary(t *testing.T) {
	want := []string{"adr", "adr-system", "awf", "catalog", "cmd", "code-design", "config", "decisions", "hooks", "invariants", "manifest", "plans", "project", "render", "rendering", "spine", "tooling"}
	if got := historicalSettings().ScopeNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("historical scopes = %v, want %v", got, want)
	}
}

func TestResolvePreservesLiveProjectScopes(t *testing.T) {
	scopes := []config.ScopeSpec{{Name: "awf"}}
	if got := Resolve(scopes).AllowedScopes; !reflect.DeepEqual(got, scopes) {
		t.Errorf("AllowedScopes = %v, want %v", got, scopes)
	}
	if got := Resolve(nil).AllowedScopes; got != nil {
		t.Errorf("AllowedScopes = %v, want nil", got)
	}
}
