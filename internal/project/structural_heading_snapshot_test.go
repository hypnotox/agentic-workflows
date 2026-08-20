package project

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// TestStructuralHeadingCutoverSnapshotParity proves the schema-36 literal was
// frozen from every catalog-backed convention-part path plus the adopter-defined
// domain path family. Once this repository advances beyond schema 36, newly
// declared headings must not widen the historical migration snapshot.
func TestStructuralHeadingCutoverSnapshotParity(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	lockBytes, err := os.ReadFile(filepath.Join(root, ".awf", "awf.lock"))
	if err != nil {
		t.Fatal(err)
	}
	var lock struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(lockBytes, &lock); err != nil {
		t.Fatal(err)
	}
	if lock.SchemaVersion != 36 {
		t.Skip("schema-36 cutover parity is frozen; later headings do not widen it")
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	expected := structuralHeadingCutoverPopulation(t, p)
	source, err := os.ReadFile(filepath.Join(root, "internal", "migrate", "structuralheadings.go"))
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(source), "var structuralHeadingSnapshot")
	end := strings.Index(string(source), "type structuralHeadingEdit")
	if start < 0 || end <= start {
		t.Fatal("structural heading snapshot literal not found")
	}
	entryRE := regexp.MustCompile(`\{"([^"]+)", "([^"]+)"\}`)
	actual := map[string]string{}
	for _, match := range entryRE.FindAllStringSubmatch(string(source[start:end]), -1) {
		if _, exists := actual[match[1]]; exists {
			t.Fatalf("duplicate migration snapshot path %q", match[1])
		}
		actual[match[1]] = match[2]
	}
	if len(actual) != len(expected) {
		t.Fatalf("migration snapshot has %d entries, cutover declarations require %d", len(actual), len(expected))
	}
	for path, heading := range expected {
		if actual[path] != heading {
			t.Errorf("migration snapshot %q = %q, want %q", path, actual[path], heading)
		}
	}
}

func structuralHeadingCutoverPopulation(t *testing.T, p *Project) map[string]string {
	t.Helper()
	entries := map[string]string{}
	add := func(kind, artifact, tid string) {
		t.Helper()
		raw, err := fs.ReadFile(templates.FS, tid)
		if err != nil {
			t.Fatal(err)
		}
		expanded, err := render.ExpandIncludes(string(raw), templates.FS)
		if err != nil {
			t.Fatal(err)
		}
		stripped, err := render.StripAuthoringComments(expanded)
		if err != nil {
			t.Fatal(err)
		}
		for _, segment := range parseSections(stripped, true) {
			if !segment.IsSection || segment.Heading == "" {
				continue
			}
			path := strings.TrimPrefix(partRel(p, kind, artifact, segment.Name), ".awf/")
			if prior, exists := entries[path]; exists && prior != segment.Heading {
				t.Fatalf("cutover path %q has conflicting headings %q and %q", path, prior, segment.Heading)
			}
			entries[path] = segment.Heading
		}
	}
	for name := range projectCatalog(p).Skills {
		add("skills", name, skillTID(p, name))
	}
	for name := range projectCatalog(p).Agents {
		add("agents", name, agentTID(p, name))
	}
	for name, entry := range projectCatalog(p).Docs {
		kind, artifact := "docs", name
		if entry.Mandatory {
			kind, artifact = name, ""
		}
		add(kind, artifact, entry.TID)
	}
	raw, err := fs.ReadFile(templates.FS, mustDescriptor("domains").templateID(catalog.Standard, ""))
	if err != nil {
		t.Fatal(err)
	}
	expanded, err := render.ExpandIncludes(string(raw), templates.FS)
	if err != nil {
		t.Fatal(err)
	}
	for _, segment := range parseSections(expanded, true) {
		if segment.IsSection && segment.Heading != "" {
			entries["domains/parts/*/"+segment.Name+".md"] = segment.Heading
		}
	}
	return entries
}
