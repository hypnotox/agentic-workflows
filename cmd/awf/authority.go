package main

import (
	"context"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func runResolveTopic(ctx context.Context, root string, paths []string, uncovered bool, stdout io.Writer) error {
	if uncovered && len(paths) != 0 {
		return &usageErr{"usage: awf resolve topic --uncovered"}
	}
	if !uncovered && len(paths) == 0 {
		return &usageErr{"usage: awf resolve topic <path>..."}
	}
	state, _, repo, err := openProjectOperation(ctx, root)
	if err != nil {
		return err
	}
	if uncovered {
		detail, err := currentstatecoord.UncoveredPaths(state.Root(), repo, ctx)
		if err != nil {
			return err
		}
		return printAuthorityDetail(stdout, detail)
	}
	normalized := make([]string, len(paths))
	for i, path := range paths {
		normalized[i], err = currentstatecoord.NormalizeAuthorityPath(state.Root(), path)
		if err != nil {
			return fmt.Errorf("resolve topic: %w", err)
		}
	}
	detail, err := currentstatecoord.ResolveTopics(state.Root(), repo, ctx, normalized)
	if err != nil {
		return err
	}
	return printAuthorityDetail(stdout, detail)
}

func printAuthorityDetail(stdout io.Writer, detail presentation.Detail) error {
	document, err := detail.Document()
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}
