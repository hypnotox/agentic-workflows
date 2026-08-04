package main

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

const topicStaticReference = `topic: static not inside an awf project

reference:
  Query active current-state topics and claims. Use history for direct ADR history,
  references for direct claim IDs, and coverage for scope and marker sites.
`

// runTopic validates the selector before inspecting project state, then mirrors
// config/context's static-fallback and in-handler version-gate boundary.
func runTopic(ctx context.Context, cwd, selector string, history, references, coverage bool, stdout io.Writer) error {
	if _, _, err := topic.ParseSelector(selector); err != nil {
		return &usageErr{err.Error()}
	}
	if _, err := os.Stat(config.ConfigPath(cwd)); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		_, err = io.WriteString(stdout, topicStaticReference)
		return err
	}
	if err := gate(ctx, cwd); err != nil {
		return err
	}
	p, err := project.Open(ctx, cwd)
	if err != nil {
		return err
	}
	result, err := p.QueryTopic(ctx, selector, topic.QueryOptions{History: history, References: references, Coverage: coverage})
	if err != nil {
		return err
	}
	return printTopic(stdout, result)
}

func printTopic(stdout io.Writer, result topic.QueryResult) error {
	detail := result.Detail()
	document, err := detail.Document()
	if err != nil { // coverage-ignore: topic.Detail assembles a nonempty tree only through validated constructors
		return err
	}
	return presentation.Render(stdout, document)
}
