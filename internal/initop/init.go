// Package initop owns the initialization application operation.
package initop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/initspec"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

// LoadProject is the command-composed project loader constructor used after the
// initialization operation has established or selected configuration authority.
type LoadProject func(string) (*project.Loader, error)

// Gate is the command-composed compatibility gate that must pass before an
// existing project is republished.
type Gate func(context.Context, string) error

// PartialError carries the complete initialization outcome after a committed
// config or publication effect, while preserving the underlying error identity.
type PartialError struct {
	Outcome initspec.Outcome
	Cause   error
}

func (e *PartialError) Error() string {
	return "initialization partially committed: " + e.Cause.Error()
}
func (e *PartialError) Unwrap() error { return e.Cause }

// Input contains parsed initialization values and CLI-selected prompt streams.
type Input struct {
	Root         string
	ResidentRoot string
	Force        bool
	Answers      map[string]string
	PromptInput  io.Reader
	PromptOutput io.Writer
	Interactive  bool
}

// Run performs one complete initialization operation and returns its semantic
// outcome. Rendering and protocol selection remain with the command.
func Run(ctx context.Context, input Input, loadProject LoadProject, gate Gate) (outcome initspec.Outcome, returnErr error) {
	root := input.Root
	residentRoot := input.ResidentRoot
	if residentRoot == "" {
		residentRoot = root
	}
	lease, err := filesystem.AcquireProjectLease(ctx, root, residentRoot)
	if err != nil {
		return initspec.Outcome{}, err
	}
	defer func() {
		if releaseErr := lease.Release(); releaseErr != nil {
			joined := errors.Join(returnErr, releaseErr)
			if outcome.ConfigPath != "" {
				returnErr = &PartialError{Outcome: outcome, Cause: joined}
			} else {
				returnErr = joined
			}
		}
	}()
	cfgPath := config.ConfigPath(root)
	lockPath := config.LockPath(root)
	_, statErr := os.Stat(cfgPath)
	configExists := statErr == nil
	_, lockStatErr := os.Stat(lockPath)
	lockExists := lockStatErr == nil
	if !configExists && !lockExists {
		if _, err := adr.LoadCorpus(filepath.Join(root, "docs", "decisions")); err != nil {
			return initspec.Outcome{}, fmt.Errorf("validate first-adoption ADR corpus: %w", err)
		}
	}
	if !input.Force {
		collisions, err := probeCollisions(ctx, root, loadProject)
		if err != nil {
			return initspec.Outcome{}, err
		}
		if len(collisions) > 0 {
			return initspec.Outcome{}, collisionRefusal(collisions)
		}
	}
	if configExists || lockExists {
		lock, found, err := manifest.LoadOptional(lockPath)
		if err != nil {
			return initspec.Outcome{}, fmt.Errorf("invalid authority: restore .awf/awf.lock from version control: %w", err)
		}
		if !found {
			return initspec.Outcome{}, errors.New("pre-tracking authority: restore a supported permanent .awf/awf.lock from version control before initializing")
		}
		state, err := lock.AuthorityState()
		if err != nil { // coverage-ignore: LoadOptional parsed and validated this unchanged lock immediately above
			return initspec.Outcome{}, fmt.Errorf("invalid authority: restore .awf/awf.lock from version control: %w", err)
		}
		if state != manifest.AuthorityPermanent {
			return initspec.Outcome{}, errors.New("pre-tracking authority: restore a supported permanent .awf/awf.lock from version control before initializing")
		}
	}

	var vars map[string]string
	var scopes []string
	profile := catalog.ProfileCore
	ignoredAnswers := configExists && len(input.Answers) > 0
	if !configExists {
		var err error
		vars, scopes, profile, err = initspec.ResolveInit(catalog.Standard.Vars, input.Answers, input.PromptInput, input.PromptOutput, input.Interactive, project.NeededVars)
		if err != nil {
			return initspec.Outcome{}, err
		}
	}

	scaffold := scaffoldCommit{}
	if !configExists {
		contents, err := project.ScaffoldConfigForProfile(filepath.Base(root), vars, scopes, profile)
		if err != nil { // coverage-ignore: ScaffoldConfig renders a static template over a dir basename; cannot fail in practice
			return initspec.Outcome{}, err
		}
		handle, openErr := filesystem.Open(root)
		if openErr != nil {
			return initspec.Outcome{}, openErr
		}
		dirInfo, dirErr := handle.LinkInfo(config.DirName)
		if dirErr != nil && !errors.Is(dirErr, fs.ErrNotExist) {
			_ = handle.Close()
			return initspec.Outcome{}, dirErr
		}
		if errors.Is(dirErr, fs.ErrNotExist) {
			if err := handle.MkdirAll(config.DirName, 0o755); err != nil {
				_ = handle.Close()
				return initspec.Outcome{}, err
			}
			scaffold.createdDir = true
			dirInfo, dirErr = handle.LinkInfo(config.DirName)
			if dirErr != nil {
				_ = handle.Close()
				return initspec.Outcome{}, dirErr
			}
		}
		configRel := filepath.ToSlash(filepath.Join(config.DirName, "config.yaml"))
		publishErr := handle.Publish(configRel, contents, 0o644)
		if publishErr == nil {
			scaffold.configInfo, publishErr = handle.LinkInfo(configRel)
			scaffold.dirInfo = dirInfo
			scaffold.committed = publishErr == nil
		}
		closeErr := handle.Close()
		if publishErr != nil || closeErr != nil {
			return initspec.Outcome{}, errors.Join(publishErr, closeErr)
		}
	}

	failScaffold := func(cause error) (initspec.Outcome, error) {
		return rollbackScaffold(root, cfgPath, scaffold, cause)
	}
	loader, err := loadProject(root)
	if err != nil {
		return failScaffold(err)
	}
	state, cfg, err := loader.OpenForOperation(ctx, root)
	if err != nil {
		return failScaffold(err)
	}
	composed := composePublisher(state, cfg)
	prepared, err := composed.Prepare()
	if err != nil {
		return failScaffold(err)
	}
	collisions, err := prepared.InitCollisions()
	if err != nil { // coverage-ignore: preparation validated every planned path; failure now requires an unportable root-confined filesystem fault
		return failScaffold(err)
	}
	if len(collisions) > 0 && !input.Force { // coverage-ignore: the non-force probe plans the same full catalog; force makes this condition false
		return failScaffold(collisionRefusal(collisions)) // coverage-ignore: the identical pre-prompt probe makes this path unreachable
	}
	if err := gate(ctx, root); err != nil {
		return failScaffold(err)
	}

	var result publisher.Result
	if !configExists && !lockExists {
		result, err = composed.InitializeLeased(ctx, lease, publisher.InitAuthority{InitializedWithVersion: project.Version})
	} else {
		result, err = composed.SyncLeased(ctx, lease)
	}
	if err != nil {
		var publisherPartial *publisher.PartialError
		if errors.As(err, &publisherPartial) || scaffold.committed {
			partialMutation, mutationErr := result.PartialMutation()
			if mutationErr != nil {
				return initspec.Outcome{}, errors.Join(err, mutationErr)
			}
			if scaffold.committed {
				configValue, valueErr := presentation.Literal("config-created " + filepath.ToSlash(filepath.Join(config.DirName, "config.yaml")) + "; recovery: retain it and rerun awf init --force, or remove it only after restoring the pre-init tree")
				if valueErr != nil { // coverage-ignore: fixed path and prose contain no line break
					return initspec.Outcome{}, errors.Join(err, valueErr)
				}
				partialMutation.Changes = append([]presentation.MutationChange{{Label: "committed init effects", Values: []presentation.Value{configValue}}}, partialMutation.Changes...)
			}
			partialOutcome := initspec.Outcome{
				Status: "initialization partially committed", ConfigPath: cfgPath,
				ExistingConfig: configExists, IgnoredAnswers: ignoredAnswers, Sync: partialMutation,
			}
			return partialOutcome, &PartialError{Outcome: partialOutcome, Cause: err}
		}
		return failScaffold(err)
	}
	mutation, err := result.Mutation()
	if err != nil { // coverage-ignore: typed Publisher results and fixed presentation grammar make this mapping failure unreachable
		partialOutcome := initspec.Outcome{Status: "initialization publication committed", ConfigPath: cfgPath, ExistingConfig: configExists, IgnoredAnswers: ignoredAnswers}
		return partialOutcome, &PartialError{Outcome: partialOutcome, Cause: err}
	}
	advisories, err := project.AdvisoryNotes(state, cfg, prepared.Plan(), projectSemantics(prepared))
	if err != nil { // coverage-ignore: Publisher preparation already validated the same advisory semantic inputs; failure requires a concurrent tree mutation
		partialOutcome := initspec.Outcome{Status: "initialization publication committed", ConfigPath: cfgPath, ExistingConfig: configExists, IgnoredAnswers: ignoredAnswers, Sync: mutation}
		return partialOutcome, &PartialError{Outcome: partialOutcome, Cause: err}
	}
	return initspec.Outcome{
		ConfigPath: cfgPath, ExistingConfig: configExists, IgnoredAnswers: ignoredAnswers,
		Sync: mutation, Advisories: advisories, NextActions: append([]string(nil), nextActions[:]...),
	}, nil
}

func composePublisher(state *project.ProjectState, cfg *config.Config) *publisher.Publisher {
	return publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(state.Root()), project.Version)
}

func projectSemantics(prepared publisher.Preparation) project.OperationSemantics {
	return project.OperationSemantics{
		ADRs: prepared.ADRs(), Pitfalls: prepared.Pitfalls(), Topics: prepared.Topics(),
		EffectiveSkills: prepared.EffectiveSkills(), Plans: prepared.Plans(), PlansError: prepared.PlansError(), GeneratedOutput: prepared.GeneratedOutput(),
		Vocabulary: prepared.Vocabulary(),
	}
}

type scaffoldCommit struct {
	committed  bool
	createdDir bool
	configInfo fs.FileInfo
	dirInfo    fs.FileInfo
}

func rollbackScaffold(root, cfgPath string, scaffold scaffoldCommit, cause error) (initspec.Outcome, error) {
	if !scaffold.committed {
		return initspec.Outcome{}, cause
	}
	handle, openErr := filesystem.Open(root)
	if openErr != nil {
		return scaffoldPartialOutcome(cfgPath, true, scaffold.createdDir, errors.Join(cause, openErr))
	}
	configRel := filepath.ToSlash(filepath.Join(config.DirName, "config.yaml"))
	removeConfigErr := handle.RemoveExpected(configRel, scaffold.configInfo)
	dirRemains := scaffold.createdDir
	var removeDirErr error
	if removeConfigErr == nil && scaffold.createdDir {
		removeDirErr = handle.RemoveExpected(config.DirName, scaffold.dirInfo)
		if removeDirErr == nil {
			dirRemains = false
		}
	}
	closeErr := handle.Close()
	rollbackErr := errors.Join(removeConfigErr, removeDirErr, closeErr)
	if rollbackErr == nil {
		return initspec.Outcome{}, cause
	}
	return scaffoldPartialOutcome(cfgPath, removeConfigErr != nil, dirRemains, errors.Join(cause, rollbackErr))
}

func scaffoldPartialOutcome(cfgPath string, configRemains, dirRemains bool, cause error) (initspec.Outcome, error) {
	values := make([]presentation.Value, 0, 2)
	if configRemains {
		value, err := presentation.Literal("config-created " + filepath.ToSlash(filepath.Join(config.DirName, "config.yaml")) + "; recovery: retain it and rerun awf init --force, or remove it only after restoring the pre-init tree")
		if err != nil { // coverage-ignore: fixed path and prose contain no line break
			return initspec.Outcome{}, errors.Join(cause, err)
		}
		values = append(values, value)
	}
	if dirRemains {
		value, err := presentation.Literal("directory-created " + config.DirName + "; recovery: remove only if empty after restoring the pre-init tree")
		if err != nil { // coverage-ignore: fixed path and prose contain no line break
			return initspec.Outcome{}, errors.Join(cause, err)
		}
		values = append(values, value)
	}
	mutation := presentation.Mutation{Status: "partially committed"}
	if len(values) > 0 {
		mutation.Changes = []presentation.MutationChange{{Label: "committed init effects", Values: values}}
	}
	outcome := initspec.Outcome{Status: "initialization partially committed", ConfigPath: cfgPath, Sync: mutation}
	return outcome, &PartialError{Outcome: outcome, Cause: cause}
}

func collisionRefusal(collisions []string) error {
	return fmt.Errorf("awf init: refusing to overwrite existing files (use --force):\n  %s", strings.Join(collisions, "\n  "))
}

func probeCollisions(ctx context.Context, root string, loadProject LoadProject) ([]string, error) {
	if _, err := os.Stat(config.ConfigPath(root)); err == nil {
		loader, err := loadProject(root)
		if err != nil {
			return nil, err
		}
		state, cfg, err := loader.OpenForOperation(ctx, root)
		if err != nil {
			return nil, err
		}
		return composePublisher(state, cfg).InitCollisions()
	}
	tmp, err := os.MkdirTemp("", "awf-init-probe-*")
	if err != nil { // coverage-ignore: MkdirTemp fails only on an unwritable TMPDIR, which a test cannot trigger portably
		return nil, err
	}
	defer os.RemoveAll(tmp)
	scaffold, err := project.ScaffoldConfig(filepath.Base(root), nil, nil)
	if err != nil { // coverage-ignore: ScaffoldConfig over the embedded catalog cannot fail at runtime
		return nil, err
	}
	cfgPath := config.ConfigPath(tmp)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil { // coverage-ignore: a fresh temp dir's child MkdirAll fails only on a permission fault root bypasses
		return nil, err
	}
	if err := os.WriteFile(cfgPath, scaffold, 0o644); err != nil { // coverage-ignore: post-MkdirAll write into a fresh temp dir cannot fail in practice
		return nil, err
	}
	loader := project.NewLoaderWithoutRepository(config.Load, catalog.Standard, func(_ context.Context, selected string) string { return selected })
	state, cfg, err := loader.OpenForOperation(ctx, tmp)
	if err != nil { // coverage-ignore: a freshly-scaffolded default config always opens
		return nil, err
	}
	prepared, err := composePublisher(state, cfg).Prepare()
	if err != nil { // coverage-ignore: rendering the embedded catalog over a fresh scaffold in an empty tree cannot fail
		return nil, err
	}
	return resident.CollisionsAt(root, prepared.Plan().Paths())
}

var nextActions = [...]string{
	"fill the Identity section at .awf/parts/agents-doc/identity.md, then run awf render",
	"set still-empty vars in .awf/config.yaml (the notes above list what each artifact misses), then run awf render",
	"wire rendered hook payloads under .awf/hooks/ into git hooks you own (see the workflow doc's local-hooks section); awf never activates hooks itself",
	"commit .awf/ and the rendered files together",
}
