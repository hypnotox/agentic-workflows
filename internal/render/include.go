package render

import (
	"fmt"
	"io/fs"
	"regexp"
	"strings"
)

// includeRE matches an awf:include directive occupying its own line, capturing the
// partial name. The trailing `\n` is consumed so the splice preserves line structure:
// the directive line is replaced wholesale by the partial body (which ends in `\n`).
var includeRE = regexp.MustCompile(`(?m)^[ \t]*<!-- awf:include (\S+) -->[ \t]*\n`)

// ExpandIncludes replaces each `<!-- awf:include NAME -->` directive line in src with the
// verbatim body of the awf-owned partial `partials/NAME.md`, read from partialFS. Expansion
// runs before section parsing, so spliced content is thereafter indistinguishable from inline
// template text. Three conditions are hard errors: a missing partial, a partial that itself
// contains an awf:include (nested includes are unsupported), and a partial that contains an
// awf:section/awf:end marker (overlay across a splice boundary is unspecified).
// touches-invariant: include-splice — include-directive splice site; proof in include_test.go
// touches-invariant: include-missing-fails — missing-partial hard error; proof in include_test.go
// touches-invariant: include-no-nested — nested-include rejection; proof in include_test.go
// touches-invariant: include-no-sections — section-marker-in-partial rejection; proof in include_test.go
func ExpandIncludes(src string, partialFS fs.FS) (string, error) {
	locs := includeRE.FindAllStringSubmatchIndex(src, -1)
	if locs == nil {
		return src, nil
	}
	var b strings.Builder
	last := 0
	for _, m := range locs {
		name := src[m[2]:m[3]]
		body, err := fs.ReadFile(partialFS, "partials/"+name+".md")
		if err != nil {
			return "", fmt.Errorf("awf:include: unknown partial %q", name)
		}
		if strings.Contains(string(body), "awf:include") {
			return "", fmt.Errorf("awf:include: partial %q contains a nested include", name)
		}
		if strings.Contains(string(body), "awf:section") || strings.Contains(string(body), "awf:end") {
			return "", fmt.Errorf("awf:include: partial %q contains a section marker", name)
		}
		b.WriteString(src[last:m[0]])
		b.Write(body)
		last = m[1]
	}
	b.WriteString(src[last:])
	return b.String(), nil
}
