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
	if err := gate(ctx, root); err != nil {
		return err
	}
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	document, err := domainop.Add(ctx, root, name, loader)
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}

func runRemoveDomain(ctx context.Context, root, name string, stdout io.Writer) error {
	if err := gate(ctx, root); err != nil {
		return err
	}
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	document, orphaned, err := domainop.Remove(ctx, root, name, loader)
	if err != nil {
		return err
	}
	if err := presentation.Render(stdout, document); err != nil {
		return err
	}
	if orphaned {
		return writeStatus(stdout, fmt.Sprintf("note: domain %q still has a sidecar or convention parts under .awf/, now orphaned", name))
	}
	return nil
}

func runList(ctx context.Context, root, kindFilter string, stdout io.Writer) error {
	state, cfg, _, err := openProjectOperation(ctx, root)
	if err != nil {
		return err
	}
	document, err := project.BuildListDocument(state, cfg, kindFilter)
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}
