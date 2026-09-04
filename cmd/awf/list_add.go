package main

import (
	"context"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/domainop"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

func runNewDomain(ctx context.Context, root, name string, stdout io.Writer) error {
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	outcome, err := domainop.Add(ctx, root, name, loader, gatedLeaseAcquirer(loader))
	if err != nil {
		touched := publisherPaths(outcome.Publisher)
		if outcome.ConfigReplaced {
			touched = append([]string{".awf/config.yaml"}, touched...)
		}
		return mutationFailure{condition: "domain creation did not complete", cause: err, touched: touched, rerun: "awf new domain " + name}
	}
	document, err := outcome.Document()
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}

func runRemoveDomain(ctx context.Context, root, name string, stdout io.Writer) error {
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	outcome, err := domainop.Remove(ctx, root, name, loader, gatedLeaseAcquirer(loader))
	if err != nil {
		touched := publisherPaths(outcome.Publisher)
		if outcome.ConfigReplaced {
			touched = append([]string{".awf/config.yaml"}, touched...)
		}
		return mutationFailure{condition: "domain removal did not complete", cause: err, touched: touched, rerun: "awf remove domain " + name}
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
