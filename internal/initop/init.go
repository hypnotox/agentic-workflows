// Package initop owns the initialization application operation.
package initop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/initspec"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
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

// Input contains parsed initialization values and CLI-selected prompt streams.
type Input struct {
	Root         string
	Force        bool
	Answers      map[string]string
	PromptInput  io.Reader
	PromptOutput io.Writer
	Interactive  bool
}

// Run performs one complete initialization operation and returns its semantic
// outcome. Rendering and protocol selection remain with the command.
func Run(ctx context.Context, input Input, loadProject LoadProject, gate Gate) (initspec.Outcome, error) {
	root := input.Root
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
			return initspec.Outcome{}, errors.New("pre-tracking authority: use the bridge release to attest before initializing")
		}
		state, err := lock.AuthorityState()
		if err != nil { // coverage-ignore: LoadOptional parsed and validated this unchanged lock immediately above
			return initspec.Outcome{}, fmt.Errorf("invalid authority: restore .awf/awf.lock from version control: %w", err)
		}
		if state != manifest.AuthorityPermanent {
			return initspec.Outcome{}, errors.New("pre-tracking authority: use the bridge release to attest before initializing")
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

	scaffolded := false
	if !configExists {
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil { // coverage-ignore: entering this block needs cfgPath absent, which precludes a parent collision making MkdirAll fail
			return initspec.Outcome{}, err
		}
		scaffold, err := project.ScaffoldConfigForProfile(filepath.Base(root), vars, scopes, profile)
		if err != nil { // coverage-ignore: ScaffoldConfig renders a static template over a dir basename; cannot fail in practice
			return initspec.Outcome{}, err
		}
		if err := os.WriteFile(cfgPath, scaffold, 0o644); err != nil { // coverage-ignore: post-MkdirAll write; fails only on a permission fault that root bypasses
			return initspec.Outcome{}, err
		}
		scaffolded = true
	}

	loader, err := loadProject(root)
	if err != nil {
		return initspec.Outcome{}, err
	}
	state, cfg, err := loader.OpenForOperation(ctx, root)
	if err != nil {
		return initspec.Outcome{}, err
	}
	composed := composePublisher(state, cfg)
	collisions, err := composed.InitCollisions()
	if err != nil {
		cleanupScaffold(cfgPath, scaffolded)
		return initspec.Outcome{}, err
	}
	if len(collisions) > 0 && !input.Force { // coverage-ignore: the non-force probe plans the same full catalog; force makes this condition false
		cleanupScaffold(cfgPath, scaffolded)
		return initspec.Outcome{}, collisionRefusal(collisions) // coverage-ignore: the identical pre-prompt probe makes this path unreachable
	}
	if err := gate(ctx, root); err != nil {
		return initspec.Outcome{}, err
	}

	var result publisher.Result
	if !configExists && !lockExists {
		result, err = composed.Initialize(publisher.InitAuthority{InitializedWithVersion: project.Version})
	} else {
		result, err = composed.Sync()
	}
	if err != nil {
		cleanupScaffold(cfgPath, scaffolded)
		return initspec.Outcome{}, err
	}
	mutation, err := result.Mutation()
	if err != nil { // coverage-ignore: typed Publisher results and fixed presentation grammar make this mapping failure unreachable
		cleanupScaffold(cfgPath, scaffolded)
		return initspec.Outcome{}, err
	}
	prepared, err := composed.Prepare()
	if err != nil { // coverage-ignore: the same immutable state and operation tree completed Publisher publication immediately above; failure requires a concurrent tree mutation
		return initspec.Outcome{}, err
	}
	advisories, err := project.AdvisoryNotes(state, cfg, prepared.Plan(), projectSemantics(prepared))
	if err != nil { // coverage-ignore: Publisher preparation already validated the same advisory semantic inputs; failure requires a concurrent tree mutation
		return initspec.Outcome{}, err
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

func cleanupScaffold(cfgPath string, scaffolded bool) {
	if !scaffolded {
		return
	}
	_ = os.Remove(cfgPath)
	_ = os.Remove(filepath.Dir(cfgPath))
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
	plan, err := composePublisher(state, cfg).Plan()
	if err != nil { // coverage-ignore: rendering the embedded catalog over a fresh scaffold in an empty tree cannot fail
		return nil, err
	}
	return resident.CollisionsAt(root, plan.Paths())
}

var nextActions = [...]string{
	"fill the Identity section at .awf/parts/agents-doc/identity.md, then run awf render",
	"set still-empty vars in .awf/config.yaml (the notes above list what each artifact misses), then run awf render",
	"wire rendered hook payloads under .awf/hooks/ into git hooks you own (see the workflow doc's local-hooks section); awf never activates hooks itself",
	"commit .awf/ and the rendered files together",
}
