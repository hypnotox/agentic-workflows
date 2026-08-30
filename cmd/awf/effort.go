package main

import (
	"context"
	"errors"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/effortop"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

// effortComposition is the wiring one effort command runs against: the resident
// authority and the managed-topology manager, both bound to the same resolved
// control roots.
type effortComposition struct {
	service *effort.Service
	manager *worktree.Manager
}

// composeEffort is how runEffort obtains that wiring. It is a parameter rather
// than a package fixture so a test names the composition it is exercising
// instead of replacing one behind the handler's back.
type composeEffort func(ctx context.Context, root string) (effortComposition, error)

// openEffortComposition is the production composition root for the effort
// command group. It resolves the control roots once and binds the seam's
// operations to the two consumers' own contracts: the effort service asks three
// questions of a handle opened on the invoking checkout, while the worktree
// manager receives the opener itself, because it reasons about the invoking and
// the managed checkout together and so opens a handle per checkout it touches.
// That is why the invoking root is opened here and again through the opener:
// a handle is pinned to one root, so the manager cannot borrow this one.
func openEffortComposition(ctx context.Context, root string) (effortComposition, error) {
	roots, err := awfgit.ResolveControlRoots(ctx, root)
	if err != nil {
		return effortComposition{}, err
	}
	repo, err := awfgit.Open(roots.InvokingRoot)
	if err != nil {
		return effortComposition{}, err
	}
	archiveMarker := func() ([]byte, error) {
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
	service, err := effort.Open(roots, effort.Dependencies{
		Clock:                 time.Now,
		UUID:                  effort.RandomUUIDv4,
		Worktrees:             repo.WorktreeList,
		BranchExists:          repo.BranchExists,
		ValidateRef:           repo.ValidateRefName,
		ExpectedArchiveMarker: archiveMarker,
	})
	if err != nil {
		return effortComposition{}, err
	}
	manager, err := worktree.Open(roots, openCheckout, func(name awfgit.ResidentName) (worktree.ResidentHandle, error) {
		residentRoot, rootErr := roots.ResidentRoot(name)
		if rootErr != nil {
			return nil, rootErr
		}
		return filesystem.Open(residentRoot)
	}, service)
	if err != nil {
		return effortComposition{}, err
	}
	return effortComposition{service: service, manager: manager}, nil
}

// openCheckout satisfies the worktree manager's checkout contract directly with
// the Git seam's handle: no adapter stands between them, so the manager's
// contract is exactly a subset of the handle's surface.
func openCheckout(root string) (worktree.Runner, error) { return awfgit.Open(root) }

func runEffort(c *cmdCtx, compose composeEffort) (returnErr error) {
	if err := validateEffortGrammar(c); err != nil {
		return err
	}
	lease, err := effortop.AcquireMutationLease(c.ctx, c.root, c.sub)
	if err != nil {
		return err
	}
	if lease != nil {
		// The effort application-composition seam chooses resident or dual-root
		// scope before any resident record or Git topology observation; command
		// composition retains it through rendered presentation.
		if c.retainLease != nil {
			c.retainLease(lease.Release)
		} else {
			defer func() { returnErr = errors.Join(returnErr, lease.Release()) }()
		}
	}
	composed, err := compose(c.ctx, c.root)
	if err != nil {
		return err
	}
	service, manager := composed.service, composed.manager
	selected := firstPos(c.inv.positionals)
	var document presentation.Document
	switch c.sub {
	case "new":
		document, err = effortop.New(c.ctx, service, manager, effort.NewInput{Slug: c.inv.values["--slug"], Title: selected}, c.inv.values["--base"])
	case "list":
		document, err = effortop.List(service)
	case "show":
		document, err = effortop.Show(service, selected)
	case "finish":
		document, err = effortop.Finish(c.ctx, service, selected)
	case "worktree":
		slug := c.inv.positionals[1]
		if c.inv.positionals[0] == "add" {
			document, err = effortop.AddWorktree(c.ctx, manager, slug, c.inv.values["--base"])
		} else {
			document, err = effortop.RemoveWorktree(c.ctx, manager, slug)
		}
	case "integrate":
		document, err = effortop.Integrate(c.ctx, c.root, manager, selected)
	default:
		return &usageErr{"usage: awf effort <new|list|show|finish|worktree|integrate>"}
	}
	if err != nil {
		return err
	}
	return presentation.Render(c.stdout, document)
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
