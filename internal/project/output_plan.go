package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
	"github.com/hypnotox/agentic-workflows/templates"
)

// The declaration and plan types live plan-side with a one-way direction
// (ADR-0195 item 1): the plan orchestrates rendering, and render files never
// call plan functions.

// ProjectTreeReader is the read-only input authority for output declarations.
// Reads distinguish absence from a fault, and Paths reports a fault rather than
// a short list: either loss would silently narrow the drift oracle.
type ProjectTreeReader interface {
	ReadFile(path string) ([]byte, bool, error)
	Paths(prefix string) ([]string, error)
}

// ArtifactRole classifies a path in the output plan and the context artifact
// report by its function in the render pipeline.
type ArtifactRole string

const (
	ArtifactConfig             ArtifactRole = "config"
	ArtifactLock               ArtifactRole = "lock"
	ArtifactManifest           ArtifactRole = "manifest"
	ArtifactTemplate           ArtifactRole = "template"
	ArtifactConventionPart     ArtifactRole = "convention-part"
	ArtifactAuthoredData       ArtifactRole = "authored-data"
	ArtifactTopicMetadata      ArtifactRole = "topic-metadata"
	ArtifactClaimPart          ArtifactRole = "claim-part"
	ArtifactDecisionRecord     ArtifactRole = "decision-record"
	ArtifactManagedOutput      ArtifactRole = "managed-output"
	ArtifactProtocolDescriptor ArtifactRole = "protocol-descriptor"
)

type OutputInput struct {
	Path string
	Role ArtifactRole
}
type OutputDeclaration struct {
	Path         string
	TemplateID   string
	Declarers    []string
	Inputs       []OutputInput
	Dependencies []string
}

type snapshotTreeReader struct{ tree *snapshot.Tree }

func (r snapshotTreeReader) ReadFile(path string) ([]byte, bool, error) {
	f, ok := r.tree.Lookup(filepath.ToSlash(path))
	if !ok || !f.Scannable() {
		return nil, false, nil
	}
	return slices.Clone(f.Bytes), true, nil
}
func (r snapshotTreeReader) Paths(prefix string) ([]string, error) {
	out := []string{}
	prefix = filepath.ToSlash(prefix)
	for _, f := range r.tree.List() {
		if f.Scannable() && strings.HasPrefix(f.Path, prefix) {
			out = append(out, f.Path)
		}
	}
	return out, nil // an in-memory tree has no read to fault
}

func (p *Project) projectTreeReader() ProjectTreeReader {
	if p.read != nil {
		return p.read
	}
	return filesystemProjectReader{root: p.Root}
}

type filesystemProjectReader struct{ root string }

func (r filesystemProjectReader) ReadFile(path string) ([]byte, bool, error) {
	b, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(path)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return slices.Clone(b), true, nil
}
func (r filesystemProjectReader) Paths(prefix string) ([]string, error) {
	out := []string{}
	base := filepath.Join(r.root, filepath.FromSlash(prefix))
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// Callers enumerate directories that need not exist yet, so an absent
			// base is an empty result. Any other fault truncates the enumeration
			// and must reach the caller instead of narrowing the oracle.
			if p == base && errors.Is(err, fs.ErrNotExist) {
				return fs.SkipAll
			}
			return err
		}
		if !d.IsDir() {
			rel, e := filepath.Rel(r.root, p)
			if e != nil { // coverage-ignore: p is always rooted at r.root, so Rel cannot fail
				return e
			}
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		// The whole-tree call passes an empty prefix, which would render as
		// "enumerate : ...".
		subject := prefix
		if subject == "" {
			subject = "project tree"
		}
		return nil, fmt.Errorf("enumerate %s: %w", subject, err)
	}
	slices.Sort(out)
	return out, nil
}

// BuildOutputDeclarations enumerates deterministic producer declarations without
// rendering or materializing the selected tree.
func BuildOutputDeclarations(cfg *config.Config, cat *catalog.Catalog, targets []Target, read ProjectTreeReader, adrs adr.Corpus) ([]OutputDeclaration, error) {
	pitfalls, err := loadPitfallCorpusFrom(read)
	if err != nil {
		return nil, err
	}
	decls := []OutputDeclaration{}
	add := func(path, tid, who string, inputs []OutputInput) {
		if path == "" {
			return
		}
		path = filepath.ToSlash(filepath.Clean(path))
		for i := range inputs {
			inputs[i].Path = filepath.ToSlash(filepath.Clean(inputs[i].Path))
		}
		for i := range decls {
			if decls[i].Path == path && decls[i].TemplateID == tid {
				decls[i].Declarers = append(decls[i].Declarers, who)
				decls[i].Inputs = append(decls[i].Inputs, inputs...)
				return
			}
		}
		decls = append(decls, OutputDeclaration{Path: path, TemplateID: tid, Declarers: []string{who}, Inputs: inputs})
	}
	configInput := []OutputInput{{Path: ".awf/config.yaml", Role: ArtifactConfig}}
	var readErr error
	exists := func(path string) bool {
		_, ok, err := read.ReadFile(path)
		if err != nil && readErr == nil {
			readErr = err
		}
		return ok
	}
	inputs := func(tid string, authored ...OutputInput) []OutputInput {
		out := slices.Clone(configInput)
		if tid != "" {
			out = append(out, OutputInput{Path: "templates/" + tid, Role: ArtifactTemplate})
		}
		for _, in := range authored {
			if exists(in.Path) {
				out = append(out, in)
			}
		}
		return out
	}
	partInputs := func(kind, name string, sections []string, overrides ...map[string]config.SectionOverride) []OutputInput {
		out := []OutputInput{}
		for _, section := range sections {
			if len(overrides) != 0 && overrides[0][section].Drop {
				continue
			}
			var p string
			if config.IsSingletonKind(kind) {
				p = ".awf/parts/" + kind + "/" + section + ".md"
			} else {
				p = ".awf/" + kind + "/parts/" + name + "/" + section + ".md"
			}
			if exists(p) {
				out = append(out, OutputInput{Path: p, Role: ArtifactConventionPart})
			}
		}
		return out
	}
	for _, t := range targets {
		for _, name := range slices.Sorted(maps.Keys(cat.Skills)) {
			sc, err := cfg.Sidecar("skills", name)
			if err != nil {
				return nil, err
			}
			tid := mustDescriptor("skills").tid(name)
			sections := cat.Skills[name].Sections
			input := inputs(tid, append([]OutputInput{{Path: ".awf/skills/" + name + ".yaml", Role: ArtifactAuthoredData}}, partInputs("skills", name, sections, sc.Sections)...)...)
			add(t.SkillPath(cfg.Prefix, name), tid, t.Name, input)
		}
		for _, name := range slices.Sorted(maps.Keys(cat.Agents)) {
			sc, err := cfg.Sidecar("agents", name)
			if err != nil {
				return nil, err
			}
			tid := mustDescriptor("agents").tid(name)
			sections := cat.Agents[name].Sections
			input := inputs(tid, append([]OutputInput{{Path: ".awf/agents/" + name + ".yaml", Role: ArtifactAuthoredData}}, partInputs("agents", name, sections, sc.Sections)...)...)
			add(t.AgentPath(name), tid, t.Name, input)
		}
		add(t.BridgeFile, t.BridgeTemplate, t.BridgeTemplate, inputs(t.BridgeTemplate))
		if err := validateTargetOutputRequirements(t, cat); err != nil {
			return nil, err
		}
		for _, o := range resolvedTargetOutputs(t, cfg.Prefix, slices.Sorted(maps.Keys(cat.Skills))) {
			declaredInputs := inputs(o.TemplateID)
			for _, input := range o.Inputs {
				declaredInputs = append(declaredInputs, OutputInput(input))
			}
			add(o.Path, o.TemplateID, t.Name, declaredInputs)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(cat.Docs)) {
		e := cat.Docs[name]
		sc, err := cfg.Sidecar(func() string {
			if e.Mandatory {
				return name
			}
			return "docs"
		}(), name)
		if err != nil {
			return nil, err
		}
		// Output shape is catalog structure, independent of Mandatory. AgentsDoc
		// owns the root guide, Path owns a structural docs output, and an empty
		// Path is a name-derived doc. Mandatory only selects sidecar location.
		out := name + ".md"
		if e.Path != "" {
			out = e.Path
		}
		if e.AgentsDoc {
			out = "AGENTS.md"
		} else {
			out = config.DocsDir + "/" + out
		}
		sidecarPath := ".awf/" + name + ".yaml"
		if !e.Mandatory {
			sidecarPath = ".awf/docs/" + name + ".yaml"
		}
		authored := []OutputInput{{Path: sidecarPath, Role: ArtifactAuthoredData}}
		if e.AgentsDoc {
			for _, doc := range slices.Sorted(maps.Keys(cat.Docs)) {
				if !cat.Docs[doc].Mandatory {
					authored = append(authored, OutputInput{Path: ".awf/docs/" + doc + ".yaml", Role: ArtifactAuthoredData})
				}
			}
		}
		authored = append(authored, partInputs(func() string {
			if e.Mandatory {
				return name
			}
			return "docs"
		}(), name, e.Sections, sc.Sections)...)
		declarer := e.TID
		if e.Generated {
			declarer = "generated-config-reference"
		}
		if name == "pitfalls" {
			for _, source := range pitfallSourcePaths(pitfalls) {
				authored = append(authored, OutputInput{Path: source, Role: ArtifactAuthoredData})
			}
		}
		add(out, e.TID, declarer, inputs(e.TID, authored...))
	}
	for _, entry := range pitfalls.All() {
		add(config.DocsDir+"/pitfalls/"+entry.Slug+".md", pitfallEntryTID, "pitfall:"+entry.Slug,
			inputs(pitfallEntryTID, OutputInput{Path: entry.SourcePath, Role: ArtifactAuthoredData}))
	}
	for _, d := range cfg.Domains {
		sc, err := cfg.Sidecar("domains", d)
		if err != nil {
			return nil, err
		}
		authored := []OutputInput{{Path: ".awf/domains/" + d + ".yaml", Role: ArtifactAuthoredData}}
		authored = append(authored, partInputs("domains", d, cat.DomainDoc.Sections, sc.Sections)...)
		domainMetadata, err := read.Paths(".awf/topics/metadata/" + d + "/")
		if err != nil {
			return nil, err
		}
		for _, metadataPath := range domainMetadata {
			if strings.HasSuffix(metadataPath, ".yaml") {
				id := strings.TrimSuffix(strings.TrimPrefix(metadataPath, ".awf/topics/metadata/"), ".yaml")
				authored = append(authored, OutputInput{Path: metadataPath, Role: ArtifactTopicMetadata}, OutputInput{Path: ".awf/topics/parts/" + id + "/current-state.md", Role: ArtifactClaimPart})
			}
		}
		domainTID := mustDescriptor("domains").tid(d)
		add(config.DocsDir+"/domains/"+d+".md", domainTID, "generated-domain", inputs(domainTID, authored...))
	}
	allMetadata, err := read.Paths(".awf/topics/metadata/")
	if err != nil {
		return nil, err
	}
	for _, p := range allMetadata {
		if !strings.HasSuffix(p, ".yaml") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(p, ".awf/topics/metadata/"), ".yaml")
		add(config.DocsDir+"/topics/"+id+".md", topicTID, "topic:"+id, inputs(topicTID, OutputInput{Path: p, Role: ArtifactTopicMetadata}, OutputInput{Path: ".awf/topics/parts/" + id + "/current-state.md", Role: ArtifactClaimPart}))
	}
	for _, d := range cfg.Domains {
		topicInputs := []OutputInput{}
		indexMetadata, err := read.Paths(".awf/topics/metadata/" + d + "/")
		if err != nil { // coverage-ignore: the same domain metadata enumeration succeeded earlier in this declaration pass
			return nil, err
		}
		for _, p := range indexMetadata {
			if strings.HasSuffix(p, ".yaml") {
				id := strings.TrimSuffix(strings.TrimPrefix(p, ".awf/topics/metadata/"), ".yaml")
				topicInputs = append(topicInputs, OutputInput{Path: p, Role: ArtifactTopicMetadata}, OutputInput{Path: ".awf/topics/parts/" + id + "/current-state.md", Role: ArtifactClaimPart})
			}
		}
		if len(topicInputs) > 0 {
			add(config.DocsDir+"/topics/"+d+"/index.md", topicIndexTID, "topic-index:"+d, inputs(topicIndexTID, topicInputs...))
		}
	}
	decisionInputs := []OutputInput{}
	for _, record := range adrs.All() {
		decisionInputs = append(decisionInputs, OutputInput{Path: config.DocsDir + "/decisions/" + record.Filename, Role: ArtifactDecisionRecord})
	}
	add(config.DocsDir+"/decisions/INDEX.md", "", "generated-index", inputs("", decisionInputs...))
	for _, unit := range conditionalUnits() {
		if !unit.enabled(cfg) {
			continue
		}
		add(unit.path, unit.tid, unit.tid, inputs(unit.tid, partInputs(unit.kind, "", unit.sections)...))
	}
	for _, name := range resident.RootNames() {
		tid := residentGitignoreTID(name)
		add(".awf/"+name+"/.gitignore", tid, tid, inputs(tid))
	}
	if readErr != nil {
		return nil, readErr
	}
	for i := range decls {
		switch decls[i].TemplateID {
		case topicTID, topicIndexTID:
			for _, input := range decls[i].Inputs {
				if input.Role == ArtifactTopicMetadata || input.Role == ArtifactClaimPart {
					decls[i].Dependencies = append(decls[i].Dependencies, input.Path)
				}
			}
		case catalog.Standard.Docs["config-reference"].TID:
			decisionIndex := config.DocsDir + "/decisions/INDEX.md"
			for _, candidate := range decls {
				if candidate.Path != decls[i].Path && candidate.Path != decisionIndex {
					decls[i].Dependencies = append(decls[i].Dependencies, candidate.Path)
				}
			}
		}
		slices.Sort(decls[i].Dependencies)
		decls[i].Dependencies = slices.Compact(decls[i].Dependencies)
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

// OutputNode is one path in the deterministic internal output plan.
type OutputNode struct {
	Path                string
	Recipe              OutputRecipe
	Policy              OutputPolicy
	Declarers           []string
	DeclarerProjections []string
	DependsOn           []string
	ConsumedInputs      []OutputInput
	ObservedTemplateID  string
	file                *RenderedFile
}

// OutputPlan is the single desired-output authority consumed by rendering,
// sync, manifest/prune, checks, and planned-output reporting.
type OutputPlan struct{ Nodes []OutputNode }

// classifyFrozenOutputFreshness compares an ordinary planned output to its
// locked bytes before either drift surface observes a worktree or staged file.
func classifyFrozenOutputFreshness(file RenderedFile, entry manifest.Entry) (manifest.Drift, bool) {
	if manifest.Hash([]byte(file.Content)) != entry.OutputHash {
		return manifest.Drift{Path: file.Path, Kind: "stale", Detail: "rendered output out of date; run awf render"}, true
	}
	return manifest.Drift{}, false
}

func classifyFrozenObservedDrift(file RenderedFile, entry manifest.Entry, observed []byte, observedDetail string) (manifest.Drift, bool) {
	if manifest.Hash(observed) != entry.OutputHash {
		return manifest.Drift{Path: file.Path, Kind: "hand-edited", Detail: observedDetail}, true
	}
	return manifest.Drift{}, false
}

func (op *OutputPlan) writeFiles() []RenderedFile {
	files := make([]RenderedFile, 0, len(op.Nodes))
	for _, n := range op.Nodes {
		if n.file != nil {
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

// resolvedTargetOutputs is the single selection and path translation point for
// target-owned outputs. Planning, rendering, and prune all consume it.
func validateTargetOutputRequirements(t Target, cat *catalog.Catalog) error {
	for _, output := range t.Outputs {
		if output.RequiresSkill != "" {
			if _, ok := cat.Skills[output.RequiresSkill]; !ok {
				return fmt.Errorf("target %q output %q requires unknown catalog skill %q", t.Name, output.Path, output.RequiresSkill)
			}
		}
	}
	return nil
}

func resolvedTargetOutputs(t Target, prefix string, selected []string) []TargetOutput {
	selectedSet := map[string]bool{}
	for _, name := range selected {
		selectedSet[name] = true
	}
	out := []TargetOutput{}
	for _, output := range t.Outputs {
		if output.RequiresSkill != "" && !selectedSet[output.RequiresSkill] {
			continue
		}
		if output.SkillName != "" {
			output.Path = t.SkillPath(prefix, output.SkillName)
		}
		out = append(out, output)
	}
	return out
}

// targetOutputDeclarations reads recipe inputs but never executes a template.
// Thus a collision is reported before any producer renders its output.
func (p *Project) targetOutputDeclarations(eff map[string]bool) (map[string]targetOutputDeclaration, error) {
	out := map[string]targetOutputDeclaration{}
	for _, t := range p.Targets {
		if err := t.validate(); err != nil {
			return nil, err
		}
		if err := validateTargetOutputRequirements(t, p.Cat); err != nil {
			return nil, err
		}
		for _, o := range resolvedTargetOutputs(t, p.Cfg.Prefix, slices.Sorted(maps.Keys(p.Cat.Skills))) {
			src, err := fs.ReadFile(templates.FS, o.TemplateID)
			if err != nil { // coverage-ignore: TestTargetOutputDeclarationsRejectUnreadableTemplate proves this error; Go's embedded-filesystem profile does not attribute its return block.
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
			configHash, err := p.artifactConfigHash(stripped, config.Sidecar{}, nil, eff, t)
			if err != nil { // coverage-ignore: no target output has parts and its descriptor projection is marshalable
				return nil, err
			}
			templateInput := []byte(expanded)
			recipe := OutputRecipe{TemplateID: o.TemplateID, TemplateHash: manifest.Hash(templateInput), ConfigHash: configHash, Policy: o.Policy, Encoder: o.Encoder, Provenance: fmt.Sprintf("%d", o.Provenance)}
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
// OutputPlan derives the ADR corpus, the topic corpus, and the effective skill
// set at its own entry and threads them to every producer that needs one. An
// operation that already derived them enters through outputPlan instead, so one
// lifecycle call performs each derivation exactly once.
func (p *Project) OutputPlan(ctx context.Context) (*OutputPlan, error) {
	corpus, pitfalls, topics, eff, err := p.deriveOperationStateWithPitfalls()
	if err != nil { // coverage-ignore: direct compatibility entry; lifecycle entries derive this corpus before calling the threaded planner
		return nil, err
	}
	return p.outputPlanWithPitfalls(ctx, corpus, pitfalls, topics, eff)
}

func (p *Project) outputPlanWithPitfalls(ctx context.Context, corpus adr.Corpus, pitfalls pitfall.Corpus, topics topic.Corpus, eff map[string]bool) (*OutputPlan, error) {
	declarations, err := p.targetOutputDeclarations(eff)
	if err != nil {
		return nil, err
	}
	base, err := p.renderAllBase(declarations, eff, pitfalls)
	if err != nil {
		return nil, err
	}
	if err := p.validateLiveTemplates(); err != nil { // coverage-ignore: renderAllBase already resolved every live identity; TestValidateLiveTemplatesRejectsMissingTargetTemplate proves the defensive check
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
		node := OutputNode{Path: f.Path, Recipe: recipe, Policy: f.Policy, Declarers: []string{f.Declarer}, DeclarerProjections: []string{f.DeclarerProjection}, DependsOn: deps, ConsumedInputs: normalizeOutputInputs(f.ConsumedInputs), ObservedTemplateID: f.ObservedTemplateID, file: &copy}
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
	pitfallLeaves, err := p.generatePitfallLeaves(pitfalls, eff)
	if err != nil { // coverage-ignore: the same embedded leaf template and validated corpus are closed inputs here
		return nil, err
	}
	for _, f := range pitfallLeaves {
		if err := add(f, f.Declarer); err != nil { // coverage-ignore: validated unique slugs derive unique leaf paths
			return nil, err
		}
	}
	topicFiles, topicDeps, err := p.generateTopicDocs(ctx, topics)
	if err != nil {
		return nil, err
	}
	for _, f := range topicFiles {
		if err := add(f, f.Declarer, topicDeps[f.Path]...); err != nil {
			return nil, err
		}
	}
	index := p.generateIndexMD(corpus)
	// coverage-ignore: generated INDEX.md has a reserved unique path.
	if err := add(index, "generated-index"); err != nil {
		return nil, err
	}
	domains, err := p.generateDomainDocs(topics, eff)
	if err != nil { // coverage-ignore: renderTarget cannot fail here: .data.domain/.data.topics are always set and the domain template is compile-time embedded
		return nil, err
	}
	for _, f := range domains {
		// coverage-ignore: validated domain names produce distinct paths.
		if err := add(f, "generated-domain"); err != nil {
			return nil, err
		}
	}
	inputs := slices.Concat(base, pitfallLeaves, domains, topicFiles)
	if cref, ok, err := p.generateConfigReference(inputs, eff); err != nil {
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

func normalizeOutputInputs(inputs []OutputInput) []OutputInput {
	out := slices.Clone(inputs)
	for i := range out {
		out[i].Path = filepath.ToSlash(filepath.Clean(out[i].Path))
	}
	slices.SortFunc(out, func(a, b OutputInput) int {
		if a.Path != b.Path {
			return strings.Compare(a.Path, b.Path)
		}
		return strings.Compare(string(a.Role), string(b.Role))
	})
	return slices.Compact(out)
}

// PlannedOutputs returns plan write paths.
func (p *Project) PlannedOutputs(ctx context.Context) ([]string, error) {
	op, err := p.OutputPlan(ctx)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, n := range op.Nodes {
		paths = append(paths, n.Path)
	}
	return paths, nil
}

// validateLiveTemplates verifies that every identity derived from the live
// declaration owners resolves in the shipped embedded filesystem.
func (p *Project) validateLiveTemplates() error {
	for tid := range p.liveTemplateIDs() {
		if _, err := fs.Stat(templates.FS, tid); err != nil {
			return fmt.Errorf("read template %s: %w", tid, err)
		}
	}
	return nil
}
