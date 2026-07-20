package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

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
	if err != nil {
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
	return plan, nil
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
