package main

import (
	"context"
	"errors"

	"github.com/hypnotox/agentic-workflows/internal/effort/application"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// executeEffort is the CLI adapter's one application-boundary contract.
type executeEffort func(context.Context, string, application.Request, func() ([]byte, error)) (application.Result, error)

// openEffortComposition binds the registered effort handler to the production
// application boundary. The historical name remains local to command wiring;
// no effort or worktree service is exposed through it.
func openEffortComposition(ctx context.Context, root string, request application.Request, marker func() ([]byte, error)) (application.Result, error) {
	return application.Execute(ctx, root, request, marker)
}

func runEffort(c *cmdCtx, execute executeEffort) (returnErr error) {
	if err := validateEffortGrammar(c); err != nil {
		return err
	}
	request, err := effortRequest(c)
	if err != nil {
		return err
	}
	result, err := execute(c.ctx, c.root, request, expectedEffortArchiveMarker(c.ctx, c.root))
	if result.Release != nil {
		// The application chooses and acquires mutation scope before opening any
		// effort state. The adapter retains that capability through final output.
		if c.retainLease != nil {
			c.retainLease(result.Release)
		} else {
			defer func() { returnErr = errors.Join(returnErr, result.Release()) }()
		}
	}
	if err != nil {
		return err
	}
	return presentation.Render(c.stdout, result.Document)
}

func effortRequest(c *cmdCtx) (application.Request, error) {
	selected := firstPos(c.inv.positionals)
	switch c.sub {
	case "new":
		return application.Request{Kind: application.New, Slug: c.inv.values["--slug"], Title: selected, Base: c.inv.values["--base"]}, nil
	case "list":
		return application.Request{Kind: application.List}, nil
	case "show":
		return application.Request{Kind: application.Show, Slug: selected}, nil
	case "finish":
		return application.Request{Kind: application.Finish, Slug: selected}, nil
	case "worktree":
		request := application.Request{Slug: c.inv.positionals[1], Base: c.inv.values["--base"]}
		if c.inv.positionals[0] == "add" {
			request.Kind = application.AddWorktree
		} else {
			request.Kind = application.RemoveWorktree
		}
		return request, nil
	case "integrate":
		return application.Request{Kind: application.Integrate, Slug: selected}, nil
	default:
		return application.Request{}, &usageErr{"usage: awf effort <new|list|show|finish|worktree|integrate>"}
	}
}

func expectedEffortArchiveMarker(ctx context.Context, root string) func() ([]byte, error) {
	return func() ([]byte, error) {
		projectState, cfg, _, err := openProjectOperation(ctx, root)
		if err != nil {
			return nil, err
		}
		prepared, err := operationPreparation(projectState, cfg)
		if err != nil {
			return nil, err
		}
		rendered, err := prepared.ResidentMarker(string(awfgit.ResidentEffortArchive))
		if err != nil {
			return nil, err
		}
		return []byte(rendered.Content()), nil
	}
}

func validateEffortGrammar(c *cmdCtx) error {
	if c.sub == "new" {
		if _, ok := c.inv.values["--slug"]; !ok {
			return &usageErr{"awf effort new: --slug is required"}
		}
		return nil
	}
	if c.sub != "worktree" {
		return nil
	}
	if len(c.inv.positionals) != 2 {
		return &usageErr{"usage: awf effort worktree <add|remove> <slug>"}
	}
	switch c.inv.positionals[0] {
	case "add":
		return nil
	case "remove":
		if c.inv.values["--base"] != "" {
			return &usageErr{"awf effort worktree remove: --base is not allowed"}
		}
		return nil
	default:
		return &usageErr{"usage: awf effort worktree <add|remove> <slug>"}
	}
}
