package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/execution"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"golang.org/x/mod/semver"
)

const (
	repoStepDrift  execution.StepID = "drift"
	repoStepState  execution.StepID = "state"
	repoStepProse  execution.StepID = "prose"
	repoStepMemory execution.StepID = "memory"

	repoRequirementConfig       execution.RequirementID = "config"
	repoRequirementProject      execution.RequirementID = "project"
	repoRequirementCheckReport  execution.RequirementID = "check-report"
	repoRequirementCurrentState execution.RequirementID = "current-state"
	repoRequirementIndex        execution.RequirementID = "index"
)

type repoCheckInputs struct {
	config       *config.Config
	project      *project.Project
	checkReport  project.CheckReport
	currentState project.CurrentStateReport
	index        *snapshot.Tree
}

type repoCheckDependencies struct {
	loadConfig   func(string) (*config.Config, error)
	openProject  func(context.Context, string, *config.Config) (*project.Project, error)
	checkReport  func(context.Context, *project.Project) (project.CheckReport, error)
	currentState func(context.Context, *project.Project) (project.CurrentStateReport, error)
	indexTree    func(context.Context, string) (*snapshot.Tree, error)
}

func productionRepoCheckDependencies() repoCheckDependencies {
	return repoCheckDependencies{
		loadConfig: config.Load,
		openProject: func(ctx context.Context, root string, cfg *config.Config) (*project.Project, error) {
			repo, _, err := awfgit.OpenContaining(root)
			if err != nil {
				if !errors.Is(err, awfgit.ErrNotARepository) {
					return nil, err
				}
				return project.NewLoaderWithoutRepository(func(dir string) (*config.Config, error) {
					if dir != config.RootDir(root) { // coverage-ignore: Loader.Open requests exactly the selected root's config directory
						return nil, fmt.Errorf("unexpected config root %q", dir)
					}
					return cfg, nil
				}, catalog.Standard, awfgit.ProjectResidentRoot).Open(ctx, root)
			}
			return project.NewLoader(func(dir string) (*config.Config, error) {
				if dir != config.RootDir(root) { // coverage-ignore: Loader.Open requests exactly the selected root's config directory
					return nil, fmt.Errorf("unexpected config root %q", dir)
				}
				return cfg, nil
			}, catalog.Standard, awfgit.ProjectResidentRoot, repo).Open(ctx, root)
		},
		checkReport: func(ctx context.Context, p *project.Project) (project.CheckReport, error) { return p.CheckReport(ctx) },
		currentState: func(ctx context.Context, p *project.Project) (project.CurrentStateReport, error) {
			return p.CheckCurrentState(ctx)
		},
		indexTree: func(ctx context.Context, root string) (*snapshot.Tree, error) {
			tree, err := stagedTree(ctx, root)
			if err != nil {
				return nil, fmt.Errorf("cannot read staged files: %w", err)
			}
			return tree, nil
		},
	}
}

// repoCheckSystem defines one operation-local capability graph. It freezes typed
// prepared inputs into output actions only after all selected requirements work.
func repoCheckSystem(root string, stdout io.Writer, aggregate bool, inputs *repoCheckInputs, deps repoCheckDependencies) execution.System {
	return execution.System{
		Requirements: []execution.Requirement{
			{ID: repoRequirementConfig, Prepare: func(context.Context) error {
				cfg, err := deps.loadConfig(config.RootDir(root))
				inputs.config = cfg
				return err
			}},
			{ID: repoRequirementProject, Dependencies: []execution.RequirementID{repoRequirementConfig}, Prepare: func(ctx context.Context) error {
				p, err := deps.openProject(ctx, root, inputs.config)
				inputs.project = p
				return err
			}},
			{ID: repoRequirementCheckReport, Dependencies: []execution.RequirementID{repoRequirementProject}, Prepare: func(ctx context.Context) error {
				r, err := deps.checkReport(ctx, inputs.project)
				inputs.checkReport = r
				return err
			}},
			{ID: repoRequirementCurrentState, Dependencies: []execution.RequirementID{repoRequirementProject}, Prepare: func(ctx context.Context) error {
				r, err := deps.currentState(ctx, inputs.project)
				inputs.currentState = r
				return err
			}},
			{ID: repoRequirementIndex, Dependencies: []execution.RequirementID{repoRequirementConfig}, Prepare: func(ctx context.Context) error {
				tree, err := deps.indexTree(ctx, root)
				inputs.index = tree
				return err
			}},
		},
		Foundations: []execution.RequirementID{repoRequirementConfig},
		Steps: []execution.Step{
			{ID: repoStepDrift, Requirements: func(context.Context) ([]execution.RequirementID, error) {
				return []execution.RequirementID{repoRequirementCheckReport}, nil
			}},
			{ID: repoStepState, Requirements: func(context.Context) ([]execution.RequirementID, error) {
				return []execution.RequirementID{repoRequirementCurrentState}, nil
			}},
			{ID: repoStepProse, Requirements: func(context.Context) ([]execution.RequirementID, error) {
				if inputs.config.ProseGate == nil || !inputs.config.ProseGate.Enabled {
					return nil, nil
				}
				return []execution.RequirementID{repoRequirementIndex}, nil
			}},
			{ID: repoStepMemory, Requirements: func(context.Context) ([]execution.RequirementID, error) {
				if inputs.config.MemoryCite == nil || !inputs.config.MemoryCite.Enabled {
					return nil, nil
				}
				return []execution.RequirementID{repoRequirementIndex}, nil
			}},
		},
		Bind: func(steps []execution.StepID) ([]execution.BoundAction, error) {
			actions := make([]execution.BoundAction, 0, len(steps))
			for _, step := range steps {
				switch step {
				case repoStepDrift:
					report := inputs.checkReport
					actions = append(actions, execution.BoundAction{Step: step, Run: func(context.Context) error {
						if aggregate {
							for _, n := range report.Notes {
								fmt.Fprintf(stdout, "note: %s\n", n)
							}
						}
						return printDrift(stdout, report.Drift)
					}})
				case repoStepState:
					report := inputs.currentState
					actions = append(actions, execution.BoundAction{Step: step, Run: func(context.Context) error { return printCurrentState(stdout, report) }})
				case repoStepProse:
					cfg, tree := inputs.config, inputs.index
					actions = append(actions, execution.BoundAction{Step: step, Run: func(context.Context) error { return runProseAction(stdout, cfg, tree) }})
				case repoStepMemory:
					cfg, tree := inputs.config, inputs.index
					actions = append(actions, execution.BoundAction{Step: step, Run: func(context.Context) error { return runMemoryAction(stdout, cfg, tree) }})
				}
			}
			return actions, nil
		},
	}
}

func runRepoCheckSelection(ctx context.Context, root string, stdout io.Writer, selected []execution.StepID, policy execution.FailurePolicy, aggregate bool, deps repoCheckDependencies) error {
	inputs := &repoCheckInputs{}
	prepared, err := execution.Prepare(ctx, repoCheckSystem(root, stdout, aggregate, inputs, deps), selected)
	if err != nil {
		return err
	}
	outcomes, err := prepared.Run(ctx, policy)
	if err != nil {
		return err
	}
	for _, outcome := range outcomes {
		if outcome.Err != nil {
			return outcome.Err
		}
	}
	return nil
}

// runCheckRepo runs the repository-universe aggregate and owns its version note.
func runCheckRepo(ctx context.Context, root string, stdout io.Writer) error {
	lockV, binV, ok, err := checkLockVsBinary(root)
	if err != nil {
		return err
	}
	if ok && semver.Compare(binV, lockV) > 0 {
		fmt.Fprintf(stdout, "note: awf %s is ahead of this project (rendered by %s); run awf render to re-pin\n", strings.TrimPrefix(binV, "v"), strings.TrimPrefix(lockV, "v"))
	}
	return runRepoCheckSelection(ctx, root, stdout, []execution.StepID{repoStepDrift, repoStepState, repoStepProse, repoStepMemory}, execution.ContinueOnFailure, true, productionRepoCheckDependencies())
}
func runCheckDrift(ctx context.Context, root string, stdout io.Writer) error {
	deps := productionRepoCheckDependencies()
	deps.checkReport = func(ctx context.Context, p *project.Project) (project.CheckReport, error) {
		drift, err := p.Check(ctx)
		return project.CheckReport{Drift: drift}, err
	}
	return runRepoCheckSelection(ctx, root, stdout, []execution.StepID{repoStepDrift}, execution.StopOnFailure, false, deps)
}
func runCheckState(ctx context.Context, root string, stdout io.Writer) error {
	return runRepoCheckSelection(ctx, root, stdout, []execution.StepID{repoStepState}, execution.StopOnFailure, false, productionRepoCheckDependencies())
}

func printDrift(stdout io.Writer, drift []manifest.Drift) error { // placeholder
	for _, d := range drift {
		fmt.Fprintf(stdout, "  %-14s %s: %s\n", d.Kind, d.Path, d.Detail)
	}
	if len(drift) == 0 {
		fmt.Fprintln(stdout, "awf check repo drift: clean")
		return nil
	}
	return fmt.Errorf("awf check repo drift: %d drift(s)", len(drift))
}
func printCurrentState(stdout io.Writer, report project.CurrentStateReport) error {
	for _, n := range report.Notes() {
		fmt.Fprintf(stdout, "note: %s\n", n)
	}
	current := report.Findings()
	for _, f := range current {
		fmt.Fprintf(stdout, "  %-14s %s\n", "current-state", f)
	}
	if len(current) == 0 {
		fmt.Fprintln(stdout, "awf check repo state: clean")
		return nil
	}
	return fmt.Errorf("awf check repo state: %d current-state issue(s)", len(current))
}
func checkLockVsBinary(root string) (lockV, binV string, ok bool, err error) {
	return lockVsBinary(root)
}
