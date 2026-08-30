package audit

import "github.com/hypnotox/agentic-workflows/internal/config"

// Settings is the repository's allowed conventional-commit scope vocabulary.
type Settings struct {
	AllowedScopes []config.ScopeSpec
}

const subjectMaxLength = 72

// ScopeNames returns just the allowed scope names, for gate matching.
func (s Settings) ScopeNames() []string {
	names := make([]string, len(s.AllowedScopes))
	for i, sc := range s.AllowedScopes {
		names[i] = sc.Name
	}
	return names
}

// Resolve preserves the one repository-specific audit fact: allowed scopes.
func Resolve(scopes []config.ScopeSpec) Settings {
	return Settings{AllowedScopes: scopes}
}

func defaultAllowedTypes() []string {
	return []string{"build", "chore", "ci", "docs", "feat", "fix", "perf", "refactor", "revert", "style", "test"}
}
