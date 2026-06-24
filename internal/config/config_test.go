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
