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

// Assemble applies the per-section plan to the parsed segments and returns the
// final template source: literal segments verbatim; each non-dropped section
// prefixed with its awf:edit pointer, then its part body or the template default.
// Section markers are consumed here and never written, so they cannot leak.
// invariant: no-section-marker-leak
func Assemble(segs []Segment, plan map[string]SectionPlan) string {
	var b strings.Builder
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
			b.WriteString(p.PartBody)
		} else {
			b.WriteString(s.Text)
		}
	}
	return b.String()
}

// Execute runs text/template over an already-assembled source with the given
// data under missingkey=zero.
func Execute(assembled string, data map[string]any) (string, error) {
	t, err := template.New("skill").Option("missingkey=zero").Parse(assembled)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var out strings.Builder
	if err := t.Execute(&out, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return out.String(), nil
}

// Render parses src, applies the plan, then executes text/template over the
// assembled source with the given data.
func Render(src string, plan map[string]SectionPlan, data map[string]any) (string, error) {
	return Execute(Assemble(ParseSections(src), plan), data)
}
