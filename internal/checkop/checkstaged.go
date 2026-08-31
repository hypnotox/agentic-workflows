package checkop

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	"golang.org/x/mod/semver"
)

type checkStagedDependencies struct {
	stateRoot func(context.Context, string) (currentstatecoord.CurrentStateReport, error)
	driftRoot func(context.Context, string) (checkresult.Result, error)
}

func productionCheckStagedDependencies() checkStagedDependencies {
	return checkStagedDependencies{
		stateRoot: currentstatecoord.CheckStagedRoot,
		driftRoot: stagedDriftResult,
	}
}

// collectCheckStaged runs the staged transition universe. The commit child is direct-only.
func collectCheckStaged(ctx context.Context, root string) (checkCollection, error) {
	return collectCheckStagedWith(ctx, root, productionCheckStagedDependencies())
}

func collectCheckStagedWith(ctx context.Context, root string, dependencies checkStagedDependencies) (checkCollection, error) {
	return collectCheckStagedSelectionWith(ctx, root, true, true, dependencies)
}

func collectCheckStagedSelection(ctx context.Context, root string, state, drift bool) (checkCollection, error) {
	return collectCheckStagedSelectionWith(ctx, root, state, drift, productionCheckStagedDependencies())
}

func collectCheckStagedSelectionWith(ctx context.Context, root string, state, drift bool, dependencies checkStagedDependencies) (checkCollection, error) {
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
			result, resultErr := informationResult([]string{fmt.Sprintf("awf %s is ahead of this project (rendered by %s); run awf render to re-pin", strings.TrimPrefix(binV, "v"), strings.TrimPrefix(lockV, "v"))})
			if resultErr != nil {
				return checkCollection{}, resultErr
			}
			collection.add("advisory", result, false)
		}
	}
	if state && lock != nil {
		report, err := dependencies.stateRoot(ctx, root)
		if err != nil {
			collection.operational = append(collection.operational, err)
		} else {
			collection.add("staged current-state", report.CurrentResult, false)
			if hasErrors(report.Result()) {
				collection.failures = append(collection.failures, errors.New("check staged state failed"))
			}
		}
	}
	if drift {
		result, err := dependencies.driftRoot(ctx, root)
		if err != nil {
			collection.operational = append(collection.operational, err)
		} else {
			collection.add("staged drift", result, true)
			if hasErrors(result) {
				collection.failures = append(collection.failures, errors.New("check staged drift failed"))
			}
		}
	}
	return collection, nil
}
