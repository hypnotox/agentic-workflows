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

var invariantMarkersRE = regexp.MustCompile(`\{\{[^{}]*[.$]invariantMarkers[^{}]*\}\}`)

// ReferencesInvariantMarkers reports whether src reads the .invariantMarkers
// render context, so the artifact folds invariants.sources into its config hash
// (ADR-0064, mirroring ReferencesScopes).
func ReferencesInvariantMarkers(src string) bool { return invariantMarkersRE.MatchString(src) }

var invariantMarkerPlaceholderRE = regexp.MustCompile(`\{\{=awf:invariantMarker[A-Za-z0-9]*\}\}`)

// ReferencesInvariantMarkerPlaceholder reports whether a raw convention-part body
// uses a {{=awf:invariantMarker*}} placeholder (ADR-0064, mirroring
// ReferencesScopePlaceholder).
func ReferencesInvariantMarkerPlaceholder(body string) bool {
	return invariantMarkerPlaceholderRE.MatchString(body)
}

// ReferencedVars returns the sorted, de-duplicated list of variable names
// referenced via {{ .vars.X }} patterns in src.
func ReferencedVars(src string) []string {
	return capturedNames(varsRE, src)
}

// capturedNames returns the sorted, de-duplicated first-group captures of re in src.
func capturedNames(re *regexp.Regexp, src string) []string {
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(src, -1) {
		seen[m[1]] = true
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

var dataRE = regexp.MustCompile(`\.data\.([A-Za-z_][A-Za-z0-9_]*)`)

// ReferencedDataKeys returns the sorted, de-duplicated list of top-level
// sidecar data keys referenced via {{ .data.K }} patterns in src (ADR-0086).
// Nested access (.data.a.b) claims its top-level key.
func ReferencedDataKeys(src string) []string {
	return capturedNames(dataRE, src)
}

var bareDataRE = regexp.MustCompile(`\.data(?:[^.A-Za-z0-9_]|$)`)

// ReferencesBareData reports whether src reads .data without a key selector
// (range/with/index or a whole-map reference). Key-level extraction cannot
// see through such access, so it conservatively marks every data key
// consumed (ADR-0086 Decision 4). No shipped template uses the form; this
// is the future-proofing escape.
func ReferencesBareData(src string) bool { return bareDataRE.MatchString(src) }

var bareVarsRE = regexp.MustCompile(`\.vars(?:[^.A-Za-z0-9_]|$)`)

// ReferencesBareVars mirrors ReferencesBareData for the vars namespace
// (ADR-0086 Decision 3).
func ReferencesBareVars(src string) bool { return bareVarsRE.MatchString(src) }

var varPlaceholderRefRE = regexp.MustCompile(`\{\{=awf:(gateCmd|checkCmd)\}\}`)

var escapedVarPlaceholderRE = regexp.MustCompile(`\\\{\{=awf:(?:gateCmd|checkCmd)\}\}`)

// PlaceholderVarRefs returns the config vars a raw convention-part body
// consumes through {{=awf:key}} placeholders — gateCmd and checkCmd are the
// only registry keys that read vars (see project.placeholderRegistry).
// Scanned on the on-disk bytes: substitution has already replaced the
// tokens in the assembled output, so this is the one consumption channel
// the assembled-source scan cannot see (ADR-0086 Decision 3). A
// backslash-escaped token (ADR-0058) renders literally and reads no var,
// so it is stripped before matching.
func PlaceholderVarRefs(body string) []string {
	return capturedNames(varPlaceholderRefRE, escapedVarPlaceholderRE.ReplaceAllString(body, ""))
}
