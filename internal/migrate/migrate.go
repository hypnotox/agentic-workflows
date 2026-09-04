// Package migrate owns supported live schema upgrades. Historical schema
// decoding belongs to internal/audit and retired layouts are recognized only
// for refusal, never parsed or relocated here.
package migrate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// FileImage is the exact regular-file state observed while planning. An absent
// image has Present false, zero mode, and no content.
type FileImage struct {
	Present bool
	Content []byte
	Mode    os.FileMode
}

// FileMutation is a planned replacement with the original preimage that must
// still be present when the caller commits the transaction.
type FileMutation struct {
	Path     string
	Expected FileImage
	Content  []byte
	Mode     os.FileMode
	Remove   bool
}

// ProposedTree is the read-only tree view supplied to one migration step. It
// overlays every earlier step's plan on the confined project root, so ordered
// steps observe the state they are collectively proposing without mutating it.
type ProposedTree struct {
	files     *filesystem.Handle
	mutations map[string]FileMutation
}

// Read returns the proposed bytes and mode for path. A non-regular final entry
// is rejected because file-image migrations cannot preserve its topology.
func (t *ProposedTree) Read(path string) ([]byte, os.FileMode, error) {
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

func proposedImage(content []byte, mode os.FileMode, err error) (FileImage, error) {
	if errors.Is(err, fs.ErrNotExist) {
		return FileImage{}, nil
	}
	if err != nil {
		return FileImage{}, err
	}
	return FileImage{Present: true, Content: append([]byte(nil), content...), Mode: mode.Perm()}, nil
}

func (t *ProposedTree) overlay(planned []FileMutation) error {
	for _, mutation := range planned {
		content, mode, err := t.Read(mutation.Path)
		current, err := proposedImage(content, mode, err)
		if err != nil {
			return err
		}
		if prior, ok := t.mutations[mutation.Path]; ok {
			mutation.Expected = prior.Expected
		} else {
			mutation.Expected = current
		}
		mutation.Expected.Content = append([]byte(nil), mutation.Expected.Content...)
		mutation.Content = append([]byte(nil), mutation.Content...)
		t.mutations[mutation.Path] = mutation
	}
	return nil
}

func (t *ProposedTree) coalesced() []FileMutation {
	paths := make([]string, 0, len(t.mutations))
	for path := range t.mutations {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	mutations := make([]FileMutation, 0, len(paths))
	for _, path := range paths {
		mutation := t.mutations[path]
		mutation.Expected.Content = append([]byte(nil), mutation.Expected.Content...)
		mutation.Content = append([]byte(nil), mutation.Content...)
		mutations = append(mutations, mutation)
	}
	return mutations
}

// Migration is one ordered upgrade step for a supported live generation.
type Migration struct {
	To    int
	Name  string
	Build func(context.Context, *ProposedTree, *Changes) ([]FileMutation, error)
}

// LiveSchemaFloor is the oldest source generation this binary can operate on.
const LiveSchemaFloor = 50

// registry begins at the live floor and advances through supported migrations.
var registry = []Migration{
	{To: LiveSchemaFloor, Name: "supported-schema-50"},
	{To: contextSkillGeneration, Name: contextSkillMigration, Build: renameRepositoryContextSkill},
	{To: skillExtractionGeneration, Name: skillExtractionMigration, Build: migrateExtractedSkills},
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

func currentConfigPresent(root string) (bool, error) {
	_, err := os.Stat(config.ConfigPath(root))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat .awf/config.yaml: %w", err)
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

// Build produces only registered supported migrations. It never decodes a
// below-floor source or writes the filesystem; the command composition maps
// its file mutations to the upgrade journal.
func Build(ctx context.Context, root string) (applied []string, resultChanges []Change, mutations []FileMutation, returnErr error) {
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
	proposed := &ProposedTree{files: files, mutations: map[string]FileMutation{}}
	changes := &Changes{}
	for _, m := range registry {
		if m.To <= from {
			continue
		}
		planned, err := m.Build(ctx, proposed, changes)
		if err != nil {
			return applied, changes.Items(), proposed.coalesced(), fmt.Errorf("migration %q (to %d): %w", m.Name, m.To, err)
		}
		if err := proposed.overlay(planned); err != nil {
			return applied, changes.Items(), proposed.coalesced(), fmt.Errorf("migration %q (to %d): validate planned files: %w", m.Name, m.To, err)
		}
		applied = append(applied, m.Name)
	}
	return applied, changes.Items(), proposed.coalesced(), nil
}

// IsRetiredLayout makes presentation boundaries distinguish layout refusal
// from a below-floor current lock without retaining a layout reader.
func IsRetiredLayout(err error) bool {
	var retired *RetiredLayoutError
	return errors.As(err, &retired)
}
