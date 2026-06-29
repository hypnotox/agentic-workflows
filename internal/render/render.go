// Package render parses awf section markers and renders templates with per-project overlays via text/template.
package render

import (
	"fmt"
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
	EditPath string
}

// editPointer is the awf:edit provenance comment emitted before a section body.
// invariant: section-edit-pointer
func editPointer(name string, p SectionPlan) string {
	if p.HasPart {
		return fmt.Sprintf("<!-- awf:edit %s — from %s -->\n", name, p.EditPath)
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
		b.WriteString(editPointer(s.Name, p))
		if p.HasPart {
			sent := partSentinel(s.Name)
			parts[sent] = p.PartBody
			b.WriteString(sent)
		} else {
			b.WriteString(s.Text)
		}
	}
	return b.String(), parts
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
