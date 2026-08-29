package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/authoringop"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func runPartAuthoring(c *cmdCtx, mode authoringop.Mode) error {
	if c.sub == "sidecar" {
		return runSidecarAuthoring(c, mode)
	}
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

func runSidecarAuthoring(c *cmdCtx, mode authoringop.Mode) error {
	if len(c.inv.positionals) != 3 {
		return &usageErr{fmt.Sprintf("usage: awf %s sidecar <kind> <name> <field>", mode)}
	}
	req := authoringop.Request{Mode: mode, Kind: c.inv.positionals[0], Name: c.inv.positionals[1], Part: c.inv.positionals[2], Sidecar: true}
	if mode == authoringop.Reset {
		req.SidecarMode = "reset"
	} else {
		flags := []struct {
			name, mode string
			json       bool
		}{{"--value", "value", false}, {"--json-value", "value", true}, {"--add", "add", false}, {"--add-json", "add", true}, {"--remove", "remove", false}, {"--remove-json", "remove", true}}
		found := 0
		for _, flag := range flags {
			if value, ok := c.inv.values[flag.name]; ok {
				found++
				req.SidecarMode = flag.mode
				if flag.json {
					v, err := config.DecodeJSONValue(value)
					if err != nil {
						return fmt.Errorf("decode %s: %w", flag.name, err)
					}
					req.Value = v
				} else {
					req.Value = value
				}
			}
		}
		if found != 1 {
			return &usageErr{"awf edit sidecar requires exactly one value mode"}
		}
	}
	loader, err := newProjectLoader(c.root)
	if err != nil {
		return err
	}
	outcome, operationErr := authoringop.Run(c.ctx, c.root, req, loader, nil)
	if operationErr != nil {
		partial, ok := authoringop.AsPartial(operationErr)
		if !ok {
			return operationErr
		}
		document, err := partial.Document()
		if err != nil {
			return errors.Join(operationErr, err)
		}
		if err := presentation.Render(c.stdout, document); err != nil {
			return errors.Join(operationErr, err)
		}
		return &producedReportError{operationErr}
	}
	document, err := outcome.Document()
	if err != nil {
		return err
	}
	return presentation.Render(c.stdout, document)
}
