// Package checkop owns repository-check preparation, ordered use cases, result assembly, and semantic presentation.
package checkop

import (
	"context"
	"errors"
	"fmt"

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

// Run prepares and executes exactly one resolved repository-check use case.
func Run(ctx context.Context, root string, leaf Leaf) (Outcome, error) {
	var collection checkCollection
	var err error
	switch leaf {
	case Check:
		collection, err = collectCheckRepo(ctx, root)
	case Repository:
		collection, err = collectCheckRepo(ctx, root)
	case RepositoryDrift:
		collection, err = collectRepoCheckSelection(ctx, root, []repositoryLane{repositoryDrift}, false, false, nil, productionRepoCheckDependencies())
	case RepositoryState:
		collection, err = collectRepoCheckSelection(ctx, root, []repositoryLane{repositoryState}, false, false, nil, productionRepoCheckDependencies())
	case RepositoryProse:
		collection, err = collectRepoCheckSelection(ctx, root, []repositoryLane{repositoryProse}, false, false, nil, productionRepoCheckDependencies())
	case RepositoryMemory:
		collection, err = collectRepoCheckSelection(ctx, root, []repositoryLane{repositoryMemory}, false, false, nil, productionRepoCheckDependencies())
	case Staged:
		collection, err = collectCheckStaged(ctx, root)
	case StagedState:
		collection, err = collectCheckStagedSelection(ctx, root, true, false)
	case StagedDrift:
		collection, err = collectCheckStagedSelection(ctx, root, false, true)
	default:
		return Outcome{}, fmt.Errorf("unknown repository-check operation %d", leaf)
	}
	if err != nil {
		return Outcome{}, err
	}
	return outcome(collection)
}

func outcome(collection checkCollection) (Outcome, error) {
	report, reportErr := checkReport(collection.results)
	if len(collection.operational) > 0 {
		return Outcome{}, errors.Join(append(collection.operational, reportErr)...)
	}
	if reportErr != nil {
		return Outcome{}, reportErr
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
