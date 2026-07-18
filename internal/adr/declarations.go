package adr

import (
	"regexp"
	"strings"
)

var (
	// declRe matches an invariant DECLARATION leading a markdown list item
	// (optionally indented): a backed `invariant: <slug>` or an unbacked
	// `unbacked-invariant: <slug>` token. Group 1 is the optional `unbacked-`
	// prefix, group 2 the slug. Only backticks and spaces may sit between the
	// bullet and the token, so both the single-backtick form and the
	// double-backtick form ADR-0007 uses to render literal backticks are
	// recognised, while a mid-prose cross-reference to another ADR's slug is
	// not (it does not lead a list item) - which would otherwise
	// phantom-duplicate that slug.
	declRe = regexp.MustCompile("(?m)^[ \\t]*[-*][ \\t]+[`\\t ]*(unbacked-)?invariant:\\s*([a-z0-9-]+)")
	// itemStartRe matches a markdown list-item lead - the boundary a wrapped
	// bullet runs until (used to group a declaration bullet with its
	// continuation lines).
	itemStartRe = regexp.MustCompile(`^[ \t]*[-*][ \t]+`)
)

// InvariantDecl is one invariant declaration in an ADR's Invariants section.
// The grammar lives here rather than in internal/invariants because ADR-0130
// item 2 makes declared slugs a question the corpus view answers, and
// corpus-owns-field-reads forbids any other package reading ADR.Sections to
// re-derive it. Bullet carries the whole declaration - lead line plus wrapped
// continuation lines - so a consumer can scan it for the `Verify:` note
// without a second pass over the section.
type InvariantDecl struct {
	Slug     string
	Unbacked bool
	Bullet   string
}

// InvariantDecls returns the declarations a's Invariants section carries, in
// declaration order. Status-independent: the ref-validity check and the
// retirement migration resolve slug anchors against any ADR's declarations,
// not just Implemented ones (ADR-0120 item 2).
func (a ADR) InvariantDecls() []InvariantDecl {
	var decls []InvariantDecl
	for _, bullet := range invariantBullets(a.Sections["Invariants"]) {
		m := declRe.FindStringSubmatch(bullet)
		if m == nil {
			continue
		}
		decls = append(decls, InvariantDecl{Slug: m[2], Unbacked: m[1] == "unbacked-", Bullet: bullet})
	}
	return decls
}

// DeclaredSlugs returns the invariant slugs a's Invariants section declares,
// backed and unbacked alike, in declaration order.
func (a ADR) DeclaredSlugs() []string {
	decls := a.InvariantDecls()
	slugs := make([]string, 0, len(decls))
	for _, d := range decls {
		slugs = append(slugs, d.Slug)
	}
	return slugs
}

// invariantBullets splits an Invariants section into markdown list items, each
// joined with its wrapped continuation lines. A bullet starts at a list-item
// lead and runs until the next list-item lead, a blank line, or the section end
// - so a declaration's `Verify:` note is scanned over the whole bullet, not
// just its first physical line.
func invariantBullets(section string) []string {
	var bullets []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			bullets = append(bullets, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, line := range strings.Split(section, "\n") {
		switch {
		case strings.TrimSpace(line) == "":
			flush()
		case itemStartRe.MatchString(line):
			flush()
			cur = []string{line}
		case len(cur) > 0:
			cur = append(cur, line)
		}
	}
	flush()
	return bullets
}
