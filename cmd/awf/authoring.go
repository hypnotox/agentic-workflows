package main

import (
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
	return executeAuthoringRequest(c, authoringop.Request{
		Mode: mode, Kind: c.inv.positionals[0], Name: c.inv.positionals[1], Part: c.inv.positionals[2], Content: content,
	})
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
	return executeAuthoringRequest(c, req)
}

func executeAuthoringRequest(c *cmdCtx, request authoringop.Request) error {
	loader, err := newProjectLoader(c.root)
	if err != nil {
		return err
	}
	outcome, operationErr := authoringop.Run(c.ctx, c.root, request, loader, gatedLeaseAcquirer(loader))
	if operationErr != nil {
		touched := append([]string(nil), outcome.CreatedParents...)
		pending := []string(nil)
		if outcome.Source != authoringop.SourceNone {
			touched = append(touched, outcome.SourcePath)
		} else if outcome.SourcePath != "" {
			pending = append(pending, outcome.SourcePath)
		}
		touched = append(touched, publisherPaths(outcome.Publisher)...)
		return mutationFailure{condition: "artifact authoring did not complete", cause: operationErr, touched: touched, pending: pending, rerun: "awf " + string(request.Mode)}
	}
	document, err := outcome.Document()
	if err != nil {
		return err
	}
	return presentation.Render(c.stdout, document)
}
