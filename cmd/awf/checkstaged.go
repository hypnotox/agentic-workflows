package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"golang.org/x/mod/semver"
)

type checkStagedDependencies struct {
	stateRoot              func(context.Context, string) (project.CurrentStateReport, error)
	driftRoot              func(context.Context, string) ([]manifest.Drift, error)
	currentStateCategories func(project.CurrentStateReport, bool) ([]presentation.ReportCategory, error)
	driftCategories        func([]manifest.Drift, bool) ([]presentation.ReportCategory, error)
}

func productionCheckStagedDependencies() checkStagedDependencies {
	return checkStagedDependencies{
		stateRoot: project.CheckStagedRoot, driftRoot: project.CheckStagedDriftRoot,
		currentStateCategories: project.CurrentStateCategories, driftCategories: project.DriftCategories,
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
	if err != nil { // coverage-ignore: absent-lock drift is covered by project staged-drift evidence
		// Drift can still construct an actionable membership finding without the
		// lock. State cannot load its staged authority, but the aggregate must
		// retain the drift result rather than returning before it is collected.
		if state && !drift { // coverage-ignore: direct state retains stagedLock's established operational refusal
			collection.operational = append(collection.operational, err)
		}
	} else {
		lockV, binV, ok := lockVsBinaryLock(lock)
		if ok && semver.Compare(binV, lockV) > 0 { // coverage-ignore: staged ahead notices are covered by the direct gate-version command suite
			collection.notes = append(collection.notes, fmt.Sprintf("awf %s is ahead of this project (rendered by %s); run awf render to re-pin", strings.TrimPrefix(binV, "v"), strings.TrimPrefix(lockV, "v")))
		}
	}
	if state && lock != nil {
		report, err := dependencies.stateRoot(ctx, root)
		if err != nil {
			collection.operational = append(collection.operational, err)
		} else {
			ordinary := report
			ordinary.PlanNotes = nil
			collection.notes = append(collection.notes, ordinary.Notes()...)
			for _, note := range report.PlanNotes {
				if _, seen := planNotes[note]; !seen {
					planNotes[note] = struct{}{}
					collection.notes = append(collection.notes, note)
				}
			}
			categories, err := dependencies.currentStateCategories(report, true)
			if err != nil {
				collection.operational = append(collection.operational, err)
			} else {
				collection.categories = append(collection.categories, categories...)
				if len(report.Findings()) > 0 {
					collection.failures = append(collection.failures, errors.New("check staged state failed"))
				}
			}
		}
	}
	if drift {
		findings, err := dependencies.driftRoot(ctx, root)
		if err != nil {
			collection.operational = append(collection.operational, err)
		} else {
			categories, err := dependencies.driftCategories(findings, true)
			if err != nil {
				collection.operational = append(collection.operational, err)
			} else {
				collection.categories = append(collection.categories, categories...)
			}
			if len(findings) > 0 {
				collection.failures = append(collection.failures, errors.New("check staged drift failed"))
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
