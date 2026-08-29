package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/authoringop"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func runPartAuthoring(c *cmdCtx, mode authoringop.Mode) error {
	if len(c.inv.positionals) != 3 {
		return &usageErr{fmt.Sprintf("usage: awf %s <kind> <name> <part>", mode)}
	}
	var content []byte
	if mode == authoringop.Edit {
		value, hasContent := c.inv.values["--content"]
		hasStdin := c.inv.bools["--stdin"]
		if hasContent == hasStdin {
			return &usageErr{"awf edit requires exactly one of --content or --stdin"}
		}
		if hasContent {
			content = []byte(value)
		} else {
			var err error
			content, err = io.ReadAll(c.stdin)
			if err != nil {
				return fmt.Errorf("read authoring content from stdin: %w", err)
			}
		}
	}
	loader, err := newProjectLoader(c.root)
	if err != nil {
		return err
	}
	outcome, operationErr := authoringop.Run(c.ctx, c.root, authoringop.Request{
		Mode: mode, Kind: c.inv.positionals[0], Name: c.inv.positionals[1], Part: c.inv.positionals[2], Content: content,
	}, loader, nil)
	if operationErr != nil {
		partial, ok := authoringop.AsPartial(operationErr)
		if !ok {
			return operationErr
		}
		document, documentErr := partial.Document()
		if documentErr != nil {
			return errors.Join(operationErr, documentErr)
		}
		if renderErr := presentation.Render(c.stdout, document); renderErr != nil {
			return errors.Join(operationErr, renderErr)
		}
		return &producedReportError{operationErr}
	}
	document, err := outcome.Document()
	if err != nil {
		return err
	}
	return presentation.Render(c.stdout, document)
}
