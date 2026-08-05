package main

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/contextdelivery"
	"github.com/hypnotox/agentic-workflows/internal/contextq"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

var deliverContext = contextdelivery.Deliver

func runContext(ctx context.Context, cwd string, paths []string, staged bool, rng string, uncovered, full bool, shows []string, stdout io.Writer) error {
	facets, err := contextq.ParseContextFacets(shows, full)
	if err != nil {
		return &usageErr{"awf context: " + err.Error()}
	}
	if uncovered && (full || len(shows) > 0) {
		return &usageErr{"awf context: --show and --full cannot be combined with --uncovered"}
	}
	if uncovered {
		return runUncovered(ctx, cwd, paths, staged, rng, stdout)
	}
	selection := contextq.SelectionExplicit
	if staged {
		selection = contextq.SelectionStaged
	} else if rng != "" {
		selection = contextq.SelectionRange
	}
	if len(paths) == 0 {
		if !staged && rng == "" {
			return &usageErr{"usage: awf context <path>... [--show <facet>] [--full] [--staged] [--range <a>..<b>]"}
		}
		repo, _, e := awfgit.OpenContaining(cwd)
		if e != nil {
			return e
		}
		resolved, e := repo.ChangedPaths(ctx, staged, rng)
		if e != nil {
			return e
		}
		if len(resolved) == 0 {
			return &usageErr{"awf context: no changed paths for the given selector"}
		}
		paths = resolved
	}
	options := contextq.ContextOptions{Selection: selection, Range: rng, Facets: facets}
	result := contextq.ContextResult{Selection: selection, Range: rng}
	header := "live state for this project"
	var state project.ContextState
	if staged {
		if err := gateStaged(ctx, cwd); err != nil {
			return err
		}
		state, err = project.StagedContextState(ctx, cwd)
		header = "staged state for this project"
	} else if _, statErr := os.Stat(config.ConfigPath(cwd)); statErr != nil {
		if !errors.Is(statErr, fs.ErrNotExist) {
			return statErr
		}
		return deliverContext([]byte(contextq.RenderContextText(result, "static: not inside an awf project; live classification and authority require an adopted project", facets)), cwd, stdout)
	} else {
		if err := gate(ctx, cwd); err != nil {
			return err
		}
		p, e := project.Open(ctx, cwd)
		if e != nil { // coverage-ignore: gate just loaded the same config and project presence; failure requires a concurrent filesystem race
			return e
		}
		state, err = p.ContextState(ctx)
	}
	if err != nil {
		return err
	}
	return deliverContext([]byte(contextq.RenderContextText(contextq.New(state).ContextForOptions(paths, options), header, facets)), cwd, stdout)
}

func runUncovered(ctx context.Context, cwd string, roots []string, staged bool, rng string, stdout io.Writer) error {
	if rng != "" {
		return &usageErr{"awf context --uncovered takes optional scan-root paths, not --range"}
	}
	header := "coverage gaps for this project"
	var state project.ContextState
	var err error
	if staged {
		if err = gateStaged(ctx, cwd); err == nil {
			state, err = project.StagedContextState(ctx, cwd)
		}
		header = "staged coverage gaps for this project"
	} else if _, statErr := os.Stat(config.ConfigPath(cwd)); statErr != nil {
		if !errors.Is(statErr, fs.ErrNotExist) {
			return statErr
		}
		static := contextq.UncoveredResult{ScanRoots: contextq.NormalizeContextPaths(roots)}
		return deliverContext([]byte(contextq.RenderUncoveredText(static, "static: not inside an awf project; live coverage appears inside one")), cwd, stdout)
	} else {
		if err = gate(ctx, cwd); err == nil {
			var p *project.Project
			p, err = project.Open(ctx, cwd)
			if err == nil { // coverage-ignore: gate just loaded the same project; an Open failure requires a concurrent filesystem race
				state, err = p.ContextState(ctx)
			}
		}
	}
	if err != nil {
		return err
	}
	return deliverContext([]byte(contextq.RenderUncoveredText(contextq.New(state).Uncovered(roots), header)), cwd, stdout)
}
