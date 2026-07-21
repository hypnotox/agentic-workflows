package project

import (
	"errors"
	"strings"
	"syscall"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

const localSkillYAML = `prefix: example
targets:
  - claude
  - cursor
skills:
  - my-skill
`

func localSkillFiles() map[string]string {
	return map[string]string{
		"skills/my-skill.yaml":             "data:\n  description: Do the thing when X.\n",
		"skills/parts/my-skill/content.md": "Body line for the local skill.\n",
	}
}

func findByTID(files []RenderedFile, tid string) []RenderedFile {
	var out []RenderedFile
	for _, f := range files {
		if f.TemplateID == tid {
			out = append(out, f)
		}
	}
	return out
}

func TestLocalSkillRendersFromBasePerTarget(t *testing.T) {
	root := scaffoldFiles(t, localSkillYAML, localSkillFiles())
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	// invariant: rendering/project-output-plan:local-renders-from-base
	got := findByTID(files, "skills/_base/SKILL.md.tmpl")
	if len(got) != 2 { // one per enabled target (claude + cursor)
		t.Fatalf("expected 2 base-rendered files, got %d", len(got))
	}
	for _, f := range got {
		for _, want := range []string{"name: example-my-skill", "Do the thing when X.", "Body line for the local skill."} {
			if !strings.Contains(f.Content, want) {
				t.Errorf("missing %q in:\n%s", want, f.Content)
			}
		}
		if strings.Contains(f.Content, "<no value>") {
			t.Errorf("publication-unsafe <no value>:\n%s", f.Content)
		}
	}
}

func TestLocalAgentRendersFromBase(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nagents:\n  - my-agent\n", map[string]string{
		"agents/my-agent.yaml":             "data:\n  description: Reviews the frobnicator.\n",
		"agents/parts/my-agent/content.md": "Agent body here.\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	got := findByTID(files, "agents/_base.md.tmpl")
	if len(got) != 1 { // default target claude only
		t.Fatalf("expected 1 base agent file, got %d", len(got))
	}
	for _, want := range []string{"name: my-agent", "Reviews the frobnicator.", "Agent body here."} {
		if !strings.Contains(got[0].Content, want) {
			t.Errorf("missing %q in:\n%s", want, got[0].Content)
		}
	}
}

func TestLocalSynthesisDoesNotMutateStandard(t *testing.T) {
	before := len(catalog.Standard.Skills)
	root := scaffoldFiles(t, localSkillYAML, localSkillFiles())
	if _, err := Open(root); err != nil {
		t.Fatal(err)
	}
	// invariant: rendering/project-output-plan:local-catalog-clone
	if got := len(catalog.Standard.Skills); got != before {
		t.Errorf("catalog.Standard.Skills mutated: before %d, after %d", before, got)
	}
	if _, ok := catalog.Standard.Skills["my-skill"]; ok {
		t.Error("local entry leaked into catalog.Standard")
	}
}

func TestLocalUndeclaredNameFailsOpen(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - ghost\n", nil)
	_, err := Open(root)
	// invariant: rendering/project-output-plan:local-requires-declaration
	if err == nil || !strings.Contains(err.Error(), "is not in the catalog") {
		t.Fatalf("expected not-in-catalog error, got %v", err)
	}
}

func TestLocalSkillSidecarStatErrorFailsOpen(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - my-skill\n", map[string]string{
		"skills": "not a directory",
	})
	_, err := Open(root)
	if !errors.Is(err, syscall.ENOTDIR) || !strings.Contains(err.Error(), "stat sidecar skills/my-skill.yaml") {
		t.Fatalf("expected skill sidecar ENOTDIR, got %v", err)
	}
}

func TestLocalReservedNameFailsOpen(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - _x\n", map[string]string{
		"skills/_x.yaml": "data:\n  description: nope\n",
	})
	_, err := Open(root)
	if err == nil || !strings.Contains(err.Error(), "kebab-case") {
		t.Fatalf("expected invalid-name error, got %v", err)
	}
}

// TestLocalAgentReservedNameFailsOpen exercises the agents-side synthesis
// error-return in effectiveCatalog (skills stay clean, agents fail).
func TestLocalAgentReservedNameFailsOpen(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nagents:\n  - _a\n", map[string]string{
		"agents/_a.yaml": "data:\n  description: x\n",
	})
	_, err := Open(root)
	if err == nil || !strings.Contains(err.Error(), "kebab-case") {
		t.Fatalf("expected invalid-name error from agents synthesis, got %v", err)
	}
}

func TestLocalMalformedSidecarFailsOpen(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - my-skill\n", map[string]string{
		"skills/my-skill.yaml": "data: [unterminated\n",
	})
	_, err := Open(root)
	if err == nil || !strings.Contains(err.Error(), "sidecar") {
		t.Fatalf("expected sidecar parse error, got %v", err)
	}
}

func TestLocalNameShadowingStandardStaysStandard(t *testing.T) {
	root := scaffoldFiles(t, sampleYAML, map[string]string{
		"skills/parts/tdd/content.md": "should be ignored\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// invariant: rendering/project-output-plan:local-no-shadow
	if p.Cat.Skills["tdd"].Base {
		t.Error("Standard skill tdd was shadowed by a local synthesis")
	}
	if secs := p.Cat.Skills["tdd"].Sections; len(secs) == 1 && secs[0] == "content" {
		t.Errorf("tdd sections overwritten to [content]: %v", secs)
	}
}

func TestLocalHandAuthoredSkipped(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - hand\n", map[string]string{
		"skills/hand.yaml": "local: true\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Cat.Skills["hand"]; ok {
		t.Error("a local:true name must not be synthesized into the catalog")
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	if got := findByTID(files, "skills/_base/SKILL.md.tmpl"); len(got) != 0 {
		t.Errorf("a local:true skill must not be base-rendered, got %d files", len(got))
	}
}

const localDocYAML = `prefix: example
docs:
  - my-doc
`

func localDocFiles() map[string]string {
	return map[string]string{
		"docs/my-doc.yaml":             "data:\n  title: My Doc\n  description: What my-doc covers.\n",
		"docs/parts/my-doc/content.md": "Body line for the local doc.\n",
	}
}

func TestLocalDocRendersFromBase(t *testing.T) {
	root := scaffoldFiles(t, localDocYAML, localDocFiles())
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	// invariant: rendering/project-output-plan:local-doc-renders-from-base
	got := findByTID(files, "docs/_base.md.tmpl")
	if len(got) != 1 { // docs render once (not per-target)
		t.Fatalf("expected 1 base-rendered doc, got %d", len(got))
	}
	for _, want := range []string{"# My Doc", "What my-doc covers.", "Body line for the local doc."} {
		if !strings.Contains(got[0].Content, want) {
			t.Errorf("missing %q in:\n%s", want, got[0].Content)
		}
	}
	if strings.Contains(got[0].Content, "<no value>") {
		t.Errorf("publication-unsafe <no value>:\n%s", got[0].Content)
	}
	if got[0].Path != "docs/my-doc.md" {
		t.Errorf("doc out path = %q, want docs/my-doc.md", got[0].Path)
	}
}

func TestLocalDocSubfolderPathAndMap(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\ndocs:\n  - guides/ci\n", map[string]string{
		"docs/guides/ci.yaml":             "data:\n  description: How CI runs.\n",
		"docs/parts/guides/ci/content.md": "CI body.\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	got := findByTID(files, "docs/_base.md.tmpl")
	if len(got) != 1 || got[0].Path != "docs/guides/ci.md" {
		t.Fatalf("nested doc path wrong: %+v", got)
	}
	// Title defaults from the last segment when the sidecar omits it (map-fields).
	if !strings.Contains(got[0].Content, "# Ci") {
		t.Errorf("expected derived '# Ci' title in:\n%s", got[0].Content)
	}
	docs, err := p.resolvedDocs()
	if err != nil {
		t.Fatal(err)
	}
	var found map[string]any
	for _, d := range docs {
		if d["name"] == "guides/ci" {
			found = d
		}
	}
	if found == nil {
		t.Fatalf("guides/ci missing from document map: %+v", docs)
	}
	// invariant: rendering/project-output-plan:local-doc-map-fields
	if found["title"] != "Ci" || found["desc"] != "How CI runs." {
		t.Errorf("map fields wrong: %+v", found)
	}
}

func TestLocalDocSynthesisDoesNotMutateStandard(t *testing.T) {
	before := len(catalog.Standard.Docs)
	root := scaffoldFiles(t, localDocYAML, localDocFiles())
	if _, err := Open(root); err != nil {
		t.Fatal(err)
	}
	// invariant: rendering/project-output-plan:local-doc-catalog-clone
	if got := len(catalog.Standard.Docs); got != before {
		t.Errorf("catalog.Standard.Docs mutated: before %d, after %d", before, got)
	}
	if _, ok := catalog.Standard.Docs["my-doc"]; ok {
		t.Error("local doc leaked into catalog.Standard.Docs")
	}
}

func TestLocalDocUndeclaredNameFailsOpen(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\ndocs:\n  - ghost\n", nil)
	_, err := Open(root)
	// invariant: rendering/project-output-plan:local-doc-requires-declaration
	if err == nil || !strings.Contains(err.Error(), "is not in the catalog") {
		t.Fatalf("expected not-in-catalog error, got %v", err)
	}
}

func TestLocalDocSidecarStatErrorFailsOpen(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\ndocs:\n  - my-doc\n", map[string]string{
		"docs": "not a directory",
	})
	_, err := Open(root)
	if !errors.Is(err, syscall.ENOTDIR) || !strings.Contains(err.Error(), "stat sidecar docs/my-doc.yaml") {
		t.Fatalf("expected doc sidecar ENOTDIR, got %v", err)
	}
}

func TestLocalDocInvalidNameFailsOpen(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\ndocs:\n  - Bad\n", map[string]string{
		"docs/Bad.yaml": "data:\n  description: x\n",
	})
	_, err := Open(root)
	if err == nil || !strings.Contains(err.Error(), "kebab-case") {
		t.Fatalf("expected invalid doc-name error, got %v", err)
	}
}

func TestLocalDocMalformedSidecarFailsOpen(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\ndocs:\n  - my-doc\n", map[string]string{
		"docs/my-doc.yaml": "data: [unterminated\n",
	})
	_, err := Open(root)
	if err == nil || !strings.Contains(err.Error(), "sidecar") {
		t.Fatalf("expected sidecar parse error, got %v", err)
	}
}

func TestLocalDocNameShadowingStandardStaysStandard(t *testing.T) {
	// "architecture" is a Standard toggleable doc; enabling it with a sidecar must
	// not synthesize a base-rendered local over it.
	root := scaffoldFiles(t, "prefix: example\ndocs:\n  - architecture\n", map[string]string{
		"docs/architecture.yaml": "data:\n  title: Should Not Win\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// invariant: rendering/project-output-plan:local-doc-no-shadow
	if p.Cat.Docs["architecture"].TID == "docs/_base.md.tmpl" {
		t.Error("Standard doc architecture was shadowed by a local synthesis")
	}
}

func TestLocalDocHandAuthoredSkipped(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\ndocs:\n  - hand\n", map[string]string{
		"docs/hand.yaml": "local: true\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Cat.Docs["hand"]; ok {
		t.Error("a local:true doc must not be synthesized into the catalog")
	}
}

// TestLocalDocDefaultDescWhenSidecarOmits covers the desc-default branch of
// synthesizeLocalDocs (a sidecar with no data.description).
func TestLocalDocDefaultDescWhenSidecarOmits(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\ndocs:\n  - bare\n", map[string]string{
		"docs/bare.yaml":             "data:\n  title: Bare\n",
		"docs/parts/bare/content.md": "Body.\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Cat.Docs["bare"].Desc; got != "Project-local documentation." {
		t.Errorf("default desc = %q, want the generic fallback", got)
	}
}

// TestRenderAllRejectsDuplicateOutputPath: a path-aware local doc name can land
// on awf's reserved output territory - here docs/decisions/template.md, also
// rendered by the adr-template singleton - and RenderAll must fail loudly.
func TestRenderAllRejectsDuplicateOutputPath(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\ndocs:\n  - decisions/template\n", map[string]string{
		"docs/decisions/template.yaml":             "data:\n  description: collide.\n",
		"docs/parts/decisions/template/content.md": "body\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.RenderAll(); err == nil || !strings.Contains(err.Error(), "same output path") {
		t.Fatalf("expected duplicate-output-path error, got %v", err)
	}
}

func TestReleasingCatalogDocRenders(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\ndocs:\n  - releasing\n", nil)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	got := findByTID(files, "docs/releasing.md.tmpl")
	if len(got) != 1 || got[0].Path != "docs/releasing.md" {
		t.Fatalf("releasing render wrong: %+v", got)
	}
	for _, want := range []string{"# Releasing", "Describe how this project cuts a release"} {
		if !strings.Contains(got[0].Content, want) {
			t.Errorf("missing %q in:\n%s", want, got[0].Content)
		}
	}
	if strings.Contains(got[0].Content, "<no value>") {
		t.Errorf("publication-unsafe render:\n%s", got[0].Content)
	}
}

// TestDeriveDocTitle covers title derivation, including the empty-segment guard
// (a trailing/double hyphen yields an empty split word).
func TestDeriveDocTitle(t *testing.T) {
	cases := map[string]string{
		"ci":                   "Ci",
		"release-process":      "Release Process",
		"guides/release-steps": "Release Steps",
		"release-":             "Release",
		"a--b":                 "A B",
	}
	for in, want := range cases {
		if got := DeriveDocTitle(in); got != want {
			t.Errorf("DeriveDocTitle(%q) = %q, want %q", in, got, want)
		}
	}
}
