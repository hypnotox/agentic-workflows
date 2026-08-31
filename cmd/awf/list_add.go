package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/domainop"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/projectmutation"
)

func runNewDomain(ctx context.Context, root, name string, stdout io.Writer) (returnErr error) {
	var outcome domainop.Outcome
	lease, err := filesystem.AcquireProjectLease(ctx, root, awfgit.ProjectResidentRoot(ctx, root))
	if err != nil {
		return err
	}
	defer func() { returnErr = domainop.Finish(outcome, returnErr, lease.Release()) }()
	if err := gate(ctx, root); err != nil {
		return err
	}
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	tx, err := projectmutation.UseProject(ctx, root, loader, lease)
	if err != nil {
		return err
	}
	outcome, err = domainop.Add(ctx, name, tx)
	if err != nil {
		var partial *domainop.PartialError
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

func runRemoveDomain(ctx context.Context, root, name string, stdout io.Writer) (returnErr error) {
	var outcome domainop.Outcome
	lease, err := filesystem.AcquireProjectLease(ctx, root, awfgit.ProjectResidentRoot(ctx, root))
	if err != nil {
		return err
	}
	defer func() { returnErr = domainop.Finish(outcome, returnErr, lease.Release()) }()
	if err := gate(ctx, root); err != nil {
		return err
	}
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	tx, err := projectmutation.UseProject(ctx, root, loader, lease)
	if err != nil {
		return err
	}
	outcome, err = domainop.Remove(ctx, name, tx)
	if err != nil {
		var partial *domainop.PartialError
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
	if err := presentation.Render(stdout, document); err != nil {
		return err
	}
	if outcome.Orphaned {
		return writeStatus(stdout, fmt.Sprintf("note: domain %q still has a sidecar or convention parts under .awf/, now orphaned", name))
	}
	return nil
}

func runList(ctx context.Context, root, kindFilter string, stdout io.Writer) error {
	session, err := loadProjectSession(ctx, root)
	if err != nil {
		return err
	}
	document, err := project.BuildListDocument(session, session.Config(), kindFilter)
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}
