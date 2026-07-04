package render

import (
	"regexp"
	"sort"
)

var varsRE = regexp.MustCompile(`\.vars\.([A-Za-z_][A-Za-z0-9_]*)`)

var skillsRE = regexp.MustCompile(`\{\{[^{}]*[.$]skills[^{}]*\}\}`)

// ReferencesSkills reports whether src reads the enabled-skills render context
// (any {{ … .skills… }} action) — such templates fold the effective skills set
// into their config hash (ADR-0046).
func ReferencesSkills(src string) bool { return skillsRE.MatchString(src) }

var scopesRE = regexp.MustCompile(`\{\{[^{}]*[.$]commitScopes[^{}]*\}\}`)

// ReferencesScopes reports whether src reads the resolved commit-scope render
// context (any {{ … .commitScopes … }} action) — such templates fold the
// resolved scope list into their config hash (ADR-0051, mirroring ADR-0046's
// ReferencesSkills).
func ReferencesScopes(src string) bool { return scopesRE.MatchString(src) }

var scopePlaceholderRE = regexp.MustCompile(`\{\{=awf:commitScope[A-Za-z0-9]*\}\}`)

// ReferencesScopePlaceholder reports whether a raw convention-part body uses a
// {{=awf:commitScope*}} sandbox placeholder (ADR-0057), so the artifact folds
// the resolved scope list into its config hash and reflags on a scopes edit.
func ReferencesScopePlaceholder(body string) bool { return scopePlaceholderRE.MatchString(body) }

// ReferencedVars returns the sorted, de-duplicated list of variable names
// referenced via {{ .vars.X }} patterns in src.
func ReferencedVars(src string) []string {
	matches := varsRE.FindAllStringSubmatch(src, -1)
	seen := map[string]bool{}
	for _, m := range matches {
		seen[m[1]] = true
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
