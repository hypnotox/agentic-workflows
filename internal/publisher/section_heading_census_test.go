package publisher

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// TestLiveMarkdownSectionHeadingCensus follows the declaration-owned live
// identity and encoder population, expands includes, and excludes historical
// recognition-only identities by construction.
func TestLiveMarkdownSectionHeadingCensus(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	visited, sections := 0, 0
	for tid, encoder := range liveTemplateEncoders(renderInputsForTest(p)) {
		if encoder != MarkdownAgentDialect {
			continue
		}
		visited++
		source, err := fs.ReadFile(templates.FS, tid)
		if err != nil {
			t.Fatal(err)
		}
		expanded, err := render.ExpandIncludes(string(source), templates.FS)
		if err != nil {
			t.Fatal(err)
		}
		stripped, err := render.StripAuthoringComments(expanded)
		if err != nil {
			t.Fatal(err)
		}
		for _, site := range censusSites(tid, stripped) {
			sections++
			if err := site.classify(); err != nil {
				t.Errorf("%s section %q: %v", tid, site.name, err)
			}
		}
	}
	if visited == 0 || sections == 0 {
		t.Fatalf("declaration-derived census visited %d Markdown templates and %d sections", visited, sections)
	}
}

type headingCensusSite struct {
	tid          string
	name         string
	before, body []string
}

func censusSites(tid, source string) []headingCensusSite {
	lines := strings.Split(source, "\n")
	var sites []headingCensusSite
	for i, line := range lines {
		if !strings.HasPrefix(line, "<!-- awf:section ") {
			continue
		}
		name := strings.Fields(strings.TrimSuffix(strings.TrimPrefix(line, "<!-- awf:section "), " -->"))[0]
		end := i + 1
		for end < len(lines) && lines[end] != "<!-- awf:end -->" {
			end++
		}
		before := []string{}
		j := i - 1
		for j >= 0 && strings.TrimSpace(lines[j]) == "" {
			j--
		}
		if j >= 0 {
			before = append(before, lines[j])
		}
		sites = append(sites, headingCensusSite{tid: tid, name: name, before: before, body: lines[i+1 : end]})
	}
	return sites
}

func (s headingCensusSite) classify() error {
	if len(s.before) > 0 && isATX(s.before[0]) {
		level := 0
		for level < len(s.before[0]) && s.before[0][level] == '#' {
			level++
		}
		// A document-level H1 remains literal for docs, domains, and plans.
		// Catalog skill titles are section structure; all H2-H6 lines adjacent
		// through blank framing are section candidates.
		if level > 1 || (level == 1 && strings.HasPrefix(s.tid, "skills/")) {
			return fmt.Errorf("preceding structural heading %q is not normalized into its section", s.before[0])
		}
	}
	if len(s.body) == 0 {
		return nil
	}
	if isATX(s.body[0]) {
		if len(s.body) > 1 && isATX(s.body[1]) {
			return fmt.Errorf("multiple adjacent structural heading candidates %q and %q", s.body[0], s.body[1])
		}
		return nil
	}
	for _, line := range s.body {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if isATX(line) {
			return fmt.Errorf("intervening blank framing precedes heading %q", line)
		}
		break
	}
	return nil
}

func TestHeadingCensusRejectsAmbiguousStructuralCandidates(t *testing.T) {
	cases := map[string]headingCensusSite{
		"preceding h2":        {tid: "docs/x.md.tmpl", before: []string{"## Outside"}, body: []string{"body"}},
		"preceding skill h1":  {tid: "skills/x/SKILL.md.tmpl", before: []string{"# Skill"}, body: []string{"body"}},
		"intervening blank":   {tid: "docs/x.md.tmpl", body: []string{"", "## Inside", "body"}},
		"multiple candidates": {tid: "docs/x.md.tmpl", body: []string{"## One", "### Two", "body"}},
	}
	for name, site := range cases {
		t.Run(name, func(t *testing.T) {
			if err := site.classify(); err == nil {
				t.Fatal("ambiguous structural candidate was accepted")
			}
		})
	}
	for name, site := range map[string]headingCensusSite{
		"literal doc title": {tid: "docs/x.md.tmpl", before: []string{"# Document"}, body: []string{"body"}},
		"headingless":       {tid: "docs/x.md.tmpl", body: []string{"body", "## Body hierarchy"}},
		"headed":            {tid: "docs/x.md.tmpl", body: []string{"## Owned", "body"}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := site.classify(); err != nil {
				t.Fatalf("valid classification failed: %v", err)
			}
		})
	}
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
