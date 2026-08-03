package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/effort"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
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
	if err != nil { // coverage-ignore: ResolveControlRoots just proved this path is a checkout; a failed open requires a concurrent repository-identity race
		return effortComposition{}, err
	}
	service, err := effort.Open(roots, effort.Dependencies{
		Clock:           time.Now,
		UUID:            effort.RandomUUIDv4,
		Worktrees:       repo.WorktreeList,
		BranchExists:    repo.BranchExists,
		ValidateRef:     repo.ValidateRefName,
		RemoveTree:      os.RemoveAll,
		ResolveCheckout: resolveEffortCheckout,
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

// resolveEffortCheckout is the sole Git-to-effort error translation boundary.
// It intentionally retains only standard-library error prose: callers can use
// effort's closed kind without receiving a Git implementation type.
func resolveEffortCheckout(ctx context.Context, path string) (effort.CheckoutFacts, error) {
	roots, err := awfgit.ResolveControlRoots(ctx, path)
	if err == nil {
		return effort.CheckoutFacts{InvokingRoot: roots.InvokingRoot, PrimaryRoot: roots.PrimaryRoot}, nil
	}
	return effort.CheckoutFacts{}, normalizeCheckoutResolutionError(err)
}

func normalizeCheckoutResolutionError(err error) *effort.CheckoutResolutionError {
	kind := effort.CheckoutRepositoryMismatch
	var hard *awfgit.HardSafetyError
	if errors.As(err, &hard) {
		switch hard.Category {
		case "symlink", "foreign-owner", "file-type", "resident-permissions":
			kind = effort.CheckoutUnsafe
		case "repository-identity", "bare-repository", "missing-primary", "ambiguous-primary", "unconfined":
			kind = effort.CheckoutRepositoryMismatch
		}
	}
	// Do not retain err as a cause: errors.As at the effort boundary must never
	// expose a Git mechanism type, including HardSafetyError with nil Err.
	return effort.NewCheckoutResolutionError(kind, errors.New(err.Error()))
}

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
			return writeEffortNew(c.stdout, record, absent, c.inv.bools["--json"])
		}
		record, result, err := manager.NewEffort(c.ctx, input, c.inv.values["--base"])
		if err != nil {
			return err
		}
		return writeEffortNew(c.stdout, record, result, c.inv.bools["--json"])
	case "list":
		records, err := service.List()
		if err != nil {
			return err
		}
		if c.inv.bools["--json"] {
			return writeEffortJSON(c.stdout, struct {
				SchemaVersion int             `json:"schemaVersion"`
				Efforts       []effort.Record `json:"efforts"`
			}{SchemaVersion: effort.SchemaVersion, Efforts: records})
		}
		for _, record := range records {
			if err := writeEffortText(c.stdout, record); err != nil {
				return err
			}
		}
		return nil
	case "show":
		record, err := service.Show(selected)
		if err != nil {
			return err
		}
		return writeEffort(c.stdout, record, c.inv.bools["--json"])
	case "finish":
		result, err := service.Finish(c.ctx, selected)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(c.stdout, "effort %s finished; changed active rename: %s; changed cleanup: %s; next action: continue without this finished effort\n", selected, yesNo(result.Renamed), yesNo(result.Cleaned))
		return err
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
	case "activity resolve", "activity attach", "activity heartbeat", "activity checkout", "activity detach":
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
	case "activity resolve":
		return service.ResolveActivity(c.ctx, slug, effort.CheckoutRole(c.inv.values["--destination"]), c.inv.values["--receiving-checkout"])
	case "activity attach":
		return service.AttachActivity(c.ctx, slug, effort.Activity{
			SchemaVersion:     1,
			Owner:             c.inv.values["--owner"],
			CWD:               c.inv.values["--cwd"],
			ReceivingCheckout: c.inv.values["--receiving-checkout"],
			Role:              effort.CheckoutRole(c.inv.values["--role"]),
		})
	case "activity heartbeat":
		return service.HeartbeatActivity(slug, c.inv.values["--owner"])
	case "activity checkout":
		return service.CheckoutActivity(c.ctx, slug, c.inv.values["--owner"], c.inv.values["--cwd"], effort.CheckoutRole(c.inv.values["--role"]))
	case "activity detach":
		return service.DetachActivity(slug, c.inv.values["--owner"])
	default: // coverage-ignore: validateEffortGrammar admits only the closed activity action set
		panic("unreachable effort activity action")
	}
}

func writeActivityReply(out io.Writer, reply effort.ActivityReply) error {
	return writeEffortJSON(out, reply)
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
		return &usageErr{"usage: awf effort activity <resolve|attach|heartbeat|checkout|detach>"}
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
	if !c.inv.bools["--json"] {
		return &usageErr{usage + " requires --json"}
	}
	for _, flag := range activityRequiredFlags(c.sub) {
		if _, ok := c.inv.values[flag]; !ok {
			return &usageErr{usage + " requires " + flag}
		}
	}
	return nil
}

func activityRequiredFlags(action string) []string {
	switch action {
	case "activity resolve":
		return []string{"--destination"}
	case "activity attach":
		return []string{"--owner", "--cwd", "--role", "--receiving-checkout"}
	case "activity heartbeat", "activity detach":
		return []string{"--owner"}
	case "activity checkout":
		return []string{"--owner", "--cwd", "--role"}
	default: // coverage-ignore: clispec admits only this closed action set
		return nil
	}
}

func writeWorktreeResult(out io.Writer, result worktree.Result, err error) error {
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, result.String())
	return err
}

func writeEffort(out io.Writer, record effort.Record, jsonOutput bool) error {
	if jsonOutput {
		return writeEffortJSON(out, struct {
			SchemaVersion int           `json:"schemaVersion"`
			Effort        effort.Record `json:"effort"`
		}{SchemaVersion: effort.SchemaVersion, Effort: record})
	}
	return writeEffortText(out, record)
}

// writeEffortNew emits the `new` reply: the effort facts with the managed
// worktree facts, then the mutation-protocol line from the one Result
// formatter. An empty Result.Path is the explicit-absence form that
// --no-worktree produces: text `worktree=none`, JSON `"worktree":null`.
func writeEffortNew(out io.Writer, record effort.Record, result worktree.Result, jsonOutput bool) error {
	if jsonOutput {
		reply := struct {
			SchemaVersion int                  `json:"schemaVersion"`
			Effort        effort.Record        `json:"effort"`
			Worktree      *effortWorktreeFacts `json:"worktree"`
		}{SchemaVersion: effort.SchemaVersion, Effort: record}
		if result.Path != "" {
			reply.Worktree = &effortWorktreeFacts{Path: result.Path, Branch: result.Branch}
		}
		return writeEffortJSON(out, reply)
	}
	facts := "worktree=none"
	if result.Path != "" {
		facts = "worktree=" + result.Path + " branch=" + result.Branch
	}
	_, err := fmt.Fprintf(out, "effort %s title=%q memory=%s %s\n%s\n", record.Slug, record.Title, record.MemoryPath, facts, result.String())
	return err
}

type effortWorktreeFacts struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
}

func writeEffortText(out io.Writer, record effort.Record) error {
	_, err := fmt.Fprintf(out, "effort %s title=%q memory=%s\n", record.Slug, record.Title, record.MemoryPath)
	return err
}

func writeEffortJSON(out io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil { // coverage-ignore: fixed protocol types cannot fail encoding; writer failures are covered at the shared output boundary
		return err
	}
	raw = append(raw, '\n')
	_, err = out.Write(raw)
	return err
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
