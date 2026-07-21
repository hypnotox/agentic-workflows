package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/templates"
)

// ProjectTreeReader is the read-only input authority for output declarations.
type ProjectTreeReader interface {
	ReadFile(path string) ([]byte, bool)
	Paths(prefix string) []string
}

type OutputInput struct {
	Path string
	Role ArtifactRole
}
type OutputDeclaration struct {
	Path        string
	TemplateID  string
	Declarers   []string
	Inputs      []OutputInput
	Reservation bool
}

type snapshotTreeReader struct{ tree *snapshot.Tree }

func (r snapshotTreeReader) ReadFile(path string) ([]byte, bool) {
	f, ok := r.tree.Lookup(filepath.ToSlash(path))
	if !ok || !f.Scannable() {
		return nil, false
	}
	return slices.Clone(f.Bytes), true
}
func (r snapshotTreeReader) Paths(prefix string) []string {
	out := []string{}
	prefix = filepath.ToSlash(prefix)
	for _, f := range r.tree.List() {
		if f.Scannable() && strings.HasPrefix(f.Path, prefix) {
			out = append(out, f.Path)
		}
	}
	return out
}

type filesystemProjectReader struct{ root string }

func (r filesystemProjectReader) ReadFile(path string) ([]byte, bool) {
	b, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(path)))
	if err != nil {
		return nil, false
	}
	return slices.Clone(b), true
}
func (r filesystemProjectReader) Paths(prefix string) []string {
	out := []string{}
	_ = filepath.WalkDir(filepath.Join(r.root, filepath.FromSlash(prefix)), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return fs.SkipAll
		}
		if !d.IsDir() {
			if rel, e := filepath.Rel(r.root, p); e == nil {
				out = append(out, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	slices.Sort(out)
	return out
}

// BuildOutputDeclarations enumerates deterministic producer declarations without
// rendering or materializing the selected tree.
func BuildOutputDeclarations(cfg *config.Config, cat *catalog.Catalog, targets []Target, read ProjectTreeReader) ([]OutputDeclaration, error) {
	decls := []OutputDeclaration{}
	add := func(path, tid, who string, inputs []OutputInput, reservation bool) {
		if path == "" {
			return
		}
		for i := range decls {
			if decls[i].Path == path && decls[i].TemplateID == tid && decls[i].Reservation == reservation {
				decls[i].Declarers = append(decls[i].Declarers, who)
				decls[i].Inputs = append(decls[i].Inputs, inputs...)
				return
			}
		}
		decls = append(decls, OutputDeclaration{Path: filepath.ToSlash(path), TemplateID: tid, Declarers: []string{who}, Inputs: inputs, Reservation: reservation})
	}
	configInput := []OutputInput{{Path: ".awf/config.yaml", Role: ArtifactConfig}}
	inputs := func(tid string, authored ...OutputInput) []OutputInput {
		out := slices.Clone(configInput)
		if tid != "" {
			out = append(out, OutputInput{Path: "templates/" + tid, Role: ArtifactTemplate})
		}
		for _, in := range authored {
			if _, ok := read.ReadFile(in.Path); ok {
				out = append(out, in)
			}
		}
		return out
	}
	partInputs := func(kind, name string, sections []string) []OutputInput {
		out := []OutputInput{}
		for _, section := range sections {
			var p string
			if config.IsSingletonKind(kind) {
				p = ".awf/parts/" + kind + "/" + section + ".md"
			} else {
				p = ".awf/" + kind + "/parts/" + name + "/" + section + ".md"
			}
			if _, ok := read.ReadFile(p); ok {
				out = append(out, OutputInput{Path: p, Role: ArtifactConventionPart})
			}
		}
		return out
	}
	agentsDoc, err := cfg.Sidecar("agents-doc", "")
	if err != nil {
		return nil, err
	}
	for _, t := range targets {
		for _, name := range cfg.Skills {
			sc, err := cfg.Sidecar("skills", name)
			if err != nil {
				return nil, err
			}
			tid := "skills/" + name + "/SKILL.md.tmpl"
			sections := []string{"content"}
			if spec, ok := cat.Skills[name]; ok {
				sections = spec.Sections
				if spec.Base {
					tid = "skills/_base/SKILL.md.tmpl"
				}
			}
			input := inputs(tid, append([]OutputInput{{Path: ".awf/skills/" + name + ".yaml", Role: ArtifactAuthoredData}}, partInputs("skills", name, sections)...)...)
			declarer := t.Name
			if sc.Local {
				declarer = "skills:" + name
			}
			add(t.SkillPath(cfg.Prefix, name), tid, declarer, input, sc.Local)
		}
		for _, name := range cfg.Agents {
			sc, err := cfg.Sidecar("agents", name)
			if err != nil {
				return nil, err
			}
			tid := "agents/" + name + ".md.tmpl"
			sections := []string{"content"}
			if spec, ok := cat.Agents[name]; ok {
				sections = spec.Sections
				if spec.Base {
					tid = "agents/_base.md.tmpl"
				}
			}
			input := inputs(tid, append([]OutputInput{{Path: ".awf/agents/" + name + ".yaml", Role: ArtifactAuthoredData}}, partInputs("agents", name, sections)...)...)
			declarer := t.Name
			if sc.Local {
				declarer = "agents:" + name
			}
			add(t.AgentPath(name), tid, declarer, input, sc.Local)
		}
		if !agentsDoc.Local {
			add(t.BridgeFile, t.BridgeTemplate, t.BridgeTemplate, inputs(t.BridgeTemplate), false)
		}
		for _, o := range t.Outputs {
			add(o.Path, o.TemplateID, t.Name, inputs(o.TemplateID), false)
		}
	}
	for name, e := range cat.Docs {
		if !e.Mandatory && !slices.Contains(cfg.Docs, name) {
			continue
		}
		sc, err := cfg.Sidecar(func() string {
			if e.Mandatory {
				return name
			}
			return "docs"
		}(), name)
		if err != nil {
			return nil, err
		}
		if _, standard := catalog.Standard.Docs[name]; standard && sc.Local {
			continue
		}
		out := e.Path
		if !e.Mandatory && out == "" {
			out = name + ".md"
		}
		if e.AgentsDoc {
			out = "AGENTS.md"
		} else if out != "" {
			out = strings.TrimRight(cfg.DocsDir, "/") + "/" + out
		}
		sidecarPath := ".awf/" + name + ".yaml"
		if !e.Mandatory {
			sidecarPath = ".awf/docs/" + name + ".yaml"
		}
		authored := []OutputInput{{Path: sidecarPath, Role: ArtifactAuthoredData}}
		authored = append(authored, partInputs(func() string {
			if e.Mandatory {
				return name
			}
			return "docs"
		}(), name, e.Sections)...)
		declarer := e.TID
		if e.Generated {
			declarer = "generated-config-reference"
		}
		add(out, e.TID, declarer, inputs(e.TID, authored...), false)
	}
	for _, d := range cfg.Domains {
		add(strings.TrimRight(cfg.DocsDir, "/")+"/domains/"+d+".md", "domains/domain.md.tmpl", "generated-domain", inputs("domains/domain.md.tmpl", OutputInput{Path: ".awf/domains/" + d + ".yaml", Role: ArtifactAuthoredData}, OutputInput{Path: ".awf/domains/parts/" + d + "/current-state.md", Role: ArtifactConventionPart}), false)
	}
	for _, p := range read.Paths(".awf/topics/metadata/") {
		if !strings.HasSuffix(p, ".yaml") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(p, ".awf/topics/metadata/"), ".yaml")
		add(strings.TrimRight(cfg.DocsDir, "/")+"/topics/"+id+".md", "topics/topic.md.tmpl", "topic:"+id, inputs("topics/topic.md.tmpl", OutputInput{Path: p, Role: ArtifactTopicMetadata}, OutputInput{Path: ".awf/topics/parts/" + id + "/current-state.md", Role: ArtifactClaimPart}), false)
	}
	for _, d := range cfg.Domains {
		topicInputs := []OutputInput{}
		for _, p := range read.Paths(".awf/topics/metadata/" + d + "/") {
			if strings.HasSuffix(p, ".yaml") {
				topicInputs = append(topicInputs, OutputInput{Path: p, Role: ArtifactTopicMetadata})
			}
		}
		if len(topicInputs) > 0 {
			add(strings.TrimRight(cfg.DocsDir, "/")+"/topics/"+d+"/index.md", "topics/index.md.tmpl", "topic-index:"+d, inputs("topics/index.md.tmpl", topicInputs...), false)
		}
	}
	decisionInputs := []OutputInput{}
	for _, p := range read.Paths(strings.TrimRight(cfg.DocsDir, "/") + "/decisions/") {
		if strings.HasSuffix(p, ".md") {
			decisionInputs = append(decisionInputs, OutputInput{Path: p, Role: ArtifactDecisionRecord})
		}
	}
	add(strings.TrimRight(cfg.DocsDir, "/")+"/decisions/INDEX.md", "decisions/index.md.tmpl", "generated-index", inputs("decisions/index.md.tmpl", decisionInputs...), false)
	if cfg.Runner != nil && cfg.Runner.Enabled {
		add("x", "runner/x.tmpl", "runner/x.tmpl", inputs("runner/x.tmpl", OutputInput{Path: "x", Role: ArtifactManagedOutput}), false)
	}
	if cfg.Bootstrap != nil && cfg.Bootstrap.Enabled {
		add(".awf/bootstrap.sh", "bootstrap/awf-bootstrap.sh.tmpl", "bootstrap/awf-bootstrap.sh.tmpl", inputs("bootstrap/awf-bootstrap.sh.tmpl"), false)
		add(".awf/upgrade.sh", "bootstrap/awf-upgrade.sh.tmpl", "bootstrap/awf-upgrade.sh.tmpl", inputs("bootstrap/awf-upgrade.sh.tmpl"), false)
	}
	if cfg.Hooks != nil && cfg.Hooks.Enabled {
		for _, n := range []string{"pre-commit", "commit-msg", "pre-push"} {
			add(".awf/hooks/"+n+".sh", "hooks/"+n+".sh.tmpl", "hooks/"+n+".sh.tmpl", inputs("hooks/"+n+".sh.tmpl"), false)
		}
	}
	add(".awf/memory/.gitignore", "memory/gitignore.tmpl", "memory/gitignore.tmpl", inputs("memory/gitignore.tmpl"), false)
	for i := range decls {
		slices.Sort(decls[i].Declarers)
		decls[i].Declarers = slices.Compact(decls[i].Declarers)
		slices.SortFunc(decls[i].Inputs, func(a, b OutputInput) int {
			if a.Path != b.Path {
				return strings.Compare(a.Path, b.Path)
			}
			return strings.Compare(string(a.Role), string(b.Role))
		})
		decls[i].Inputs = slices.Compact(decls[i].Inputs)
	}
	slices.SortFunc(decls, func(a, b OutputDeclaration) int { return strings.Compare(a.Path, b.Path) })
	return decls, nil
}

// OutputPolicy declares lifecycle behavior for a planned path. It is data on the
// node, not an inference made by sync or check from a template name or suffix.
type OutputPolicy struct {
	ValidateFrontmatter bool
	ScanReferences      bool
	ScanSkillReferences bool
	Regenerate          bool
	LocalValidation     bool
}

// OutputRecipe is the normalized, output-affecting declaration used for
// collision diagnostics and configuration hashes. Target identity is kept on
// OutputNode declarers rather than here, so compatible shared outputs coalesce.
type OutputRecipe struct {
	TemplateID, TemplateHash, ConfigHash string
	Policy                               OutputPolicy
	Encoder                              AgentDialect
	Provenance                           string
}

// OutputNode is one path in the deterministic internal output plan. A node is
// either a write or a reservation; reservations protect local artifacts but are
// never written or entered in the lock manifest.
type OutputNode struct {
	Path                string
	Recipe              OutputRecipe
	Policy              OutputPolicy
	Declarers           []string
	DeclarerProjections []string
	DependsOn           []string
	Reservation         bool
	file                *RenderedFile
}

// OutputPlan is the single desired-output authority consumed by rendering,
// sync, manifest/prune, checks, and planned-output reporting.
type OutputPlan struct{ Nodes []OutputNode }

func (op *OutputPlan) writeFiles() []RenderedFile {
	files := make([]RenderedFile, 0, len(op.Nodes))
	for _, n := range op.Nodes {
		if !n.Reservation && n.file != nil {
			files = append(files, *n.file)
		}
	}
	return files
}

// declaredPolicy is assigned by a producer family, never inferred by a
// template identifier or output filename. Consumers inspect only node Policy.
func declaredPolicy(kind string, regen bool) OutputPolicy {
	policy := OutputPolicy{Regenerate: regen}
	switch kind {
	case "skills", "agents":
		policy.ValidateFrontmatter, policy.ScanReferences, policy.ScanSkillReferences = true, true, true
	case "docs", "agents-doc", "adr-readme", "plans-readme", "doc-standard", "agents-md-standard", "working-with-awf", "workflow", "architecture", "development", "glossary", "pitfalls", "roadmap", "testing", "releasing", "domains", "topics":
		policy.ScanReferences, policy.ScanSkillReferences = true, true
	}
	return policy
}

// targetOutputDeclaration is a pre-render, normalized descriptor for an
// extension output. It lets the planner settle compatibility before Execute.
type targetOutputDeclaration struct {
	recipe      OutputRecipe
	declarers   []string
	projections []string
	canonical   string
}

// targetOutputDeclarations reads recipe inputs but never executes a template.
// Thus a collision is reported before any producer renders its output.
func (p *Project) targetOutputDeclarations() (map[string]targetOutputDeclaration, error) {
	eff, err := p.effectiveSkills()
	if err != nil { // coverage-ignore: OutputPlan's declaration-first pass just parsed every enabled skill sidecar
		return nil, err
	}
	p.effSkills = eff
	out := map[string]targetOutputDeclaration{}
	for _, t := range p.Targets {
		if err := t.validate(); err != nil {
			return nil, err
		}
		for _, o := range t.Outputs {
			src, err := fs.ReadFile(templates.FS, o.TemplateID)
			if err != nil {
				return nil, fmt.Errorf("read template %s: %w", o.TemplateID, err)
			}
			expanded, err := render.ExpandIncludes(string(src), templates.FS)
			if err != nil { // coverage-ignore: embedded target-output templates are include-well-formed; render package tests own malformed includes
				return nil, fmt.Errorf("render %s: %w", o.TemplateID, err)
			}
			stripped, err := render.StripAuthoringComments(expanded)
			if err != nil { // coverage-ignore: embedded target-output templates have well-formed authoring comments; render package tests malformed input
				return nil, fmt.Errorf("render %s: %w", o.TemplateID, err)
			}
			configHash, err := p.artifactConfigHash(stripped, config.Sidecar{}, nil, t)
			if err != nil { // coverage-ignore: no target output has parts and its descriptor projection is marshalable
				return nil, err
			}
			recipe := OutputRecipe{TemplateID: o.TemplateID, TemplateHash: manifest.Hash([]byte(expanded)), ConfigHash: configHash, Policy: o.Policy, Encoder: o.Encoder, Provenance: fmt.Sprintf("%d", o.Provenance)}
			decl := out[o.Path]
			if decl.canonical != "" && decl.recipe != recipe {
				return nil, fmt.Errorf("two artifacts render to the same output path %q: conflicting output recipes", o.Path)
			}
			if decl.canonical == "" {
				decl.recipe, decl.canonical = recipe, t.Name
			}
			decl.declarers = append(decl.declarers, t.Name)
			decl.projections = append(decl.projections, targetDescriptorProjection(t))
			out[o.Path] = decl
		}
	}
	for path, decl := range out {
		slices.Sort(decl.declarers)
		slices.Sort(decl.projections)
		out[path] = decl
	}
	return out, nil
}

// OutputPlan compiles all output producers. Generated nodes are constructed in
// dependency order; config reference observes ordinary/domain metadata but is
// deliberately excluded from its own input.
func (p *Project) OutputPlan() (*OutputPlan, error) {
	p.beginInvocation()
	outputDeclarations, err := BuildOutputDeclarations(p.Cfg, p.Cat, p.Targets, filesystemProjectReader{root: p.Root})
	if err != nil {
		return nil, err
	}
	declarations, err := p.targetOutputDeclarations()
	if err != nil {
		return nil, err
	}
	base, err := p.renderAllBase(declarations)
	if err != nil {
		return nil, err
	}
	plan := &OutputPlan{}
	add := func(f RenderedFile, declarer string, deps ...string) error {
		recipe := OutputRecipe{TemplateID: f.TemplateID, TemplateHash: f.TemplateHash, ConfigHash: f.ConfigHash, Policy: f.Policy, Encoder: f.Encoder, Provenance: fmt.Sprintf("%d", f.Provenance)}
		if f.Declarer == "" {
			f.Declarer = declarer
		}
		// Compare all output-affecting normalized recipe inputs before a node is
		// accepted. Declarer identity is intentionally excluded here.

		for i := range plan.Nodes {
			if plan.Nodes[i].Path != f.Path {
				continue
			}
			if plan.Nodes[i].Recipe != recipe {
				return fmt.Errorf("two artifacts render to the same output path %q: conflicting output recipes", f.Path)
			}
			// coverage-ignore: target-output duplicates coalesce before rendering and all other producer paths are unique.
			plan.Nodes[i].Declarers = append(plan.Nodes[i].Declarers, f.Declarer)
			plan.Nodes[i].DeclarerProjections = append(plan.Nodes[i].DeclarerProjections, f.DeclarerProjection)
			return nil
		}
		copy := f
		node := OutputNode{Path: f.Path, Recipe: recipe, Policy: f.Policy, Declarers: []string{f.Declarer}, DeclarerProjections: []string{f.DeclarerProjection}, DependsOn: deps, file: &copy}
		if decl, ok := declarations[f.Path]; ok {
			node.Declarers, node.DeclarerProjections = decl.declarers, decl.projections
		}
		plan.Nodes = append(plan.Nodes, node)
		return nil
	}
	for _, f := range base {
		// coverage-ignore: base output paths are unique by renderAllBase's precondition.
		if err := add(f, f.TemplateID); err != nil {
			return nil, err
		}
	}
	topicFiles, topicDeps, err := p.generateTopicDocs()
	if err != nil {
		return nil, err
	}
	localDocs := map[string]bool{}
	for _, name := range p.Cfg.Docs {
		sc, err := p.Cfg.Sidecar("docs", name)
		if err != nil { // coverage-ignore: renderAllBase already read every enabled doc sidecar
			return nil, err
		}
		if sc.Local {
			localDocs[p.docOutPath(name)] = true
		}
	}
	for _, f := range topicFiles {
		if localDocs[f.Path] {
			return nil, fmt.Errorf("local document and topic output render to the same output path %q", f.Path)
		}
		if err := add(f, f.Declarer, topicDeps[f.Path]...); err != nil {
			return nil, err
		}
	}
	index, err := p.generateIndexMD()
	if err != nil { // coverage-ignore: topic generation loaded the same ADR corpus first
		return nil, err
	}
	// coverage-ignore: generated INDEX.md has a reserved unique path.
	if err := add(index, "generated-index"); err != nil {
		return nil, err
	}
	domains, err := p.generateDomainDocs()
	if err != nil { // coverage-ignore: INDEX.md parses the same ADR directory first and reports malformed input
		return nil, err
	}
	for _, f := range domains {
		// coverage-ignore: validated domain names produce distinct paths.
		if err := add(f, "generated-domain"); err != nil {
			return nil, err
		}
	}
	inputs := slices.Concat(base, domains, topicFiles)
	if cref, ok, err := p.generateConfigReference(inputs); err != nil {
		return nil, err
	} else if ok {
		deps := make([]string, 0, len(inputs))
		for _, f := range inputs {
			deps = append(deps, f.Path)
		}
		// coverage-ignore: config reference has a reserved unique path.
		if err := add(*cref, "generated-config-reference", deps...); err != nil {
			return nil, err
		}
	}
	for _, kv := range []struct {
		kind  string
		names []string
	}{{"skills", p.Cfg.Skills}, {"agents", p.Cfg.Agents}} {
		for _, name := range kv.names {
			sc, err := p.Cfg.Sidecar(kv.kind, name)
			if err != nil { // coverage-ignore: renderAllBase has already read this sidecar
				return nil, err
			}
			if !sc.Local {
				continue
			}
			for _, path := range p.localOutPaths(kv.kind, name) {
				encoder := MarkdownAgentDialect
				for _, target := range p.Targets {
					if d, ok := descriptorByPlural(kv.kind); ok && d.outPath(target, p.Cfg.Prefix, name) == path {
						encoder = target.AgentDialect
						break
					}
				}
				policy := OutputPolicy{LocalValidation: true, ValidateFrontmatter: true}
				plan.Nodes = append(plan.Nodes, OutputNode{Path: path, Policy: policy, Recipe: OutputRecipe{Policy: policy, Encoder: encoder}, Declarers: []string{kv.kind + ":" + name}, Reservation: true})
			}
		}
	}
	slices.SortFunc(plan.Nodes, func(a, b OutputNode) int { return strings.Compare(a.Path, b.Path) })
	for i := range plan.Nodes {
		slices.Sort(plan.Nodes[i].Declarers)
		slices.Sort(plan.Nodes[i].DeclarerProjections)
		slices.Sort(plan.Nodes[i].DependsOn)
		if plan.Nodes[i].file != nil {
			// Membership and each normalized declarer descriptor are observable
			// even when a coalesced output's bytes are identical.
			plan.Nodes[i].file.ConfigHash = manifest.Hash([]byte(plan.Nodes[i].Recipe.ConfigHash + "\\x00" + strings.Join(plan.Nodes[i].DeclarerProjections, "\\x00")))
		}
	}
	if err := validateDeclarationPlanParity(plan.Nodes, outputDeclarations); err != nil { // coverage-ignore: parity unit tests own mismatch diagnostics; every producer test requires equality
		return nil, err
	}
	return plan, nil
}

func validateDeclarationPlanParity(nodes []OutputNode, declarations []OutputDeclaration) error {
	if len(nodes) != len(declarations) {
		np, dp := []string{}, []string{}
		for _, n := range nodes {
			np = append(np, n.Path)
		}
		for _, d := range declarations {
			dp = append(dp, d.Path)
		}
		return fmt.Errorf("output declaration parity: declarations-only %v, plan-only %v", difference(dp, np), difference(np, dp))
	}
	for i := range nodes {
		if nodes[i].Path != declarations[i].Path || nodes[i].Reservation != declarations[i].Reservation || !slices.Equal(nodes[i].Declarers, declarations[i].Declarers) {
			return fmt.Errorf("output declaration parity at %q: plan %v declaration %v", nodes[i].Path, nodes[i].Declarers, declarations[i].Declarers)
		}
	}
	return nil
}
func difference(a, b []string) []string {
	out := []string{}
	for _, x := range a {
		if !slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

// RenderAll renders only plan write nodes in deterministic path order.
func (p *Project) RenderAll() ([]RenderedFile, error) {
	op, err := p.OutputPlan()
	if err != nil {
		return nil, err
	}
	return op.writeFiles(), nil
}

// PlannedOutputs returns plan write paths, excluding local reservations.
func (p *Project) PlannedOutputs() ([]string, error) {
	op, err := p.OutputPlan()
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, n := range op.Nodes {
		if !n.Reservation {
			paths = append(paths, n.Path)
		}
	}
	return paths, nil
}

// localReservations validates plan reservation nodes rather than reconstructing
// local output paths in lifecycle callers.
func (p *Project) localReservations(op *OutputPlan, fail func(string, error)) {
	for _, n := range op.Nodes {
		if !n.Reservation || !n.Policy.LocalValidation {
			continue
		}
		b, err := os.ReadFile(filepath.Join(p.Root, n.Path))
		if err != nil {
			fail(n.Path, errors.New("local artifact file absent"))
			continue
		}
		if err := validateArtifact(b, n.Recipe.Encoder); err != nil {
			fail(n.Path, err)
		}
	}
}
