package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/project"
	"golang.org/x/mod/semver"
)

var checkStagedDriftRoot = project.CheckStagedDriftRoot

// runCheckStaged runs the staged transition universe. The commit child is direct-only.
func runCheckStaged(ctx context.Context, root string, stdout io.Writer) error {
	return runCheckStagedWithPlanNotes(ctx, root, stdout, planNoteSink{})
}

func collectCheckStaged(ctx context.Context, root string, planNotes planNoteSink) (checkCollection, error) {
	return collectCheckStagedSelection(ctx, root, planNotes, true, true)
}

func collectCheckStagedSelection(ctx context.Context, root string, planNotes planNoteSink, state, drift bool) (checkCollection, error) {
	lock, err := stagedLock(ctx, root)
	if err != nil {
		return checkCollection{}, err
	}
	collection := checkCollection{}
	lockV, binV, ok := lockVsBinaryLock(lock)
	if ok && semver.Compare(binV, lockV) > 0 {
		collection.notes = append(collection.notes, fmt.Sprintf("awf %s is ahead of this project (rendered by %s); run awf render to re-pin", strings.TrimPrefix(binV, "v"), strings.TrimPrefix(lockV, "v")))
	}
	if state {
		report, err := project.CheckStagedRoot(ctx, root)
		if err != nil {
			return checkCollection{}, err
		}
		collection.notes = append(collection.notes, report.Notes()...)
		for _, note := range report.PlanNotes {
			if _, seen := planNotes[note]; !seen {
				planNotes[note] = struct{}{}
				collection.notes = append(collection.notes, note)
			}
		}
		for _, finding := range report.Findings() {
			collection.findings = append(collection.findings, checkFinding{severity: "error", check: "staged current-state", detail: finding})
		}
		if len(report.Findings()) > 0 {
			collection.failures = append(collection.failures, errors.New("check staged state failed"))
		}
	}
	if drift {
		findings, err := checkStagedDriftRoot(ctx, root)
		if err != nil {
			return checkCollection{}, err
		}
		for _, finding := range findings {
			collection.findings = append(collection.findings, checkFinding{severity: "error", check: "staged drift", detail: fmt.Sprintf("%s: %s: %s", finding.Kind, finding.Path, finding.Detail)})
		}
		if len(findings) > 0 {
			collection.failures = append(collection.failures, errors.New("check staged drift failed"))
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
