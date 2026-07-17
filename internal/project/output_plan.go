package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
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
	Path        string
	Recipe      OutputRecipe
	Policy      OutputPolicy
	Declarers   []string
	DependsOn   []string
	Reservation bool
	file        *RenderedFile
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
	case "docs", "agents-doc", "adr-readme", "plans-readme", "doc-standard", "agents-md-standard", "working-with-awf", "workflow", "architecture", "development", "glossary", "pitfalls", "roadmap", "testing", "releasing", "domains":
		policy.ScanReferences, policy.ScanSkillReferences = true, true
	}
	return policy
}

// OutputPlan compiles all output producers. Generated nodes are constructed in
// dependency order; config reference observes ordinary/domain metadata but is
// deliberately excluded from its own input.
func (p *Project) OutputPlan() (*OutputPlan, error) {
	// Validate every descriptor and reject target-owned collisions before any
	// template is rendered. This is the planner's precondition boundary.
	seen := map[string]OutputRecipe{}
	for _, t := range p.Targets {
		if err := t.validate(); err != nil {
			return nil, err
		}
		for _, o := range t.Outputs {
			r := OutputRecipe{TemplateID: o.TemplateID, Policy: o.Policy, Encoder: o.Encoder, Provenance: fmt.Sprintf("%d", o.Provenance)}
			if prior, ok := seen[o.Path]; ok && prior != r {
				return nil, fmt.Errorf("two artifacts render to the same output path %q: conflicting output recipes", o.Path)
			}
			seen[o.Path] = r
		}
	}
	base, err := p.renderAllBase()
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
			if plan.Nodes[i].Path != f.Path { // coverage-ignore: unique base paths make the duplicate-only branch unreachable
				continue
			}
			if plan.Nodes[i].Recipe != recipe {
				return fmt.Errorf("two artifacts render to the same output path %q: conflicting output recipes", f.Path)
			}
			plan.Nodes[i].Declarers = append(plan.Nodes[i].Declarers, f.Declarer)
			return nil
		}
		copy := f
		plan.Nodes = append(plan.Nodes, OutputNode{Path: f.Path, Recipe: recipe, Policy: f.Policy, Declarers: []string{f.Declarer}, DependsOn: deps, file: &copy})
		return nil
	}
	for _, f := range base {
		// coverage-ignore: base output paths are unique by renderAllBase's precondition.
		if err := add(f, f.TemplateID); err != nil {
			return nil, err
		}
	}
	active, err := p.generateActiveMD()
	if err != nil {
		return nil, err
	}
	// coverage-ignore: generated ACTIVE.md has a reserved unique path.
	if err := add(active, "generated-active"); err != nil {
		return nil, err
	}
	domains, err := p.generateDomainDocs()
	if err != nil { // coverage-ignore: ACTIVE.md parses the same ADR directory first and reports malformed input
		return nil, err
	}
	for _, f := range domains {
		// coverage-ignore: validated domain names produce distinct paths.
		if err := add(f, "generated-domain"); err != nil {
			return nil, err
		}
	}
	inputs := slices.Concat(base, domains)
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
		slices.Sort(plan.Nodes[i].DependsOn)
		if plan.Nodes[i].file != nil {
			// Membership is observable even for byte-identical shared output.
			plan.Nodes[i].file.ConfigHash = manifest.Hash([]byte(plan.Nodes[i].Recipe.ConfigHash + "\\x00" + strings.Join(plan.Nodes[i].Declarers, "\\x00")))
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
