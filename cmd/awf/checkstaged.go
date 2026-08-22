package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/repositorycheck"
	"golang.org/x/mod/semver"
)

type checkStagedDependencies struct {
	stateRoot func(context.Context, string) (project.CurrentStateReport, error)
	driftRoot func(context.Context, string) (checkresult.Result, error)
	present   func(checkresult.Result, string, bool) (repositorycheck.Presentation, error)
}

func productionCheckStagedDependencies() checkStagedDependencies {
	return checkStagedDependencies{
		stateRoot: project.CheckStagedRoot,
		driftRoot: stagedDriftResult,
		present: func(result checkresult.Result, check string, evidence bool) (repositorycheck.Presentation, error) {
			if evidence {
				return repositorycheck.PresentEvidence(result, check)
			}
			return repositorycheck.Present(result, check)
		},
	}
}

// runCheckStaged runs the staged transition universe. The commit child is direct-only.
func runCheckStaged(ctx context.Context, root string, stdout io.Writer) error {
	return runCheckStagedWithPlanNotes(ctx, root, stdout, planNoteSink{})
}

func collectCheckStaged(ctx context.Context, root string, planNotes planNoteSink) (checkCollection, error) {
	return collectCheckStagedWith(ctx, root, planNotes, productionCheckStagedDependencies())
}

func collectCheckStagedWith(ctx context.Context, root string, planNotes planNoteSink, dependencies checkStagedDependencies) (checkCollection, error) {
	return collectCheckStagedSelectionWith(ctx, root, planNotes, true, true, dependencies)
}

func collectCheckStagedSelection(ctx context.Context, root string, planNotes planNoteSink, state, drift bool) (checkCollection, error) {
	return collectCheckStagedSelectionWith(ctx, root, planNotes, state, drift, productionCheckStagedDependencies())
}

func collectCheckStagedSelectionWith(ctx context.Context, root string, planNotes planNoteSink, state, drift bool, dependencies checkStagedDependencies) (checkCollection, error) {
	lock, err := stagedLock(ctx, root)
	if err != nil && !errors.Is(err, errNoStagedLock) {
		return checkCollection{}, err
	}
	collection := checkCollection{}
	if err != nil {
		// Drift can still construct an actionable membership finding without the
		// lock. State cannot load its staged authority, but the aggregate must
		// retain the drift result rather than returning before it is collected.
		if state && !drift {
			collection.operational = append(collection.operational, err)
		}
	} else {
		lockV, binV, ok := lockVsBinaryLock(lock)
		if ok && semver.Compare(binV, lockV) > 0 {
			collection.information = append(collection.information, fmt.Sprintf("awf %s is ahead of this project (rendered by %s); run awf render to re-pin", strings.TrimPrefix(binV, "v"), strings.TrimPrefix(lockV, "v")))
		}
	}
	if state && lock != nil {
		report, err := dependencies.stateRoot(ctx, root)
		if err != nil {
			collection.operational = append(collection.operational, err)
		} else {
			ordinary := report
			ordinary.PlanNotes = nil
			collection.warnings = append(collection.warnings, ordinary.Warnings()...)
			collection.information = append(collection.information, ordinary.Information()...)
			for _, note := range report.PlanNotes {
				if _, seen := planNotes[note]; !seen {
					planNotes[note] = struct{}{}
					collection.warnings = append(collection.warnings, note)
				}
			}
			result := report.Result()
			projected, projectErr := dependencies.present(repositorycheck.ErrorsOnly(result), "staged current-state", false)
			if projectErr != nil {
				collection.operational = append(collection.operational, projectErr)
			} else {
				collection.presentation = collection.presentation.Append(projected)
				if repositorycheck.HasErrors(result) {
					collection.failures = append(collection.failures, errors.New("check staged state failed"))
				}
			}
		}
	}
	if drift {
		result, err := dependencies.driftRoot(ctx, root)
		if err != nil {
			collection.operational = append(collection.operational, err)
		} else {
			projected, projectErr := dependencies.present(result, "staged drift", true)
			if projectErr != nil {
				collection.operational = append(collection.operational, projectErr)
			} else {
				collection.presentation = collection.presentation.Append(projected)
				if repositorycheck.HasErrors(result) {
					collection.failures = append(collection.failures, errors.New("check staged drift failed"))
				}
			}
		}
	}
	return collection, nil
}

func runCheckStagedWithPlanNotes(ctx context.Context, root string, stdout io.Writer, planNotes planNoteSink) error {
	collection, err := collectCheckStaged(ctx, root, planNotes)
	if err != nil {
		return err
	}
	return renderCheckCollection(stdout, collection)
}

func runCheckStagedState(ctx context.Context, root string, stdout io.Writer) error {
	collection, err := collectCheckStagedSelection(ctx, root, planNoteSink{}, true, false)
	if err != nil {
		return err
	}
	return renderCheckCollection(stdout, collection)
}

func runCheckStagedDrift(ctx context.Context, root string, stdout io.Writer) error {
	collection, err := collectCheckStagedSelection(ctx, root, planNoteSink{}, false, true)
	if err != nil {
		return err
	}
	return renderCheckCollection(stdout, collection)
}
