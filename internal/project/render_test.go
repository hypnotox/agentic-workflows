package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/templates"
)

// syncedWorkflowDoc scaffolds a minimal project whose commit-discipline part is
// body, syncs it, and returns the rendered docs/workflow.md content.
func syncedWorkflowDoc(t *testing.T, body string) string {
	t.Helper()
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars: {}\n",
		map[string]string{"parts/workflow/commit-discipline.md": body})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "docs", "workflow.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCommitPolicyRenderDataProjection(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := p.data(config.Sidecar{}, map[string]bool{})["commitPolicy"].(*config.CommitPolicyConfig); !ok || got != nil {
		t.Fatalf("absent commitPolicy projection = %#v, want typed nil", got)
	}
	policy := &config.CommitPolicyConfig{
		GrandfatheredThrough: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		AllowedIdentities:    []config.CommitPolicyIdentity{{Name: "Ada", Email: "ada@example.test"}},
	}
	p.Cfg.CommitPolicy = policy
	if got := p.data(config.Sidecar{}, map[string]bool{})["commitPolicy"]; got != policy {
		t.Fatalf("commitPolicy projection = %#v, want typed policy %#v", got, policy)
	}
}

// A whole-line awf:comment directive in a convention part never reaches
// rendered output, while a mid-line occurrence and a fenced whole-line demo
// render verbatim (ADR-0121 Decisions 1-3; the template-source seam is proven
// by the render-layer unit tests plus the strip call in renderTarget).
// invariant: rendering/inplace-and-placeholders:authoring-comment-stripped (TestAuthoringCommentStrippedFromPart)
func TestAuthoringCommentStrippedFromPart(t *testing.T) {
	out := syncedWorkflowDoc(t,
		"<!-- awf:comment touches-state: demo/topic:demo-slug - an internal tag -->\n"+
			"KEEP-TOP\n"+
			"mid-line <!-- awf:comment inline note --> kept\n"+
			"```\n<!-- awf:comment fenced demo -->\n```\n")
	if strings.Contains(out, "demo-slug") {
		t.Errorf("whole-line directive leaked into rendered output:\n%s", out)
	}
	for _, want := range []string{"KEEP-TOP", "mid-line <!-- awf:comment inline note --> kept", "<!-- awf:comment fenced demo -->"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// A part whose only content is directive lines strips to an empty body: the
// section renders empty with its awf:edit pointer present, never falling back
// to the template default (ADR-0034 Decision 4 semantics preserved).
func TestCommentOnlyPartRendersEmptySection(t *testing.T) {
	out := syncedWorkflowDoc(t,
		"<!-- awf:comment first note -->\n<!-- awf:comment second note -->\n")
	if !strings.Contains(out, "from .awf/parts/workflow/commit-discipline.md") {
		t.Errorf("comment-only part must still be consumed (pointer present):\n%s", out)
	}
	if strings.Contains(out, "Use Conventional Commits, one concern per commit.") {
		t.Errorf("empty part must not fall back to the template default:\n%s", out)
	}
	if strings.Contains(out, "awf:comment") {
		t.Errorf("directive lines leaked:\n%s", out)
	}
}

// A malformed whole-line opener in a part fails the render naming the part path.
func TestMalformedAuthoringCommentFailsSync(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars: {}\n",
		map[string]string{"parts/workflow/commit-discipline.md": "<!-- awf:comment unclosed\n"})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	err = p.Sync()
	if err == nil {
		t.Fatal("a malformed opener must fail the sync")
	}
	if !strings.Contains(err.Error(), ".awf/parts/workflow/commit-discipline.md") ||
		!strings.Contains(err.Error(), "malformed awf:comment") {
		t.Errorf("error must name the part path and the directive, got %v", err)
	}
}

// An unknown {{=awf:key}} placeholder demonstrated inside an authoring comment
// must not hard-error: the strip runs before placeholder substitution
// (ADR-0121 Decision 2).
func TestUnknownPlaceholderInsideCommentRenders(t *testing.T) {
	out := syncedWorkflowDoc(t,
		"<!-- awf:comment mentions {{=awf:nonexistent}} -->\nBODY\n")
	if !strings.Contains(out, "BODY") || strings.Contains(out, "nonexistent") {
		t.Errorf("comment-wrapped unknown placeholder must strip cleanly:\n%s", out)
	}
}

// The template seam end-to-end: the embedded adr-readme template includes a
// partial containing only a qualified touches-state authoring comment, so any
// regression in include expansion or renderTarget strip wiring leaks it into
// every scaffolded project's rendered README.
// touches-state: rendering/inplace-and-placeholders:authoring-comment-stripped - the renderTarget wiring, proven end-to-end over the real embedded template
func TestEmbeddedTemplateAuthoringCommentStripped(t *testing.T) {
	const directive = "<!-- awf:comment touches-state: rendering/templates:template-source-residue - the embedded ADR README include is source-only -->"
	src, err := fs.ReadFile(templates.FS, "adr-readme/README.md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "<!-- awf:include template-source-observation -->") {
		t.Fatal("embedded ADR README source lacks the comment-only include")
	}
	partial, err := fs.ReadFile(templates.FS, "partials/template-source-observation.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(partial), directive) {
		t.Fatalf("embedded comment-only include lacks qualified directive %q", directive)
	}

	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars: {}\n", nil)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "docs", "decisions", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), directive) || strings.Contains(string(b), "awf:comment") {
		t.Errorf("the embedded template's qualified authoring comment leaked into rendered output:\n%s", b)
	}
}

func TestRenderTargetTemplateSourceActivation(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nrender:\n  templateSourceRoot: templates\n")
	const tid = "adr-readme/README.md.tmpl"
	src, err := fs.ReadFile(templates.FS, tid)
	if err != nil {
		t.Fatal(err)
	}
	expanded, err := render.ExpandIncludesSource(string(src), tid, templates.FS)
	if err != nil {
		t.Fatal(err)
	}
	const commentOnlySource = "partials/template-source-observation.md"
	var commentOnlyText strings.Builder
	for _, span := range expanded.Spans {
		if span.Source == "" {
			continue
		}
		if span.Source == commentOnlySource {
			commentOnlyText.WriteString(span.Text)
			continue
		}
		path := filepath.Join(root, "templates", filepath.FromSlash(span.Source))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(span.Text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if commentOnlyText.Len() == 0 {
		t.Fatal("expanded template lost the comment-only include identity")
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.renderTarget("adr-readme", "", tid, p.Cat.Docs["adr-readme"].Sections, config.Sidecar{}, p.data(config.Sidecar{}, map[string]bool{}), "docs/decisions/README.md", map[string]bool{}); err == nil || !strings.Contains(err.Error(), "templates/"+commentOnlySource) {
		t.Fatalf("missing comment-only include mapping error = %v", err)
	}
	commentOnlyPath := filepath.Join(root, "templates", filepath.FromSlash(commentOnlySource))
	if err := os.MkdirAll(filepath.Dir(commentOnlyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(commentOnlyPath, []byte(commentOnlyText.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	rf, err := p.renderTarget("adr-readme", "", tid, p.Cat.Docs["adr-readme"].Sections, config.Sidecar{}, p.data(config.Sidecar{}, map[string]bool{}), "docs/decisions/README.md", map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rf.Content, "<!-- awf:template-source templates/"+tid+" -->") || strings.Contains(rf.Content, commentOnlySource) || rf.TemplateHash == "" || rf.ConfigHash == "" {
		t.Fatalf("provenance render did not validate then strip the comment-only include: %s", rf.Content)
	}
	p.Cfg.Render.TemplateSourceRoot = "missing"
	if _, err := p.renderTarget("adr-readme", "", tid, p.Cat.Docs["adr-readme"].Sections, config.Sidecar{}, p.data(config.Sidecar{}, map[string]bool{}), "docs/decisions/README.md", map[string]bool{}); err == nil {
		t.Fatal("unresolved configured source root accepted")
	}
}

func TestValidateTemplateSourcesUsesSelectedTree(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: "templates/doc.md", Mode: snapshot.Regular, Bytes: []byte("source")},
		{Path: "templates/linked.md", Mode: snapshot.Symlink, Bytes: []byte("../outside.md")},
	})
	if err != nil {
		t.Fatal(err)
	}
	p := &Project{read: snapshotTreeReader{tree: tree}}
	src := render.SourceText{Root: "doc.md", Spans: []render.SourceSpan{{Source: "doc.md", Text: "x"}}}
	if err := p.validateTemplateSources(src, "templates"); err != nil {
		t.Fatal(err)
	}
	if err := p.validateTemplateSources(render.SourceText{Spans: []render.SourceSpan{{Source: "missing.md", Text: "x"}}}, "templates"); err == nil {
		t.Fatal("missing source accepted")
	}
	if err := p.validateTemplateSources(render.SourceText{Spans: []render.SourceSpan{{Source: "linked.md", Text: "x"}}}, "templates"); err == nil {
		t.Fatal("staged symlink source accepted")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "templates", "directory.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (&Project{Root: root}).validateTemplateSources(render.SourceText{Spans: []render.SourceSpan{{Source: "directory.md", Text: "x"}}}, "templates"); err == nil {
		t.Fatal("directory source accepted")
	}
	if err := os.WriteFile(filepath.Join(root, "outside.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outside.md"), filepath.Join(root, "templates", "linked.md")); err != nil {
		t.Fatal(err)
	}
	if err := (&Project{Root: root}).validateTemplateSources(render.SourceText{Spans: []render.SourceSpan{{Source: "linked.md", Text: "x"}}}, "templates"); err == nil {
		t.Fatal("symlink source accepted")
	}
	// An included partial that strips to empty still has an authored identity
	// and must therefore resolve before stripping removes its output bytes.
	if err := os.WriteFile(filepath.Join(root, "templates", "comment-only.md"), []byte("<!-- awf:comment note -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&Project{Root: root}).validateTemplateSources(render.SourceText{Spans: []render.SourceSpan{{Source: "comment-only.md", Text: "<!-- awf:comment note -->\n"}}}, "templates"); err != nil {
		t.Fatalf("comment-only included source was not observed: %v", err)
	}
}

func TestRenderTargetStructuralHeadingFollowsOutputEncoder(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	part := p.Cfg.PartPath("docs", "architecture", "overview")
	if err := os.MkdirAll(filepath.Dir(part), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, []byte("PART BODY\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sc, err := p.Cfg.Sidecar("docs", "architecture")
	if err != nil {
		t.Fatal(err)
	}
	entry := p.Cat.Docs["architecture"]
	cases := []struct {
		name        string
		options     *renderOutputOptions
		wantHeading bool
	}{
		{"ordinary Markdown", nil, true},
		{"generated Markdown", &renderOutputOptions{encoder: MarkdownAgentDialect}, true},
		{"Markdown target", &renderOutputOptions{encoder: MarkdownAgentDialect, bannerStyle: render.HTMLComment}, true},
		{"plain target", &renderOutputOptions{encoder: PlainAgentDialect, bannerStyle: render.SlashComment}, false},
		{"plain conditional", &renderOutputOptions{encoder: PlainAgentDialect}, false},
		{"plain resident", &renderOutputOptions{encoder: PlainAgentDialect}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := []*renderOutputOptions{}
			if tc.options != nil {
				args = append(args, tc.options)
			}
			file, err := p.renderTarget("docs", "architecture", entry.TID, entry.Sections, sc,
				p.data(sc, map[string]bool{}), "out.md", map[string]bool{}, args...)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(file.Content, "## Overview"); got != tc.wantHeading {
				t.Fatalf("structural heading present = %v, want %v:\n%s", got, tc.wantHeading, file.Content)
			}
			if !strings.Contains(file.Content, "PART BODY") {
				t.Fatalf("part body missing:\n%s", file.Content)
			}
		})
	}
}

// TestRenderProducerCallsitesForwardEncoder is the mutation-sensitive wiring
// complement to the behavior test above: every actual producer family must pass
// its declaration into the shared renderTarget seam.
func TestCaptureStructuralHeadingsReportsDefaultExpressionOmittedByOverride(t *testing.T) {
	// Capture executes the complete template skeleton before assembly, so this
	// invalid default expression is observable even though the convention-part
	// override below would omit it from the final output.
	segs := parseSections("<!-- awf:section body -->\n## Heading\n{{ .missing.field }}\n<!-- awf:end -->", true)
	_, err := captureStructuralHeadings(segs, map[string]any{}, "test template")
	if err == nil || !strings.Contains(err.Error(), "render test template headings: execute template") || !strings.Contains(err.Error(), "nil pointer evaluating") {
		t.Fatalf("contextual capture error = %v", err)
	}
	assembled, parts := assemble(segs, map[string]render.SectionPlan{"body": {HasPart: true, PartBody: "override"}}, render.HTMLComment)
	if output, assembleErr := render.Execute(assembled, map[string]any{}, parts, "test final assembly"); assembleErr != nil || !strings.Contains(output, "override") {
		t.Fatalf("override final assembly = %q, %v", output, assembleErr)
	}

	// Exercise the renderTarget contextual return path with an embedded template:
	// the capture sees this invalid data before section assembly can substitute a
	// convention part.
	root := scaffold(t, sampleYAML)
	p, openErr := Open(testContext(t), root)
	if openErr != nil {
		t.Fatal(openErr)
	}
	sc, sidecarErr := p.Cfg.Sidecar("docs", "workflow")
	if sidecarErr != nil {
		t.Fatal(sidecarErr)
	}
	data := p.data(sc, map[string]bool{})
	data["layout"] = nil
	entry := p.Cat.Docs["workflow"]
	if _, targetErr := p.renderTarget("docs", "workflow", entry.TID, entry.Sections, sc, data, "out.md", map[string]bool{}); targetErr == nil || !strings.Contains(targetErr.Error(), "render "+entry.TID+" headings: execute template") {
		t.Fatalf("renderTarget capture error = %v", targetErr)
	}

	collisionSegs := parseSections("<!-- awf:section body -->\n## {{ .heading }}\ndefault\n<!-- awf:end -->", true)
	_, tokens := render.StructuralHeadingCapture(collisionSegs)
	if _, collisionErr := captureStructuralHeadings(collisionSegs, map[string]any{"heading": tokens["body"][1]}, "collision template"); collisionErr == nil || !strings.Contains(collisionErr.Error(), "ambiguous framing") {
		t.Fatalf("contextual collision error = %v", collisionErr)
	}
}

func TestRenderProducerCallsitesForwardEncoder(t *testing.T) {
	renderSource, err := os.ReadFile("render.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(renderSource)
	for name, fragment := range map[string]string{
		"catalog target Markdown":  "options = &renderOutputOptions{bannerStyle: render.HTMLComment, target: &target, encoder: MarkdownAgentDialect}",
		"agent target encoder":     "options.encoder = spec.target.AgentDialect",
		"target output encoder":    "encoder:     targetOutput.Encoder,",
		"generated domain encoder": "Encoder: rf.Encoder,",
	} {
		if !strings.Contains(source, fragment) {
			t.Errorf("%s callsite stopped forwarding its declared encoder", name)
		}
	}
	if got := strings.Count(source, "&renderOutputOptions{encoder: PlainAgentDialect}"); got != 2 {
		t.Errorf("conditional and resident plain callsites = %d, want 2", got)
	}
	configReferenceSource, err := os.ReadFile("configreference.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(configReferenceSource), "Encoder: rf.Encoder") {
		t.Error("generated config-reference wrapper stopped forwarding its encoder")
	}
}
