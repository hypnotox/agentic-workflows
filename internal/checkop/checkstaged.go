package checkop

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	"github.com/hypnotox/agentic-workflows/internal/repositorycheck"
	"golang.org/x/mod/semver"
)

type checkStagedDependencies struct {
	stateRoot func(context.Context, string) (currentstatecoord.CurrentStateReport, error)
	driftRoot func(context.Context, string) (checkresult.Result, error)
	present   func(checkresult.Result, string, bool) (repositorycheck.Presentation, error)
}

func productionCheckStagedDependencies() checkStagedDependencies {
	return checkStagedDependencies{
		stateRoot: currentstatecoord.CheckStagedRoot,
		driftRoot: stagedDriftResult,
		present: func(result checkresult.Result, check string, evidence bool) (repositorycheck.Presentation, error) {
			if evidence {
				return repositorycheck.PresentEvidence(result, check)
			}
			return repositorycheck.Present(result, check)
		},
	}
}

func unseenPlanWarnings(result checkresult.Result, seen planNoteSink) checkresult.Result {
	var findings []checkresult.Finding
	for _, finding := range result.Findings() {
		note := finding.Evidence.Detail
		if _, exists := seen[note]; exists {
			continue
		}
		seen[note] = struct{}{}
		findings = append(findings, finding)
	}
	filtered, err := checkresult.New(findings, nil)
	if err != nil { // coverage-ignore: callers supply only validated immutable Warning partitions
		return checkresult.Result{}
	}
	return filtered
}

// runCheckStaged runs the staged transition universe. The commit child is direct-only.

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
			projected, projectErr := presentCurrentStateReport(report, "staged current-state", planNotes, dependencies.present)
			if projectErr != nil {
				collection.operational = append(collection.operational, projectErr)
			} else {
				collection.presentation = collection.presentation.Append(projected)
				if repositorycheck.HasErrors(report.Result()) {
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
