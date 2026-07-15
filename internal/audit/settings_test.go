package audit

import (
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

func intPtr(i int) *int    { return &i }
func boolPtr(b bool) *bool { return &b }

func TestResolveDefaultsWhenNil(t *testing.T) {
	s := Resolve(nil)
	if !s.DomainDocStaleness || !s.DomainCodeStaleness || !s.UndocumentedDomain || !s.UncommittedChanges || !s.PlainPunctuation {
		t.Errorf("toggles default to on: domStale=%v codeStale=%v undoc=%v uncommitted=%v plain=%v", s.DomainDocStaleness, s.DomainCodeStaleness, s.UndocumentedDomain, s.UncommittedChanges, s.PlainPunctuation)
	}
	if s.BaseBranch != "main" {
		t.Errorf("baseBranch = %q, want main", s.BaseBranch)
	}
	if !slices.Contains(s.AllowedTypes, "feat") {
		t.Errorf("allowedTypes default missing feat: %v", s.AllowedTypes)
	}
	if s.AllowedScopes != nil {
		t.Errorf("allowedScopes default = %v, want nil (accept any)", s.AllowedScopes)
	}
	if !slices.Contains(s.DependencyManifests, "**/go.mod") {
		t.Errorf("dependencyManifests default missing go.mod: %v", s.DependencyManifests)
	}
	if s.SubjectMaxLength != 72 || s.DiffThreshold != 400 {
		t.Errorf("max=%d thr=%d, want 72/400", s.SubjectMaxLength, s.DiffThreshold)
	}
}

func TestResolveZeroAuditFallsBackToDefaults(t *testing.T) {
	s := Resolve(&config.AuditConfig{})
	if !s.DomainDocStaleness || !s.DomainCodeStaleness || !s.UndocumentedDomain || !s.PlainPunctuation {
		t.Errorf("empty AuditConfig should keep toggles on: %v %v %v %v", s.DomainDocStaleness, s.DomainCodeStaleness, s.UndocumentedDomain, s.PlainPunctuation)
	}
	if s.BaseBranch != "main" || !slices.Contains(s.AllowedTypes, "feat") || s.AllowedScopes != nil ||
		!slices.Contains(s.DependencyManifests, "**/go.mod") || s.SubjectMaxLength != 72 || s.DiffThreshold != 400 {
		t.Errorf("empty AuditConfig did not fall back to defaults: base=%q types=%v scopes=%v max=%d thr=%d",
			s.BaseBranch, s.AllowedTypes, s.AllowedScopes, s.SubjectMaxLength, s.DiffThreshold)
	}
}

func TestResolveExplicitOverrides(t *testing.T) {
	s := Resolve(&config.AuditConfig{
		BaseBranch:          "develop",
		AllowedTypes:        []string{}, // explicit empty = accept any
		AllowedScopes:       []config.ScopeSpec{{Name: "awf"}},
		SubjectMaxLength:    intPtr(0),
		DependencyManifests: []string{}, // explicit empty = disabled
		DiffThreshold:       intPtr(0),
		DomainDocStaleness:  boolPtr(false),
		DomainCodeStaleness: boolPtr(false),
		UndocumentedDomain:  boolPtr(false),
		PlainPunctuation:    boolPtr(false),
		UncommittedChanges:  boolPtr(false),
	})
	if s.DomainDocStaleness || s.DomainCodeStaleness || s.UndocumentedDomain || s.UncommittedChanges || s.PlainPunctuation {
		t.Errorf("explicit false toggles not honored: domStale=%v codeStale=%v undoc=%v uncommitted=%v plain=%v", s.DomainDocStaleness, s.DomainCodeStaleness, s.UndocumentedDomain, s.UncommittedChanges, s.PlainPunctuation)
	}
	if s.BaseBranch != "develop" {
		t.Errorf("baseBranch = %q, want develop", s.BaseBranch)
	}
	if len(s.AllowedTypes) != 0 {
		t.Errorf("allowedTypes = %v, want empty (accept any)", s.AllowedTypes)
	}
	if len(s.AllowedScopes) != 1 || s.AllowedScopes[0].Name != "awf" {
		t.Errorf("allowedScopes = %v, want [awf]", s.AllowedScopes)
	}
	if len(s.DependencyManifests) != 0 {
		t.Errorf("dependencyManifests = %v, want empty (disabled)", s.DependencyManifests)
	}
	if s.SubjectMaxLength != 0 || s.DiffThreshold != 0 {
		t.Errorf("max=%d thr=%d, want 0/0", s.SubjectMaxLength, s.DiffThreshold)
	}
}
