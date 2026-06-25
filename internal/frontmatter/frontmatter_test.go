package frontmatter_test

import (
	"strings"
	"testing"

	"agentic-workflows/internal/frontmatter"
)

func TestSplitWellFormed(t *testing.T) {
	in := "---\nname: x\ndesc: y\n---\nbody here\n"
	yamlBlock, body, found := frontmatter.Split([]byte(in))
	if !found {
		t.Fatal("expected found")
	}
	if !strings.Contains(string(yamlBlock), "name: x") {
		t.Errorf("yaml block wrong: %q", yamlBlock)
	}
	if string(body) != "body here\n" {
		t.Errorf("body wrong: %q", body)
	}
}

func TestSplitNoFrontmatter(t *testing.T) {
	in := "# heading\nno frontmatter\n"
	yamlBlock, body, found := frontmatter.Split([]byte(in))
	if found {
		t.Error("expected not found")
	}
	if yamlBlock != nil {
		t.Errorf("yaml block should be nil, got %q", yamlBlock)
	}
	if string(body) != in {
		t.Errorf("body should equal input, got %q", body)
	}
}

func TestSplitMissingClosing(t *testing.T) {
	in := "---\nname: x\nbody never closes\n"
	_, body, found := frontmatter.Split([]byte(in))
	if found {
		t.Error("expected not found for missing closing delimiter")
	}
	if string(body) != in {
		t.Errorf("body should equal input, got %q", body)
	}
}

func TestSplitCRLF(t *testing.T) {
	in := "---\r\nname: x\r\n---\r\nbody\r\n"
	yamlBlock, _, found := frontmatter.Split([]byte(in))
	if !found {
		t.Fatal("expected found with CRLF")
	}
	if !strings.Contains(string(yamlBlock), "name: x") {
		t.Errorf("yaml block wrong: %q", yamlBlock)
	}
}

func TestParseIntoStruct(t *testing.T) {
	var fm struct {
		Name string `yaml:"name"`
	}
	in := "---\nname: hello\n---\nbody\n"
	body, found, err := frontmatter.Parse([]byte(in), &fm)
	if err != nil || !found {
		t.Fatalf("Parse: found=%v err=%v", found, err)
	}
	if fm.Name != "hello" {
		t.Errorf("Name = %q, want hello", fm.Name)
	}
	if string(body) != "body\n" {
		t.Errorf("body = %q", body)
	}
}

func TestParseMalformedYAML(t *testing.T) {
	var fm struct {
		Name string `yaml:"name"`
	}
	in := "---\nname: [unterminated\n---\nbody\n"
	if _, _, err := frontmatter.Parse([]byte(in), &fm); err == nil {
		t.Error("expected error for malformed YAML")
	}
}
