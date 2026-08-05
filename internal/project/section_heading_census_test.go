package project

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// TestLiveMarkdownSectionHeadingCensus visits the include-independent embedded
// Markdown population. Every marker is either headed in its structural slot or
// deliberately headingless; a preceding heading would be an unnormalised site.
func TestLiveMarkdownSectionHeadingCensus(t *testing.T) {
	visited, sections := 0, 0
	err := fs.WalkDir(templates.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !markdownTemplate(path) {
			return nil
		}
		visited++
		source, err := templates.FS.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.Split(string(source), "\n")
		for i, line := range lines {
			if !strings.HasPrefix(line, "<!-- awf:section ") {
				continue
			}
			sections++
			if i > 0 && isATX(lines[i-1]) {
				t.Errorf("%s:%d has unnormalised preceding heading %q", path, i+1, lines[i-1])
			}
		}
		for _, segment := range render.ParseSections(string(source), true) {
			if segment.IsSection && segment.Heading != "" && !isATX(segment.Heading) {
				t.Errorf("%s section %s has non-ATX heading %q", path, segment.Name, segment.Heading)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if visited == 0 || sections == 0 {
		t.Fatalf("census must visit Markdown templates and section sites, got %d and %d", visited, sections)
	}
}

func markdownTemplate(path string) bool {
	return strings.HasSuffix(path, ".md.tmpl") || strings.HasSuffix(path, "/README.md.tmpl") || strings.HasSuffix(path, "/SKILL.md.tmpl") || strings.HasSuffix(path, "/AGENTS.md.tmpl") || strings.HasSuffix(path, "/template.md.tmpl")
}

func isATX(line string) bool {
	if len(line) < 2 || line[0] != '#' {
		return false
	}
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	return i <= 6 && i < len(line) && (line[i] == ' ' || line[i] == '\t')
}
