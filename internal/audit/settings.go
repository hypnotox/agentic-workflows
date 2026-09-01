package audit

import "github.com/hypnotox/agentic-workflows/internal/config"

// Settings is the repository's allowed conventional-commit scope vocabulary.
type Settings struct {
	AllowedScopes []config.ScopeSpec
}

const subjectMaxLength = 72

// historicalAllowedScopes is the complete conventional-commit scope vocabulary
// accepted when auditing committed history. It is intentionally independent of
// audit.allowedScopes, which remains live project policy for the commit gate
// and rendered project materials.
var historicalAllowedScopes = []config.ScopeSpec{
	{Name: "adr"},
	{Name: "adr-system"},
	{Name: "awf"},
	{Name: "catalog"},
	{Name: "cmd"},
	{Name: "code-design"},
	{Name: "config"},
	{Name: "decisions"},
	{Name: "hooks"},
	{Name: "invariants"},
	{Name: "manifest"},
	{Name: "plans"},
	{Name: "project"},
	{Name: "render"},
	{Name: "rendering"},
	{Name: "spine"},
	{Name: "tooling"},
}

// ScopeNames returns just the allowed scope names, for gate matching.
func (s Settings) ScopeNames() []string {
	names := make([]string, len(s.AllowedScopes))
	for i, sc := range s.AllowedScopes {
		names[i] = sc.Name
	}
	return names
}

// Resolve preserves the project-specific scope policy used by the live commit
// gate and rendered project materials.
func Resolve(scopes []config.ScopeSpec) Settings {
	return Settings{AllowedScopes: scopes}
}

func historicalSettings() Settings {
	scopes := make([]config.ScopeSpec, len(historicalAllowedScopes))
	copy(scopes, historicalAllowedScopes)
	return Settings{AllowedScopes: scopes}
}

func defaultAllowedTypes() []string {
	return []string{"build", "chore", "ci", "docs", "feat", "fix", "perf", "refactor", "revert", "style", "test"}
}
