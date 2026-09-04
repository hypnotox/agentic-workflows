// Package migrate owns supported live schema upgrades. Historical schema
// decoding belongs to internal/audit and retired layouts are recognized only
// for refusal, never parsed or relocated here.
package migrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// fileImage is the exact entry state observed while planning. An absent image
// has Present false, zero mode, and no content or children. A directory image
// records its permission mode and sorted direct-child names.
type fileImage struct {
	Present  bool
	Content  []byte
	Mode     os.FileMode
	Children []string
}

// fileMutation is a planned regular-file replacement/removal or empty-directory
// prune with the original preimage that must still be present when committed.
type fileMutation struct {
	Path           string
	Expected       fileImage
	Content        []byte
	Mode           os.FileMode
	Remove         bool
	EmptyDirectory bool
}

// proposedTree is the read-only tree view supplied to one migration step. It
// overlays every earlier step's plan on the confined project root, so ordered
// steps observe the state they are collectively proposing without mutating it.
type proposedTree struct {
	files     *filesystem.Handle
	mutations map[string]fileMutation
}

// Read returns the proposed bytes and mode for path. A non-regular final entry
// is rejected because file-image migrations cannot preserve its topology.
func (t *proposedTree) Read(path string) ([]byte, os.FileMode, error) {
	if mutation, ok := t.mutations[path]; ok {
		if mutation.Remove {
			return nil, 0, fs.ErrNotExist
		}
		return append([]byte(nil), mutation.Content...), mutation.Mode.Perm(), nil
	}
	expected, err := t.files.ExpectedIdentity(path)
	if err != nil {
		return nil, 0, err
	}
	defer expected.Release() //nolint:errcheck // planning read owns no mutation
	if expected.Mode()&fs.ModeSymlink != 0 {
		return nil, 0, fmt.Errorf("migration source %s is a final symlink; replace it with a regular file and retry", path)
	}
	if !expected.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("migration source %s is not a regular file; replace it with a regular file and retry", path)
	}
	return t.files.ReadExpected(path, expected)
}

// PlanEmptyDirectory returns a prune only when path's proposed direct children
// are all removed by earlier steps or the supplied same-step plan. The exact
// current child inventory remains the commit-time stale-plan preimage.
func (t *proposedTree) PlanEmptyDirectory(path string, sameStep []fileMutation) (fileMutation, bool, error) {
	expected, err := t.files.ExpectedIdentity(path)
	if errors.Is(err, fs.ErrNotExist) {
		return fileMutation{}, false, nil
	}
	if err != nil {
		return fileMutation{}, false, err
	}
	defer expected.Release() //nolint:errcheck // planning read owns no mutation
	if !expected.IsDir() {
		return fileMutation{}, false, fmt.Errorf("migration prune source %s is not a directory", path)
	}
	entries, err := t.files.ReadDirExpected(path, expected)
	if err != nil {
		return fileMutation{}, false, err
	}
	children := make([]string, 0, len(entries))
	planned := make(map[string]fileMutation, len(t.mutations)+len(sameStep))
	for mutationPath, mutation := range t.mutations {
		planned[mutationPath] = mutation
	}
	for _, mutation := range sameStep {
		planned[mutation.Path] = mutation
	}
	for mutationPath, mutation := range planned {
		if filepath.ToSlash(filepath.Dir(filepath.FromSlash(mutationPath))) == path && !mutation.Remove {
			return fileMutation{}, false, nil
		}
	}
	for _, entry := range entries {
		children = append(children, entry.Name())
		childPath := filepath.ToSlash(filepath.Join(filepath.FromSlash(path), entry.Name()))
		mutation, found := planned[childPath]
		if !found || !mutation.Remove {
			return fileMutation{}, false, nil
		}
	}
	sort.Strings(children)
	return fileMutation{
		Path: path, Expected: fileImage{Present: true, Mode: expected.Mode().Perm(), Children: children},
		Remove: true, EmptyDirectory: true,
	}, true, nil
}

func proposedImage(content []byte, mode os.FileMode, err error) (fileImage, error) {
	if errors.Is(err, fs.ErrNotExist) {
		return fileImage{}, nil
	}
	if err != nil {
		return fileImage{}, err
	}
	return fileImage{Present: true, Content: append([]byte(nil), content...), Mode: mode.Perm()}, nil
}

func (t *proposedTree) overlay(planned []fileMutation) error {
	for _, mutation := range planned {
		var current fileImage
		if mutation.EmptyDirectory {
			current = mutation.Expected
		} else {
			content, mode, err := t.Read(mutation.Path)
			var imageErr error
			current, imageErr = proposedImage(content, mode, err)
			if imageErr != nil {
				return imageErr
			}
		}
		if prior, ok := t.mutations[mutation.Path]; ok {
			mutation.Expected = prior.Expected
		} else {
			mutation.Expected = current
		}
		mutation.Expected.Content = append([]byte(nil), mutation.Expected.Content...)
		mutation.Expected.Children = append([]string(nil), mutation.Expected.Children...)
		mutation.Content = append([]byte(nil), mutation.Content...)
		t.mutations[mutation.Path] = mutation
	}
	return nil
}

func (t *proposedTree) coalesced() []fileMutation {
	paths := make([]string, 0, len(t.mutations))
	for path := range t.mutations {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	mutations := make([]fileMutation, 0, len(paths))
	for _, path := range paths {
		mutation := t.mutations[path]
		// A path created and retired entirely inside the bridge has no live
		// mutation. Likewise, an exact rewrite is already converged.
		if !mutation.Expected.Present && mutation.Remove ||
			mutation.Expected.Present && !mutation.Remove && !mutation.EmptyDirectory &&
				mutation.Expected.Mode.Perm() == mutation.Mode.Perm() && bytes.Equal(mutation.Expected.Content, mutation.Content) {
			continue
		}
		mutation.Expected.Content = append([]byte(nil), mutation.Expected.Content...)
		mutation.Expected.Children = append([]string(nil), mutation.Expected.Children...)
		mutation.Content = append([]byte(nil), mutation.Content...)
		mutations = append(mutations, mutation)
	}
	return mutations
}

// migration is one ordered upgrade step for a supported live generation.
type migration struct {
	To    int
	Name  string
	Build func(context.Context, *proposedTree, *Changes) ([]fileMutation, error)
}

// LiveSchemaFloor is the oldest source generation this binary can operate on.
const LiveSchemaFloor = 50

// registry begins at the live floor and advances through supported migrations.
var registry = []migration{
	{To: LiveSchemaFloor, Name: "supported-schema-50"},
	{To: contextSkillGeneration, Name: contextSkillMigration, Build: renameRepositoryContextSkill},
	{To: skillExtractionGeneration, Name: skillExtractionMigration, Build: migrateExtractedSkills},
	{To: workflowSurfaceGeneration, Name: workflowSurfaceMigration, Build: retireWorkflowSurface},
}

func Current() int                { return registry[len(registry)-1].To }
func LiveSchemaRange() (int, int) { return LiveSchemaFloor, Current() }

func validateRegistry() error {
	if len(registry) == 0 || registry[0].To != LiveSchemaFloor {
		return fmt.Errorf("migration registry must begin at supported floor %d", LiveSchemaFloor)
	}
	for i := 1; i < len(registry); i++ {
		if registry[i].To <= registry[i-1].To {
			return fmt.Errorf("migration registry is not strictly ascending at schema %d", registry[i].To)
		}
	}
	return nil
}

// RetiredLayoutError identifies a removed layout without decoding its config.
type RetiredLayoutError struct{ Layout string }

func (e *RetiredLayoutError) Error() string {
	return fmt.Sprintf("retired project layout %s is unsupported at live floor %d", e.Layout, LiveSchemaFloor)
}
func (e *RetiredLayoutError) Is(target error) bool {
	return target == manifest.ErrUnsupportedLiveSource
}

func retiredLayout(root string) (string, error) {
	for _, layout := range []struct{ path, name string }{
		{filepath.Join(root, ".claude", "awf.yaml"), ".claude/awf.yaml"},
		{filepath.Join(root, ".claude", "awf"), ".claude/awf/"},
	} {
		if _, err := os.Stat(layout.path); err == nil {
			return layout.name, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("stat %s: %w", layout.name, err)
		}
	}
	return "", nil
}

// CurrentAuthorityPresence inspects the two current control entries without
// following their final path components.
func CurrentAuthorityPresence(root string) (currentConfig, currentLock bool, returnErr error) {
	files, err := filesystem.Open(root)
	if err != nil {
		return false, false, err
	}
	defer func() { returnErr = errors.Join(returnErr, files.Close()) }()
	for path, destination := range map[string]*bool{
		config.DirName + "/config.yaml": &currentConfig,
		config.DirName + "/awf.lock":    &currentLock,
	} {
		if _, err := files.LinkInfo(path); err == nil {
			*destination = true
		} else if !errors.Is(err, fs.ErrNotExist) {
			return false, false, fmt.Errorf("inspect %s: %w", path, err)
		}
	}
	return currentConfig, currentLock, nil
}

// Generation reads only a current-layout lock schema. Retired layouts receive a
// typed refusal before their authority representation can be decoded.
func Generation(root string) (int, error) {
	currentConfig, _, err := CurrentAuthorityPresence(root)
	if err != nil {
		return 0, err
	}
	layout, err := retiredLayout(root)
	if err != nil {
		return 0, err
	}
	if layout != "" && !currentConfig {
		return 0, &RetiredLayoutError{Layout: layout}
	}
	if !currentConfig {
		return Current(), nil
	}
	generation, found, err := manifest.LoadSchemaConfinedOptional(root, config.DirName+"/awf.lock")
	if err != nil {
		return 0, err
	}
	if !found {
		return Current(), nil
	}
	return generation, nil
}

// ProjectPresent recognizes current control files and retired layouts without
// interpreting content or following final path components.
func ProjectPresent(root string) (present bool, returnErr error) {
	files, err := filesystem.Open(root)
	if err != nil {
		return false, err
	}
	defer func() { returnErr = errors.Join(returnErr, files.Close()) }()
	for _, path := range []string{config.DirName + "/config.yaml", config.DirName + "/awf.lock", ".claude/awf.yaml", ".claude/awf"} {
		if _, err := files.LinkInfo(path); err == nil {
			return true, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return false, fmt.Errorf("inspect project authority %s: %w", path, err)
		}
	}
	return false, nil
}

func ProjectPresentFromFiles(has func(string) bool) bool {
	return has(config.DirName+"/config.yaml") || has(config.DirName+"/awf.lock") || has(".claude/awf.yaml") || has(".claude/awf/config.yaml") || has(".claude/awf/awf.lock")
}

func registryTos() []int {
	tos := make([]int, len(registry))
	for i, m := range registry {
		tos[i] = m.To
	}
	return tos
}
func gateStateFor(gen, current int, tos []int) string {
	if gen > current {
		return "ahead"
	}
	if gen == current {
		return "ok"
	}
	for _, to := range tos {
		if to > gen && to <= current {
			return "gate"
		}
	}
	return "autobump"
}
func GateStateForGeneration(gen int) string { return gateStateFor(gen, Current(), registryTos()) }

type UpgradeRequiredError struct{ Generation, Current int }

func (e *UpgradeRequiredError) Error() string {
	return fmt.Sprintf("schema %d requires migration to schema %d", e.Generation, e.Current)
}
func CheckLiveGeneration(gen int) error {
	if err := manifest.ValidateLive(&manifest.Lock{SchemaVersion: gen}, LiveSchemaFloor, Current()); err != nil {
		return err
	}
	if GateStateForGeneration(gen) == "gate" {
		return &UpgradeRequiredError{Generation: gen, Current: Current()}
	}
	return nil
}
func CheckLive(root string) (int, error) {
	gen, err := Generation(root)
	if err != nil {
		return 0, err
	}
	return gen, CheckLiveGeneration(gen)
}
func GateState(root string) (string, int, error) {
	gen, err := Generation(root)
	if err != nil {
		return "", 0, err
	}
	return GateStateForGeneration(gen), gen, nil
}

// plan produces only registered supported migrations and never writes the
// filesystem. Its file images are private, temporary bridge details.
func planMigrations(ctx context.Context, root string) (planned []string, resultChanges []Change, mutations []fileMutation, returnErr error) {
	if err := validateRegistry(); err != nil {
		return nil, nil, nil, err
	}
	from, err := Generation(root)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := manifest.ValidateLive(&manifest.Lock{SchemaVersion: from}, LiveSchemaFloor, Current()); err != nil {
		return nil, nil, nil, err
	}
	files, err := filesystem.Open(root)
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { returnErr = errors.Join(returnErr, files.Close()) }()
	proposed := &proposedTree{files: files, mutations: map[string]fileMutation{}}
	changes := &Changes{}
	for _, m := range registry {
		if m.To <= from {
			continue
		}
		stepMutations, err := m.Build(ctx, proposed, changes)
		if err != nil {
			return planned, changes.Items(), proposed.coalesced(), fmt.Errorf("migration %q (to %d): %w", m.Name, m.To, err)
		}
		if err := proposed.overlay(stepMutations); err != nil {
			return planned, changes.Items(), proposed.coalesced(), fmt.Errorf("migration %q (to %d): validate planned files: %w", m.Name, m.To, err)
		}
		planned = append(planned, m.Name)
	}
	return planned, changes.Items(), proposed.coalesced(), nil
}

// Result is the semantic and path-level evidence from one straight-line run.
// Touched contains successful filesystem operations; Pending contains the
// current failed path and every operation that was not attempted.
type Result struct {
	Planned []string
	Applied []string
	Changes []Change
	Touched []string
	Pending []string
}

type preparedMutation struct {
	mutation fileMutation
	expected *filesystem.ExpectedIdentity
}

func captureAuthority(root string) (image fileImage, returnErr error) {
	files, err := filesystem.Open(root)
	if err != nil {
		return fileImage{}, err
	}
	defer func() { returnErr = errors.Join(returnErr, files.Close()) }()
	expected, err := files.ExpectedIdentity(config.DirName + "/awf.lock")
	if errors.Is(err, fs.ErrNotExist) {
		return fileImage{}, nil
	}
	if err != nil {
		return fileImage{}, fmt.Errorf("inspect migration authority lock: %w", err)
	}
	defer expected.Release() //nolint:errcheck // read-only capture owns no mutation
	if !expected.Mode().IsRegular() {
		return fileImage{}, errors.New("migration authority lock is not a regular file")
	}
	contents, mode, err := files.ReadExpected(config.DirName+"/awf.lock", expected)
	if err != nil {
		return fileImage{}, fmt.Errorf("read migration authority lock: %w", err)
	}
	return fileImage{Present: true, Content: contents, Mode: mode.Perm()}, nil
}

func validMigrationPath(name string) bool {
	return fs.ValidPath(name) && name != "." && !strings.Contains(name, "\\") &&
		!strings.HasPrefix(name, "/") && path.Clean(name) == name &&
		name != ".." && !strings.HasPrefix(name, "../") &&
		!strings.ContainsFunc(name, unicode.IsControl)
}

func gitMode(mode os.FileMode) awfgit.BlobMode {
	if mode.Perm()&0o111 != 0 {
		return awfgit.BlobExecutable
	}
	return awfgit.BlobRegular
}

func blobsByPath(blobs []awfgit.IndexBlob) map[string]awfgit.IndexBlob {
	byPath := make(map[string]awfgit.IndexBlob, len(blobs))
	for _, blob := range blobs {
		byPath[blob.Path] = blob
	}
	return byPath
}

func mutationLess(a, b preparedMutation) bool {
	class := func(op preparedMutation) int {
		switch {
		case op.mutation.EmptyDirectory:
			return 3
		case !op.mutation.Expected.Present:
			return 0
		case !op.mutation.Remove:
			return 1
		default:
			return 2
		}
	}
	ac, bc := class(a), class(b)
	if ac != bc {
		return ac < bc
	}
	if ac == 3 {
		ad, bd := strings.Count(a.mutation.Path, "/"), strings.Count(b.mutation.Path, "/")
		if ad != bd {
			return ad > bd
		}
	}
	return a.mutation.Path < b.mutation.Path
}

func preflight(ctx context.Context, root string, authority fileImage, mutations []fileMutation) (_ *filesystem.Handle, prepared []preparedMutation, returnErr error) {
	seen := make(map[string]fileMutation, len(mutations))
	destructive := make([]string, 0, len(mutations))
	for _, mutation := range mutations {
		if _, duplicate := seen[mutation.Path]; !validMigrationPath(mutation.Path) || duplicate {
			return nil, nil, fmt.Errorf("invalid planned migration path %q", mutation.Path)
		}
		seen[mutation.Path] = mutation
		if mutation.EmptyDirectory {
			if !mutation.Remove || !mutation.Expected.Present || len(mutation.Content) != 0 || mutation.Mode != 0 {
				return nil, nil, fmt.Errorf("invalid planned directory cleanup %q", mutation.Path)
			}
		} else if mutation.Remove {
			if len(mutation.Content) != 0 || mutation.Mode != 0 {
				return nil, nil, fmt.Errorf("planned removal %q carries replacement data", mutation.Path)
			}
		} else if mutation.Mode == 0 {
			return nil, nil, fmt.Errorf("planned migration path %q has no mode", mutation.Path)
		}
		if mutation.Expected.Present && !mutation.EmptyDirectory {
			destructive = append(destructive, mutation.Path)
		}
	}
	sort.Strings(destructive)
	var headByPath, indexByPath map[string]awfgit.IndexBlob
	if len(destructive) > 0 {
		repo, _, err := awfgit.OpenContaining(root)
		if err != nil {
			return nil, nil, fmt.Errorf("prove migration sources restorable from Git: %w", err)
		}
		head, err := repo.CommitBlobsAt(ctx, "HEAD", destructive)
		if err != nil {
			return nil, nil, fmt.Errorf("prove migration sources present in HEAD: %w", err)
		}
		index, err := repo.IndexBlobs(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("prove migration sources present in the stage-0 index: %w", err)
		}
		headByPath, indexByPath = blobsByPath(head), blobsByPath(index)
		for _, name := range destructive {
			h, hok := headByPath[name]
			i, iok := indexByPath[name]
			if !hok || !iok || h.Mode == awfgit.BlobSymlink || i.Mode == awfgit.BlobSymlink ||
				h.Mode != i.Mode || !bytes.Equal(h.Bytes, i.Bytes) {
				return nil, nil, fmt.Errorf("migration source %s must be an unchanged stage-0 regular file identical to HEAD", name)
			}
		}
	}

	files, err := filesystem.Open(root)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if returnErr != nil {
			for _, op := range prepared {
				if op.expected != nil {
					_ = op.expected.Release()
				}
			}
			returnErr = errors.Join(returnErr, files.Close())
		}
	}()
	lockExpected, lockErr := files.ExpectedIdentity(config.DirName + "/awf.lock")
	if errors.Is(lockErr, fs.ErrNotExist) {
		if authority.Present {
			return nil, nil, errors.New("migration authority lock changed after planning: expected present")
		}
	} else if lockErr != nil {
		return nil, nil, fmt.Errorf("inspect migration authority lock after planning: %w", lockErr)
	} else {
		defer lockExpected.Release() //nolint:errcheck // preflight comparison owns no mutation
		if !authority.Present || !lockExpected.Mode().IsRegular() {
			return nil, nil, errors.New("migration authority lock changed after planning")
		}
		contents, mode, err := files.ReadExpected(config.DirName+"/awf.lock", lockExpected)
		if err != nil {
			return nil, nil, fmt.Errorf("read migration authority lock after planning: %w", err)
		}
		if mode.Perm() != authority.Mode.Perm() || !bytes.Equal(contents, authority.Content) {
			return nil, nil, errors.New("migration authority lock changed after planning")
		}
	}
	for _, mutation := range mutations {
		expected, inspectErr := files.ExpectedIdentity(mutation.Path)
		if errors.Is(inspectErr, fs.ErrNotExist) {
			expected = nil
		} else if inspectErr != nil {
			return nil, prepared, fmt.Errorf("inspect planned migration path %s: %w", mutation.Path, inspectErr)
		}
		prepared = append(prepared, preparedMutation{mutation: mutation, expected: expected})
		if !mutation.Expected.Present {
			if expected != nil {
				return nil, prepared, fmt.Errorf("planned migration destination %s must remain absent", mutation.Path)
			}
			continue
		}
		if expected == nil {
			return nil, prepared, fmt.Errorf("planned migration source %s is missing", mutation.Path)
		}
		if mutation.EmptyDirectory {
			if !expected.IsDir() || expected.Mode().Perm() != mutation.Expected.Mode.Perm() {
				return nil, prepared, fmt.Errorf("planned cleanup %s changed after planning", mutation.Path)
			}
			entries, err := files.ReadDirExpected(mutation.Path, expected)
			if err != nil {
				return nil, prepared, err
			}
			names := make([]string, len(entries))
			for i, entry := range entries {
				names[i] = entry.Name()
			}
			sort.Strings(names)
			if !slices.Equal(names, mutation.Expected.Children) {
				return nil, prepared, fmt.Errorf("planned cleanup %s contains unplanned entries", mutation.Path)
			}
			for _, child := range names {
				childPath := path.Join(mutation.Path, child)
				childMutation, ok := seen[childPath]
				if !ok || !childMutation.Remove {
					return nil, prepared, fmt.Errorf("planned cleanup %s contains unplanned child %s", mutation.Path, child)
				}
			}
			continue
		}
		if !expected.Mode().IsRegular() {
			return nil, prepared, fmt.Errorf("planned migration source %s is not a regular file", mutation.Path)
		}
		contents, mode, err := files.ReadExpected(mutation.Path, expected)
		if err != nil {
			return nil, prepared, err
		}
		if mode.Perm() != mutation.Expected.Mode.Perm() || !bytes.Equal(contents, mutation.Expected.Content) {
			return nil, prepared, fmt.Errorf("planned migration source %s changed after planning", mutation.Path)
		}
		if mutation.Expected.Present {
			// Git stores only the executable distinction; exact bytes and that mode
			// distinction must agree across HEAD, index, and worktree.
			head := headByPath[mutation.Path]
			if head.Mode != gitMode(mode) || !bytes.Equal(head.Bytes, contents) {
				return nil, prepared, fmt.Errorf("migration source %s must be unchanged from HEAD", mutation.Path)
			}
		}
	}
	sort.Slice(prepared, func(i, j int) bool { return mutationLess(prepared[i], prepared[j]) })
	return files, prepared, nil
}

// Apply plans, completely preflights, and applies the supported migration
// chain. It deliberately does not update the schema lock.
func Apply(ctx context.Context, root string) (Result, error) {
	return applyWithHook(ctx, root, nil)
}

func applyWithHook(ctx context.Context, root string, before func(int, string) error) (result Result, returnErr error) {
	authority, err := captureAuthority(root)
	if err != nil {
		return result, err
	}
	planned, changes, mutations, err := planMigrations(ctx, root)
	result.Planned, result.Changes = planned, changes
	if err != nil {
		result.Pending = orderedMutationPaths(mutations)
		return result, err
	}
	files, prepared, err := preflight(ctx, root, authority, mutations)
	if err != nil {
		result.Pending = orderedMutationPaths(mutations)
		return result, err
	}
	defer func() {
		for _, op := range prepared {
			if op.expected != nil {
				_ = op.expected.Release()
			}
		}
		returnErr = errors.Join(returnErr, files.Close())
	}()
	for i, op := range prepared {
		if err := ctx.Err(); err != nil {
			result.Pending = preparedPaths(prepared[i:])
			return result, err
		}
		mutation := op.mutation
		if before != nil {
			if err := before(i, mutation.Path); err != nil {
				result.Pending = preparedPaths(prepared[i:])
				return result, err
			}
		}
		if !mutation.Remove {
			if op.expected == nil {
				if parent := path.Dir(mutation.Path); parent != "." {
					missing, inspectErr := missingParentDirectories(files, parent)
					if inspectErr != nil {
						result.Pending = preparedPaths(prepared[i:])
						return result, inspectErr
					}
					mkdirErr := files.MkdirAll(parent, 0o755)
					for _, dir := range missing {
						if info, statErr := files.LinkInfo(dir); statErr == nil && info.IsDir() {
							result.Touched = append(result.Touched, dir)
						}
					}
					if mkdirErr != nil {
						result.Pending = preparedPaths(prepared[i:])
						return result, mkdirErr
					}
				}
				err = files.ReplaceExpected(mutation.Path, nil, mutation.Content, mutation.Mode)
			} else {
				err = files.ReplaceExpectedRegularFile(mutation.Path, op.expected, mutation.Expected.Content, mutation.Expected.Mode, mutation.Content, mutation.Mode)
			}
		} else if mutation.EmptyDirectory {
			err = files.RemoveExpected(mutation.Path, op.expected)
		} else {
			err = files.RemoveExpectedRegularFile(mutation.Path, op.expected, mutation.Expected.Content, mutation.Expected.Mode)
		}
		if err != nil {
			result.Pending = preparedPaths(prepared[i:])
			return result, fmt.Errorf("apply migration path %s: %w", mutation.Path, err)
		}
		result.Touched = append(result.Touched, mutation.Path)
	}
	result.Applied = append([]string(nil), result.Planned...)
	return result, nil
}

func missingParentDirectories(files *filesystem.Handle, parent string) ([]string, error) {
	var parents []string
	for current := parent; current != "."; current = path.Dir(current) {
		parents = append(parents, current)
	}
	slices.Reverse(parents)
	missing := make([]string, 0, len(parents))
	for _, dir := range parents {
		info, err := files.LinkInfo(dir)
		if errors.Is(err, fs.ErrNotExist) {
			missing = append(missing, dir)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect migration parent %s: %w", dir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("migration parent %s is not a directory", dir)
		}
	}
	return missing, nil
}

func orderedMutationPaths(mutations []fileMutation) []string {
	ops := make([]preparedMutation, len(mutations))
	for i := range mutations {
		ops[i].mutation = mutations[i]
	}
	sort.Slice(ops, func(i, j int) bool { return mutationLess(ops[i], ops[j]) })
	return preparedPaths(ops)
}
func preparedPaths(ops []preparedMutation) []string {
	paths := make([]string, len(ops))
	for i := range ops {
		paths[i] = ops[i].mutation.Path
	}
	return paths
}

// IsRetiredLayout makes presentation boundaries distinguish layout refusal
// from a below-floor current lock without retaining a layout reader.
func IsRetiredLayout(err error) bool {
	var retired *RetiredLayoutError
	return errors.As(err, &retired)
}
