package audit

import "github.com/hypnotox/agentic-workflows/internal/config"

// Settings is the fixed audit policy plus the repository's scope vocabulary.
type Settings struct {
	AllowedScopes []config.ScopeSpec
}

const (
	subjectMaxLength = 72
	diffThreshold    = 400
)

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

func defaultDependencyManifests() []string {
	return []string{
		"**/go.mod", "**/package.json", "**/pyproject.toml", "**/setup.py", "**/requirements*.txt",
		"**/Cargo.toml", "**/Gemfile", "**/*.gemspec", "**/composer.json", "**/pom.xml", "**/build.gradle",
		"**/build.gradle.kts", "**/*.csproj", "**/Directory.Packages.props", "**/mix.exs",
		"**/Package.swift", "**/pubspec.yaml", "**/*.cabal", "**/package.yaml",
	}
}
