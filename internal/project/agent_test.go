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
	p := &Project{cat: catalog.NewView(&catalog.Catalog{Agents: map[string]catalog.AgentSpec{
		"reviewer": {Name: "literal-reviewer", Description: "Rendered {{ .audience }} description."},
	}}).Catalog()}
	// The body deliberately begins with valid but conflicting frontmatter. A
	// structured encoder preserves it as instructions; an implementation that
	// reparses an intermediate Markdown artifact would substitute this decoy.
	body := "---\nname: body-decoy\ndescription: body decoy\n---\n\n# independently-rendered-body\n\nReview this body.\n"
	encoded, err := p.encodeAgent(piTarget, "reviewer", body, map[string]any{"audience": "target"})
	if err != nil {
		t.Fatal(err)
	}
	var fm skillFrontmatter
	parsedBody, found, err := frontmatter.Parse([]byte(encoded), &fm)
	if err != nil || !found {
		t.Fatalf("parse encoded agent: found=%t err=%v", found, err)
	}
	if fm.Name != "literal-reviewer" || fm.Description != "Rendered target description." || string(parsedBody) != "\n"+body {
		t.Fatalf("structured agent = name %q description %q body %q", fm.Name, fm.Description, parsedBody)
	}

	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	project, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := project.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	for i := range files {
		if files[i].Path == ".pi/extensions/awf-subagents/index.ts" {
			if files[i].Encoder != PlainAgentDialect {
				t.Fatalf("Pi target-owned encoder = %q", files[i].Encoder)
			}
			return
		}
	}
	t.Fatal("missing Pi target-owned plain output")
}

func TestProjectEncodeAgentRejectsUnknownDialect(t *testing.T) {
	t.Parallel()

	p := &Project{cat: catalog.NewView(&catalog.Catalog{Agents: map[string]catalog.AgentSpec{
		"reviewer": {Name: "reviewer", Description: "description"},
	}}).Catalog()}
	if _, err := p.encodeAgent(Target{AgentDialect: "unknown"}, "reviewer", "# reviewer\n", map[string]any{}); err == nil {
		t.Fatal("encodeAgent accepted an unknown dialect")
	}
}

func TestProjectEncodeMarkdownAgentRejectsInvalidDescriptionTemplate(t *testing.T) {
	t.Parallel()

	p := &Project{cat: catalog.NewView(&catalog.Catalog{Agents: map[string]catalog.AgentSpec{
		"reviewer": {Name: "reviewer", Description: "{{"},
	}}).Catalog()}
	if _, err := p.encodeAgent(claudeTarget, "reviewer", "# reviewer\n", map[string]any{}); err == nil {
		t.Fatal("encodeMarkdownAgent accepted an invalid description template")
	}
}
