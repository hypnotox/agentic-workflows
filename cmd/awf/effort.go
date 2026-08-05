package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/effort"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

// effortComposition is the wiring one effort command runs against: the resident
// authority and the managed-topology manager, both bound to the same resolved
// control roots.
var (
	activitySlugPattern  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	activityOwnerPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

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
	if err != nil { // coverage-ignore: ResolveControlRoots just proved this path is a checkout; a failed open requires a concurrent repository-identity race
		return effortComposition{}, err
	}
	service, err := effort.Open(roots, effort.Dependencies{
		Clock:        time.Now,
		UUID:         effort.RandomUUIDv4,
		Worktrees:    repo.WorktreeList,
		BranchExists: repo.BranchExists,
		ValidateRef:  repo.ValidateRefName,
		RemoveTree:   os.RemoveAll,
	})
	if err != nil {
		return effortComposition{}, err
	}
	manager, err := worktree.Open(roots, openCheckout, service)
	if err != nil { // coverage-ignore: openCheckout just opened this same root above; a second failure requires a concurrent repository-identity race
		return effortComposition{}, err
	}
	return effortComposition{service: service, manager: manager}, nil
}

// openCheckout satisfies the worktree manager's checkout contract directly with
// the Git seam's handle: no adapter stands between them, so the manager's
// contract is exactly a subset of the handle's surface.
func openCheckout(root string) (worktree.Runner, error) { return awfgit.Open(root) }

func runEffort(c *cmdCtx, compose composeEffort) error {
	if err := validateEffortGrammar(c); err != nil {
		return err
	}
	composed, err := compose(c.ctx, c.root)
	if err != nil {
		return err
	}
	service, manager := composed.service, composed.manager
	selected := firstPos(c.inv.positionals)
	switch c.sub {
	case "new":
		input := effort.NewInput{Slug: c.inv.values["--slug"], Title: selected}
		if c.inv.bools["--no-worktree"] {
			record, err := service.New(c.ctx, input)
			if err != nil {
				return err
			}
			absent := worktree.Result{Condition: "no managed worktree", ChangedTopology: false, NextAction: "continue the effort in " + service.InvokingRoot()}
			return writeEffortNew(c.stdout, record, absent)
		}
		record, result, err := manager.NewEffort(c.ctx, input, c.inv.values["--base"])
		if err != nil {
			return err
		}
		return writeEffortNew(c.stdout, record, result)
	case "list":
		records, err := service.List()
		if err != nil {
			return err
		}
		document, err := effort.ListDocument(records)
		if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
			return err
		}
		return presentation.Render(c.stdout, document)
	case "show":
		record, err := service.Show(selected)
		if err != nil {
			return err
		}
		return writeEffort(c.stdout, record)
	case "finish":
		result, err := service.Finish(c.ctx, selected)
		if err != nil {
			return err
		}
		mutation, err := result.FinishMutation(selected)
		if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
			return err
		}
		document, err := mutation.Document()
		if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
			return err
		}
		return presentation.Render(c.stdout, document)
	case "worktree":
		action, selected := c.inv.positionals[0], c.inv.positionals[1]
		var result worktree.Result
		var err error
		switch action {
		case "add":
			result, err = manager.Add(c.ctx, selected, c.inv.values["--base"])
		case "remove":
			result, err = manager.Remove(c.ctx, selected)
		default: // coverage-ignore: validateEffortGrammar accepts only add or remove before this closed dispatch
			return &usageErr{"usage: awf effort worktree <add|remove> <slug>"}
		}
		return writeWorktreeResult(c.stdout, result, err)
	case "integrate":
		gateCommand, err := integrationGateCommand(c.root)
		if err != nil {
			return err
		}
		result, err := manager.Integrate(c.ctx, selected, gateCommand)
		return writeWorktreeResult(c.stdout, result, err)
	case "memory update":
		return service.UpdateMemory(selected, effort.MemoryUpdate{Phase: effortValue(c.inv, "--phase"), Next: effortValue(c.inv, "--next")})
	case "activity attach", "activity heartbeat", "activity detach":
		return writeActivityReply(c.stdout, runEffortActivity(c, service))
	default:
		return &usageErr{"usage: awf effort <new|list|show|finish|worktree|integrate|memory|activity>"}
	}
}

func effortValue(inv invocation, flag string) *string {
	value, ok := inv.values[flag]
	if !ok {
		return nil
	}
	return &value
}

func runEffortActivity(c *cmdCtx, service *effort.Service) effort.ActivityReply {
	slug := firstPos(c.inv.positionals)
	switch c.sub {
	case "activity attach":
		return service.AttachActivity(slug, c.inv.values["--owner"])
	case "activity heartbeat":
		return service.HeartbeatActivity(slug, c.inv.values["--owner"])
	case "activity detach":
		return service.DetachActivity(slug, c.inv.values["--owner"])
	default:
		panic("unreachable effort activity action")
	}
}
func writeActivityReply(out io.Writer, reply effort.ActivityReply) error {
	return writeEffortActivityProtocol(out, reply)
}

func integrationGateCommand(root string) (string, error) {
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	command, ok := cfg.Vars["gateCmd"].(string)
	if !ok {
		return "", nil
	}
	return strings.TrimSpace(command), nil
}

func validateEffortGrammar(c *cmdCtx) error {
	if c.sub == "memory" {
		return &usageErr{"usage: awf effort memory update <slug> [--phase <text>] [--next <text>]"}
	}
	if c.sub == "activity" {
		return &usageErr{"usage: awf effort activity <attach|heartbeat|detach>"}
	}
	if c.sub == "memory update" {
		if _, phase := c.inv.values["--phase"]; !phase {
			if _, next := c.inv.values["--next"]; !next {
				return &usageErr{"usage: awf effort memory update <slug> [--phase <text>] [--next <text>]"}
			}
		}
		return nil
	}
	if strings.HasPrefix(c.sub, "activity ") {
		return validateEffortActivityGrammar(c)
	}
	if c.sub == "new" {
		if _, ok := c.inv.values["--slug"]; !ok {
			return &usageErr{"awf effort new: --slug is required"}
		}
		if c.inv.bools["--no-worktree"] && c.inv.values["--base"] != "" {
			return &usageErr{"awf effort new: --base is invalid with --no-worktree"}
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

func validateEffortActivityGrammar(c *cmdCtx) error {
	usage := "usage: awf effort " + c.sub
	if len(c.inv.positionals) != 1 || !activitySlugPattern.MatchString(firstPos(c.inv.positionals)) || len(firstPos(c.inv.positionals)) > 63 {
		return &usageErr{usage + " requires a canonical 1-63-byte slug"}
	}
	if !c.inv.bools["--json"] {
		return &usageErr{usage + " requires --json"}
	}
	for _, flag := range activityRequiredFlags(c.sub) {
		if _, ok := c.inv.values[flag]; !ok {
			return &usageErr{usage + " requires " + flag}
		}
	}
	if len(c.inv.values) != 1 {
		return &usageErr{usage + " accepts only --owner and --json"}
	}
	if !activityOwnerPattern.MatchString(c.inv.values["--owner"]) {
		return &usageErr{usage + " requires a lowercase UUIDv4 owner"}
	}
	return nil
}

func activityRequiredFlags(action string) []string {
	switch action {
	case "activity attach", "activity heartbeat", "activity detach":
		return []string{"--owner"}
	default:
		return nil
	}
}
func writeWorktreeResult(out io.Writer, result worktree.Result, operationErr error) error {
	if operationErr != nil {
		return operationErr
	}
	mutation, err := result.Mutation()
	if err != nil {
		return err
	}
	document, err := mutation.Document()
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return err
	}
	return presentation.Render(out, document)
}

func writeEffort(out io.Writer, record effort.Record) error {
	detail, err := record.Detail()
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return err
	}
	document, err := detail.Document()
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return err
	}
	return presentation.Render(out, document)
}

func writeEffortNew(out io.Writer, record effort.Record, result worktree.Result) error {
	mutation, err := result.Mutation()
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return err
	}
	mutation, err = record.NewEffortMutation(mutation)
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return err
	}
	document, err := mutation.Document()
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return err
	}
	return presentation.Render(out, document)
}

// writeEffortActivityProtocol writes the documented activity JSON protocol.
// It is a closed successful protocol bypass.
func writeEffortActivityProtocol(out io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil { // coverage-ignore: fixed protocol types cannot fail encoding; writer failures are covered at the shared output boundary
		return err
	}
	raw = append(raw, '\n')
	_, err = out.Write(raw)
	return err
}
