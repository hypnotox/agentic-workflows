package main

import (
	"context"
	"errors"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/topicop"
)

// runReadTopic composes the live operation and retains command rendering and exits.
func runReadTopic(ctx context.Context, cwd, selector string, references, coverage bool, stdout io.Writer) error {
	detail, err := topicop.Run(ctx, cwd, topicop.Input{Selector: selector, References: references, Coverage: coverage}, openProjectOperation, gate)
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
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}
