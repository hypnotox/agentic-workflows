// Package checkop owns repository-check preparation, ordered use cases, result assembly, and semantic presentation.
package checkop

import (
	"context"
	"errors"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/execution"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// Leaf identifies one resolved repository-check use case after CLI parsing.
type Leaf uint8

const (
	Check Leaf = iota
	Repository
	RepositoryDrift
	RepositoryState
	RepositoryProse
	RepositoryMemory
	Staged
	StagedState
	StagedDrift
)

// Outcome is the immutable semantic result of one repository-check use case.
// Failure is non-nil only after a complete produced report contains Error findings.
type Outcome struct {
	Document presentation.Document
	Failure  error
}

type planNoteSink map[string]struct{}

// Run prepares and executes exactly one resolved repository-check use case.
func Run(ctx context.Context, root string, leaf Leaf) (Outcome, error) {
	planNotes := planNoteSink{}
	var collection checkCollection
	var err error
	switch leaf {
	case Check:
		collection, err = collectCheckRepoWithPlanNotes(ctx, root, planNotes)
		if err != nil {
			collection.operational = append(collection.operational, err)
		}
		_, _, gitErr := awfgit.OpenContaining(root)
		if errors.Is(gitErr, awfgit.ErrNotARepository) {
			collection.information = append(collection.information, "staged check universe unavailable outside a git repository")
			return outcome(collection)
		}
		if gitErr != nil {
			collection.operational = append(collection.operational, gitErr)
			return outcome(collection)
		}
		staged, stagedErr := collectCheckStaged(ctx, root, planNotes)
		collection = collection.append(staged)
		if stagedErr != nil {
			collection.operational = append(collection.operational, stagedErr)
		}
	case Repository:
		collection, err = collectCheckRepoWithPlanNotes(ctx, root, planNotes)
	case RepositoryDrift:
		collection, err = collectRepoCheckSelectionWithPlanNotes(ctx, root, []execution.StepID{repoStepDrift}, execution.StopOnFailure, false, nil, planNotes, productionRepoCheckDependencies())
	case RepositoryState:
		collection, err = collectRepoCheckSelectionWithPlanNotes(ctx, root, []execution.StepID{repoStepState}, execution.StopOnFailure, false, nil, planNotes, productionRepoCheckDependencies())
	case RepositoryProse:
		collection, err = collectRepoCheckSelectionWithPlanNotes(ctx, root, []execution.StepID{repoStepProse}, execution.StopOnFailure, false, nil, planNotes, productionRepoCheckDependencies())
	case RepositoryMemory:
		collection, err = collectRepoCheckSelectionWithPlanNotes(ctx, root, []execution.StepID{repoStepMemory}, execution.StopOnFailure, false, nil, planNotes, productionRepoCheckDependencies())
	case Staged:
		collection, err = collectCheckStaged(ctx, root, planNotes)
	case StagedState:
		collection, err = collectCheckStagedSelection(ctx, root, planNotes, true, false)
	case StagedDrift:
		collection, err = collectCheckStagedSelection(ctx, root, planNotes, false, true)
	default:
		return Outcome{}, fmt.Errorf("unknown repository-check operation %d", leaf)
	}
	if err != nil {
		return Outcome{}, err
	}
	return outcome(collection)
}

func outcome(collection checkCollection) (Outcome, error) {
	if len(collection.operational) > 0 {
		return Outcome{}, errors.Join(collection.operational...)
	}
	report, err := checkReport(collection.warnings, collection.information, collection.presentation)
	if err != nil {
		return Outcome{}, err
	}
	document, err := report.Document()
	if err != nil {
		return Outcome{}, err
	}
	out := Outcome{Document: document}
	if len(collection.failures) > 0 {
		out.Failure = &producedReportError{collection.failures[0]}
	}
	return out, nil
}
