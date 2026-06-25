package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "awf.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadParsesAllFields(t *testing.T) {
	p := writeTemp(t, `prefix: example
vars:
  testCmd: go test ./...
skills:
  tdd:
    data:
      testSurfaces:
        - {name: Logic, location: internal, kind: unit}
    sections:
      notes: {replaceWith: parts/tdd-notes.md}
  bugfix: {}
  adding-thing: {local: true}
agents:
  code-reviewer: {}
hooks: [pre-commit]
`)
	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Prefix != "example" {
		t.Errorf("prefix = %q", c.Prefix)
	}
	if c.Vars["testCmd"] != "go test ./..." {
		t.Errorf("vars.testCmd = %v", c.Vars["testCmd"])
	}
	if !c.Skills["adding-thing"].Local {
		t.Errorf("adding-thing should be local")
	}
	if c.Skills["tdd"].Sections["notes"].ReplaceWith != "parts/tdd-notes.md" {
		t.Errorf("notes replaceWith = %q", c.Skills["tdd"].Sections["notes"].ReplaceWith)
	}
	if len(c.Raw()) == 0 {
		t.Errorf("Raw() should retain original bytes")
	}
}

func TestLoadParsesAgentMap(t *testing.T) {
	p := writeTemp(t, `prefix: example
agents:
  code-reviewer:
    data:
      foo: bar
`)
	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	ac, ok := c.Agents["code-reviewer"]
	if !ok {
		t.Fatal("code-reviewer not in Agents map")
	}
	if ac.Data["foo"] != "bar" {
		t.Errorf("Agents[code-reviewer].Data[foo] = %v, want bar", ac.Data["foo"])
	}
}

func TestValidateRejectsEmptyPrefix(t *testing.T) {
	p := writeTemp(t, "prefix: \"\"\nskills: {}\n")
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err == nil {
		t.Errorf("expected error for empty prefix")
	}
}

func TestValidateRejectsPathInPrefix(t *testing.T) {
	cases := []string{"../evil", "foo/bar", "a\\b"}
	for _, prefix := range cases {
		p := writeTemp(t, "prefix: "+prefix+"\nskills: {}\n")
		c, err := Load(p)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if err := c.Validate(); err == nil {
			t.Errorf("expected error for prefix %q containing path separator", prefix)
		}
	}
}

func TestValidateRejectsDropAndReplace(t *testing.T) {
	p := writeTemp(t, `prefix: example
skills:
  tdd:
    sections:
      notes: {drop: true, replaceWith: parts/x.md}
`)
	c, _ := Load(p)
	if err := c.Validate(); err == nil {
		t.Errorf("expected error: section cannot both drop and replaceWith")
	}
}

func TestValidateRejectsDropAndReplaceOnAgentAndDoc(t *testing.T) {
	agentCfg := writeTemp(t, `prefix: example
agents:
  code-reviewer:
    sections:
      doc-currency: {drop: true, replaceWith: parts/x.md}
`)
	if c, _ := Load(agentCfg); c.Validate() == nil {
		t.Errorf("expected error for agent section drop+replaceWith")
	}
	docCfg := writeTemp(t, `prefix: example
agentsDoc:
  sections:
    overview: {drop: true, replaceWith: parts/x.md}
`)
	if c, _ := Load(docCfg); c.Validate() == nil {
		t.Errorf("expected error for agentsDoc section drop+replaceWith")
	}
}

func TestLoadRejectsUnknownTopLevelKey(t *testing.T) {
	p := writeTemp(t, `prefix: example
skils: {}
`)
	_, err := Load(p)
	if err == nil {
		t.Errorf("expected error for unknown top-level key 'skils'")
	}
}

func TestLoadRejectsUnknownSkillKey(t *testing.T) {
	p := writeTemp(t, `prefix: example
skills:
  tdd:
    dat: {}
`)
	_, err := Load(p)
	if err == nil {
		t.Errorf("expected error for unknown skill key 'dat' (typo of 'data')")
	}
}

func TestLoadParsesDocsMap(t *testing.T) {
	p := writeTemp(t, `prefix: example
docs:
  architecture:
    sections:
      body: {replaceWith: parts/doc-architecture.md}
`)
	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Docs["architecture"].Sections["body"].ReplaceWith != "parts/doc-architecture.md" {
		t.Errorf("docs.architecture.body replaceWith = %q", c.Docs["architecture"].Sections["body"].ReplaceWith)
	}
}

func TestValidateRejectsDropAndReplaceOnDoc(t *testing.T) {
	p := writeTemp(t, `prefix: example
docs:
  architecture:
    sections:
      body: {drop: true, replaceWith: parts/x.md}
`)
	c, _ := Load(p)
	if err := c.Validate(); err == nil {
		t.Errorf("expected error: doc section cannot both drop and replaceWith")
	}
}

func TestLoadValidConfigSucceeds(t *testing.T) {
	p := writeTemp(t, `prefix: example
vars:
  testCmd: go test ./...
skills:
  tdd:
    data:
      testSurfaces:
        - {name: Logic, location: internal, kind: unit}
    sections:
      notes: {replaceWith: parts/tdd-notes.md}
  bugfix: {}
  adding-thing: {local: true}
agents:
  code-reviewer: {}
hooks: [pre-commit]
`)
	_, err := Load(p)
	if err != nil {
		t.Errorf("expected valid config to load cleanly, got: %v", err)
	}
}

// invariant: docsdir-default
func TestDocsDirDefaultsToDocs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "awf.yaml")
	if err := os.WriteFile(path, []byte("prefix: example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.DocsDir != "docs" {
		t.Errorf("DocsDir = %q, want \"docs\"", c.DocsDir)
	}
}

func TestDocsDirExplicitValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "awf.yaml")
	if err := os.WriteFile(path, []byte("prefix: example\ndocsDir: documentation\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.DocsDir != "documentation" {
		t.Errorf("DocsDir = %q, want \"documentation\"", c.DocsDir)
	}
}

func TestDocsDirRejectsEscapingPath(t *testing.T) {
	c := &Config{Prefix: "example", DocsDir: "../escape"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for escaping docsDir")
	}
}
