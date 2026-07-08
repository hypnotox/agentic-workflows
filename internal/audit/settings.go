package audit

import "github.com/hypnotox/agentic-workflows/internal/config"

// Settings is the resolved, default-applied audit configuration the rules consume.
type Settings struct {
	BaseBranch          string
	AllowedTypes        []string
	AllowedScopes       []config.ScopeSpec
	DependencyManifests []string
	SubjectMaxLength    int
	DiffThreshold       int
	DomainDocStaleness  bool
	UndocumentedDomain  bool
	UncommittedChanges  bool
}

// ScopeNames returns just the allowed scope names, for gate matching.
func (s Settings) ScopeNames() []string {
	names := make([]string, len(s.AllowedScopes))
	for i, sc := range s.AllowedScopes {
		names[i] = sc.Name
	}
	return names
}

// Resolve resolves the effective audit settings from the raw config, applying
// defaults. A nil AuditConfig yields the full default set.
func Resolve(a *config.AuditConfig) Settings {
	s := Settings{
		BaseBranch:          "main",
		AllowedTypes:        defaultAllowedTypes(),
		DependencyManifests: defaultDependencyManifests(),
		SubjectMaxLength:    72,
		DiffThreshold:       400,
		DomainDocStaleness:  true,
		UndocumentedDomain:  true,
		UncommittedChanges:  true,
	}
	if a == nil {
		return s
	}
	if a.BaseBranch != "" {
		s.BaseBranch = a.BaseBranch
	}
	if a.AllowedTypes != nil { // explicit (incl. empty = accept any)
		s.AllowedTypes = a.AllowedTypes
	}
	s.AllowedScopes = a.AllowedScopes // nil default = accept any
	if a.DependencyManifests != nil {
		s.DependencyManifests = a.DependencyManifests
	}
	if a.SubjectMaxLength != nil {
		s.SubjectMaxLength = *a.SubjectMaxLength
	}
	if a.DiffThreshold != nil {
		s.DiffThreshold = *a.DiffThreshold
	}
	if a.DomainDocStaleness != nil {
		s.DomainDocStaleness = *a.DomainDocStaleness
	}
	if a.UndocumentedDomain != nil {
		s.UndocumentedDomain = *a.UndocumentedDomain
	}
	if a.UncommittedChanges != nil {
		s.UncommittedChanges = *a.UncommittedChanges
	}
	return s
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
