package main

import (
	"context"
	"errors"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

// newProjectLoader composes the project-opening policy for one invocation.
func newProjectLoader(root string) (*project.Loader, error) {
	repo, _, err := awfgit.OpenContaining(root)
	if err != nil {
		if !errors.Is(err, awfgit.ErrNotARepository) {
			return nil, err
		}
		return project.NewLoaderWithoutRepository(config.Load, catalog.Standard, awfgit.ProjectResidentRoot), nil
	}
	return project.NewLoader(config.Load, catalog.Standard, awfgit.ProjectResidentRoot, repo), nil
}

func runSync(ctx context.Context, root string, stdout io.Writer) error {
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	return runSyncPrinting(ctx, loader, root, stdout)
}

func runSyncPrinting(ctx context.Context, loader *project.Loader, root string, stdout io.Writer) error {
	residentRoot := awfgit.ProjectResidentRoot(ctx, root)
	lease, err := filesystem.AcquireProjectLease(ctx, root, residentRoot)
	if err != nil {
		return err
	}
	if err := guardMutationSession(ctx, root); err != nil {
		return errors.Join(err, lease.Release())
	}
	if err := gate(ctx, root); err != nil {
		return errors.Join(err, lease.Release())
	}
	session, err := loader.Load(ctx, root)
	if err != nil {
		return errors.Join(err, lease.Release())
	}
	result, syncErr := composePublisher(session).SyncLeased(ctx, lease)
	return finishSyncPrinting(stdout, result, syncErr, lease.Release())
}

func gatedLeaseAcquirer(loader *project.Loader) func(context.Context, string) (*filesystem.Lease, func() error, error) {
	return func(ctx context.Context, root string) (*filesystem.Lease, func() error, error) {
		lease, err := loader.AcquireProjectLease(ctx, root)
		if err != nil {
			return nil, nil, err
		}
		if err := guardMutationSession(ctx, root); err != nil {
			return nil, nil, errors.Join(err, lease.Release())
		}
		if err := gate(ctx, root); err != nil {
			return nil, nil, errors.Join(err, lease.Release())
		}
		return lease, lease.Release, nil
	}
}

func finishSyncPrinting(stdout io.Writer, result publisher.Result, syncErr, releaseErr error) error {
	if combined := errors.Join(syncErr, releaseErr); combined != nil {
		return syncFailure{result: result, cause: combined}
	}
	mutation, err := result.Mutation()
	if err != nil {
		return err
	}
	return renderSyncMutation(stdout, mutation)
}

type syncFailure struct {
	result publisher.Result
	cause  error
}

func (e syncFailure) Error() string { return e.cause.Error() }
func (e syncFailure) Unwrap() error { return e.cause }

func (e syncFailure) Diagnostic() (presentation.Diagnostic, error) {
	changed := make([]presentation.Field, 0, len(e.result.Touched()))
	for _, path := range e.result.Touched() {
		value, err := presentation.Literal(path)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		field, err := presentation.NewField("touched path", value)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
	texts := []string{
		"run git status --short and git diff to inspect visible changes",
		"correct or restore the desired paths",
		"rerun awf render",
	}
	steps := make([]presentation.Value, len(texts))
	for i, text := range texts {
		value, err := presentation.Prose(text)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		steps[i] = value
	}
	return presentation.Diagnostic{Condition: "render did not complete", State: "operation", Changed: changed, Cause: e.cause.Error(), Steps: steps}, nil
}

func renderSyncMutation(stdout io.Writer, mutation presentation.Mutation) error {
	document, err := mutation.Document()
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}
