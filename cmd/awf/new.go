package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/localdocop"
	"github.com/hypnotox/agentic-workflows/internal/pitfallop"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/projectmutation"
	"github.com/hypnotox/agentic-workflows/internal/topicop"
)

const localDocumentKind = "doc"

// runNew scaffolds one of the surviving authored artifacts.
func runNew(ctx context.Context, root, kind string, args []string, stdout io.Writer) error {
	switch {
	case kind == "topic":
		return newTopic(ctx, root, args, stdout)
	case kind == "pitfall":
		return newPitfall(ctx, root, args, stdout)
	case kind == localDocumentKind:
		return newDoc(ctx, root, args, nil, stdout)
	case project.IsFreeformDomainKind(kind):
		if len(args) != 1 {
			return &usageErr{"usage: awf new domain <name>"}
		}
		return runNewDomain(ctx, root, args[0], stdout)
	default:
		return &usageErr{fmt.Sprintf("unknown kind %q (want: topic, domain, pitfall, doc)", kind)}
	}
}

func newDoc(ctx context.Context, root string, args []string, title *string, stdout io.Writer) (returnErr error) {
	if len(args) != 2 {
		return &usageErr{"usage: awf new doc <name> <description> [--title <title>]"}
	}
	resolvedTitle := derivedLocalDocTitle(args[0])
	if title != nil {
		resolvedTitle = *title
	}
	var outcome localdocop.Outcome
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	tx, err := projectmutation.AcquireProject(ctx, root, loader, nil)
	if err != nil {
		return err
	}
	defer func() { returnErr = localdocop.Finish(outcome, returnErr, tx.Release()) }()
	if err := gate(ctx, root); err != nil {
		return err
	}
	outcome, err = localdocop.Run(ctx, config.LocalDoc{Name: args[0], Title: resolvedTitle, Description: args[1]}, tx)
	if err != nil {
		var partial *localdocop.PartialError
		if !errors.As(err, &partial) {
			return err
		}
		document, documentErr := partial.Document()
		if documentErr != nil {
			return errors.Join(err, documentErr)
		}
		return errors.Join(err, presentation.Render(stdout, document))
	}
	document, err := outcome.Document()
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}

func derivedLocalDocTitle(name string) string {
	segment := name[strings.LastIndex(name, "/")+1:]
	words := strings.Split(segment, "-")
	for i, word := range words {
		if word != "" {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

func newPitfall(ctx context.Context, root string, args []string, stdout io.Writer) error {
	return newPitfallWithFaults(ctx, root, args, stdout, nil, pitfallop.Create)
}

type pitfallCreate func(context.Context, string, *projectmutation.Transaction) (pitfallop.Outcome, error)

func pitfallReleaseFailure(err error) error {
	if err == nil {
		return nil
	}
	if phase, ok := projectmutation.FailurePhase(err); ok && phase == projectmutation.PhaseRelease {
		return err
	}
	return &projectmutation.Failure{Phase: projectmutation.PhaseRelease, Cause: err}
}

func newPitfallWithFaults(ctx context.Context, root string, args []string, stdout io.Writer, releaseFault func() error, create pitfallCreate) (returnErr error) {
	if len(args) != 1 {
		return &usageErr{"usage: awf new pitfall <title>"}
	}
	lease, err := filesystem.AcquireTrackedLease(ctx, root)
	if err != nil {
		return err
	}
	released := false
	defer func() {
		if !released {
			returnErr = errors.Join(returnErr, pitfallReleaseFailure(lease.Release()))
		}
	}()
	if err := gate(ctx, root); err != nil {
		return err
	}
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	tx, err := projectmutation.UseTracked(ctx, root, loader, lease)
	if err != nil {
		return err
	}
	outcome, operationErr := create(ctx, args[0], tx)
	releaseErr := lease.Release()
	released = true
	if releaseFault != nil {
		releaseErr = errors.Join(releaseErr, releaseFault())
	}
	releaseErr = pitfallReleaseFailure(releaseErr)
	operationErr = pitfallop.Finish(outcome, operationErr, releaseErr)
	if operationErr != nil {
		var partial *pitfallop.PartialError
		if !errors.As(operationErr, &partial) {
			return operationErr
		}
		document, documentErr := partial.Document()
		if documentErr != nil {
			return errors.Join(operationErr, documentErr)
		}
		if renderErr := presentation.Render(stdout, document); renderErr != nil {
			return errors.Join(operationErr, renderErr)
		}
		return &producedReportError{operationErr}
	}
	document, err := outcome.Document()
	if err != nil {
		return pitfallop.Finish(outcome, err, nil)
	}
	if err := presentation.Render(stdout, document); err != nil {
		return pitfallop.Finish(outcome, err, nil)
	}
	return nil
}

func newTopic(ctx context.Context, root string, args []string, stdout io.Writer) (returnErr error) {
	if len(args) < 2 {
		return &usageErr{"usage: awf new topic <domain> <title>"}
	}
	var outcome topicop.Outcome
	lease, err := filesystem.AcquireTrackedLease(ctx, root)
	if err != nil {
		return err
	}
	defer func() { returnErr = topicop.Finish(outcome, returnErr, lease.Release()) }()
	if err := gate(ctx, root); err != nil {
		return err
	}
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	tx, err := projectmutation.UseTracked(ctx, root, loader, lease)
	if err != nil {
		return err
	}
	outcome, err = topicop.Create(ctx, args[0], strings.Join(args[1:], " "), tx)
	if err != nil {
		var partial *topicop.PartialScaffoldError
		if !errors.As(err, &partial) {
			return err
		}
		partialDocument, documentErr := partial.Document()
		if documentErr != nil {
			return errors.Join(err, documentErr)
		}
		return errors.Join(err, presentation.Render(stdout, partialDocument))
	}
	return presentation.Render(stdout, outcome.Document)
}
