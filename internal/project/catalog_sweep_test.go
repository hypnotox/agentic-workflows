package project

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"text/template"
	"text/template/parse"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/configspec"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// skillRefRe matches a rendered example-prefixed skill reference. Greedy, so
// the longest hyphenated token wins (an overlapping local fixture never
// reports a nested shorter skill token).
var skillRefRe = regexp.MustCompile(`example-[a-z][a-z-]*[a-z]`)

// doubleBacktickRe matches a double backtick not adjacent to a third - an
// empty inline-code span or a literal double-backtick quoting span, never a
// triple-backtick code fence. (Spelled out because gofmt rewrites a literal
// double-backtick pair in a doc comment into a curly quote.)
var doubleBacktickRe = regexp.MustCompile("(^|[^`])``([^`]|$)")

// doubleBacktickExempt lists templates whose double-backtick spans are
// deliberate; entries fail when stale (ADR-0080 Decision 7). No skill or agent
// template renders a double-backtick span under the current-state authority
// model; the map stays declared so a future deliberate span registers here.
var doubleBacktickExempt = map[string]bool{}

// TestCatalogTemplatesDegradeLeakFree renders every catalog skill and agent
// template under empty adopter data (full awf-given layout, RequiresDoc doc
// seeded) and fails on leak residue, on skill-reference residue outside the
// artifact's RequiresSkills declaration, and on stale declarations or
// exemptions. The artifact set derives from catalog.Standard, never a hand
// list (ADR-0080).
// invariant: rendering/templates:catalog-template-sweep (TestCatalogTemplatesDegradeLeakFree)
func TestCatalogTemplatesDegradeLeakFree(t *testing.T) {
	assertV3ADRTemplatePublicationSafe(t)
	cat := catalog.Standard
	sweep := func(tid, requiresDoc string) {
		t.Run(tid, func(t *testing.T) {
			layout := testLayout()
			if requiresDoc != "" {
				layout["docs"] = map[string]any{requiresDoc: "docs/" + requiresDoc + ".md"}
			}
			data := map[string]any{
				"prefix": "example",
				"vars":   map[string]any{},
				"data":   map[string]any{},
				"skills": map[string]bool{},
				"layout": layout,
			}
			out := renderGolden(t, tid, data)
			// Declarations are exact: undeclared reference residue and stale
			// RequiresSkills entries both fail (ADR-0080 Decision 2).
			// invariant: rendering/catalog-and-targets:requires-skills-exact (TestCatalogTemplatesDegradeLeakFree)
			found := map[string]bool{}
			for _, m := range skillRefRe.FindAllString(out, -1) {
				name := strings.TrimPrefix(m, "example-")
				if _, ok := cat.Skills[name]; !ok {
					continue // prose or section-name token, not a skill reference
				}
				found[name] = true
				// Workflow-profile neighbors are advisory and are not structural
				// requirements. Only artifact references declared in RequiresSkills
				// are checked by the catalog sweep.
			}
			// Standard workflow relationships are intentionally not required to
			// appear as unconditional rendered references.
			hasDouble := doubleBacktickRe.MatchString(out)
			if hasDouble && !doubleBacktickExempt[tid] {
				t.Errorf("double-backtick span rendered under empty data - fix the template or add a doubleBacktickExempt entry:\n%s", out)
			}
			if !hasDouble && doubleBacktickExempt[tid] {
				t.Errorf("stale doubleBacktickExempt entry - the template no longer renders a double-backtick span")
			}
		})
	}
	for name, spec := range cat.Skills {
		sweep(fmt.Sprintf("skills/%s/SKILL.md.tmpl", name), spec.RequiresDoc)
	}
	for name := range cat.Agents {
		sweep(fmt.Sprintf("agents/%s.md.tmpl", name), "")
	}
}

// conditionalActionRe matches any template conditional carrying fallback
// prose: if, with, or range actions (with/else is the dominant form).
var conditionalActionRe = regexp.MustCompile(`\{\{-?\s*(if|with|range)\b`)

// TestConditionalTemplatesHaveFallbackCases requires a hand-authored
// unset-data case for every catalog template whose post-include-expansion
// source contains a conditional action - only a human knows what the degraded
// prose should say, so its presence is machine-forced (ADR-0080 Decision 3).
// invariant: rendering/templates:conditional-fallback-case-guard (TestConditionalTemplatesHaveFallbackCases)
func TestConditionalTemplatesHaveFallbackCases(t *testing.T) {
	covered := map[string]bool{}
	for _, tc := range unsetFallbackCases {
		covered[tc.tmpl] = true
	}
	check := func(tid string) {
		src, err := fs.ReadFile(templates.FS, tid)
		if err != nil {
			t.Fatalf("read %s: %v", tid, err)
		}
		expanded, err := render.ExpandIncludes(string(src), templates.FS)
		if err != nil {
			t.Fatalf("expand %s: %v", tid, err)
		}
		if conditionalActionRe.MatchString(expanded) && !covered[tid] {
			t.Errorf("%s has conditional fallback prose but no unsetFallbackCases entry - add a hand-authored case pinning its degraded output", tid)
		}
	}
	for name := range catalog.Standard.Skills {
		check(fmt.Sprintf("skills/%s/SKILL.md.tmpl", name))
	}
	for name := range catalog.Standard.Agents {
		check(fmt.Sprintf("agents/%s.md.tmpl", name))
	}
}

type singletonTemplateContext struct {
	tid          string
	data         map[string]any
	dataArtifact string
}

type singletonConditional struct {
	id        int
	kind      string
	pipe      string
	paths     [][]string
	literals  []string
	ancestors []conditionalState
}

type conditionalState struct {
	condition singletonConditional
	truth     bool
}

type singletonInspection struct {
	template   *template.Template
	conditions []singletonConditional
}

var supportedConditionalFuncs = map[string]bool{
	"and": true, "eq": true, "ge": true, "gt": true, "le": true,
	"lt": true, "ne": true, "not": true, "or": true,
}

func cloneTemplateValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, nested := range value {
			out[key] = cloneTemplateValue(nested)
		}
		return out
	case map[string]bool:
		out := make(map[string]bool, len(value))
		for key, nested := range value {
			out[key] = nested
		}
		return out
	case []any:
		out := make([]any, len(value))
		for i, nested := range value {
			out[i] = cloneTemplateValue(nested)
		}
		return out
	default:
		return value
	}
}

func cloneRenderData(in map[string]any) map[string]any {
	return cloneTemplateValue(in).(map[string]any)
}

func singletonTemplateContexts(t *testing.T, p *Project, eff map[string]bool) []singletonTemplateContext {
	t.Helper()
	var contexts []singletonTemplateContext
	for _, kind := range catalog.SingletonKinds() {
		entry := p.Cat.Docs[kind]
		sc, err := p.Cfg.Sidecar(kind, "")
		if err != nil {
			t.Fatalf("read %s sidecar: %v", kind, err)
		}
		sc = withDefaultData(sc, entry.Data)
		data := p.data(sc, eff)
		switch {
		case entry.AgentsDoc:
			data["docs"] = p.resolvedDocs()
			data["mandatoryDocs"] = p.documentMapDocs()
			data["localDocs"] = p.localDocumentMapDocs()
		case entry.Generated:
			files, err := p.RenderAll()
			if err != nil {
				t.Fatal(err)
			}
			collections, err := p.configReferenceData(files)
			if err != nil {
				t.Fatal(err)
			}
			data["data"] = collections
		}
		contexts = append(contexts, singletonTemplateContext{tid: entry.TID, data: data, dataArtifact: kind})
	}
	for _, unit := range conditionalUnits() {
		contexts = append(contexts, singletonTemplateContext{tid: unit.tid, data: p.data(config.Sidecar{}, eff)})
	}
	return contexts
}

func appendUniquePaths(dst [][]string, paths ...[]string) [][]string {
	for _, path := range paths {
		key, found := strings.Join(path, "."), false
		for _, current := range dst {
			if strings.Join(current, ".") == key {
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, path)
		}
	}
	return dst
}

func conditionalNodePaths(node parse.Node, scope []string, vars map[string][][]string) ([][]string, error) {
	switch node := node.(type) {
	case *parse.FieldNode:
		return [][]string{append(append([]string{}, scope...), node.Ident...)}, nil
	case *parse.VariableNode:
		if len(node.Ident) == 0 {
			return nil, nil
		}
		if node.Ident[0] == "$" {
			if len(node.Ident) == 1 {
				return nil, nil
			}
			return [][]string{append([]string{}, node.Ident[1:]...)}, nil
		}
		base := vars[node.Ident[0]]
		var out [][]string
		for _, path := range base {
			out = appendUniquePaths(out, append(append([]string{}, path...), node.Ident[1:]...))
		}
		return out, nil
	case *parse.ChainNode:
		base, err := conditionalNodePaths(node.Node, scope, vars)
		if err != nil {
			return nil, err
		}
		var out [][]string
		for _, path := range base {
			out = appendUniquePaths(out, append(append([]string{}, path...), node.Field...))
		}
		return out, nil
	case *parse.PipeNode:
		var out [][]string
		for _, command := range node.Cmds {
			paths, err := conditionalNodePaths(command, scope, vars)
			if err != nil {
				return nil, err
			}
			out = appendUniquePaths(out, paths...)
		}
		return out, nil
	case *parse.CommandNode:
		var out [][]string
		for _, arg := range node.Args {
			if identifier, ok := arg.(*parse.IdentifierNode); ok {
				if !supportedConditionalFuncs[identifier.Ident] {
					return nil, fmt.Errorf("unsupported conditional function %q", identifier.Ident)
				}
				continue
			}
			paths, err := conditionalNodePaths(arg, scope, vars)
			if err != nil {
				return nil, err
			}
			out = appendUniquePaths(out, paths...)
		}
		return out, nil
	case *parse.DotNode, *parse.BoolNode, *parse.NilNode, *parse.NumberNode, *parse.StringNode:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported conditional node %T", node)
	}
}

func inspectSingletonConditionals(src, name string) (singletonInspection, error) {
	tmpl, err := template.New(name).Option("missingkey=zero").Parse(src)
	if err != nil {
		return singletonInspection{}, err
	}
	vars := map[string][][]string{}
	var found []singletonConditional
	addCondition := func(kind string, branch *parse.BranchNode, paths [][]string, ancestors []conditionalState) singletonConditional {
		id := len(found)
		var literals []string
		for _, command := range branch.Pipe.Cmds {
			for _, arg := range command.Args {
				if value, ok := arg.(*parse.StringNode); ok && value.Text != "" {
					literals = append(literals, value.Text)
				}
			}
		}
		trueMarker := &parse.TextNode{NodeType: parse.NodeText, Text: []byte(fmt.Sprintf("AWF_CONDITION_%d_TRUE", id))}
		falseMarker := &parse.TextNode{NodeType: parse.NodeText, Text: []byte(fmt.Sprintf("AWF_CONDITION_%d_FALSE", id))}
		branch.List.Nodes = append([]parse.Node{trueMarker}, branch.List.Nodes...)
		if branch.ElseList == nil {
			branch.ElseList = &parse.ListNode{NodeType: parse.NodeList, Nodes: []parse.Node{falseMarker}}
		} else {
			branch.ElseList.Nodes = append([]parse.Node{falseMarker}, branch.ElseList.Nodes...)
		}
		condition := singletonConditional{id: id, kind: kind, pipe: branch.Pipe.String(), paths: paths, literals: literals, ancestors: append([]conditionalState{}, ancestors...)}
		found = append(found, condition)
		return condition
	}
	var walkList func(*parse.ListNode, []string, []conditionalState) error
	walkList = func(list *parse.ListNode, scope []string, ancestors []conditionalState) error {
		if list == nil {
			return nil
		}
		for _, node := range list.Nodes {
			switch node := node.(type) {
			case *parse.TextNode, *parse.CommentNode, *parse.BreakNode, *parse.ContinueNode:
				continue
			case *parse.ActionNode:
				paths, pathErr := conditionalNodePaths(node.Pipe, scope, vars)
				if pathErr != nil {
					return pathErr
				}
				for _, declaration := range node.Pipe.Decl {
					vars[declaration.Ident[0]] = appendUniquePaths(vars[declaration.Ident[0]], paths...)
				}
			case *parse.IfNode:
				paths, pathErr := conditionalNodePaths(node.Pipe, scope, vars)
				if pathErr != nil {
					return pathErr
				}
				if len(paths) == 0 {
					return fmt.Errorf("if conditional %q has no render-context path", node.Pipe.String())
				}
				condition := addCondition("if", &node.BranchNode, paths, ancestors)
				if err := walkList(node.List, scope, append(ancestors, conditionalState{condition: condition, truth: true})); err != nil {
					return err
				}
				if err := walkList(node.ElseList, scope, append(ancestors, conditionalState{condition: condition, truth: false})); err != nil {
					return err
				}
			case *parse.WithNode:
				paths, pathErr := conditionalNodePaths(node.Pipe, scope, vars)
				if pathErr != nil {
					return pathErr
				}
				if len(paths) != 1 {
					return fmt.Errorf("with conditional %q has %d render-context paths", node.Pipe.String(), len(paths))
				}
				condition := addCondition("with", &node.BranchNode, paths, ancestors)
				if err := walkList(node.List, paths[0], append(ancestors, conditionalState{condition: condition, truth: true})); err != nil {
					return err
				}
				if err := walkList(node.ElseList, scope, append(ancestors, conditionalState{condition: condition, truth: false})); err != nil {
					return err
				}
			case *parse.RangeNode:
				paths, pathErr := conditionalNodePaths(node.Pipe, scope, vars)
				if pathErr != nil {
					return pathErr
				}
				if len(paths) != 1 {
					return fmt.Errorf("range conditional %q has %d render-context paths", node.Pipe.String(), len(paths))
				}
				condition := addCondition("range", &node.BranchNode, paths, ancestors)
				itemScope := append(append([]string{}, paths[0]...), "*")
				if err := walkList(node.List, itemScope, append(ancestors, conditionalState{condition: condition, truth: true})); err != nil {
					return err
				}
				if err := walkList(node.ElseList, scope, append(ancestors, conditionalState{condition: condition, truth: false})); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported template list node %T", node)
			}
		}
		return nil
	}
	if len(tmpl.Templates()) != 1 {
		return singletonInspection{}, fmt.Errorf("inspect %s: named templates are unsupported", name)
	}
	if err := walkList(tmpl.Root, nil, nil); err != nil {
		return singletonInspection{}, fmt.Errorf("inspect %s: %w", name, err)
	}
	instrumented, err := template.New(name).Option("missingkey=zero").Parse(tmpl.Root.String())
	if err != nil {
		return singletonInspection{}, err
	}
	return singletonInspection{template: instrumented, conditions: found}, nil
}

func conditionalPathExists(value any, path []string) bool {
	if len(path) == 0 {
		return true
	}
	if path[0] == "*" {
		reflected := reflect.ValueOf(value)
		for reflected.IsValid() && (reflected.Kind() == reflect.Interface || reflected.Kind() == reflect.Pointer) {
			if reflected.IsNil() {
				return false
			}
			reflected = reflected.Elem()
		}
		if !reflected.IsValid() || (reflected.Kind() != reflect.Slice && reflected.Kind() != reflect.Array) {
			return false
		}
		if reflected.Len() == 0 {
			return true
		}
		for i := range reflected.Len() {
			if conditionalPathExists(reflected.Index(i).Interface(), path[1:]) {
				return true
			}
		}
		return false
	}
	reflected := reflect.ValueOf(value)
	for reflected.IsValid() && (reflected.Kind() == reflect.Interface || reflected.Kind() == reflect.Pointer) {
		if reflected.IsNil() {
			return false
		}
		reflected = reflected.Elem()
	}
	if !reflected.IsValid() {
		return false
	}
	switch reflected.Kind() {
	case reflect.Map:
		key := reflect.ValueOf(path[0])
		if !key.Type().AssignableTo(reflected.Type().Key()) {
			return false
		}
		next := reflected.MapIndex(key)
		return next.IsValid() && conditionalPathExists(next.Interface(), path[1:])
	case reflect.Struct:
		for i := range reflected.NumField() {
			field := reflected.Type().Field(i)
			if strings.EqualFold(field.Name, path[0]) {
				return conditionalPathExists(reflected.Field(i).Interface(), path[1:])
			}
		}
	default:
		return false
	}
	return false
}

func conditionalPathUsesLiveContext(data map[string]any, dataArtifact string, path []string) bool {
	if len(path) > 1 && path[0] == "vars" {
		for _, descriptor := range catalog.Standard.Vars {
			if descriptor.Key == path[1] {
				return len(path) == 2
			}
		}
		return false
	}
	if len(path) > 1 && path[0] == "data" {
		for _, descriptor := range configspec.DataKeys() {
			if descriptor.Artifact != dataArtifact || descriptor.Key != path[1] {
				continue
			}
			if len(path) == 2 {
				return true
			}
			if len(path) != 4 || path[2] != "*" {
				return false
			}
			for _, field := range descriptor.Fields {
				if field == path[3] {
					return true
				}
			}
			return false
		}
		return conditionalPathExists(data, path)
	}
	return conditionalPathExists(data, path)
}

func setConditionalPath(value any, path []string, kind string, set bool, literal string) any {
	if len(path) == 0 {
		if literal != "" {
			return literal
		}
		if kind == "range" {
			if !set {
				return []any{}
			}
			if current, ok := value.([]any); ok && len(current) != 0 {
				return current
			}
			return []any{map[string]any{}}
		}
		switch current := value.(type) {
		case bool:
			return set
		case string:
			if set {
				return "fixture-value"
			}
			return ""
		case []any:
			if set {
				if len(current) == 0 {
					return []any{map[string]any{}}
				}
				return current
			}
			return []any{}
		case nil:
			if set {
				return "fixture-value"
			}
			return nil
		default:
			if set {
				return current
			}
			return nil
		}
	}
	if path[0] == "*" {
		items, _ := value.([]any)
		if len(items) == 0 {
			items = []any{map[string]any{}}
		}
		for i := range items {
			items[i] = setConditionalPath(items[i], path[1:], kind, set, literal)
		}
		return items
	}
	mapping, _ := value.(map[string]any)
	if mapping == nil {
		mapping = map[string]any{}
	}
	mapping[path[0]] = setConditionalPath(mapping[path[0]], path[1:], kind, set, literal)
	return mapping
}

func applyConditionalState(data map[string]any, condition singletonConditional, truth bool) {
	set, literal := truth, ""
	switch {
	case strings.HasPrefix(condition.pipe, "not "):
		set = !truth
	case strings.HasPrefix(condition.pipe, "eq "):
		if truth && len(condition.literals) != 0 {
			literal = condition.literals[0]
		} else {
			set = false
		}
	case strings.HasPrefix(condition.pipe, "ne "):
		if !truth && len(condition.literals) != 0 {
			literal = condition.literals[0]
		} else {
			set = true
		}
	}
	for _, path := range condition.paths {
		data[path[0]] = setConditionalPath(data[path[0]], path[1:], condition.kind, set, literal)
	}
}

func renderConditionalFixture(t *testing.T, tmpl *template.Template, data map[string]any) string {
	t.Helper()
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// TestSingletonConditionalKeysUseLiveRenderContext derives every catalog and
// config-tree singleton from its owning declaration, inspects its expanded Go
// template parse tree, and exercises both values for every referenced context
// path. Historical recognition-only templates never enter either declaration.
// invariant: rendering/templates:singleton-conditional-key-live (TestSingletonConditionalKeysUseLiveRenderContext)
func TestSingletonConditionalKeysUseLiveRenderContext(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, eff, err := p.deriveOperationStateWithPitfalls()
	if err != nil {
		t.Fatal(err)
	}
	seenTemplates, seenConditions := 0, 0
	for _, context := range singletonTemplateContexts(t, p, eff) {
		raw, err := fs.ReadFile(templates.FS, context.tid)
		if err != nil {
			t.Fatal(err)
		}
		expanded, err := render.ExpandIncludes(string(raw), templates.FS)
		if err != nil {
			t.Fatal(err)
		}
		baselineTemplate, err := template.New(context.tid).Option("missingkey=zero").Parse(expanded)
		if err != nil {
			t.Fatal(err)
		}
		if baseline := renderConditionalFixture(t, baselineTemplate, context.data); strings.Contains(baseline, "<no value>") {
			t.Errorf("%s real singleton fallback rendered missing-value residue", context.tid)
		}
		inspection, err := inspectSingletonConditionals(expanded, context.tid)
		if err != nil {
			t.Error(err)
			continue
		}
		if len(inspection.conditions) == 0 {
			continue
		}
		seenTemplates++
		variants := []map[string]any{cloneRenderData(context.data)}
		for _, condition := range inspection.conditions {
			for _, truth := range []bool{false, true} {
				data := cloneRenderData(context.data)
				for _, ancestor := range condition.ancestors {
					applyConditionalState(data, ancestor.condition, ancestor.truth)
				}
				applyConditionalState(data, condition, truth)
				variants = append(variants, data)
			}
			seenConditions++
			for _, path := range condition.paths {
				if !conditionalPathUsesLiveContext(context.data, context.dataArtifact, path) {
					t.Errorf("%s %s conditional path %s has no root on its real render context", context.tid, condition.kind, strings.Join(path, "."))
					continue
				}
				for _, candidate := range []struct {
					set     bool
					literal string
				}{{}, {set: true}} {
					data := cloneRenderData(context.data)
					data[path[0]] = setConditionalPath(data[path[0]], path[1:], condition.kind, candidate.set, candidate.literal)
					variants = append(variants, data)
				}
				for _, literal := range condition.literals {
					data := cloneRenderData(context.data)
					data[path[0]] = setConditionalPath(data[path[0]], path[1:], condition.kind, true, literal)
					variants = append(variants, data)
				}
			}
		}
		outcomes := map[string]bool{}
		for _, data := range variants {
			out := renderConditionalFixture(t, inspection.template, data)
			for _, condition := range inspection.conditions {
				for _, outcome := range []string{"TRUE", "FALSE"} {
					marker := fmt.Sprintf("AWF_CONDITION_%d_%s", condition.id, outcome)
					outcomes[marker] = outcomes[marker] || strings.Contains(out, marker)
				}
			}
		}
		for _, condition := range inspection.conditions {
			for _, outcome := range []string{"TRUE", "FALSE"} {
				marker := fmt.Sprintf("AWF_CONDITION_%d_%s", condition.id, outcome)
				if !outcomes[marker] {
					t.Errorf("%s %s conditional %d paths=%v literals=%v never exercised its %s outcome", context.tid, condition.kind, condition.id, condition.paths, condition.literals, strings.ToLower(outcome))
				}
			}
		}
	}
	if seenTemplates == 0 || seenConditions == 0 {
		t.Fatalf("conditional singleton census was vacuous: templates=%d conditions=%d", seenTemplates, seenConditions)
	}
}

func TestSingletonConditionalInspectionRejectsUnsupportedForms(t *testing.T) {
	fixtures := []struct {
		name string
		src  string
		want string
	}{
		{"wrapped-function", `{{ if print .vars.gateCmd }}configured{{ else }}fallback{{ end }}`, `unsupported conditional function "print"`},
		{"named-template", `{{ define "wrapped" }}{{ if .vars.gateCmd }}configured{{ end }}{{ end }}{{ template "wrapped" . }}`, "named templates are unsupported"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			if _, err := inspectSingletonConditionals(fixture.src, fixture.name); err == nil || !strings.Contains(err.Error(), fixture.want) {
				t.Fatalf("unsupported conditional form was not rejected: %v", err)
			}
		})
	}
}

func TestSingletonConditionalInspectionRejectsMissingContextDescendant(t *testing.T) {
	fixtures := []struct {
		name         string
		src          string
		dataArtifact string
		data         map[string]any
	}{
		{name: "missing-var", src: `{{ if .vars.reviewMissingKey }}configured{{ else }}fallback{{ end }}`},
		{name: "non-record-descendant", src: `{{ with .data.commands }}{{ if .reviewMissingKey }}configured{{ end }}{{ end }}`, dataArtifact: "agents-doc"},
		{name: "missing-record-field-empty-list", src: `{{ range .data.commands }}{{ if .reviewMissingKey }}configured{{ end }}{{ end }}`, dataArtifact: "agents-doc", data: map[string]any{"commands": []any{}}},
		{name: "missing-record-field-heterogeneous", src: `{{ range .data.commands }}{{ if .reviewMissingKey }}configured{{ end }}{{ end }}`, dataArtifact: "agents-doc", data: map[string]any{"commands": []any{map[string]any{"cmd": "one"}, map[string]any{"reviewMissingKey": true}}}},
		{name: "other-artifact-data", src: `{{ if .data.adrSections }}configured{{ else }}fallback{{ end }}`, dataArtifact: "agents-doc"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			inspection, err := inspectSingletonConditionals(fixture.src, fixture.name)
			if err != nil {
				t.Fatal(err)
			}
			path := inspection.conditions[len(inspection.conditions)-1].paths[0]
			if conditionalPathUsesLiveContext(map[string]any{"vars": map[string]any{}, "data": fixture.data}, fixture.dataArtifact, path) {
				t.Fatalf("missing render-context descendant was accepted: %s", strings.Join(path, "."))
			}
		})
	}
}

// kebabToCamel converts a kebab-case artifact name to its test-func stem
// ("subagent-driven-development" → "SubagentDrivenDevelopment").
func kebabToCamel(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}

// TestEveryCatalogArtifactHasGoldenTest asserts a per-artifact golden test
// func exists in this package's test source for every catalog skill and
// agent - the goldens live in spine_test.go by convention (source-scan
// mechanic, precedent TestArchitectureDocNamesEveryCmd; ADR-0080 Decision 4).
// invariant: rendering/templates:golden-test-completeness (TestEveryCatalogArtifactHasGoldenTest)
func TestEveryCatalogArtifactHasGoldenTest(t *testing.T) {
	src, err := os.ReadFile("spine_test.go")
	if err != nil {
		t.Fatalf("read spine_test.go: %v", err)
	}
	for name := range catalog.Standard.Skills {
		if needle := "func Test" + kebabToCamel(name) + "Template("; !strings.Contains(string(src), needle) {
			t.Errorf("no golden test for skill %q - add %s to internal/project/spine_test.go", name, needle)
		}
	}
	for name := range catalog.Standard.Agents {
		if needle := "func Test" + kebabToCamel(name) + "Agent("; !strings.Contains(string(src), needle) {
			t.Errorf("no golden test for agent %q - add %s to internal/project/spine_test.go", name, needle)
		}
	}
}

// goldenFuncRe matches a golden-shaped test declaration in spine_test.go:
// Test<Stem>Template or Test<Stem>Agent with the suffix directly before the
// parenthesis (TestAgentsDocTemplateConfigDriven is not golden-shaped).
var goldenFuncRe = regexp.MustCompile(`func Test([A-Za-z0-9]+)(Template|Agent)\(`)

// nonArtifactGoldens lists the golden-shaped Template test stems in
// spine_test.go that test non-catalog artifacts (doc singletons); entries
// fail when stale (ADR-0080 Decision 7).
var nonArtifactGoldens = map[string]bool{
	"DocArchitecture":   true,
	"Glossary":          true,
	"RoadmapGraduation": true,
}

// TestNoOrphanGoldenTest is the reverse of TestEveryCatalogArtifactHasGoldenTest:
// every golden-shaped test func in spine_test.go must name a current catalog
// artifact, so a golden orphaned by a catalog removal fails here even while
// its lingering .tmpl file keeps it rendering.
func TestNoOrphanGoldenTest(t *testing.T) {
	src, err := os.ReadFile("spine_test.go")
	if err != nil {
		t.Fatalf("read spine_test.go: %v", err)
	}
	skills, agents := map[string]bool{}, map[string]bool{}
	for name := range catalog.Standard.Skills {
		skills[kebabToCamel(name)] = true
	}
	for name := range catalog.Standard.Agents {
		agents[kebabToCamel(name)] = true
	}
	seenExempt := map[string]bool{}
	for _, m := range goldenFuncRe.FindAllStringSubmatch(string(src), -1) {
		stem, kind := m[1], m[2]
		switch {
		case kind == "Template" && nonArtifactGoldens[stem]:
			seenExempt[stem] = true
		case kind == "Template" && !skills[stem]:
			t.Errorf("orphan golden Test%sTemplate: no catalog skill matches - remove it or list it in nonArtifactGoldens", stem)
		case kind == "Agent" && !agents[stem]:
			t.Errorf("orphan golden Test%sAgent: no catalog agent matches - remove it", stem)
		}
	}
	for stem := range nonArtifactGoldens {
		if !seenExempt[stem] {
			t.Errorf("stale nonArtifactGoldens entry %q: no such golden-shaped func in spine_test.go", stem)
		}
	}
}
