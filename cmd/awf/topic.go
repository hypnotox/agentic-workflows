package main

import (
	"context"
	"errors"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/topicop"
)

// runTopic composes the live operation and retains command rendering and exits.
func runTopic(ctx context.Context, cwd, selector string, history, references, coverage bool, stdout io.Writer) error {
	detail, err := topicop.Run(ctx, cwd, topicop.Input{Selector: selector, History: history, References: references, Coverage: coverage}, openProjectOperation, gate)
	if err != nil {
		var usage *topicop.UsageError
		if errors.As(err, &usage) {
			return &usageErr{usage.Message}
		}
		return err
	}
	return printTopicDetail(stdout, detail)
}

func printTopicDetail(stdout io.Writer, detail presentation.Detail) error {
	document, err := detail.Document()
	if err != nil { // coverage-ignore: topic.Detail assembles a nonempty tree only through validated constructors
		return err
	}
	return presentation.Render(stdout, document)
}
