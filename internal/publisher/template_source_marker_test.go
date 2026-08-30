package publisher

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
	"github.com/hypnotox/agentic-workflows/internal/render"
)

// invariant: rendering/render-engine:template-source-symbol (TestTemplateSourceMarkerProducerMatrix)
func TestTemplateSourceMarkerProducerMatrix(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	testConfig(p).Render = nil
	plain, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	testConfig(p).Render = &config.RenderConfig{TemplateSourceRoot: "templates"}
	active, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	corpus, _, _, _, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	declarations, err := buildOutputDeclarations(testConfig(p), projectCatalog(renderInputsForTest(p)), p.Targets(), filesystemProjectReader{root: p.Root()}, corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := outputDeclarationParityError(active.Nodes, declarations); err != nil {
		t.Fatal(err)
	}

	plainByPath := map[string]OutputNode{}
	for _, node := range plain.Nodes {
		plainByPath[node.Path] = node
	}
	encoders := liveTemplateEncoders(renderInputsForTest(p))
	activeByPath := map[string]OutputNode{}
	for _, node := range active.Nodes {
		activeByPath[node.Path] = node
		if node.file == nil {
			t.Fatalf("planned output %s has no rendered file", node.Path)
		}
		content := node.file.Content
		declaredEncoder, declared := encoders[node.ObservedTemplateID]
		wantMarker := node.ObservedTemplateID != "" && declared && declaredEncoder == MarkdownAgentDialect
		hasMarker := strings.Contains(content, "<!-- awf:template-source ")
		if hasMarker != wantMarker {
			t.Errorf("%s marker participation = %t, want %t (template %q encoder %q)", node.Path, hasMarker, wantMarker, node.ObservedTemplateID, declaredEncoder)
		}
		before, ok := plainByPath[node.Path]
		if !ok || before.file == nil {
			t.Fatalf("active output %s absent from disabled plan", node.Path)
		}
		if wantMarker {
			rootMarker := "<!-- awf:template-source templates/" + node.ObservedTemplateID + " -->"
			if !strings.Contains(content, rootMarker) {
				t.Errorf("%s lacks exact root identity %q", node.Path, rootMarker)
			}
			if node.Recipe.ConfigHash == before.Recipe.ConfigHash || content == before.file.Content {
				t.Errorf("%s activation did not change both config hash and rendered bytes", node.Path)
			}
		} else if node.Recipe.ConfigHash != before.Recipe.ConfigHash || content != before.file.Content {
			t.Errorf("excluded output %s changed under template source activation", node.Path)
		}
		for _, line := range strings.Split(content, "\n") {
			if strings.Contains(line, "awf:template-source") && strings.Contains(line, "<no value>") {
				t.Errorf("%s contains an unresolved template-source identity", node.Path)
			}
		}
	}

	index := activeByPath["docs/decisions/INDEX.md"].file
	if index == nil || index.ObservedTemplateID != "" || strings.Contains(index.Content, "awf:template-source") {
		t.Fatalf("template-less ADR index gained attribution: %#v", index)
	}
	for _, path := range []string{".awf/hooks/pre-commit.sh", ".pi/extensions/awf-subagents/index.ts"} {
		if node := activeByPath[path]; node.file == nil || strings.Contains(node.file.Content, "awf:template-source") {
			t.Errorf("native-format output %s gained attribution", path)
		}
	}

	skill := activeByPath[".pi/skills/awf-reviewing/SKILL.md"].file
	if skill == nil {
		t.Fatal("reviewing skill missing from producer matrix")
	}
	rootMarker := "<!-- awf:template-source templates/skills/reviewing/SKILL.md.tmpl -->"
	if !strings.Contains(skill.Content, rootMarker) {
		t.Fatalf("reviewing skill lacks template-source marker:\n%s", skill.Content)
	}
	if yamlBlock, body, found := frontmatter.Split([]byte(skill.Content)); !found || strings.Contains(string(yamlBlock), "awf:template-source") || !strings.Contains(string(body), "awf:template-source") {
		t.Fatalf("frontmatter marker placement invalid:\n%s", skill.Content)
	}

	agents := activeByPath["AGENTS.md"].file
	section := "<!-- awf:template-source templates/agents-doc/AGENTS.md.tmpl#awf-setup -->\n"
	pointer := "<!-- awf:edit awf-setup:"
	if agents == nil || strings.Count(agents.Content, section) != 1 || !strings.Contains(agents.Content, section+pointer) {
		t.Fatalf("overridden section structural marker missing or misplaced:\n%s", agents.Content)
	}
	topic := activeByPath["docs/topics/rendering/render-engine.md"].file
	if topic == nil {
		t.Fatal("topic output missing")
	}
	banner := "<!-- " + bannerText + " -->\n"
	source := "<!-- awf:source .awf/topics/metadata/rendering/render-engine.yaml .awf/topics/parts/rendering/render-engine/current-state.md -->\n"
	topicMarker := "<!-- awf:template-source templates/topics/topic.md.tmpl -->\n"
	if !strings.Contains(topic.Content, banner+source+topicMarker) {
		t.Fatalf("banner/source/root ordering invalid:\n%s", topic.Content)
	}

	// Every active template input is mirrored by its declaration, including
	// included partials. This also proves the matrix is declaration-derived,
	// rather than inferred from a filename suffix.
	for _, node := range active.Nodes {
		if node.ObservedTemplateID == "" || encoders[node.ObservedTemplateID] != MarkdownAgentDialect {
			continue
		}
		if !slices.Contains(node.ConsumedInputs, OutputInput{Path: "templates/" + node.ObservedTemplateID, Role: ArtifactTemplate}) {
			t.Errorf("%s lacks configured root template input: %#v", node.Path, node.ConsumedInputs)
		}
	}
}

// invariant: rendering/render-engine:template-source-symbol (TestTemplateSourceSectionSemantics)
func TestTemplateSourceSectionSemantics(t *testing.T) {
	source := render.SourceText{Root: "guide.md.tmpl", Spans: []render.SourceSpan{
		{Source: "guide.md.tmpl", Text: "<!-- awf:section body -->\n"},
		{Source: "partials/default.md", Text: "DEFAULT\n"},
		{Source: "guide.md.tmpl", Text: "<!-- awf:end -->\n"},
	}}
	segments := render.ParseSourceSections(source)
	provenance := render.TemplateSource{Root: "templates"}

	dropped, _ := render.AssembleSourceWithTemplateSource(segments, map[string]render.SectionPlan{"body": {Drop: true}}, render.HTMLComment, provenance)
	if got := dropped.AuthoredText(); strings.Contains(got, "#body") || strings.Contains(got, "awf:edit") || strings.Contains(got, "DEFAULT") {
		t.Fatalf("dropped section retained structural provenance or content: %q", got)
	}

	reinjected, reinjectedParts := render.AssembleSourceWithTemplateSource(segments, map[string]render.SectionPlan{"body": {
		HasPart: true, PartBody: "BEFORE\n" + render.SectionDefaultSentinel + "\nAFTER\n",
	}}, render.HTMLComment, provenance)
	got, err := render.ExecuteSourceWithTemplateSource(reinjected, nil, reinjectedParts, "reinjected", provenance)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<!-- awf:template-source templates/guide.md.tmpl#body -->\n<!-- awf:edit body:",
		"<!-- awf:template-source templates/partials/default.md -->\nDEFAULT",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("sectionDefault reinjection lacks %q:\n%s", want, got)
		}
	}

	inPlace, inPlaceParts := render.AssembleSourceWithTemplateSource(segments, map[string]render.SectionPlan{"body": {InPlace: true}}, render.HTMLComment, provenance)
	got, err = render.ExecuteSourceWithTemplateSource(inPlace, nil, inPlaceParts, "in-place", provenance)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "templates/guide.md.tmpl#body -->\n<!-- awf:edit-in-place") || strings.Contains(got, "partials/default.md") {
		t.Fatalf("in-place body structural/interior provenance invalid:\n%s", got)
	}
}

func TestTemplateSourceRootMarkerErrors(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	testConfig(p).Render = &config.RenderConfig{TemplateSourceRoot: "templates"}
	if _, _, err := templateSourceRootMarker(renderInputsForTest(p), "missing/template.md.tmpl"); err == nil || !strings.Contains(err.Error(), "read template") {
		t.Fatalf("missing manual-producer template error = %v", err)
	}
	testConfig(p).Render.TemplateSourceRoot = "missing"
	if _, _, err := templateSourceRootMarker(renderInputsForTest(p), "topics/topic.md.tmpl"); err == nil || !strings.Contains(err.Error(), "cannot resolve template source") {
		t.Fatalf("missing manual-producer source mapping error = %v", err)
	}
}
