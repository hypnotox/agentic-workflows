package publisher

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/topic"
	"github.com/hypnotox/agentic-workflows/templates"
)

// The declaration and plan types live plan-side with a one-way direction
// (ADR-0195 item 1): the plan orchestrates rendering, and render files never
// call plan functions.

// ProjectTreeReader preserves the concise producer-internal name for the neutral operation tree reader.
type ProjectTreeReader = outputplan.TreeReader

// ArtifactRole preserves the project package's declaration-role compatibility name.
type ArtifactRole = outputplan.ArtifactRole

const (
	// ArtifactConfig identifies an authored configuration input.
	ArtifactConfig = outputplan.ArtifactConfig
	// ArtifactLock identifies the managed project lock.
	ArtifactLock = outputplan.ArtifactLock
	// ArtifactManifest identifies manifest authority.
	ArtifactManifest = outputplan.ArtifactManifest
	// ArtifactTemplate identifies an embedded template input.
	ArtifactTemplate = outputplan.ArtifactTemplate
	// ArtifactConventionPart identifies an authored convention part.
	ArtifactConventionPart = outputplan.ArtifactConventionPart
	// ArtifactAuthoredData identifies authored sidecar data.
	ArtifactAuthoredData = outputplan.ArtifactAuthoredData
	// ArtifactTopicMetadata identifies authored topic metadata.
	ArtifactTopicMetadata = outputplan.ArtifactTopicMetadata
	// ArtifactClaimPart identifies an authored current-state claim part.
	ArtifactClaimPart = outputplan.ArtifactClaimPart
	// ArtifactDecisionRecord identifies an architecture decision record.
	ArtifactDecisionRecord = outputplan.ArtifactDecisionRecord
	// ArtifactManagedOutput identifies an existing managed output input.
	ArtifactManagedOutput = outputplan.ArtifactManagedOutput
	// ArtifactProtocolDescriptor identifies a runtime protocol descriptor.
	ArtifactProtocolDescriptor = outputplan.ArtifactProtocolDescriptor
)

// OutputInput records one semantic input consumed by a declared output.
type OutputInput struct {
	Path string
	Role ArtifactRole
}

// OutputDeclaration records one deterministic declared output and its inputs.
type OutputDeclaration struct {
	Path         string
	TemplateID   string
	Declarers    []string
	Inputs       []OutputInput
	Dependencies []string
}

func projectTreeReader(p renderInputs) ProjectTreeReader {
	return p.read
}

type filesystemProjectReader struct{ root string }

// NewFilesystemReader opens the ordinary working-tree reader used by Publisher.
func NewFilesystemReader(root string) ProjectTreeReader { return filesystemProjectReader{root: root} }

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

func (r filesystemProjectReader) ReadLines(path string, maxLineBytes int, visit func(string) error) (bool, error) {
	file, err := os.Open(filepath.Join(r.root, filepath.FromSlash(path)))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, min(64*1024, maxLineBytes)), maxLineBytes)
	for scanner.Scan() {
		if err := visit(scanner.Text()); err != nil {
			return true, err
		}
	}
	if err := scanner.Err(); err != nil {
		return true, fmt.Errorf("scan lines %s: %w", path, err)
	}
	return true, nil
}
func (r filesystemProjectReader) Entries(prefix string) ([]generatedcheck.TreeEntry, error) {
	var out []generatedcheck.TreeEntry
	base := filepath.Join(r.root, filepath.FromSlash(prefix))
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == base && errors.Is(err, fs.ErrNotExist) {
				return fs.SkipAll
			}
			return err
		}
		supported, e := filesystem.SupportedTreeEntry(d)
		if e != nil {
			return e
		}
		if !supported {
			return nil
		}
		rel, e := filepath.Rel(r.root, p)
		if e != nil { // coverage-ignore: WalkDir supplies paths rooted beneath r.root, so Rel cannot fail on a supported platform
			return e
		}
		out = append(out, generatedcheck.TreeEntry{Path: filepath.ToSlash(rel), Directory: d.IsDir()})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("enumerate %s: %w", prefix, err)
	}
	slices.SortFunc(out, func(a, b generatedcheck.TreeEntry) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
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
		supported, e := filesystem.SupportedTreeEntry(d)
		if e != nil {
			return e
		}
		if !supported {
			return nil
		}
		if d.IsDir() {
			if p == r.root {
				return nil
			}
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			// The operation tree is rooted at the invoking project, not at every
			// repository or adopted project nested below it. Detect boundaries
			// before entering a directory so neither its .git marker nor an .awf
			// config (which sort before ordinary children) can leak partial state.
			if filesystemProjectBoundary(p) {
				return fs.SkipDir
			}
			return nil
		}
		rel, e := filepath.Rel(r.root, p)
		if e != nil { // coverage-ignore: p is always rooted at r.root, so Rel cannot fail
			return e
		}
		// A root gitfile identifies the invoking linked checkout and is metadata,
		// not a reason to prune the invoking project itself.
		if filepath.Clean(p) == filepath.Join(r.root, ".git") {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
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

func filesystemProjectBoundary(dir string) bool {
	for _, marker := range []string{filepath.Join(dir, ".git"), filepath.Join(dir, config.DirName, "config.yaml")} {
		if _, err := os.Lstat(marker); err == nil {
			return true
		} else if !errors.Is(err, fs.ErrNotExist) {
			// WalkDir will surface an unreadable boundary candidate when it reaches
			// that entry; do not turn a metadata fault into silent pruning here.
			return false
		}
	}
	return false
}

// buildOutputDeclarations enumerates deterministic producer declarations without
// rendering or materializing the selected tree.
func buildOutputDeclarations(cfg *config.Config, cat *catalog.Catalog, targets []Target, read ProjectTreeReader, adrs adr.Corpus) ([]OutputDeclaration, error) {
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
		if presence, ok := read.(declarationPathPresence); ok {
			return presence.PathExists(path)
		}

		return declarationPathExists(read, path, &readErr)
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
	// Ordinary Markdown renderTarget consumers observe each authored expanded
	// source mapping when the configured repository fact is active. Declarations
	// mirror that observation without changing TemplateHash ownership.
	markdownInputs := func(tid string, authored ...OutputInput) ([]OutputInput, error) {
		out := inputs(tid, authored...)
		if cfg.Render == nil || cfg.Render.TemplateSourceRoot == "" || tid == "" {
			return out, nil
		}
		src, err := fs.ReadFile(templates.FS, tid)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", tid, err)
		}
		expanded, err := render.ExpandIncludesSource(string(src), tid, templates.FS)
		if err != nil { // coverage-ignore: embedded templates are validated by the production render path and focused include tests
			return nil, fmt.Errorf("render %s: %w", tid, err)
		}
		seen := map[string]bool{}
		for _, span := range expanded.Spans {
			if span.Source != "" && !seen[span.Source] {
				seen[span.Source] = true
				out = append(out, OutputInput{Path: path.Join(cfg.Render.TemplateSourceRoot, span.Source), Role: ArtifactTemplate})
			}
		}
		return out, nil
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
			tid := mustDescriptor("skills").templateID(cat, name)
			sections := cat.Skills[name].Sections
			input, err := markdownInputs(tid, append([]OutputInput{{Path: ".awf/skills/" + name + ".yaml", Role: ArtifactAuthoredData}}, partInputs("skills", name, sections, sc.Sections)...)...)
			if err != nil { // coverage-ignore: catalog skill template IDs and embedded sources are validated static authority
				return nil, err
			}
			add(t.SkillPath(cfg.Prefix, name), tid, t.Name, input)
		}
		for _, name := range slices.Sorted(maps.Keys(cat.Agents)) {
			sc, err := cfg.Sidecar("agents", name)
			if err != nil {
				return nil, err
			}
			tid := mustDescriptor("agents").templateID(cat, name)
			sections := cat.Agents[name].Sections
			input := inputs(tid, append([]OutputInput{{Path: ".awf/agents/" + name + ".yaml", Role: ArtifactAuthoredData}}, partInputs("agents", name, sections, sc.Sections)...)...)
			if t.AgentDialect == MarkdownAgentDialect {
				input, err = markdownInputs(tid, append([]OutputInput{{Path: ".awf/agents/" + name + ".yaml", Role: ArtifactAuthoredData}}, partInputs("agents", name, sections, sc.Sections)...)...)
				if err != nil { // coverage-ignore: catalog agent template IDs and embedded sources are validated static authority
					return nil, err
				}
			}
			add(t.AgentPath(name), tid, t.Name, input)
		}
		bridgeInputs, err := markdownInputs(t.BridgeTemplate)
		if err != nil {
			return nil, err
		}
		add(t.BridgeFile, t.BridgeTemplate, t.BridgeTemplate, bridgeInputs)
		if err := validateTargetOutputRequirements(t, cat); err != nil {
			return nil, err
		}
		for _, o := range resolvedTargetOutputs(t, cfg.Prefix, slices.Sorted(maps.Keys(cat.Skills))) {
			declaredInputs := inputs(o.TemplateID)
			if o.Encoder == MarkdownAgentDialect {
				declaredInputs, err = markdownInputs(o.TemplateID)
				if err != nil { // coverage-ignore: validated target-output descriptors own embedded Markdown template identities; markdownInputs error propagation is covered through enabled bridge declarations
					return nil, err
				}
			}
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
		declaredInputs, err := markdownInputs(e.TID, authored...)
		if err != nil {
			return nil, err
		}
		add(out, e.TID, declarer, declaredInputs)
	}
	for _, local := range cfg.NormalizedLocalDocs() {
		// A local document's only section is edit-in-place, so a present output is
		// read back to preserve the authored body and is genuinely an input to its
		// own next render. The authored-input filter drops it on the first render,
		// when the output is still absent, exactly as the renderer does.
		outPath := config.DocsDir + "/" + local.Name + ".md"
		declaredInputs, err := markdownInputs(localDocTID, OutputInput{Path: outPath, Role: ArtifactManagedOutput})
		if err != nil { // coverage-ignore: localDocTID is a closed embedded Markdown identity, validated by the template census
			return nil, err
		}
		add(outPath, localDocTID, "local-doc:"+local.Name, declaredInputs)
	}
	for _, entry := range pitfalls.All() {
		declaredInputs, err := markdownInputs(pitfallEntryTID, OutputInput{Path: entry.SourcePath, Role: ArtifactAuthoredData})
		if err != nil { // coverage-ignore: the pitfall entry descriptor owns one validated embedded Markdown template identity
			return nil, err
		}
		add(config.DocsDir+"/pitfalls/"+entry.Slug+".md", pitfallEntryTID, "pitfall:"+entry.Slug, declaredInputs)
	}
	if cfg.Profile != catalog.ProfileCore {
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
			domainTID := mustDescriptor("domains").templateID(cat, d)
			declaredInputs, err := markdownInputs(domainTID, authored...)
			if err != nil { // coverage-ignore: the domain descriptor owns one validated embedded template identity
				return nil, err
			}
			add(config.DocsDir+"/domains/"+d+".md", domainTID, "generated-domain", declaredInputs)
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
			declaredInputs, err := markdownInputs(topicTID, OutputInput{Path: p, Role: ArtifactTopicMetadata}, OutputInput{Path: ".awf/topics/parts/" + id + "/current-state.md", Role: ArtifactClaimPart})
			if err != nil { // coverage-ignore: topicTID is a validated compile-time embedded identity
				return nil, err
			}
			add(config.DocsDir+"/topics/"+id+".md", topicTID, "topic:"+id, declaredInputs)
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
				declaredInputs, err := markdownInputs(topicIndexTID, topicInputs...)
				if err != nil { // coverage-ignore: topicIndexTID is a validated compile-time embedded identity
					return nil, err
				}
				add(config.DocsDir+"/topics/"+d+"/index.md", topicIndexTID, "topic-index:"+d, declaredInputs)
			}
		}
		decisionInputs := []OutputInput{}
		for _, record := range adrs.All() {
			decisionInputs = append(decisionInputs, OutputInput{Path: config.DocsDir + "/decisions/" + record.Filename, Role: ArtifactDecisionRecord})
		}
		add(config.DocsDir+"/decisions/INDEX.md", "", "generated-index", inputs("", decisionInputs...))
	}
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
		if decls[i].Path == config.DocsDir+"/pitfalls.md" {
			decls[i].Dependencies = append(decls[i].Dependencies, pitfallSourcePaths(pitfalls)...)
		} else if strings.HasPrefix(decls[i].Path, config.DocsDir+"/pitfalls/") && decls[i].TemplateID == pitfallEntryTID {
			slug := strings.TrimSuffix(strings.TrimPrefix(decls[i].Path, config.DocsDir+"/pitfalls/"), ".md")
			decls[i].Dependencies = append(decls[i].Dependencies, pitfall.SourceDir+"/"+slug+".md")
		}
		switch decls[i].TemplateID {
		case topicTID, topicIndexTID:
			for _, input := range decls[i].Inputs {
				if input.Role == ArtifactTopicMetadata || input.Role == ArtifactClaimPart {
					decls[i].Dependencies = append(decls[i].Dependencies, input.Path)
				}
			}
		case cat.Docs["config-reference"].TID:
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

// OutputPolicy preserves the project package's output-policy compatibility name.
type OutputPolicy = outputplan.Policy

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
type OutputPlan struct {
	Nodes        []OutputNode
	Declarations []OutputDeclaration
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
	case "docs", "agents-doc", "adr-readme", "plans-readme", "doc-standard", "agents-md-standard", "working-with-awf", "pi-runtime-reference", "workflow", "architecture", "development", "glossary", "pitfalls", "roadmap", "testing", "releasing", "domains", "topics":
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
func targetOutputDeclarations(p renderInputs, eff map[string]bool) (map[string]targetOutputDeclaration, error) {
	out := map[string]targetOutputDeclaration{}
	for _, t := range p.targets() {
		if err := projectstate.ValidateTarget(t); err != nil {
			return nil, err
		}
		if err := validateTargetOutputRequirements(t, projectCatalog(p)); err != nil {
			return nil, err
		}
		for _, o := range resolvedTargetOutputs(t, p.cfg.Prefix, slices.Sorted(maps.Keys(projectCatalog(p).Skills))) {
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
			configHash, err := artifactConfigHash(p, stripped, config.Sidecar{}, nil, eff, t)
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
func outputPlan(p renderInputs) (*OutputPlan, error) {
	corpus, pitfalls, topics, eff, err := deriveOperationStateWithPitfalls(p)
	if err != nil {
		return nil, err
	}
	return outputPlanWithPitfalls(p, corpus, pitfalls, topics, eff)
}

func outputPlanWithPitfalls(p renderInputs, corpus adr.Corpus, pitfalls pitfall.Corpus, topics topic.Corpus, eff map[string]bool) (*OutputPlan, error) {
	declarationInventory, err := buildOutputDeclarations(p.cfg, projectCatalog(p), p.targets(), projectTreeReader(p), corpus)
	if err != nil {
		return nil, err
	}
	if err := validateLocalDocOutputCollisions(p, declarationInventory); err != nil {
		return nil, err
	}
	declarations, err := targetOutputDeclarations(p, eff)
	if err != nil {
		return nil, err
	}
	base, err := renderAllBase(p, declarations, eff, pitfalls)
	if err != nil {
		return nil, err
	}
	if err := validateLiveTemplates(p); err != nil { // coverage-ignore: renderAllBase already resolved every live identity; TestValidateLiveTemplatesRejectsMissingTargetTemplate proves the defensive check
		return nil, err
	}
	plan := &OutputPlan{Declarations: declarationInventory}
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
		deps := []string(nil)
		if f.Path == config.DocsDir+"/pitfalls.md" {
			deps = pitfallSourcePaths(pitfalls)
		}
		// coverage-ignore: base output paths are unique by renderAllBase's precondition.
		if err := add(f, f.TemplateID, deps...); err != nil {
			return nil, err
		}
	}
	localDocs, err := generateLocalDocs(p, eff)
	if err != nil {
		return nil, err
	}
	for _, f := range localDocs {
		if err := add(f, f.Declarer); err != nil {
			return nil, err
		}
	}
	pitfallLeaves, err := generatePitfallLeaves(p, pitfalls, eff)
	if err != nil { // coverage-ignore: the same embedded leaf template and validated corpus are closed inputs here
		return nil, err
	}
	for _, f := range pitfallLeaves {
		slug := strings.TrimSuffix(strings.TrimPrefix(f.Path, config.DocsDir+"/pitfalls/"), ".md")
		if err := add(f, f.Declarer, pitfall.SourceDir+"/"+slug+".md"); err != nil { // coverage-ignore: validated unique slugs derive unique leaf paths
			return nil, err
		}
	}
	topicFiles := []RenderedFile{}
	domains := []RenderedFile{}
	if fullProfile(p) {
		var topicDeps map[string][]string
		topicFiles, topicDeps, err = generateTopicDocs(p, topics)
		if err != nil {
			return nil, err
		}
		for _, f := range topicFiles {
			if err := add(f, f.Declarer, topicDeps[f.Path]...); err != nil {
				return nil, err
			}
		}
		index := generateIndexMD(p, corpus)
		if err := add(index, "generated-index"); err != nil { // coverage-ignore: generated INDEX.md has a reserved unique path.
			return nil, err
		}
		domains, err = generateDomainDocs(p, topics, eff)
		if err != nil { // coverage-ignore: renderTarget cannot fail here: .data.domain/.data.topics are always set and the domain template is compile-time embedded
			return nil, err
		}
		for _, f := range domains {
			if err := add(f, "generated-domain"); err != nil { // coverage-ignore: validated domain names produce distinct paths.
				return nil, err
			}
		}
	}
	inputs := slices.Concat(base, localDocs, pitfallLeaves, domains, topicFiles)
	if cref, ok, err := generateConfigReference(p, inputs, eff); err != nil {
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

// PreflightLocalDoc validates one candidate declaration against the complete
// project output plan without mutating the opened project's configuration.
func preflightLocalDoc(p renderInputs, doc config.LocalDoc) error {
	candidateConfig := *p.cfg
	candidateConfig.LocalDocs = append(slices.Clone(p.cfg.LocalDocs), doc)
	candidate := p
	candidate.cfg = &candidateConfig
	_, err := outputPlan(candidate)
	return err
}

// validateLocalDocOutputCollisions compares configured local paths with the
// complete declaration inventory before any producer renders. Intrinsic name
// grammar remains config-owned; project owns collisions with every output
// family, including generated and target-owned outputs.
func validateLocalDocOutputCollisions(p renderInputs, declarations []OutputDeclaration) error {
	for _, local := range p.cfg.NormalizedLocalDocs() {
		localPath := config.DocsDir + "/" + local.Name + ".md"
		for _, declaration := range declarations {
			if declaration.Path == localPath && !slices.Contains(declaration.Declarers, "local-doc:"+local.Name) {
				return fmt.Errorf("local document %q collides with managed output %q", local.Name, localPath)
			}
		}
	}
	return nil
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

// validateLiveTemplates verifies that every identity derived from the live
// declaration owners resolves in the shipped embedded filesystem.
func validateLiveTemplates(p renderInputs) error {
	for tid := range liveTemplateIDs(p) {
		if _, err := fs.Stat(templates.FS, tid); err != nil {
			return fmt.Errorf("read template %s: %w", tid, err)
		}
	}
	return nil
}

type declarationPathPresence interface{ PathExists(string) bool }

func declarationPathExists(read ProjectTreeReader, path string, readErr *error) bool {
	_, ok, err := read.ReadFile(path)
	if err != nil && *readErr == nil {
		*readErr = err
	}
	return ok
}
