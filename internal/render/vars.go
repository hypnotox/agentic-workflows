package render

import (
	"regexp"
	"sort"
)

var varsRE = regexp.MustCompile(`\.vars\.([A-Za-z_][A-Za-z0-9_]*)`)
var dataLenRE = regexp.MustCompile(`len\s+\.data\.([A-Za-z_][A-Za-z0-9_]*)`)

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

// RequiredDataSlices returns the sorted, de-duplicated list of data field names
// that are passed to len (e.g. {{ len .data.adrStates }}) in src. These fields
// must be initialised to a non-nil slice — range over nil is fine, len of nil
// is not.
func RequiredDataSlices(src string) []string {
	matches := dataLenRE.FindAllStringSubmatch(src, -1)
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
