package publisher

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/topic"
	"github.com/hypnotox/agentic-workflows/templates"
)

// The output and plan types live plan-side with a one-way direction
// (ADR-0195 item 1): the plan orchestrates rendering, and render files never
// call plan functions.

// OutputInput records one semantic input consumed by a declared output.
type OutputInput struct {
	Path string
	Role outputplan.ArtifactRole
}

// outputDefinition records one pre-render output identity and its coalesced owners.
type outputDefinition struct {
	Path             string
	TemplateID       string
	RecipeProjection string
	Declarers        []string
	Projections      []string
	Dependencies     []string
}

func projectTreeReader(p renderInputs) outputplan.TreeReader {
	return p.read
}

type filesystemProjectReader struct{ root string }

// NewFilesystemReader opens the ordinary working-tree reader used by Publisher.
func NewFilesystemReader(root string) outputplan.TreeReader {
	return filesystemProjectReader{root: root}
}

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
	scanner.Buffer(make([]byte, min(64*1024, maxLineBytes+2)), maxLineBytes+2)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > maxLineBytes {
			return true, fmt.Errorf("scan lines %s: line exceeds %d bytes", path, maxLineBytes)
		}
		if err := visit(line); err != nil {
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
		if e != nil {
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
		if e != nil {
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

// buildOutputDefinitions registers managed output identities without executing
// a renderer. Init uses this path-only projection before an operation exists.
func buildOutputDefinitions(cfg *config.Config, cat *catalog.Catalog, targets []artifactregistry.Target, read outputplan.TreeReader) ([]outputDefinition, error) {
	pitfalls, err := loadPitfallCorpusFrom(read)
	if err != nil {
		return nil, err
	}
	metadata, err := read.Paths(".awf/topics/metadata/")
	if err != nil {
		return nil, err
	}
	topics := make([]definitionTopic, 0, len(metadata))
	for _, metadataPath := range metadata {
		if !strings.HasSuffix(metadataPath, ".yaml") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(metadataPath, ".awf/topics/metadata/"), ".yaml")
		topics = append(topics, definitionTopic{id: id, metadataPath: metadataPath, partPath: ".awf/topics/parts/" + id + "/current-state.md"})
	}
	return buildOutputDefinitionsFromState(cfg, cat, targets, pitfalls, topics)
}

type definitionTopic struct {
	id, metadataPath, partPath string
}

func definitionTopicsFromCorpus(root string, corpus topic.Corpus) []definitionTopic {
	all := corpus.All()
	out := make([]definitionTopic, 0, len(all))
	for _, current := range all {
		out = append(out, definitionTopic{
			id: current.ID.String(), metadataPath: relSlash(root, current.MetadataPath), partPath: relSlash(root, current.PartPath),
		})
	}
	return out
}

// buildOutputDefinitionsFromState is the authoritative operation population.
// It consumes the operation's already-derived corpora instead of traversing the
// selected tree a second time.
func buildOutputDefinitionsFromState(cfg *config.Config, cat *catalog.Catalog, targets []artifactregistry.Target, pitfalls pitfall.Corpus, topics []definitionTopic) ([]outputDefinition, error) {
	definitions := []outputDefinition{}
	add := func(outputPath, tid, declarer, recipeProjection string, dependencies ...string) {
		if outputPath == "" {
			return
		}
		definitions = append(definitions, outputDefinition{
			Path: filepath.ToSlash(filepath.Clean(outputPath)), TemplateID: tid,
			RecipeProjection: recipeProjection,
			Declarers:        []string{declarer}, Dependencies: normalizePaths(dependencies),
		})
	}
	addTarget := func(outputPath, tid, producer string, target artifactregistry.Target) {
		add(outputPath, tid, target.Name, producer+"\x00"+tid+"\x00"+targetRecipeProjection(target))
		definitions[len(definitions)-1].Projections = []string{targetDescriptorProjection(target)}
	}
	for _, target := range targets {
		if err := artifactregistry.ValidateTarget(target); err != nil {
			return nil, err
		}
		for _, name := range slices.Sorted(maps.Keys(cat.Skills)) {
			addTarget(artifactregistry.OutputPath(cat, target, cfg.Prefix, "skills", name), mustDescriptor("skills").templateID(cat, name), "skills:"+name, target)
		}
		add(target.BridgeFile, target.BridgeTemplate, target.BridgeTemplate, "bridge\x00"+target.BridgeTemplate)
	}
	for _, name := range slices.Sorted(maps.Keys(cat.Docs)) {
		entry := cat.Docs[name]
		declarer := entry.TID
		if entry.Generated {
			declarer = "generated-config-reference"
		}
		deps := []string(nil)
		if name == "pitfalls" {
			deps = pitfallSourcePaths(pitfalls)
		}
		add(artifactregistry.OutputPath(cat, artifactregistry.Target{}, cfg.Prefix, "docs", name), entry.TID, declarer, "docs\x00"+name, deps...)
	}
	for _, local := range cfg.NormalizedLocalDocs() {
		add(artifactregistry.LocalDocOutputPath(local.Name), localDocTID, "local-doc:"+local.Name, "local-doc\x00"+local.Name)
	}
	for _, entry := range pitfalls.All() {
		add(artifactregistry.PitfallOutputPath(entry.Slug), pitfallEntryTID, "pitfall:"+entry.Slug, "pitfall\x00"+entry.Slug, entry.SourcePath)
	}
	for _, domain := range cfg.Domains {
		add(artifactregistry.OutputPath(cat, artifactregistry.Target{}, cfg.Prefix, "domains", domain), mustDescriptor("domains").templateID(cat, domain), "generated-domain", "domain\x00"+domain)
	}
	byDomain := map[string][]string{}
	for _, current := range topics {
		add(artifactregistry.TopicOutputPath(current.id), topicTID, "topic:"+current.id, "topic\x00"+current.id, current.metadataPath, current.partPath)
		if domain, _, ok := strings.Cut(current.id, "/"); ok {
			byDomain[domain] = append(byDomain[domain], current.metadataPath, current.partPath)
		}
	}
	for _, domain := range cfg.Domains {
		if len(byDomain[domain]) != 0 {
			add(artifactregistry.TopicIndexOutputPath(domain), topicIndexTID, "topic-index:"+domain, "topic-index\x00"+domain, byDomain[domain]...)
		}
	}
	for _, unit := range conditionalUnits() {
		if unit.enabled(cfg) {
			add(unit.path, unit.tid, unit.tid, "conditional\x00"+unit.kind)
		}
	}
	for _, name := range resident.RootNames() {
		artifact := artifactregistry.Resident(name)
		if !artifact.Participation.Check {
			continue
		}
		if artifact.Owner != artifactregistry.OwnerResident {
			return nil, fmt.Errorf("resident artifact %q has invalid owner %q", artifact.Name, artifact.Owner)
		}
		add(artifact.OutputPath, artifact.TemplateID, artifact.TemplateID, "resident\x00"+artifact.Name)
	}
	configReferencePath := artifactregistry.OutputPath(cat, artifactregistry.Target{}, cfg.Prefix, "docs", "config-reference")
	for i := range definitions {
		if definitions[i].Path != configReferencePath {
			continue
		}
		for _, definition := range definitions {
			if definition.Path != configReferencePath {
				definitions[i].Dependencies = append(definitions[i].Dependencies, definition.Path)
			}
		}
	}
	return coalesceDefinitions(cfg, definitions)
}

func normalizePaths(paths []string) []string {
	out := slices.Clone(paths)
	for i := range out {
		out[i] = filepath.ToSlash(filepath.Clean(out[i]))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func coalesceDefinitions(cfg *config.Config, definitions []outputDefinition) ([]outputDefinition, error) {
	for _, local := range cfg.NormalizedLocalDocs() {
		outputPath := artifactregistry.LocalDocOutputPath(local.Name)
		owners := 0
		for _, definition := range definitions {
			if definition.Path == outputPath {
				owners += len(definition.Declarers)
			}
		}
		if owners > 1 {
			return nil, fmt.Errorf("local document %q collides with managed output %q", local.Name, outputPath)
		}
	}
	byPath := map[string]outputDefinition{}
	for _, definition := range definitions {
		current, found := byPath[definition.Path]
		if !found {
			byPath[definition.Path] = definition
			continue
		}
		if current.TemplateID != definition.TemplateID || current.RecipeProjection != definition.RecipeProjection {
			return nil, fmt.Errorf("two artifacts render to the same output path %q: conflicting output recipes", definition.Path)
		}
		current.Declarers = append(current.Declarers, definition.Declarers...)
		current.Projections = append(current.Projections, definition.Projections...)
		current.Dependencies = append(current.Dependencies, definition.Dependencies...)
		byPath[definition.Path] = current
	}
	out := slices.Collect(maps.Values(byPath))
	for i := range out {
		slices.Sort(out[i].Declarers)
		out[i].Declarers = slices.Compact(out[i].Declarers)
		slices.Sort(out[i].Projections)
		out[i].Projections = slices.Compact(out[i].Projections)
		out[i].Dependencies = normalizePaths(out[i].Dependencies)
	}
	slices.SortFunc(out, func(a, b outputDefinition) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}

// OutputRecipe is the normalized, output-affecting declaration used for
// collision diagnostics and configuration hashes. Target identity is kept on
// OutputNode declarers rather than here, so compatible shared outputs coalesce.
type OutputRecipe struct {
	TemplateID, TemplateHash, ConfigHash string
	Policy                               outputplan.Policy
}

// OutputNode is one path in the deterministic internal output plan.
type OutputNode struct {
	Path                string
	Recipe              OutputRecipe
	Policy              outputplan.Policy
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
	Nodes []OutputNode
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
func declaredPolicy(kind string, regen bool) outputplan.Policy {
	return artifactregistry.Policy(kind, regen)
}

// OutputPlan compiles all output producers. Generated nodes are constructed in
// dependency order; config reference observes ordinary/domain metadata but is
// deliberately excluded from its own input.
// OutputPlan derives the topic and pitfall corpora once and threads them to
// every producer that needs them.
func outputPlan(p renderInputs) (*OutputPlan, error) {
	pitfalls, topics, err := deriveOperationStateWithPitfalls(p)
	if err != nil {
		return nil, err
	}
	return outputPlanWithPitfalls(p, pitfalls, topics)
}

func outputPlanWithPitfalls(p renderInputs, pitfalls pitfall.Corpus, topics topic.Corpus) (*OutputPlan, error) {
	definitions, err := buildOutputDefinitionsFromState(p.cfg, projectCatalog(p), p.targets(), pitfalls, definitionTopicsFromCorpus(p.root(), topics))
	if err != nil {
		return nil, err
	}
	// Resolve every live template after the complete definition set has
	// coalesced, but before any render closure can execute.
	if err := validateLiveTemplates(p); err != nil {
		return nil, err
	}
	definitionByPath := make(map[string]outputDefinition, len(definitions))
	for _, definition := range definitions {
		definitionByPath[definition.Path] = definition
	}

	base, err := renderAllBase(p, pitfalls)
	if err != nil {
		return nil, err
	}
	plan := &OutputPlan{}
	materialized := make(map[string]bool, len(definitions))
	add := func(f RenderedFile) error {
		definition, declared := definitionByPath[f.Path]
		if !declared {
			return fmt.Errorf("materialized undeclared output %q", f.Path)
		}
		if materialized[f.Path] {
			return fmt.Errorf("output %q materialized more than once", f.Path)
		}
		materialized[f.Path] = true
		if f.ObservedTemplateID != "" && f.ObservedTemplateID != definition.TemplateID {
			return fmt.Errorf("output %q materialized template %q, want definition %q", f.Path, f.ObservedTemplateID, definition.TemplateID)
		}
		recipe := OutputRecipe{TemplateID: f.TemplateID, TemplateHash: f.TemplateHash, ConfigHash: f.ConfigHash, Policy: f.Policy}
		projections := []string{f.DeclarerProjection}
		if len(definition.Projections) != 0 {
			projections = slices.Clone(definition.Projections)
		}
		copy := f
		plan.Nodes = append(plan.Nodes, OutputNode{
			Path: f.Path, Recipe: recipe, Policy: f.Policy,
			Declarers: slices.Clone(definition.Declarers), DeclarerProjections: projections,
			DependsOn:      slices.Clone(definition.Dependencies),
			ConsumedInputs: normalizeOutputInputs(f.ConsumedInputs), ObservedTemplateID: f.ObservedTemplateID, file: &copy,
		})
		return nil
	}
	for _, f := range base {
		if err := add(f); err != nil {
			return nil, err
		}
	}
	localDocs, err := generateLocalDocs(p)
	if err != nil {
		return nil, err
	}
	for _, f := range localDocs {
		if err := add(f); err != nil {
			return nil, err
		}
	}
	pitfallLeaves, err := generatePitfallLeaves(p, pitfalls)
	if err != nil {
		return nil, err
	}
	for _, f := range pitfallLeaves {
		if err := add(f); err != nil {
			return nil, err
		}
	}
	topicFiles, _, err := generateTopicDocs(p, topics)
	if err != nil {
		return nil, err
	}
	for _, f := range topicFiles {
		if err := add(f); err != nil {
			return nil, err
		}
	}
	domains, err := generateDomainDocs(p, topics)
	if err != nil {
		return nil, err
	}
	for _, f := range domains {
		if err := add(f); err != nil {
			return nil, err
		}
	}
	inputs := slices.Concat(base, localDocs, pitfallLeaves, domains, topicFiles)
	if cref, ok, err := generateConfigReference(p, inputs); err != nil {
		return nil, err
	} else if ok {
		if err := add(*cref); err != nil {
			return nil, err
		}
	}
	for _, definition := range definitions {
		if !materialized[definition.Path] {
			return nil, fmt.Errorf("defined output %q was not materialized", definition.Path)
		}
	}
	slices.SortFunc(plan.Nodes, func(a, b OutputNode) int { return strings.Compare(a.Path, b.Path) })
	for i := range plan.Nodes {
		slices.Sort(plan.Nodes[i].DeclarerProjections)
		if plan.Nodes[i].file != nil {
			// The recipe hash remains available independently; the published hash
			// additionally observes the complete coalesced declarer population.
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
