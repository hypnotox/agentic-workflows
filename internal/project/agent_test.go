package project

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
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
		if _, err := encodeTOMLAgent(a); err == nil {
			t.Fatalf("encodeTOMLAgent(%#v) succeeded", a)
		}
	}
}

func TestValidateTOMLAgentRejectsInvalidProfiles(t *testing.T) {
	t.Parallel()

	for _, content := range []string{
		"name =\n",
		"name = \"reviewer\"\ndescription = \"description\"\nextra = \"nope\"\n",
		"name = \"\"\ndescription = \"description\"\ndeveloper_instructions = \"body\"\n",
	} {
		if err := validateTOMLAgent([]byte(content)); err == nil {
			t.Fatalf("validateTOMLAgent(%q) succeeded", content)
		}
	}
}

func TestProjectRendersStandardAgentMetadataAndBody(t *testing.T) {
	root := scaffold(t, "prefix: example\nagents:\n  - code-reviewer\n")
	p, err := Open(root)
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

func TestTOMLEncoderDoesNotDependOnMarkdownParser(t *testing.T) {
	t.Parallel()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate agent_test.go")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(file), "agent.go"))
	if err != nil {
		t.Fatalf("read agent.go: %v", err)
	}
	if strings.Contains(string(src), "frontmatter.") {
		t.Fatal("TOML encoder must not parse rendered Markdown frontmatter")
	}
}

// invariant: structured-agent-encoding
func TestEncodeTOMLAgentRoundTripsMultilineInstructions(t *testing.T) {
	t.Parallel()

	want := agent{
		Name:        "reviewer",
		Description: "Reviews \"quoted\" changes.",
		Body:        "# reviewer\n\nUse \"care\".\n",
	}
	got, err := encodeTOMLAgent(want)
	if err != nil {
		t.Fatalf("encodeTOMLAgent: %v", err)
	}
	var profile codexAgentProfile
	if _, err := toml.Decode(got, &profile); err != nil {
		t.Fatalf("decode TOML: %v", err)
	}
	if profile != (codexAgentProfile{Name: want.Name, Description: want.Description, DeveloperInstructions: want.Body}) {
		t.Fatalf("profile = %#v", profile)
	}
	if !strings.Contains(got, "developer_instructions") {
		t.Fatalf("TOML missing instructions: %q", got)
	}
}
