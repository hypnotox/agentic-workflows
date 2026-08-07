package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/templates"
)

// TestDocsSectionParity asserts that for every catalog doc the declared section
// set equals the template's marker-block set, and that each doc renders from
// template defaults with no leaked <no value> token.
// invariant: rendering/guide-and-doc-templates:docs-section-parity (TestDocsSectionParity)
func TestDocsSectionParity(t *testing.T) {
	cat := catalog.Standard
	for name, spec := range cat.Docs {
		if spec.AgentsDoc || spec.Path != "" {
			// Root and structural docs have their own output shape and are covered
			// by their singleton parity tests. Mandatory is sidecar location only.
			continue
		}
		tid := spec.TID
		if tid == "" {
			tid = fmt.Sprintf("docs/%s.md.tmpl", name)
		}
		src, err := fs.ReadFile(templates.FS, tid)
		if err != nil {
			t.Fatalf("read %s: %v", tid, err)
		}
		var markers []string
		for _, s := range render.ParseSections(string(src)) {
			if s.IsSection {
				markers = append(markers, s.Name)
			}
		}
		want := append([]string(nil), spec.Sections...)
		got := append([]string(nil), markers...)
		sort.Strings(want)
		sort.Strings(got)
		if strings.Join(want, ",") != strings.Join(got, ",") {
			t.Errorf("%s: section mismatch: catalog %v vs template markers %v", name, want, got)
		}
		asm, parts := render.Assemble(render.ParseSections(string(src)), nil, render.HTMLComment)
		out, err := render.Execute(asm,
			map[string]any{"prefix": "awf", "vars": map[string]any{},
				"layout": map[string]any{"adrReadme": "docs/decisions/README.md"}, "data": map[string]any{}}, parts, "test")
		if err != nil {
			t.Fatalf("render %s: %v", tid, err)
		}
		if strings.Contains(out, "<no value>") {
			t.Errorf("%s: <no value> leaked into rendered doc", name)
		}
	}
}

// TestDocsSectionParityMembershipUsesOutputShape prevents Mandatory from
// becoming a hidden membership oracle: name-derived docs participate regardless
// of sidecar location, while root and structural docs do not.
func TestDocsSectionParityMembershipUsesOutputShape(t *testing.T) {
	entries := map[string]catalog.DocEntry{
		"root":       {AgentsDoc: true, Mandatory: false},
		"structural": {Path: "structural.md", Mandatory: false},
		"named-root": {Mandatory: true},
		"named-docs": {Mandatory: false},
	}
	got := map[string]bool{}
	for name, entry := range entries {
		if !entry.AgentsDoc && entry.Path == "" {
			got[name] = true
		}
	}
	want := map[string]bool{"named-root": true, "named-docs": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("name-derived parity membership = %v, want %v", got, want)
	}
}

// invariant: rendering/inplace-and-placeholders:section-orphan-flagged (TestSectionOrphanDetection)
func TestSectionOrphanDetection(t *testing.T) {
	valid := catalog.Standard.Docs["architecture"].Sections[0]
	const orphan = "definitely-not-a-section"
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n"+sprintfVars(""), map[string]string{
		"docs/parts/architecture/" + valid + ".md":  "## Valid\n\noverride body\n",
		"docs/parts/architecture/" + orphan + ".md": "## Bogus\n\nstray\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	var sawOrphan, sawValid bool
	for _, finding := range drift {
		if finding.Kind != "orphaned" {
			continue
		}
		switch finding.Path {
		case ".awf/docs/parts/architecture/" + orphan + ".md":
			sawOrphan = true
		case ".awf/docs/parts/architecture/" + valid + ".md":
			sawValid = true
		}
	}
	if !sawOrphan || sawValid {
		t.Fatalf("section drift = %#v", drift)
	}
}

// invariant: rendering/guide-and-doc-templates:agents-doc-section-parity (TestAgentsDocSectionParity)
func TestAgentsDocSectionParity(t *testing.T) {
	cat := catalog.Standard
	entry := cat.Docs["agents-doc"]
	src, err := fs.ReadFile(templates.FS, entry.TID)
	if err != nil {
		t.Fatalf("read %s: %v", entry.TID, err)
	}
	var markers []string
	for _, s := range render.ParseSections(string(src)) {
		if s.IsSection {
			markers = append(markers, s.Name)
		}
	}
	if strings.Join(markers, ",") != strings.Join(entry.Sections, ",") {
		t.Errorf("%s markers %v != catalog sections %v", entry.TID, markers, entry.Sections)
	}
}

// TestWorkflowDocChainOrder asserts the workflow doc's default render carries
// the canonical chain string: ADR review precedes every affected ordinary plan
// review and the workflow exposes no separate reconciliation node.
// invariant: rendering/guide-and-doc-templates:maintainable-code-design-guide (TestWorkflowDocChainOrder)
func TestWorkflowDocChainOrder(t *testing.T) {
	out := renderGolden(t, "docs/workflow.md.tmpl", map[string]any{
		"vars":   map[string]any{},
		"layout": testLayout(),
		"data":   map[string]any{},
	})
	// invariant: rendering/workflow-skill-templates:workflow-chain-adr-before-plan (TestWorkflowDocChainOrder)
	if strings.Index(out, "**ADR**") > strings.Index(out, "**Planning**") {
		t.Errorf("workflow guidance must present ADR before plan:\n%s", out)
	}
	// invariant: rendering/workflow-skill-templates:linked-plan-review-freshness (TestWorkflowDocChainOrder)
	if !strings.Contains(out, "settle ADR review first") || !strings.Contains(out, "every linked Proposed plan") {
		t.Errorf("workflow guidance must route ADR-first ordinary plan review:\n%s", out)
	}
	for _, forbidden := range []string{"workflow profiles", "depth controls", "routers", "classifiers", "runtime policy knobs"} {
		if !strings.Contains(out, "no "+forbidden) && !strings.Contains(out, "no workflow profiles, depth controls, routers, classifiers, or runtime policy knobs") {
			t.Errorf("workflow guidance does not forbid %q:\n%s", forbidden, out)
		}
	}
	planSelection := []string{"sequencing, coordination, or resumability materially helps", "records and operationalizes approved choices", "rather than inventing speculative structure, checks, or work"}
	for _, want := range planSelection {
		if !strings.Contains(out, want) {
			t.Errorf("workflow plan selection missing %q:\n%s", want, out)
		}
	}
	override, err := os.ReadFile(filepath.Join(repoRootDir(t), ".awf", "parts", "workflow", "chain.md"))
	if err != nil {
		t.Fatalf("read project workflow override: %v", err)
	}
	for _, want := range planSelection {
		if !strings.Contains(string(override), want) {
			t.Errorf("project workflow override missing plan selection %q:\n%s", want, override)
		}
	}
}

// invariant: rendering/workflow-skill-templates:single-workflow-no-depth-controls (TestSingleWorkflowHasNoDepthControls)
func TestSingleWorkflowHasNoDepthControls(t *testing.T) {
	var configSurface []string
	seen := map[reflect.Type]bool{}
	var collectConfig func(reflect.Type)
	collectConfig = func(typ reflect.Type) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Map {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || typ.PkgPath() != reflect.TypeOf(config.Config{}).PkgPath() || seen[typ] {
			return
		}
		seen[typ] = true
		for i := range typ.NumField() {
			field := typ.Field(i)
			configSurface = append(configSurface, field.Name, field.Tag.Get("yaml"))
			collectConfig(field.Type)
		}
	}
	collectConfig(reflect.TypeOf(config.Config{}))
	liveConfig, err := os.ReadFile(filepath.Join(repoRootDir(t), ".awf", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	configSurface = append(configSurface, string(liveConfig))

	production := map[string]string{}
	legacyAdvisoryProfileCounts := map[string]struct {
		symbol int
		phrase int
	}{
		"internal/catalog/catalog.go":  {symbol: 3},
		"internal/catalog/standard.go": {symbol: 20},
		"internal/catalog/workflow.go": {symbol: 2, phrase: 1},
		"internal/project/local.go":    {symbol: 1},
		"internal/project/project.go":  {symbol: 1},
	}
	testsupport.WalkRepoSources(t, repoRootDir(t), func(path string, body []byte) {
		source := string(body)
		if counts, ok := legacyAdvisoryProfileCounts[path]; ok {
			// WorkflowProfile is the pre-existing advisory skill-selection metadata,
			// not a selectable workflow operating profile. Pin and remove only these
			// known occurrences so a new use anywhere, including these files, fails.
			if got := strings.Count(source, "WorkflowProfile"); got != counts.symbol {
				t.Errorf("%s WorkflowProfile occurrence count = %d, want %d", path, got, counts.symbol)
			}
			if got := strings.Count(strings.ToLower(source), "workflow profile"); got != counts.phrase {
				t.Errorf("%s workflow profile phrase count = %d, want %d", path, got, counts.phrase)
			}
			source = strings.ReplaceAll(source, "WorkflowProfile", "")
			source = strings.ReplaceAll(strings.ToLower(source), "workflow profile", "")
		}
		production[path] = source
	})

	templateRuntime := map[string]string{}
	if err := fs.WalkDir(templates.FS, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}
		body, err := fs.ReadFile(templates.FS, path)
		if err != nil {
			return err
		}
		source := strings.ReplaceAll(string(body), "awf ships this as one workflow, with no workflow profiles, depth controls, routers, classifiers, or runtime policy knobs.", "")
		templateRuntime[path+" body"] = source
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	surfaces := workflowControlSurfaces{
		configuration: strings.Join(configSurface, "\n"),
		production:    production,
		templates:     templateRuntime,
	}
	if violations := workflowControlCensus(surfaces); len(violations) != 0 {
		t.Errorf("workflow control census found prohibited controls: %v", violations)
	}

	for name, mutation := range map[string]workflowControlSurfaces{
		"configuration": {configuration: "workflowProfile: governed"},
		"production":    {production: map[string]string{"internal/plan/control.go": "type WorkflowProfile struct{}"}},
		"template runtime": {templates: map[string]string{
			"skills/control/SKILL.md.tmpl body": "Select a WorkflowRouter before execution.",
		}},
	} {
		t.Run("rejects "+name+" control", func(t *testing.T) {
			if violations := workflowControlCensus(mutation); len(violations) != 1 {
				t.Fatalf("mutation produced violations %v, want one", violations)
			}
		})
	}
	for class, mutations := range map[string][]string{
		"profile":        {"governance profile", "WorkflowProfile", "WorkflowProfileSelection"},
		"depth control":  {"review depth", "DepthControl"},
		"router":         {"workflow router", "ReviewRouter"},
		"classifier":     {"workflow classifier", "ReviewClassifier"},
		"runtime policy": {"runtime policy", "runtime policy knob", "RuntimePolicy", "RuntimeWorkflowPolicy"},
	} {
		for _, mutation := range mutations {
			if violations := prohibitedWorkflowControlTokens(mutation); len(violations) != 1 {
				t.Errorf("%s mutation %q produced violations %v", class, mutation, violations)
			}
		}
	}
}

type workflowControlSurfaces struct {
	configuration string
	production    map[string]string
	templates     map[string]string
}

func workflowControlCensus(surfaces workflowControlSurfaces) []string {
	var out []string
	collect := func(surface, body string) {
		for _, token := range prohibitedWorkflowControlTokens(body) {
			out = append(out, surface+": "+token)
		}
	}
	collect("configuration", surfaces.configuration)
	for path, body := range surfaces.production {
		collect("production "+path, body)
	}
	for path, body := range surfaces.templates {
		collect("template "+path, body)
	}
	sort.Strings(out)
	return out
}

func prohibitedWorkflowControlTokens(surface string) []string {
	words := strings.FieldsFunc(surface, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	seen := map[string]bool{}
	for _, word := range words {
		word = strings.ToLower(word)
		for _, token := range []string{"governanceprofile", "workflowprofile", "reviewdepth", "workflowdepth", "depthcontrol", "workflowrouter", "reviewrouter", "workflowclassifier", "reviewclassifier", "runtimepolicy", "runtimeworkflowpolicy"} {
			if strings.Contains(word, token) {
				seen[token] = true
			}
		}
	}
	for token, pattern := range map[string]string{
		"governanceprofile":  `(?i)\bgovernance[\s_-]+profiles?\b`,
		"workflowprofile":    `(?i)\bworkflow[\s_-]+profiles?\b`,
		"reviewdepth":        `(?i)\breview[\s_-]+depth\b`,
		"workflowdepth":      `(?i)\bworkflow[\s_-]+depth\b`,
		"depthcontrol":       `(?i)\bdepth[\s_-]+controls?\b`,
		"workflowrouter":     `(?i)\bworkflow[\s_-]+routers?\b`,
		"reviewrouter":       `(?i)\breview[\s_-]+routers?\b`,
		"workflowclassifier": `(?i)\bworkflow[\s_-]+classifiers?\b`,
		"reviewclassifier":   `(?i)\breview[\s_-]+classifiers?\b`,
		"runtimepolicy":      `(?i)\bruntime[\s_-]+polic(?:y|ies)\b`,
	} {
		if regexp.MustCompile(pattern).MatchString(surface) {
			seen[token] = true
		}
	}
	out := make([]string, 0, len(seen))
	for token := range seen {
		out = append(out, token)
	}
	sort.Strings(out)
	return out
}

// invariant: rendering/guide-and-doc-templates:maintainable-code-design-guide (TestMaintainableCodeDesignGuide)
func TestMaintainableCodeDesignGuide(t *testing.T) {
	entry, ok := catalog.Standard.Docs["maintainable-code-design"]
	if !ok {
		t.Fatal("maintainable-code-design catalog entry missing")
	}
	wantSections := []string{"decision-posture", "contextual-heuristics", "semantic-modeling", "readability", "boundaries-and-dependencies", "pattern-toolbox", "preparatory-refactoring", "failure-modes"}
	if !entry.Mandatory || !entry.DocumentMap || entry.Title != "Maintainable Code Design" || entry.Desc != "decision framework for cohesive models, explicit boundaries, dependencies, refactoring, and testable design" || entry.Path != "maintainable-code-design.md" || entry.TemplateKey != "maintainableCodeDesign" || entry.TID != "docs/maintainable-code-design.md.tmpl" || strings.Join(entry.Sections, ",") != strings.Join(wantSections, ",") {
		t.Errorf("catalog entry = %#v, want mandatory document-map guide with sections %v", entry, wantSections)
	}

	src, err := fs.ReadFile(templates.FS, entry.TID)
	if err != nil {
		t.Fatalf("read %s: %v", entry.TID, err)
	}
	var markers []string
	for _, section := range render.ParseSections(string(src)) {
		if section.IsSection {
			markers = append(markers, section.Name)
		}
	}
	if strings.Join(markers, ",") != strings.Join(wantSections, ",") {
		t.Errorf("markers = %v, want %v", markers, wantSections)
	}

	out := renderGolden(t, entry.TID, map[string]any{"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}})
	assertNoLeaks(t, out)
	for _, want := range append([]string{"# Maintainable Code Design", "SOLID", "DRY", "YAGNI", "Strategy", "Adapter", "without mechanically adding wrapper types", "Make the simplest sufficient solution the default", "Added abstraction, indirection, validation, test machinery, tooling, cleanup, or process must be justified by", "requested behavior", "reproduced defect", "existing documented contract", "clearly applicable project invariant", "Generic robustness, hypothetical future use, and the mere possibility of doing more are insufficient"}, append([]string{"## Decision posture", "## SOLID, DRY, and YAGNI", "## Semantic modeling", "## Readability", "## Boundaries and dependency direction", "## Illustrative pattern toolbox", "## Preparatory refactoring", "## Failure modes"}, []string{"perform it first", "include it in the current effort", "defer it in a durable project-owned record", "decline it with the trade-off stated"}...)...) {
		if !strings.Contains(out, want) {
			t.Errorf("guide missing %q:\n%s", want, out)
		}
	}
	for _, forbidden := range []string{"./x", "github.com/hypnotox/agentic-workflows", "internal/", ".go", "Go package", "<no value>", "{{", "}}", "awf:section", "awf:end"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("guide leaked %q:\n%s", forbidden, out)
		}
	}

	root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/maintainable-code-design.md")); err != nil {
		t.Errorf("maintainable-code guide not written: %v", err)
	}
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	line := "- **Maintainable Code Design:** [docs/maintainable-code-design.md](docs/maintainable-code-design.md), decision framework for cohesive models, explicit boundaries, dependencies, refactoring, and testable design"
	if !strings.Contains(string(agents), line) {
		t.Errorf("AGENTS.md missing mandatory guide document-map entry %q:\n%s", line, agents)
	}
}

// invariant: rendering/catalog-and-targets:adr-singleton-section-parity (TestAdrSingletonSectionParity)
func TestAdrSingletonSectionParity(t *testing.T) {
	cat := catalog.Standard
	lay := testLayout()
	for _, sg := range plainSingletons {
		src, err := fs.ReadFile(templates.FS, sg.tid)
		if err != nil {
			t.Fatalf("read %s: %v", sg.tid, err)
		}
		var markers []string
		for _, s := range render.ParseSections(string(src)) {
			if s.IsSection {
				markers = append(markers, s.Name)
			}
		}
		wantSections := sg.sections(cat)
		if strings.Join(markers, ",") != strings.Join(wantSections, ",") {
			t.Errorf("%s markers %v != catalog sections %v", sg.tid, markers, wantSections)
		}
		asm, parts := render.Assemble(render.ParseSections(string(src)), nil, render.HTMLComment)
		out, err := render.Execute(asm, map[string]any{
			"prefix": "awf", "vars": map[string]any{}, "layout": lay, "data": map[string]any{}}, parts, "test")
		if err != nil {
			t.Fatalf("render %s: %v", sg.tid, err)
		}
		if strings.Contains(out, "<no value>") {
			t.Errorf("%s: <no value> leaked", sg.tid)
		}
	}
}
