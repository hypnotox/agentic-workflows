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
}

// editPointer is the awf:edit provenance comment emitted before a section body.
// A stub-attributed section rendering its template default gets a distinct
// pointer so the rendered file itself distinguishes a must-replace default from
// a valid one (ADR-0070).
// invariant: section-edit-pointer
func editPointer(name string, stub bool, p SectionPlan) string {
	if p.HasPart {
		return fmt.Sprintf("<!-- awf:edit %s — from %s -->\n", name, p.EditPath)
	}
	if stub {
		return fmt.Sprintf("<!-- awf:edit %s — stub; replace by creating %s -->\n", name, p.EditPath)
	}
	return fmt.Sprintf("<!-- awf:edit %s — default; create %s to override -->\n", name, p.EditPath)
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
// invariant: no-section-marker-leak
func Assemble(segs []Segment, plan map[string]SectionPlan) (string, map[string]string) {
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
		b.WriteString(editPointer(s.Name, s.Stub, p))
		if p.HasPart {
			writePartBody(&b, parts, s, p)
		} else {
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
// invariant: parts-raw
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
