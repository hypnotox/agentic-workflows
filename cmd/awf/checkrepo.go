package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/execution"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
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
	categories   []presentation.ReportCategory
	notes        []string
}

type repoCheckDependencies struct {
	loadConfig             func(string) (*config.Config, error)
	openProject            func(context.Context, string, *config.Config) (*project.Project, error)
	checkReport            func(context.Context, *project.Project) (project.CheckReport, error)
	currentState           func(context.Context, *project.Project) (project.CurrentStateReport, error)
	indexTree              func(context.Context, string) (*snapshot.Tree, error)
	driftCategories        func([]manifest.Drift, bool) ([]presentation.ReportCategory, error)
	currentStateCategories func(project.CurrentStateReport, bool) ([]presentation.ReportCategory, error)
}

type repoIndexPreparationError struct{ err error }

func (e *repoIndexPreparationError) Error() string { return e.err.Error() }
func (e *repoIndexPreparationError) Unwrap() error { return e.err }

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
		driftCategories:        project.DriftCategories,
		currentStateCategories: project.CurrentStateCategories,
		indexTree: func(ctx context.Context, root string) (*snapshot.Tree, error) {
			tree, err := stagedTree(ctx, root)
			if err != nil {
				return nil, &repoIndexPreparationError{err: fmt.Errorf("cannot read staged files: %w", err)}
			}
			return tree, nil
		},
	}
}

// repoCheckSystem defines one operation-local capability graph. It freezes typed
// prepared inputs into output actions only after all selected requirements work.
func repoCheckSystem(root string, aggregate bool, leadingNotes []string, planNotes planNoteSink, inputs *repoCheckInputs, deps repoCheckDependencies) execution.System {
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
							inputs.notes = append(inputs.notes, leadingNotes...)
							inputs.notes = append(inputs.notes, report.Notes...)
							for _, note := range report.PlanNotes {
								if _, seen := planNotes[note]; !seen {
									planNotes[note] = struct{}{}
									inputs.notes = append(inputs.notes, note)
								}
							}
						}
						categories, err := deps.driftCategories(report.Drift, false)
						if err != nil {
							return err
						}
						inputs.categories = append(inputs.categories, categories...)
						if len(report.Drift) > 0 {
							return producedCheckFailure{errors.New("check repo drift failed")}
						}
						return nil
					}})
				case repoStepState:
					report := inputs.currentState
					actions = append(actions, execution.BoundAction{Step: step, Run: func(context.Context) error {
						inputs.notes = append(inputs.notes, report.Notes()...)
						categories, err := deps.currentStateCategories(report, false)
						if err != nil {
							return err
						}
						inputs.categories = append(inputs.categories, categories...)
						if len(report.Findings()) > 0 {
							return producedCheckFailure{errors.New("check repo state failed")}
						}
						return nil
					}})
				case repoStepProse:
					cfg, tree := inputs.config, inputs.index
					actions = append(actions, execution.BoundAction{Step: step, Run: func(context.Context) error {
						categories, err := proseCheckFindings(cfg, tree)
						inputs.categories = append(inputs.categories, categories...)
						return err
					}})
				case repoStepMemory:
					cfg, tree := inputs.config, inputs.index
					actions = append(actions, execution.BoundAction{Step: step, Run: func(context.Context) error {
						categories, err := memoryCheckFindings(cfg, tree)
						inputs.categories = append(inputs.categories, categories...)
						return err
					}})
				}
			}
			return actions, nil
		},
	}
}

func runRepoCheckSelection(ctx context.Context, root string, stdout io.Writer, selected []execution.StepID, policy execution.FailurePolicy, aggregate bool, deps repoCheckDependencies) error {
	return runRepoCheckSelectionWithPlanNotes(ctx, root, stdout, selected, policy, aggregate, nil, planNoteSink{}, deps)
}

func runRepoCheckSelectionWithPlanNotes(ctx context.Context, root string, stdout io.Writer, selected []execution.StepID, policy execution.FailurePolicy, aggregate bool, leadingNotes []string, planNotes planNoteSink, deps repoCheckDependencies) error {
	collection, err := collectRepoCheckSelectionWithPlanNotes(ctx, root, selected, policy, aggregate, leadingNotes, planNotes, deps)
	if err != nil {
		return err
	}
	return renderCheckCollection(stdout, collection)
}

func collectRepoCheckSelectionWithPlanNotes(ctx context.Context, root string, selected []execution.StepID, policy execution.FailurePolicy, aggregate bool, leadingNotes []string, planNotes planNoteSink, deps repoCheckDependencies) (checkCollection, error) {
	inputs := &repoCheckInputs{}
	prepared, err := execution.Prepare(ctx, repoCheckSystem(root, aggregate, leadingNotes, planNotes, inputs, deps), selected)
	if err != nil {
		var indexErr *repoIndexPreparationError
		if errors.As(err, &indexErr) {
			return checkCollection{}, fmt.Errorf("%s: %w", repoScannerErrorPrefix(selected, inputs.config), indexErr)
		}
		return checkCollection{}, err
	}
	outcomes, err := prepared.Run(ctx, policy)
	if err != nil {
		return checkCollection{}, err
	}
	collection := checkCollection{notes: inputs.notes, categories: inputs.categories}
	for _, outcome := range outcomes {
		if outcome.Err == nil {
			continue
		}
		var produced producedCheckFailure
		if errors.As(outcome.Err, &produced) {
			collection.failures = append(collection.failures, outcome.Err)
			continue
		}
		collection.operational = append(collection.operational, outcome.Err)
	}
	return collection, nil
}

func renderCheckCollection(stdout io.Writer, collection checkCollection) error {
	// A report is complete produced evidence. Operational step failures mean it
	// cannot be complete, even when continuation ran later selected steps.
	if len(collection.operational) > 0 {
		return errors.Join(collection.operational...)
	}
	report, err := checkReport(collection.notes, collection.categories)
	if err != nil {
		return err
	}
	document, err := report.Document()
	if err != nil {
		return err
	}
	if err := presentation.Render(stdout, document); err != nil {
		return err
	}
	if len(collection.failures) > 0 {
		return &producedReportError{errors.Join(collection.failures...)}
	}
	return nil
}

func repoScannerErrorPrefix(selected []execution.StepID, cfg *config.Config) string {
	for _, step := range []execution.StepID{repoStepProse, repoStepMemory} {
		if !slices.Contains(selected, step) {
			continue
		}
		if step == repoStepProse && cfg.ProseGate != nil && cfg.ProseGate.Enabled {
			return "check repo prose"
		}
		if step == repoStepMemory && cfg.MemoryCite != nil && cfg.MemoryCite.Enabled {
			return "check repo memory"
		}
	}
	panic("repo index preparation without a selected enabled scanner") // coverage-ignore: only enabled scanner resolvers request the index requirement
}

// runCheckRepo runs the repository-universe aggregate and owns its version note.
func runCheckRepo(ctx context.Context, root string, stdout io.Writer) error {
	return runCheckRepoWithPlanNotes(ctx, root, stdout, planNoteSink{})
}

func runCheckRepoWithPlanNotes(ctx context.Context, root string, stdout io.Writer, planNotes planNoteSink) error {
	collection, err := collectCheckRepoWithPlanNotes(ctx, root, planNotes)
	if err != nil {
		return err
	}
	return renderCheckCollection(stdout, collection)
}

func collectCheckRepoWithPlanNotes(ctx context.Context, root string, planNotes planNoteSink) (checkCollection, error) {
	lockV, binV, ok, err := checkLockVsBinary(root)
	if err != nil {
		return checkCollection{}, err
	}
	var leadingNotes []string
	if ok && semver.Compare(binV, lockV) > 0 {
		leadingNotes = append(leadingNotes, fmt.Sprintf("awf %s is ahead of this project (rendered by %s); run awf render to re-pin", strings.TrimPrefix(binV, "v"), strings.TrimPrefix(lockV, "v")))
	}
	return collectRepoCheckSelectionWithPlanNotes(ctx, root, []execution.StepID{repoStepDrift, repoStepState, repoStepProse, repoStepMemory}, execution.ContinueOnFailure, true, leadingNotes, planNotes, productionRepoCheckDependencies())
}
func runCheckDrift(ctx context.Context, root string, stdout io.Writer) error {
	deps := productionRepoCheckDependencies()
	// Preserve Project.Check as the production compatibility projection. It
	// still derives one complete CheckReport; this adapter discards only notes
	// before the shared direct action, which has no advisory presentation role.
	deps.checkReport = func(ctx context.Context, p *project.Project) (project.CheckReport, error) {
		drift, err := p.Check(ctx)
		return project.CheckReport{Drift: drift}, err
	}
	return runRepoCheckSelection(ctx, root, stdout, []execution.StepID{repoStepDrift}, execution.StopOnFailure, false, deps)
}
func runCheckState(ctx context.Context, root string, stdout io.Writer) error {
	return runRepoCheckSelection(ctx, root, stdout, []execution.StepID{repoStepState}, execution.StopOnFailure, false, productionRepoCheckDependencies())
}

func checkLockVsBinary(root string) (lockV, binV string, ok bool, err error) {
	return lockVsBinary(root)
}
