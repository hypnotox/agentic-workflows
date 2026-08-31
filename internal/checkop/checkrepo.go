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
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"golang.org/x/mod/semver"
)

type repositoryLane uint8

const (
	repositoryDrift repositoryLane = iota + 1
	repositoryState
	repositoryProse
	repositoryMemory
)

func orderedRepositoryLanes() []repositoryLane {
	return []repositoryLane{repositoryDrift, repositoryState, repositoryProse, repositoryMemory}
}

type repoCheckInputs struct {
	config       *config.Config
	session      *project.Session
	checkReport  project.CheckReport
	currentState currentstatecoord.CurrentStateReport
	index        *snapshot.Tree
	collection   checkCollection
}

type repoCheckDependencies struct {
	loadConfig   func(string) (*config.Config, error)
	loadSession  func(context.Context, string, *config.Config) (*project.Session, error)
	checkReport  func(context.Context, *project.Session) (project.CheckReport, error)
	currentState func(context.Context, *project.Session) (currentstatecoord.CurrentStateReport, error)
	indexTree    func(context.Context, string) (*snapshot.Tree, error)
}

func hasCheckResults(result checkresult.Result) bool {
	return len(result.Findings()) > 0 || len(result.Information()) > 0
}

func hasErrors(result checkresult.Result) bool {
	for _, finding := range result.Findings() {
		if finding.Rank == severity.Error {
			return true
		}
	}
	return false
}

func informationResult(notes []string) (checkresult.Result, error) {
	information := make([]checkresult.Information, 0, len(notes))
	for _, note := range notes {
		information = append(information, checkresult.Information{Evidence: checkresult.Evidence{Kind: "repository-check", Detail: note}})
	}
	return checkresult.New(nil, information)
}

type repoIndexPreparationError struct{ err error }

func (e *repoIndexPreparationError) Error() string { return e.err.Error() }
func (e *repoIndexPreparationError) Unwrap() error { return e.err }

func productionRepoCheckDependencies() repoCheckDependencies {
	return repoCheckDependencies{
		loadConfig: config.Load,
		loadSession: func(ctx context.Context, root string, cfg *config.Config) (*project.Session, error) {
			repo, _, err := awfgit.OpenContaining(root)
			load := func(dir string) (*config.Config, error) {
				if dir != config.RootDir(root) {
					return nil, fmt.Errorf("unexpected config root %q", dir)
				}
				return cfg, nil
			}
			if err != nil {
				if !errors.Is(err, awfgit.ErrNotARepository) {
					return nil, err
				}
				return project.NewLoaderWithoutRepository(load, catalog.Standard, awfgit.ProjectResidentRoot).Load(ctx, root)
			}
			return project.NewLoader(load, catalog.Standard, awfgit.ProjectResidentRoot, repo).Load(ctx, root)
		},
		checkReport: func(ctx context.Context, session *project.Session) (project.CheckReport, error) {
			operation := composePublisher(session)
			plan, err := operation.Plan()
			if err != nil {
				return project.CheckReport{}, err
			}
			pitfalls, err := operation.Pitfalls()
			if err != nil {
				return project.CheckReport{}, err
			}
			skills, err := operation.EffectiveSkills()
			if err != nil {
				return project.CheckReport{}, err
			}
			generated, err := operation.GeneratedOutput()
			if err != nil {
				return project.CheckReport{}, err
			}
			glossary, err := operation.Glossary()
			if err != nil {
				return project.CheckReport{}, err
			}
			return project.BuildCheckReport(session, session.Config(), session.Repository(), ctx, plan, pitfalls, skills, generated, glossary)
		},
		currentState: func(ctx context.Context, session *project.Session) (currentstatecoord.CurrentStateReport, error) {
			return currentstatecoord.CheckWorking(session.Root(), session.Repository(), ctx)
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

// prepareRepositoryChecks makes the real lane dependencies explicit. Every
// selected input is ready before any result is collected, preserving the
// operation's no-partial-report preparation barrier.
func prepareRepositoryChecks(ctx context.Context, root string, selected []repositoryLane, deps repoCheckDependencies) (*repoCheckInputs, error) {
	for _, lane := range selected {
		if !slices.Contains(orderedRepositoryLanes(), lane) {
			return nil, fmt.Errorf("unknown repository check lane %d", lane)
		}
	}
	inputs := &repoCheckInputs{}
	cfg, err := deps.loadConfig(config.RootDir(root))
	if err != nil {
		return nil, fmt.Errorf("prepare requirement %q: %w", "config", err)
	}
	inputs.config = cfg

	needsSession := slices.Contains(selected, repositoryDrift) || slices.Contains(selected, repositoryState)
	if needsSession {
		session, err := deps.loadSession(ctx, root, cfg)
		if err != nil {
			return nil, fmt.Errorf("prepare requirement %q: %w", "project", err)
		}
		inputs.session = session
	}
	if slices.Contains(selected, repositoryDrift) {
		report, err := deps.checkReport(ctx, inputs.session)
		if err != nil {
			return nil, fmt.Errorf("prepare requirement %q: %w", "check-report", err)
		}
		inputs.checkReport = report
	}
	if slices.Contains(selected, repositoryState) {
		report, err := deps.currentState(ctx, inputs.session)
		if err != nil {
			return nil, fmt.Errorf("prepare requirement %q: %w", "current-state", err)
		}
		inputs.currentState = report
	}
	if slices.Contains(selected, repositoryProse) || slices.Contains(selected, repositoryMemory) {
		tree, err := deps.indexTree(ctx, root)
		if err != nil {
			return nil, fmt.Errorf("prepare requirement %q: %w", "index", err)
		}
		inputs.index = tree
	}
	return inputs, nil
}

func collectRepoCheckSelection(ctx context.Context, root string, selected []repositoryLane, continueOnFailure, aggregate bool, leadingNotes []string, deps repoCheckDependencies) (checkCollection, error) {
	inputs, err := prepareRepositoryChecks(ctx, root, selected, deps)
	if err != nil {
		var indexErr *repoIndexPreparationError
		if errors.As(err, &indexErr) {
			return checkCollection{}, fmt.Errorf("%s: %w", repoScannerErrorPrefix(selected), indexErr)
		}
		return checkCollection{}, err
	}
	for _, lane := range orderedRepositoryLanes() {
		if !slices.Contains(selected, lane) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return checkCollection{}, err
		}
		laneErr := collectRepositoryLane(inputs, lane, aggregate, leadingNotes)
		if laneErr != nil {
			laneErr = fmt.Errorf("execute step %q: %w", laneName(lane), laneErr)
		}
		if err := ctx.Err(); err != nil {
			return checkCollection{}, err
		}
		if laneErr != nil {
			var produced producedCheckFailure
			if errors.As(laneErr, &produced) {
				inputs.collection.failures = append(inputs.collection.failures, laneErr)
			} else {
				inputs.collection.operational = append(inputs.collection.operational, laneErr)
			}
			if !continueOnFailure {
				break
			}
		}
	}
	return inputs.collection, nil
}

func collectRepositoryLane(inputs *repoCheckInputs, lane repositoryLane, aggregate bool, leadingNotes []string) error {
	switch lane {
	case repositoryDrift:
		report := inputs.checkReport
		inputs.collection.add("advisory", report.TrackingInformationResult(), false)
		if aggregate {
			leading, err := informationResult(leadingNotes)
			if err != nil {
				return err
			}
			inputs.collection.add("advisory", leading, false)
			inputs.collection.add("advisory", report.AggregateAdvisoryResult(), false)
		}
		inputs.collection.add("drift", report.DirectResult, true)
		if hasErrors(report.DirectResult) {
			return producedCheckFailure{errors.New("check repo drift failed")}
		}
	case repositoryState:
		report := inputs.currentState
		inputs.collection.add("current-state", report.CurrentResult, false)
		if hasErrors(report.Result()) {
			return producedCheckFailure{errors.New("check repo state failed")}
		}
	case repositoryProse:
		result, err := proseCheckResult(inputs.config, inputs.index)
		inputs.collection.add("prose", result, false)
		return err
	case repositoryMemory:
		result, err := memoryCheckResult(inputs.config, inputs.index)
		inputs.collection.add("memory", result, false)
		return err
	default:
		return fmt.Errorf("unknown repository check lane %d", lane)
	}
	return nil
}

func laneName(lane repositoryLane) string {
	switch lane {
	case repositoryDrift:
		return "drift"
	case repositoryState:
		return "state"
	case repositoryProse:
		return "prose"
	case repositoryMemory:
		return "memory"
	default:
		return fmt.Sprintf("lane-%d", lane)
	}
}

func repoScannerErrorPrefix(selected []repositoryLane) string {
	for _, lane := range []repositoryLane{repositoryProse, repositoryMemory} {
		if !slices.Contains(selected, lane) {
			continue
		}
		if lane == repositoryProse {
			return "check repo prose"
		}
		return "check repo memory"
	}
	panic("repo index preparation without a selected scanner")
}

// collectCheckRepo runs the repository-universe aggregate and owns its version note.
func collectCheckRepo(ctx context.Context, root string) (checkCollection, error) {
	lockV, binV, ok, err := checkLockVsBinary(root)
	if err != nil {
		return checkCollection{}, err
	}
	var leadingNotes []string
	if ok && semver.Compare(binV, lockV) > 0 {
		leadingNotes = append(leadingNotes, fmt.Sprintf("awf %s is ahead of this project (rendered by %s); run awf render to re-pin", strings.TrimPrefix(binV, "v"), strings.TrimPrefix(lockV, "v")))
	}
	return collectRepoCheckSelection(ctx, root, orderedRepositoryLanes(), true, true, leadingNotes, productionRepoCheckDependencies())
}

func checkLockVsBinary(root string) (lockV, binV string, ok bool, err error) {
	return lockVsBinary(root)
}
