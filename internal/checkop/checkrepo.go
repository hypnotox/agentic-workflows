package checkop

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	"github.com/hypnotox/agentic-workflows/internal/execution"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/repositorycheck"
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
	projectState *project.ProjectState
	repo         *awfgit.Repo
	checkReport  project.CheckReport
	currentState currentstatecoord.CurrentStateReport
	index        *snapshot.Tree
	presentation repositorycheck.Presentation
}

type repoCheckDependencies struct {
	loadConfig   func(string) (*config.Config, error)
	openProject  func(context.Context, string, *config.Config) (*project.ProjectState, *awfgit.Repo, error)
	checkReport  func(context.Context, *project.ProjectState, *config.Config, *awfgit.Repo) (project.CheckReport, error)
	currentState func(context.Context, string, *awfgit.Repo) (currentstatecoord.CurrentStateReport, error)
	indexTree    func(context.Context, string) (*snapshot.Tree, error)
	present      func(checkresult.Result, string, bool) (repositorycheck.Presentation, error)
}

type checkResultPresenter func(checkresult.Result, string, bool) (repositorycheck.Presentation, error)

func hasCheckResults(result checkresult.Result) bool {
	return len(result.Findings()) > 0 || len(result.Information()) > 0
}

func informationResult(notes []string) checkresult.Result {
	information := make([]checkresult.Information, 0, len(notes))
	for _, note := range notes {
		information = append(information, checkresult.Information{Evidence: checkresult.Evidence{Kind: "repository-check", Detail: note}})
	}
	result, err := checkresult.New(nil, information)
	if err != nil {
		return checkresult.Result{}
	}
	return result
}

func presentCurrentStateReport(report currentstatecoord.CurrentStateReport, check string, _ planNoteSink, present checkResultPresenter) (repositorycheck.Presentation, error) {
	return present(report.CurrentResult, check, false)
}

type repoIndexPreparationError struct{ err error }

func (e *repoIndexPreparationError) Error() string { return e.err.Error() }
func (e *repoIndexPreparationError) Unwrap() error { return e.err }

func productionRepoCheckDependencies() repoCheckDependencies {
	return repoCheckDependencies{
		loadConfig: config.Load,
		openProject: func(ctx context.Context, root string, cfg *config.Config) (*project.ProjectState, *awfgit.Repo, error) {
			repo, _, err := awfgit.OpenContaining(root)
			if err != nil {
				if !errors.Is(err, awfgit.ErrNotARepository) {
					return nil, nil, err
				}
				state, _, openErr := project.NewLoaderWithoutRepository(func(dir string) (*config.Config, error) {
					if dir != config.RootDir(root) {
						return nil, fmt.Errorf("unexpected config root %q", dir)
					}
					return cfg, nil
				}, catalog.Standard, awfgit.ProjectResidentRoot).OpenForOperation(ctx, root)
				return state, nil, openErr
			}
			state, _, openErr := project.NewLoader(func(dir string) (*config.Config, error) {
				if dir != config.RootDir(root) {
					return nil, fmt.Errorf("unexpected config root %q", dir)
				}
				return cfg, nil
			}, catalog.Standard, awfgit.ProjectResidentRoot, repo).OpenForOperation(ctx, root)
			return state, repo, openErr
		},
		checkReport: func(ctx context.Context, state *project.ProjectState, cfg *config.Config, repo *awfgit.Repo) (project.CheckReport, error) {
			prepared, err := operationPreparation(state, cfg)
			if err != nil {
				return project.CheckReport{}, err
			}
			return project.BuildCheckReport(state, cfg, repo, ctx, prepared.Plan(), project.OperationSemantics{
				Pitfalls: prepared.Pitfalls(), Topics: prepared.Topics(), EffectiveSkills: prepared.EffectiveSkills(),
				GeneratedOutput: prepared.GeneratedOutput(), Glossary: prepared.Glossary(),
			})
		},
		currentState: func(ctx context.Context, root string, repo *awfgit.Repo) (currentstatecoord.CurrentStateReport, error) {
			return currentstatecoord.CheckWorking(root, repo, ctx)
		},
		present: func(result checkresult.Result, check string, evidence bool) (repositorycheck.Presentation, error) {
			if evidence {
				return repositorycheck.PresentEvidence(result, check)
			}
			return repositorycheck.Present(result, check)
		},
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
				state, repo, err := deps.openProject(ctx, root, inputs.config)
				inputs.projectState = state
				inputs.repo = repo
				return err
			}},
			{ID: repoRequirementCheckReport, Dependencies: []execution.RequirementID{repoRequirementProject}, Prepare: func(ctx context.Context) error {
				r, err := deps.checkReport(ctx, inputs.projectState, inputs.config, inputs.repo)
				inputs.checkReport = r
				return err
			}},
			{ID: repoRequirementCurrentState, Dependencies: []execution.RequirementID{repoRequirementProject}, Prepare: func(ctx context.Context) error {
				r, err := deps.currentState(ctx, root, inputs.repo)
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
				return []execution.RequirementID{repoRequirementIndex}, nil
			}},
			{ID: repoStepMemory, Requirements: func(context.Context) ([]execution.RequirementID, error) {
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
						tracking, err := deps.present(report.TrackingInformationResult(), "advisory", false)
						if err != nil {
							return err
						}
						inputs.presentation = inputs.presentation.Append(tracking)
						if aggregate {
							leading := informationResult(leadingNotes)
							projected, err := deps.present(leading, "advisory", false)
							if err != nil {
								return err
							}
							inputs.presentation = inputs.presentation.Append(projected)
							projected, err = deps.present(report.AggregateAdvisoryResult(), "advisory", false)
							if err != nil {
								return err
							}
							inputs.presentation = inputs.presentation.Append(projected)
						}
						projected, err := deps.present(report.DirectResult, "drift", true)
						if err != nil {
							return err
						}
						inputs.presentation = inputs.presentation.Append(projected)
						if repositorycheck.HasErrors(report.DirectResult) {
							return producedCheckFailure{errors.New("check repo drift failed")}
						}
						return nil
					}})
				case repoStepState:
					report := inputs.currentState
					actions = append(actions, execution.BoundAction{Step: step, Run: func(context.Context) error {
						projected, err := presentCurrentStateReport(report, "current-state", planNotes, deps.present)
						if err != nil {
							return err
						}
						inputs.presentation = inputs.presentation.Append(projected)
						if repositorycheck.HasErrors(report.Result()) {
							return producedCheckFailure{errors.New("check repo state failed")}
						}
						return nil
					}})
				case repoStepProse:
					cfg, tree := inputs.config, inputs.index
					actions = append(actions, execution.BoundAction{Step: step, Run: func(context.Context) error {
						result, err := proseCheckResult(cfg, tree)
						if err != nil {
							return err
						}
						projected, err := deps.present(result, "prose", false)
						inputs.presentation = inputs.presentation.Append(projected)
						return err
					}})
				case repoStepMemory:
					cfg, tree := inputs.config, inputs.index
					actions = append(actions, execution.BoundAction{Step: step, Run: func(context.Context) error {
						result, err := memoryCheckResult(cfg, tree)
						projected, projectErr := deps.present(result, "memory", false)
						inputs.presentation = inputs.presentation.Append(projected)
						if projectErr != nil {
							return projectErr
						}
						return err
					}})
				}
			}
			return actions, nil
		},
	}
}

func collectRepoCheckSelectionWithPlanNotes(ctx context.Context, root string, selected []execution.StepID, policy execution.FailurePolicy, aggregate bool, leadingNotes []string, planNotes planNoteSink, deps repoCheckDependencies) (checkCollection, error) {
	inputs := &repoCheckInputs{}
	prepared, err := execution.Prepare(ctx, repoCheckSystem(root, aggregate, leadingNotes, planNotes, inputs, deps), selected)
	if err != nil {
		var indexErr *repoIndexPreparationError
		if errors.As(err, &indexErr) {
			return checkCollection{}, fmt.Errorf("%s: %w", repoScannerErrorPrefix(selected), indexErr)
		}
		return checkCollection{}, err
	}
	outcomes, err := prepared.Run(ctx, policy)
	if err != nil {
		return checkCollection{}, err
	}
	collection := checkCollection{presentation: inputs.presentation}
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

func repoScannerErrorPrefix(selected []execution.StepID) string {
	for _, step := range []execution.StepID{repoStepProse, repoStepMemory} {
		if !slices.Contains(selected, step) {
			continue
		}
		if step == repoStepProse {
			return "check repo prose"
		}
		if step == repoStepMemory {
			return "check repo memory"
		}
	}
	panic("repo index preparation without a selected scanner")
}

// collectCheckRepoWithPlanNotes runs the repository-universe aggregate and owns its version note.
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

func checkLockVsBinary(root string) (lockV, binV string, ok bool, err error) {
	return lockVsBinary(root)
}
