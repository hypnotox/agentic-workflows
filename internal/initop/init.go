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

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/initspec"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
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

type advisoryNotesFunc func(*project.Session, *config.Config, *publisher.Publisher) ([]string, error)
type releaseLeaseFunc func(*filesystem.Lease) error

// Run performs one complete initialization operation and returns its semantic
// outcome. Rendering and protocol selection remain with the command.
func Run(ctx context.Context, input Input, loadProject LoadProject, gate Gate) (initspec.Outcome, error) {
	return runWithDependencies(ctx, input, loadProject, gate, func(state *project.Session, cfg *config.Config, operation *publisher.Publisher) ([]string, error) {
		plan, err := operation.Plan()
		if err != nil {
			return nil, err
		}
		glossary, err := operation.Glossary()
		if err != nil {
			return nil, err
		}
		return project.AdvisoryNotes(state, cfg, plan, glossary)
	}, (*filesystem.Lease).Release)
}

func runWithDependencies(ctx context.Context, input Input, loadProject LoadProject, gate Gate, advisoryNotes advisoryNotesFunc, releaseLease releaseLeaseFunc) (outcome initspec.Outcome, returnErr error) {
	root := input.Root
	residentRoot := input.ResidentRoot
	var result publisher.Result
	var scaffold scaffoldCommit
	if residentRoot == "" {
		residentRoot = root
	}
	lease, err := filesystem.AcquireProjectLease(ctx, root, residentRoot)
	if err != nil {
		return initspec.Outcome{}, err
	}
	defer func() {
		if releaseErr := releaseLease(lease); releaseErr != nil {
			joined := errors.Join(returnErr, releaseErr)
			if outcome.ConfigPath != "" && len(result.Effects()) > 0 {
				outcome, returnErr = publisherPartialOutcome(outcome, scaffold, result, joined)
			} else if outcome.ConfigPath != "" {
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
		if err != nil {
			return initspec.Outcome{}, fmt.Errorf("invalid authority: restore .awf/awf.lock from version control: %w", err)
		}
		if state != manifest.AuthorityPermanent {
			return initspec.Outcome{}, errors.New("pre-tracking authority: restore a supported permanent .awf/awf.lock from version control before initializing")
		}
	}

	var vars map[string]string
	var scopes []string
	ignoredAnswers := configExists && len(input.Answers) > 0
	if !configExists {
		var err error
		vars, scopes, err = initspec.Resolve(catalog.Standard.Vars, input.Answers, input.PromptInput, input.PromptOutput, input.Interactive, project.NeededVars)
		if err != nil {
			return initspec.Outcome{}, err
		}
	}

	if !configExists {
		contents, err := project.ScaffoldConfig(filepath.Base(root), vars, scopes)
		if err != nil {
			return initspec.Outcome{}, err
		}
		handle, openErr := filesystem.Open(root)
		if openErr != nil {
			return initspec.Outcome{}, openErr
		}
		scaffold, err = createScaffold(handle, contents)
		defer func() {
			_ = scaffold.configInfo.Release()
			_ = scaffold.dirInfo.Release()
		}()
		closeErr := handle.Close()
		if err != nil || closeErr != nil {
			return rollbackScaffold(root, cfgPath, scaffold, errors.Join(err, closeErr))
		}
	}

	failScaffold := func(cause error) (initspec.Outcome, error) {
		return rollbackScaffold(root, cfgPath, scaffold, cause)
	}
	loader, err := loadProject(root)
	if err != nil {
		return failScaffold(err)
	}
	session, err := loader.Load(ctx, root)
	if err != nil {
		return failScaffold(err)
	}
	cfg := session.Config()
	composed := composePublisher(session)
	collisions, err := composed.InitCollisions()
	if err != nil {
		return failScaffold(err)
	}
	if len(collisions) > 0 && !input.Force {
		return failScaffold(collisionRefusal(collisions))
	}
	if err := gate(ctx, root); err != nil {
		return failScaffold(err)
	}

	if !configExists && !lockExists {
		result, err = composed.InitializeLeased(ctx, lease, publisher.InitAuthority{InitializedWithVersion: project.Version})
	} else {
		result, err = composed.SyncLeased(ctx, lease)
	}
	if err != nil {
		var publisherPartial *publisher.PartialError
		if errors.As(err, &publisherPartial) || scaffold.committed() {
			return publisherPartialOutcome(initspec.Outcome{ConfigPath: cfgPath, ExistingConfig: configExists, IgnoredAnswers: ignoredAnswers}, scaffold, result, err)
		}
		return failScaffold(err)
	}
	mutation, err := result.Mutation()
	if err != nil {
		return publisherPartialOutcome(initspec.Outcome{ConfigPath: cfgPath, ExistingConfig: configExists, IgnoredAnswers: ignoredAnswers}, scaffold, result, err)
	}
	advisories, err := advisoryNotes(session, cfg, composed)
	if err != nil {
		return publisherPartialOutcome(initspec.Outcome{ConfigPath: cfgPath, ExistingConfig: configExists, IgnoredAnswers: ignoredAnswers}, scaffold, result, err)
	}
	return initspec.Outcome{
		ConfigPath: cfgPath, ExistingConfig: configExists, IgnoredAnswers: ignoredAnswers,
		Sync: mutation, Advisories: advisories, NextActions: append([]string(nil), nextActions[:]...),
	}, nil
}

func composePublisher(session *project.Session) *publisher.Publisher {
	return publisher.New(session, project.Version)
}

type scaffoldFilesystem interface {
	CreateDirectory(string, fs.FileMode) (*filesystem.ExpectedIdentity, error)
	Publish(string, []byte, fs.FileMode) error
	LinkInfo(string) (fs.FileInfo, error)
	ExpectedIdentity(string) (*filesystem.ExpectedIdentity, error)
}

type scaffoldCommit struct {
	configCommitted bool
	createdDir      bool
	configInfo      *filesystem.ExpectedIdentity
	dirInfo         *filesystem.ExpectedIdentity
	residue         []string
}

func (s scaffoldCommit) committed() bool {
	return s.configCommitted || s.createdDir || len(s.residue) > 0
}

func createScaffold(handle scaffoldFilesystem, contents []byte) (scaffold scaffoldCommit, returnErr error) {
	_, dirErr := handle.LinkInfo(config.DirName)
	if dirErr != nil && !errors.Is(dirErr, fs.ErrNotExist) {
		return scaffold, dirErr
	}
	if errors.Is(dirErr, fs.ErrNotExist) {
		createdInfo, createErr := handle.CreateDirectory(
			config.DirName, 0o755,
		)
		if createErr == nil {
			scaffold.createdDir = true
			scaffold.dirInfo = createdInfo
		} else if !errors.Is(createErr, fs.ErrExist) {
			return scaffold, createErr
		} else if _, dirErr = handle.LinkInfo(config.DirName); dirErr != nil {
			return scaffold, errors.Join(createErr, dirErr)
		}
	}
	configRel := filepath.ToSlash(filepath.Join(config.DirName, "config.yaml"))
	if err := handle.Publish(configRel, contents, 0o644); err != nil {
		_, residue, committed := filesystem.CommittedPublication(err)
		scaffold.configCommitted = committed
		if residue != "" {
			scaffold.residue = append(scaffold.residue, residue)
		}
		return scaffold, err
	}
	scaffold.configCommitted = true
	configInfo, err := handle.ExpectedIdentity(configRel)
	if err != nil {
		return scaffold, err
	}
	scaffold.configInfo = configInfo
	return scaffold, nil
}

func rollbackScaffold(root, cfgPath string, scaffold scaffoldCommit, cause error) (initspec.Outcome, error) {
	if !scaffold.committed() {
		return initspec.Outcome{}, cause
	}
	handle, openErr := filesystem.Open(root)
	if openErr != nil {
		return scaffoldPartialOutcome(cfgPath, scaffold.configCommitted, scaffold.createdDir, scaffold.residue, errors.Join(cause, openErr))
	}
	configRel := filepath.ToSlash(filepath.Join(config.DirName, "config.yaml"))
	configRemains := scaffold.configCommitted
	var removeConfigErr error
	if scaffold.configCommitted && scaffold.configInfo != nil {
		removeConfigErr = handle.RemoveExpected(configRel, scaffold.configInfo)
		if removeConfigErr == nil {
			configRemains = false
		}
	}
	dirRemains := scaffold.createdDir
	var removeDirErr error
	if !configRemains && scaffold.createdDir && scaffold.dirInfo != nil {
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
	return scaffoldPartialOutcome(cfgPath, configRemains, dirRemains, scaffold.residue, errors.Join(cause, rollbackErr))
}

func publisherPartialOutcome(base initspec.Outcome, scaffold scaffoldCommit, result publisher.Result, cause error) (initspec.Outcome, error) {
	mutation, mutationErr := result.PartialMutation()
	if mutationErr != nil {
		return scaffoldPartialOutcome(base.ConfigPath, scaffold.configCommitted, scaffold.createdDir, scaffold.residue, errors.Join(cause, mutationErr))
	}
	if scaffold.committed() {
		values, valueErr := scaffoldEffectValues(scaffold.configCommitted, scaffold.createdDir, scaffold.residue)
		if valueErr != nil {
			base.Status = "initialization partially committed"
			base.Sync = mutation
			return base, &PartialError{Outcome: base, Cause: errors.Join(cause, valueErr)}
		}
		mutation.Changes = append([]presentation.MutationChange{{Label: "committed init effects", Values: values}}, mutation.Changes...)
	}
	base.Status = "initialization partially committed"
	base.Sync = mutation
	return base, &PartialError{Outcome: base, Cause: cause}
}

func scaffoldEffectValues(configRemains, dirRemains bool, residue []string) ([]presentation.Value, error) {
	values := make([]presentation.Value, 0, 2+len(residue))
	if configRemains {
		value, err := presentation.Literal("config-created " + filepath.ToSlash(filepath.Join(config.DirName, "config.yaml")) + "; recovery: retain it and rerun awf init --force, or remove it only after restoring the pre-init tree")
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if dirRemains {
		value, err := presentation.Literal("directory-created " + config.DirName + "; recovery: remove only if empty after restoring the pre-init tree")
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	for _, path := range residue {
		value, err := presentation.Literal("publication-residue " + path + "; recovery: remove this temporary residue, then rerun awf init --force")
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func scaffoldPartialOutcome(cfgPath string, configRemains, dirRemains bool, residue []string, cause error) (initspec.Outcome, error) {
	values, valueErr := scaffoldEffectValues(configRemains, dirRemains, residue)
	if valueErr != nil {
		return initspec.Outcome{}, errors.Join(cause, valueErr)
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
		session, err := loader.Load(ctx, root)
		if err != nil {
			return nil, err
		}
		return composePublisher(session).InitCollisions()
	}
	tmp, err := os.MkdirTemp("", "awf-init-probe-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	scaffold, err := project.ScaffoldConfig(filepath.Base(root), nil, nil)
	if err != nil {
		return nil, err
	}
	cfgPath := config.ConfigPath(tmp)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(cfgPath, scaffold, 0o644); err != nil {
		return nil, err
	}
	loader := project.NewLoaderWithoutRepository(config.Load, catalog.Standard, func(_ context.Context, selected string) string { return selected })
	session, err := loader.Load(ctx, tmp)
	if err != nil {
		return nil, err
	}
	return composePublisher(session).InitCollisionsAt(root)
}

var nextActions = [...]string{
	"fill the Identity section at .awf/parts/agents-doc/identity.md, then run awf render",
	"set still-empty vars in .awf/config.yaml (the notes above list what each artifact misses), then run awf render",
	"wire rendered hook payloads under .awf/hooks/ into git hooks you own (see the workflow doc's local-hooks section); awf never activates hooks itself",
	"commit .awf/ and the rendered files together",
}
