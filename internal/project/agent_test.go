package project

import (
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
)

func TestEncodeMarkdownAgent(t *testing.T) {
	t.Parallel()

	got, err := encodeMarkdownAgent(agent{
		Name:        "reviewer",
		Description: "Reviews changes.\nReturns findings.",
		Body:        "# reviewer\n\nReview carefully.\n",
	})
	if err != nil {
		t.Fatalf("encodeMarkdownAgent: %v", err)
	}
	var fm skillFrontmatter
	body, found, err := frontmatter.Parse([]byte(got), &fm)
	if err != nil {
		t.Fatalf("parse encoded frontmatter: %v", err)
	}
	if !found || fm.Name != "reviewer" || fm.Description != "Reviews changes. Returns findings.\n" {
		t.Fatalf("frontmatter = %#v, found %t", fm, found)
	}
	if string(body) != "\n# reviewer\n\nReview carefully.\n" {
		t.Fatalf("body = %q", body)
	}
}

func TestEncodeMarkdownAgentQuotesUnsafeSingleLineDescriptions(t *testing.T) {
	t.Parallel()

	for _, description := range []string{"Reviews \"changes\".", "- a list item"} {
		got, err := encodeMarkdownAgent(agent{Name: "reviewer", Description: description, Body: "# reviewer\n"})
		if err != nil {
			t.Fatalf("encodeMarkdownAgent: %v", err)
		}
		if !strings.Contains(got, "description: "+strconv.Quote(description)) {
			t.Fatalf("single-line description = %q", got)
		}
	}
}

func TestEncodeAgentRejectsInvalidMetadata(t *testing.T) {
	t.Parallel()

	for _, a := range []agent{
		{Description: "description"},
		{Name: "bad\nname", Description: "description"},
		{Name: "reviewer"},
	} {
		if _, err := encodeMarkdownAgent(a); err == nil {
			t.Fatalf("encodeMarkdownAgent(%#v) succeeded", a)
		}
	}
}

// invariant: rendering/catalog-and-targets:structured-agent-encoding (TestProjectRendersStandardAgentMetadataAndBody)
func TestProjectRendersStandardAgentMetadataAndBody(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nagents:\n  - code-reviewer\ntargets: [pi]\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	got := findByTID(files, "agents/code-reviewer.md.tmpl")
	if len(got) != 1 {
		t.Fatalf("standard agent files = %d, want 1", len(got))
	}
	for _, want := range []string{"name: code-reviewer", "Independent fresh-context reviewer for example", "# code-reviewer"} {
		if !strings.Contains(got[0].Content, want) {
			t.Errorf("missing %q in:\n%s", want, got[0].Content)
		}
	}
	if got[0].Encoder != MarkdownAgentDialect {
		t.Fatalf("agent encoder = %q", got[0].Encoder)
	}
	var plain *RenderedFile
	for i := range files {
		if files[i].Path == ".pi/extensions/awf-context-usage/index.ts" {
			plain = &files[i]
			break
		}
	}
	if plain == nil || plain.Encoder != PlainAgentDialect {
		t.Fatalf("Pi target-owned plain output = %#v", plain)
	}
}

func TestProjectEncodeAgentRejectsUnknownDialect(t *testing.T) {
	t.Parallel()

	p := &Project{Cat: &catalog.Catalog{Agents: map[string]catalog.AgentSpec{
		"reviewer": {Name: "reviewer", Description: "description"},
	}}}
	if _, err := p.encodeAgent(Target{AgentDialect: "unknown"}, "reviewer", "# reviewer\n", map[string]any{}); err == nil {
		t.Fatal("encodeAgent accepted an unknown dialect")
	}
}

func TestProjectEncodeMarkdownAgentRejectsInvalidDescriptionTemplate(t *testing.T) {
	t.Parallel()

	p := &Project{Cat: &catalog.Catalog{Agents: map[string]catalog.AgentSpec{
		"reviewer": {Name: "reviewer", Description: "{{"},
	}}}
	if _, err := p.encodeAgent(claudeTarget, "reviewer", "# reviewer\n", map[string]any{}); err == nil {
		t.Fatal("encodeMarkdownAgent accepted an invalid description template")
	}
}
