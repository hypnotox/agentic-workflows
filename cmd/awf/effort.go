package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

var openWorktreeManager = worktree.Open

func runEffort(c *cmdCtx) error {
	if err := validateEffortGrammar(c); err != nil {
		return err
	}
	service, err := effort.Open(context.Background(), c.root, effort.Options{})
	if err != nil {
		return err
	}
	id := firstPos(c.inv.positionals)
	switch c.sub {
	case "new":
		record, err := service.New(id, !c.inv.bools["--no-memory"])
		if err != nil {
			return err
		}
		if c.inv.bools["--worktree"] {
			// Keep the durable allocation separately: a failed Add returns a zero
			// record and must not erase the ID reported by the partial-create
			// contract.
			createdID := record.ID
			manager, openErr := openWorktreeManager(context.Background(), c.root, worktree.Options{})
			if openErr != nil {
				return newWorktreeAttachmentError(createdID, openErr)
			}
			record, err = manager.Add(createdID, c.inv.values["--base"])
			if err != nil {
				return newWorktreeAttachmentError(createdID, err)
			}
		}
		return writeEffortText(c.stdout, record)
	case "list":
		records, err := service.List()
		if err != nil {
			return err
		}
		if c.inv.bools["--json"] {
			return writeEffortJSON(c.stdout, struct {
				SchemaVersion int             `json:"schemaVersion"`
				Efforts       []effort.Record `json:"efforts"`
			}{effort.SchemaVersion, records})
		}
		for _, record := range records {
			if err := writeEffortText(c.stdout, record); err != nil {
				return err
			}
		}
		return nil
	case "show":
		record, err := service.Show(id)
		if err != nil {
			return err
		}
		if c.inv.bools["--json"] {
			return writeEffortJSON(c.stdout, record)
		}
		return writeEffortText(c.stdout, record)
	case "rename":
		record, err := service.Rename(id, c.inv.positionals[1])
		if err != nil {
			return err
		}
		return writeEffortText(c.stdout, record)
	case "memory":
		path, record, err := service.Memory(id)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(c.stdout, "effort %s memory=%s\n", record.ID, path)
		return err
	case "complete":
		record, err := service.Complete(id)
		return runEffortMutation(c.stdout, record, err)
	case "abandon":
		record, err := service.Abandon(id)
		return runEffortMutation(c.stdout, record, err)
	case "reopen":
		record, err := service.Reopen(id)
		return runEffortMutation(c.stdout, record, err)
	case "worktree":
		if len(c.inv.positionals) != 2 {
			return &usageErr{"usage: awf effort worktree <add|remove> <id>"}
		}
		manager, err := openWorktreeManager(context.Background(), c.root, worktree.Options{})
		if err != nil {
			return err
		}
		var record effort.Record
		switch c.inv.positionals[0] {
		case "add":
			record, err = manager.Add(c.inv.positionals[1], c.inv.values["--base"])
		case "remove":
			record, err = manager.Remove(c.inv.positionals[1], c.inv.bools["--force"], c.inv.values["--reason"])
		default:
			return &usageErr{"usage: awf effort worktree <add|remove> <id>"}
		}
		return runEffortMutation(c.stdout, record, err)
	case "integrate":
		manager, err := openWorktreeManager(context.Background(), c.root, worktree.Options{})
		if err != nil {
			return err
		}
		record, err := manager.Integrate(id, c.inv.bools["--force"], c.inv.values["--reason"])
		return runEffortMutation(c.stdout, record, err)
	case "integrated":
		manager, err := openWorktreeManager(context.Background(), c.root, worktree.Options{})
		if err != nil {
			return err
		}
		record, err := manager.RecordManualIntegration(id, c.inv.values["--commit"], c.inv.bools["--force"], c.inv.values["--reason"])
		return runEffortMutation(c.stdout, record, err)
	case "repair":
		result, err := service.Repair(id)
		if err != nil {
			return err
		}
		if c.inv.bools["--json"] {
			return writeEffortJSON(c.stdout, result)
		}
		if _, err := fmt.Fprintf(c.stdout, "effort %s repaired changes=%d\n", result.Record.ID, len(result.Changes)); err != nil {
			return err
		}
		for _, change := range result.Changes {
			if _, err := fmt.Fprintf(c.stdout, "change %s from=%v to=%v\n", change.Field, change.From, change.To); err != nil { // coverage-ignore: fixed result types cannot fail encoding; writer failures are covered at the shared output boundary
				return err
			}
		}
		return nil
	default:
		return &usageErr{"usage: awf effort <new|list|show|rename|memory|worktree|integrate|integrated|complete|abandon|reopen|repair>"}
	}
}

type worktreeAttachmentError struct {
	EffortID string
	Category string
	Cause    error
}

func (e *worktreeAttachmentError) Error() string {
	return fmt.Sprintf("effortId=%s state=active worktreeAttached=false category=%s next=\"awf effort worktree add %s\": %v", e.EffortID, e.Category, e.EffortID, e.Cause)
}
func (e *worktreeAttachmentError) Unwrap() error { return e.Cause }

func newWorktreeAttachmentError(id string, err error) error {
	category := "unknown"
	var refusal *worktree.RefusalError
	if errors.As(err, &refusal) {
		category = refusal.Category
	}
	return &worktreeAttachmentError{EffortID: id, Category: category, Cause: err}
}

func validateEffortGrammar(c *cmdCtx) error {
	force, reason := c.inv.bools["--force"], strings.TrimSpace(c.inv.values["--reason"])
	if c.sub == "worktree" && len(c.inv.positionals) > 0 && c.inv.positionals[0] == "add" && (force || reason != "") {
		return &usageErr{"awf effort worktree add: --force and --reason are not allowed"}
	}
	if force != (reason != "") {
		return &usageErr{fmt.Sprintf("awf effort %s: --force and --reason must be provided together", c.sub)}
	}
	switch c.sub {
	case "new":
		if c.inv.values["--base"] != "" && !c.inv.bools["--worktree"] {
			return &usageErr{"awf effort new: --base requires --worktree"}
		}
	case "worktree":
		if len(c.inv.positionals) > 0 && c.inv.positionals[0] != "add" && c.inv.values["--base"] != "" {
			return &usageErr{"awf effort worktree remove: --base is not allowed"}
		}
	case "integrate":
		if c.inv.values["--base"] != "" {
			return &usageErr{"awf effort integrate: --base is not allowed"}
		}
	case "integrated":
		if c.inv.values["--base"] != "" {
			return &usageErr{"awf effort integrated: --base is not allowed"}
		}
		if c.inv.values["--commit"] == "" {
			return &usageErr{"awf effort integrated: --commit is required"}
		}
	}
	return nil
}

func runEffortMutation(out io.Writer, record effort.Record, err error) error {
	if err != nil {
		return err
	}
	return writeEffortText(out, record)
}

func writeEffortText(out io.Writer, record effort.Record) error {
	sessions := strings.Join(record.AssignedSessionIDs, ",")
	_, err := fmt.Fprintf(out, "effort %s state=%s title=%s memory=%t integration=%s sessions=%s\n", record.ID, record.State, strconv.Quote(record.Title), record.MemoryPresent, record.Integration, sessions)
	return err
}

func writeEffortJSON(out io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil { // coverage-ignore: fixed result types cannot fail encoding; writer failures are covered at the shared output boundary
		return err
	}
	raw = append(raw, '\n')
	_, err = out.Write(raw)
	return err
}
