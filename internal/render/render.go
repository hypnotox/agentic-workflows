// Package render parses awf section markers and renders templates with per-project overlays via text/template.
package render

import (
	"fmt"
	"strconv"
	"strings"
	"text/template"
)

// SectionPlan is the project layer's per-section resolution handed to Assemble.
// Exactly one of Drop / HasPart / (neither) holds: Drop omits the section,
// HasPart substitutes PartBody, neither renders the template default. EditPath is
// the project-relative convention part path named by the awf:edit pointer.
type SectionPlan struct {
	Drop     bool
	HasPart  bool
	PartBody string
	// PartStub marks a part body carrying the whole-line awf:stub marker —
	// declared-unauthored starter content (ADR-0070). Set by the project layer,
	// which reads part bodies; consumed by StubSections.
	PartStub bool
	// PartMarker marks a part whose raw, fence-excluded body carries a
	// whole-line section-marker residue (ADR-0083). Set by the project layer
	// over the on-disk bytes — pre placeholder substitution, whose multi-line
	// values must never create or mask a match; consumed part-keyed by the
	// marker advisory.
	PartMarker bool
	// PartVarRefs lists the config vars the raw part body consumes via
	// {{=awf:key}} placeholders (ADR-0086). Set by the project layer over
	// the on-disk bytes; consumed by the unused-var union, which cannot see
	// part bodies in the assembled source (they are sentinel-substituted raw).
	PartVarRefs []string
	EditPath    string
	// InPlace marks a section whose body the adopter edits directly in the
	// rendered output, preserved across syncs (ADR-0100). Mutually exclusive
	// with HasPart. InPlaceFound reports whether the section's region was located
	// in the existing output (its pointer was present); InPlaceBody is that
	// region's read-back content (possibly empty — an adopter may empty the
	// region). When InPlaceFound is false (first render / deleted pointer) the
	// template default renders instead; a found-but-empty region stays empty, so
	// emptying a region is not silently reverted to the default (ADR-0100 Decision 2).
	InPlace      bool
	InPlaceFound bool
	InPlaceBody  string
}

// CommentStyle is the comment syntax a rendered target uses for the surviving
// awf:edit-family provenance pointers. Because the pointers survive into output as
// comments, they must be valid comments in the target's language (ADR-0100
// Decision 7): a `#`-line comment for a `#!`-shebang target such as a shell script,
// an HTML comment otherwise. The zero value is HTMLComment, the historical default.
type CommentStyle int

const (
	HTMLComment CommentStyle = iota // <!-- <text> -->
	HashComment                     // # <text>
)

// CommentStyleForSource picks the pointer comment style for a target from its
// (expanded) template source, by the same `#!`-shebang sniff injectBanner uses so
// the pointer emitter and the read-back matcher derive the style identically and
// cannot diverge (ADR-0100 Decision 7).
func CommentStyleForSource(src string) CommentStyle {
	if strings.HasPrefix(src, "#!") {
		return HashComment
	}
	return HTMLComment
}

// wrap renders inner as a one-line comment in this style, trailing newline included.
func (style CommentStyle) wrap(inner string) string {
	if style == HashComment {
		return "# " + inner + "\n"
	}
	return "<!-- " + inner + " -->\n"
}

// open is the comment opener this style prefixes a pointer line with.
func (style CommentStyle) open() string {
	if style == HashComment {
		return "# "
	}
	return "<!-- "
}

// PointerLinePrefixes returns the awf:edit-family pointer line prefixes (the
// awf:edit and awf:edit-in-place variants) for a section named `name` in the given
// comment style, up to and including the ` — ` separator. Every editPointer variant
// emits `<open>awf:edit[-in-place] <name> — …`, so a trimmed output line is that
// section's pointer iff it begins with one of these prefixes. Read-back matches a
// region boundary by these exact per-section strings — never a generic
// pointer shape — so adopter text resembling a pointer for a non-registered name
// cannot bound a region (ADR-0100 Decision 2 / in-place-readback).
func PointerLinePrefixes(name string, style CommentStyle) []string {
	o := style.open()
	return []string{
		o + "awf:edit " + name + " — ",
		o + "awf:edit-in-place " + name + " — ",
	}
}

// editPointer is the awf:edit provenance comment emitted before a section body,
// in the target's CommentStyle (ADR-0100 Decision 7). A stub-attributed section
// rendering its template default gets a distinct pointer so the rendered file
// itself distinguishes a must-replace default from a valid one (ADR-0070). An
// in-place section gets a distinct `awf:edit-in-place` pointer whose token is
// deliberately not awf:section/awf:end-shaped, so it survives into the shipped
// output (unlike structural markers) and bounds the read-back region without
// tripping the residual-marker guards (ADR-0100). Only the comment delimiters
// vary by style; the token and phrasing are constant.
// touches-invariant: section-edit-pointer — awf:edit provenance pointer emission; proof in render_test.go
func editPointer(name string, stub bool, p SectionPlan, style CommentStyle) string {
	switch {
	case p.InPlace:
		return style.wrap(fmt.Sprintf("awf:edit-in-place %s — your edits below are preserved across syncs; awf owns the rest", name))
	case p.HasPart:
		return style.wrap(fmt.Sprintf("awf:edit %s — from %s", name, p.EditPath))
	case stub:
		return style.wrap(fmt.Sprintf("awf:edit %s — stub; replace by creating %s", name, p.EditPath))
	default:
		return style.wrap(fmt.Sprintf("awf:edit %s — default; create %s to override", name, p.EditPath))
	}
}

// partSentinel is the brace-free, NUL-delimited placeholder emitted in a part's
// slot. NUL bytes cannot occur in template or markdown text, so the token can
// never collide with rendered content, and being brace-free it is inert to the
// template parser.
func partSentinel(name string) string {
	return "\x00awf:part:" + name + "\x00"
}

// SectionDefaultSentinel is the brace-free, NUL-delimited token the project layer
// substitutes for the {{=awf:sectionDefault}} placeholder (ADR-0072). Assemble splits
// a part body at each occurrence and splices the section's raw default source between
// the verbatim fragments, so Execute renders the default in place. Brace-free (inert to
// the template parser) and NUL-delimited (cannot collide with template or markdown text).
const SectionDefaultSentinel = "\x00awf:section-default\x00"

// Assemble applies the per-section plan to the parsed segments and returns the
// template skeleton plus a sentinel→raw-body map. Literal segments pass through
// verbatim; each non-dropped section is prefixed with its awf:edit pointer, then
// either a sentinel standing in for its part body (restored after Execute) or the
// template default. Section markers are consumed here and never written.
// touches-invariant: no-section-marker-leak — section markers consumed, never written; proof in render_test.go
func Assemble(segs []Segment, plan map[string]SectionPlan, style CommentStyle) (string, map[string]string) {
	var b strings.Builder
	parts := map[string]string{}
	for _, s := range segs {
		if !s.IsSection {
			b.WriteString(s.Text)
			continue
		}
		p := plan[s.Name]
		if p.Drop {
			continue
		}
		b.WriteString(editPointer(s.Name, s.Stub, p, style))
		switch {
		case p.InPlace:
			// touches-invariant: in-place-pointer-distinct — distinct awf:edit-in-place pointer + verbatim interior; proof in render_test.go
			// A located region's read-back body is emitted verbatim after the
			// distinct awf:edit-in-place pointer (no re-templating), even when the
			// adopter emptied it; only an unlocated region (first render / deleted
			// pointer) falls back to the template default.
			if p.InPlaceFound {
				b.WriteString(p.InPlaceBody)
			} else {
				b.WriteString(s.Text)
			}
		case p.HasPart:
			writePartBody(&b, parts, s, p)
		default:
			b.WriteString(s.Text)
		}
	}
	return b.String(), parts
}

// writePartBody emits a section's part into the skeleton. When the part re-injects its
// section default via the sectionDefault split marker (ADR-0072), it is split at each
// marker into verbatim fragments — distinct sentinels restored after Execute —
// interleaved with the section's raw default source (s.Text), which Execute templates in
// place. A part without the marker emits a single sentinel for the whole body, the
// pre-ADR-0072 behaviour.
// invariant: section-default-splice
func writePartBody(b *strings.Builder, parts map[string]string, s Segment, p SectionPlan) {
	if !strings.Contains(p.PartBody, SectionDefaultSentinel) {
		sent := partSentinel(s.Name)
		parts[sent] = p.PartBody
		b.WriteString(sent)
		return
	}
	for i, frag := range strings.Split(p.PartBody, SectionDefaultSentinel) {
		if i > 0 {
			b.WriteString(s.Text)
		}
		// The index separator is a NUL, which can never occur in a section name
		// (template source is text): it guarantees a fragment sentinel can never
		// equal a plain part sentinel of some other section, whatever its name.
		sent := partSentinel(s.Name + "\x00" + strconv.Itoa(i))
		parts[sent] = frag
		b.WriteString(sent)
	}
}

// StubSections reports a parsed template's unauthored stub content under a plan
// (ADR-0070): defaults = stub-attributed sections rendering their template
// default; parts = sections whose convention part carries the awf:stub marker.
// Dropped sections report nothing.
func StubSections(segs []Segment, plan map[string]SectionPlan) (defaults, parts []string) {
	for _, s := range segs {
		if !s.IsSection {
			continue
		}
		p := plan[s.Name]
		switch {
		case p.Drop:
		case p.HasPart && p.PartStub:
			parts = append(parts, s.Name)
		case !p.HasPart && s.Stub:
			defaults = append(defaults, s.Name)
		}
	}
	return defaults, parts
}

// CheckSectionDefaultStubs hard-errors when a part re-injects the default of a
// stub-attributed section (ADR-0072 Decision 4): a stub default is an authoring prompt,
// not shippable prose, so there is nothing valid to re-inject and the section must stay
// in must-author state. Runs pre-Assemble on the same segs+plan StubSections consumes;
// it scans the substituted part body for the render-layer sentinel, since planSections
// has already replaced the {{=awf:sectionDefault}} token.
// invariant: section-default-stub-error
func CheckSectionDefaultStubs(segs []Segment, plan map[string]SectionPlan) error {
	for _, s := range segs {
		if !s.IsSection {
			continue
		}
		p := plan[s.Name]
		if s.Stub && p.HasPart && strings.Contains(p.PartBody, SectionDefaultSentinel) {
			return fmt.Errorf("section %q re-injects a stub default via {{=awf:sectionDefault}}: a stub default is an authoring prompt, not shippable prose — author the part instead", s.Name)
		}
	}
	return nil
}

// Execute runs text/template over the awf-owned skeleton (part bodies stood in by
// sentinels) under missingkey=zero, then restores each raw part body verbatim — so
// a convention part is never parsed or executed as a template. name labels parse
// and execute errors with the target rather than a hardcoded literal.
// touches-invariant: parts-raw — part bodies restored verbatim, never templated; proof in render_test.go
func Execute(assembled string, data map[string]any, parts map[string]string, name string) (string, error) {
	t, err := template.New(name).Option("missingkey=zero").Parse(assembled)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var out strings.Builder
	if err := t.Execute(&out, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	rendered := out.String()
	for sent, body := range parts {
		rendered = strings.ReplaceAll(rendered, sent, body)
	}
	return rendered, nil
}
