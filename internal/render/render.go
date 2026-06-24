package render

import (
	"fmt"
	"strings"
	"text/template"

	"agentic-workflows/internal/config"
)

type PartFunc func(name string) (string, error)

// Assemble applies the overlay to the parsed segments and returns the final
// template source with markers stripped. Order per section: drop > replaceWith > default.
func Assemble(segs []Segment, ov map[string]config.SectionOverride, parts PartFunc) (string, error) {
	var b strings.Builder
	for _, s := range segs {
		if !s.IsSection {
			b.WriteString(s.Text)
			continue
		}
		o := ov[s.Name]
		switch {
		case o.Drop:
			// omit entirely
		case o.ReplaceWith != "":
			body, err := parts(o.ReplaceWith)
			if err != nil {
				return "", fmt.Errorf("section %q replaceWith %q: %w", s.Name, o.ReplaceWith, err)
			}
			b.WriteString(body)
		default:
			b.WriteString(s.Text)
		}
	}
	return b.String(), nil
}

// Render parses src, applies the overlay, then executes text/template over the
// assembled source with the given data.
func Render(src string, ov map[string]config.SectionOverride, parts PartFunc, data map[string]any) (string, error) {
	assembled, err := Assemble(ParseSections(src), ov, parts)
	if err != nil {
		return "", err
	}
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
