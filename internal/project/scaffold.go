package project

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// ScaffoldConfig generates a .awf/config.yaml with every catalog template's
// referenced vars, the self-pinning bootstrap, and the resolved commit scopes.
func ScaffoldConfig(prefix string, vars map[string]string, scopes []string) ([]byte, error) {
	cat := catalog.Standard

	// Collect referenced var names from every catalog template family - not only
	// the core ones - so an opt-in target added later renders without <no value>.
	// touches-state: rendering/project-output-plan:scaffold-seeds-all-vars - seeds every referenced var; proof in scaffold_test.go
	varSet := map[string]bool{}
	for _, kind := range []string{"skills", "agents", "docs"} {
		d, _ := descriptorByPlural(kind)
		for _, name := range d.poolNames(cat) {
			if err := collectVars(templates.FS, d.tid(name), varSet); err != nil { // coverage-ignore: every catalog name has a backing template in the embedded FS, so collectVars cannot fail
				return nil, err
			}
		}
	}
	// Plain singletons (workflow, doc-standard, agents-md-standard included) always
	// render - their vars must be seeded even though they left cat.Docs (ADR-0043).
	for _, sg := range plainSingletons {
		if err := collectVars(templates.FS, sg.tid, varSet); err != nil { // coverage-ignore: every plainSingletons entry has a backing template in the embedded FS, so collectVars cannot fail
			return nil, err
		}
	}
	// Hook payloads render by default (ADR-0048) - seed their vars (commitGateCmd)
	// so an init prompt answer is not silently dropped.
	for _, name := range hookNames {
		if err := collectVars(templates.FS, hookTID(name), varSet); err != nil { // coverage-ignore: every hookNames entry has a backing template in the embedded FS, so collectVars cannot fail
			return nil, err
		}
	}
	varNames := slices.Sorted(maps.Keys(varSet))

	seeded := make(map[string]string, len(varNames))
	for _, v := range varNames {
		seeded[v] = vars[v] // resolved value, or "" for an absent/unresolved var
	}
	// A non-empty resolved commitScopes answer becomes the audit block; an empty
	// answer writes nothing - nil audit.allowedScopes = accept any (ADR-0017,
	// ADR-0051 Decision 2).
	var auditBlk *config.SkeletonAudit
	if len(scopes) > 0 {
		auditBlk = &config.SkeletonAudit{AllowedScopes: scopes}
	}
	out, err := config.MarshalSkeleton(config.Skeleton{
		Prefix: prefix,
		// The default integration branch a fresh project starts on. It is
		// written, never defaulted in code, so an adopter sees and can change
		// the branch name the ADR scaffold and the pending-record block key
		// off (ADR-0202 Decision 6).
		IntegrationBranch: "main",
		Vars:              seeded,
		Audit:             auditBlk,
		Bootstrap:         &config.BootstrapConfig{Enabled: true},
	})
	if err != nil { // coverage-ignore: MarshalSkeleton serializes an in-memory struct; it cannot fail on this input
		return nil, err
	}
	return out, nil
}

// NewPitfall loads the current corpus, creates one canonical source exclusively,
// and returns project-owned presentation for its repository-relative path.
func (p *Project) NewPitfall(title string) (document presentation.Document, returnErr error) {
	tree, err := filesystem.Open(p.Root)
	if err != nil {
		return presentation.Document{}, err
	}
	defer func() { returnErr = errors.Join(returnErr, tree.Close()) }()
	return p.newPitfallWith(title, tree)
}

type pitfallScaffoldFilesystem interface {
	LinkInfo(string) (fs.FileInfo, error)
	Walk(string, func(string, fs.FileInfo) (bool, error)) error
	Read(string) ([]byte, error)
	MkdirAll(string, fs.FileMode) error
	Publish(string, []byte, fs.FileMode) error
}

func (p *Project) newPitfallWith(title string, tree pitfallScaffoldFilesystem) (presentation.Document, error) {
	corpus, err := loadPitfallScaffoldCorpus(tree)
	if err != nil {
		return presentation.Document{}, err
	}
	entry, source, err := corpus.Scaffold(title)
	if err != nil {
		return presentation.Document{}, err
	}
	if err := tree.MkdirAll(pitfallsSourceDir, 0o755); err != nil {
		return presentation.Document{}, fmt.Errorf("create pitfall source directory: %w", err)
	}
	if err := tree.Publish(entry.SourcePath, source, 0o644); err != nil {
		return presentation.Document{}, fmt.Errorf("create pitfall source %s exclusively: %w", entry.SourcePath, err)
	}
	return PitfallScaffoldDocument(entry.SourcePath)
}

func loadPitfallScaffoldCorpus(tree pitfallScaffoldFilesystem) (pitfall.Corpus, error) {
	info, err := tree.LinkInfo(pitfallsSourceDir)
	if errors.Is(err, fs.ErrNotExist) {
		return pitfall.Load(nil)
	}
	if err != nil {
		return pitfall.Corpus{}, fmt.Errorf("inspect pitfall source root %s: %w", pitfallsSourceDir, err)
	}
	if !info.IsDir() {
		return pitfall.Corpus{}, fmt.Errorf("pitfall source root %s is not a directory", pitfallsSourceDir)
	}
	var files []pitfall.SourceFile
	err = tree.Walk(pitfallsSourceDir, func(source string, info fs.FileInfo) (bool, error) {
		if source == pitfallsSourceDir {
			return true, nil
		}
		if info.IsDir() {
			return true, nil
		}
		file := pitfall.SourceFile{Path: source, Regular: info.Mode().IsRegular()}
		if file.Regular {
			file.Bytes, err = tree.Read(source)
			if err != nil {
				return false, fmt.Errorf("read pitfall source %s: %w", source, err)
			}
		}
		files = append(files, file)
		return false, nil
	})
	if err != nil {
		return pitfall.Corpus{}, err
	}
	return pitfall.Load(files)
}

// PitfallScaffoldDocument maps a created source path to the CLI presentation grammar.
func PitfallScaffoldDocument(sourcePath string) (presentation.Document, error) {
	statusValue, err := presentation.Prose("pitfall created")
	if err != nil { // coverage-ignore: fixed status prose is valid
		return presentation.Document{}, err
	}
	status, err := presentation.NewField("status", statusValue)
	if err != nil { // coverage-ignore: fixed label and validated value are valid
		return presentation.Document{}, err
	}
	pathValue, err := presentation.Literal(sourcePath)
	if err != nil {
		return presentation.Document{}, err
	}
	authoredPath, err := presentation.NewField("authored path", pathValue)
	if err != nil { // coverage-ignore: fixed label and validated value are valid
		return presentation.Document{}, err
	}
	return presentation.NewDocument(status, authoredPath)
}

// NeededVars returns the var names referenced by the full rendered catalog.
func NeededVars() (map[string]bool, error) {
	return neededVarsFromFS(templates.FS)
}

func neededVarsFromFS(fsys fs.FS) (map[string]bool, error) {
	cat := catalog.Standard
	varSet := map[string]bool{}
	for _, kind := range []string{"skills", "agents", "docs"} {
		d, _ := descriptorByPlural(kind)
		for _, n := range d.poolNames(cat) {
			if err := collectVars(fsys, d.tid(n), varSet); err != nil {
				return nil, err
			}
		}
	}
	if err := collectVars(fsys, cat.Docs["agents-doc"].TID, varSet); err != nil { // coverage-ignore: the agents-doc template is always embedded
		return nil, err
	}
	for _, sg := range plainSingletons {
		if err := collectVars(fsys, sg.tid, varSet); err != nil { // coverage-ignore: every plainSingletons entry has a backing embedded template
			return nil, err
		}
	}
	for _, name := range hookNames {
		if err := collectVars(fsys, hookTID(name), varSet); err != nil { // coverage-ignore: every hookNames entry has a backing embedded template
			return nil, err
		}
	}
	return varSet, nil
}

// collectVars reads the template at path and adds all .vars.X names to varSet.
func collectVars(fsys fs.FS, path string, varSet map[string]bool) error {
	src, err := fs.ReadFile(fsys, path)
	if err != nil {
		return fmt.Errorf("scaffold: read template %s: %w", path, err)
	}
	for _, v := range render.ReferencedVars(string(src)) {
		varSet[v] = true
	}
	return nil
}
