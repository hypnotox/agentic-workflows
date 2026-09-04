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

// LoadProject constructs the project loader used after configuration authority exists.
type LoadProject func(string) (*project.Loader, error)

// Gate validates the selected live command universe.
type Gate func(context.Context, string) error

// Input contains parsed initialization values and CLI-selected prompt streams.
type Input struct {
	Root, ResidentRoot string
	Answers            map[string]string
	PromptInput        io.Reader
	PromptOutput       io.Writer
	Interactive        bool
}

type advisoryNotesFunc func(*project.Session, *config.Config, *publisher.Publisher) ([]string, error)
type releaseLeaseFunc func(*filesystem.Lease) error

// Run initializes without overwriting collisions. Once config or output is
// created, later failures leave it visible and return an ordinary error.
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
	root, residentRoot := input.Root, input.ResidentRoot
	if residentRoot == "" {
		residentRoot = root
	}
	lease, err := filesystem.AcquireProjectLease(ctx, root, residentRoot)
	if err != nil {
		return initspec.Outcome{}, fmt.Errorf("acquire project lease for %s: %w", root, err)
	}
	defer func() {
		if err := releaseLease(lease); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("release project lease for %s: %w", root, err))
		}
	}()

	cfgPath, lockPath := config.ConfigPath(root), config.LockPath(root)
	_, statErr := os.Stat(cfgPath)
	configExists := statErr == nil
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return outcome, fmt.Errorf("inspect config path %s: %w", cfgPath, statErr)
	}
	_, lockStatErr := os.Stat(lockPath)
	lockExists := lockStatErr == nil
	if lockStatErr != nil && !errors.Is(lockStatErr, fs.ErrNotExist) {
		return outcome, fmt.Errorf("inspect lock path %s: %w", lockPath, lockStatErr)
	}

	if lockExists {
		_, found, err := manifest.LoadOptional(lockPath)
		if err != nil {
			return outcome, fmt.Errorf("invalid authority: restore .awf/awf.lock from version control: %w", err)
		}
		if !found {
			return outcome, errors.New("pre-tracking authority: restore a supported permanent .awf/awf.lock from version control before initializing")
		}
		if !configExists {
			return outcome, errors.New("pre-tracking authority: restore a supported permanent .awf/awf.lock from version control before initializing")
		}
	}

	var collisions []string
	if !configExists {
		collisions, err = probeCollisions(ctx, root, loadProject)
		if err != nil {
			return outcome, err
		}
		if len(collisions) > 0 {
			return outcome, collisionRefusal(collisions)
		}
	}

	ignoredAnswers := configExists && len(input.Answers) > 0
	if !configExists {
		vars, scopes, err := initspec.Resolve(catalog.Standard.Vars, input.Answers, input.PromptInput, input.PromptOutput, input.Interactive, project.NeededVars)
		if err != nil {
			return outcome, err
		}
		contents, err := project.ScaffoldConfig(filepath.Base(root), vars, scopes)
		if err != nil {
			return outcome, err
		}
		handle, err := filesystem.Open(root)
		if err != nil {
			return outcome, fmt.Errorf("open initialization root %s: %w", root, err)
		}
		created, createErr := createScaffold(handle, contents)
		closeErr := handle.Close()
		if closeErr != nil {
			closeErr = fmt.Errorf("close initialization root %s: %w", root, closeErr)
		}
		if created {
			outcome.ConfigPath = cfgPath
		}
		if createErr != nil || closeErr != nil {
			return outcome, errors.Join(createErr, closeErr)
		}
	}
	outcome.ConfigPath, outcome.ExistingConfig, outcome.IgnoredAnswers = cfgPath, configExists, ignoredAnswers

	loader, err := loadProject(root)
	if err != nil {
		return outcome, fmt.Errorf("construct loader for %s: %w", root, err)
	}
	session, err := loader.Load(ctx, root)
	if err != nil {
		return outcome, fmt.Errorf("load initialized config %s: %w", cfgPath, err)
	}
	cfg := session.Config()
	composed := composePublisher(session)
	if !configExists {
		collisions, err = composed.InitCollisions()
		if err != nil {
			return outcome, fmt.Errorf("preflight initialization outputs: %w", err)
		}
		if len(collisions) > 0 {
			return outcome, collisionRefusal(collisions)
		}
	}
	if err := gate(ctx, root); err != nil {
		return outcome, fmt.Errorf("gate initialized project %s: %w", root, err)
	}

	var result publisher.Result
	if !lockExists {
		// A prior attempt may already have committed config.yaml. Its strictly
		// loaded contents select the same first-adoption plan; initialization
		// publishes the permanent lock last without recreating the config.
		result, err = composed.InitializeLeased(ctx, lease, publisher.InitAuthority{InitializedWithVersion: project.Version})
	} else {
		result, err = composed.SyncLeased(ctx, lease)
	}
	mutation, mutationErr := resultMutation(result)
	outcome.Sync = mutation
	outcome.Touched = result.Touched()
	if err != nil {
		err = fmt.Errorf("publish initialized project %s: %w", root, err)
	}
	if mutationErr != nil {
		mutationErr = fmt.Errorf("render initialization result for %s: %w", root, mutationErr)
	}
	if err != nil || mutationErr != nil {
		return outcome, errors.Join(err, mutationErr)
	}
	advisories, err := advisoryNotes(session, cfg, composed)
	if err != nil {
		return outcome, fmt.Errorf("derive advisories for %s: %w", cfgPath, err)
	}
	outcome.Advisories = advisories
	outcome.NextActions = append([]string(nil), nextActions[:]...)
	return outcome, nil
}

func composePublisher(session *project.Session) *publisher.Publisher {
	return publisher.New(session, project.Version)
}

type scaffoldFilesystem interface {
	CreateDirectory(string, fs.FileMode) (*filesystem.ExpectedIdentity, error)
	Publish(string, []byte, fs.FileMode) error
	LinkInfo(string) (fs.FileInfo, error)
}

func createScaffold(handle scaffoldFilesystem, contents []byte) (bool, error) {
	_, dirErr := handle.LinkInfo(config.DirName)
	if dirErr != nil && !errors.Is(dirErr, fs.ErrNotExist) {
		return false, fmt.Errorf("inspect initialization directory %s: %w", config.DirName, dirErr)
	}
	if errors.Is(dirErr, fs.ErrNotExist) {
		identity, err := handle.CreateDirectory(config.DirName, 0o755)
		if identity != nil {
			defer func() { _ = identity.Release() }()
		}
		if err != nil {
			return false, fmt.Errorf("create initialization directory %s exclusively: %w", config.DirName, err)
		}
	}
	configRel := filepath.ToSlash(filepath.Join(config.DirName, "config.yaml"))
	if err := handle.Publish(configRel, contents, 0o644); err != nil {
		return false, fmt.Errorf("create initialization config %s exclusively: %w", configRel, err)
	}
	return true, nil
}

func resultMutation(result publisher.Result) (presentation.Mutation, error) {
	groups := []presentation.MutationChange{}
	if changes := result.Changes(); len(changes) != 0 {
		values := make([]presentation.Value, 0, len(changes))
		for _, change := range changes {
			value, err := presentation.Literal(change.Path)
			if err != nil {
				return presentation.Mutation{}, err
			}
			values = append(values, value)
		}
		groups = append(groups, presentation.MutationChange{Label: "outputs", Values: values})
	}
	if pruned := result.Pruned(); len(pruned) != 0 {
		values := make([]presentation.Value, 0, len(pruned))
		for _, path := range pruned {
			value, err := presentation.Literal(path)
			if err != nil {
				return presentation.Mutation{}, err
			}
			values = append(values, value)
		}
		groups = append(groups, presentation.MutationChange{Label: "pruned", Values: values})
	}
	return presentation.Mutation{Status: "completed", Changes: groups}, nil
}

func collisionRefusal(collisions []string) error {
	return fmt.Errorf("awf init: refusing to overwrite existing files:\n  %s", strings.Join(collisions, "\n  "))
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
