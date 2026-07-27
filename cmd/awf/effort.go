package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/effort"
)

func runEffort(c *cmdCtx) error {
	service, err := effort.Open(context.Background(), c.root, effort.Options{})
	if err != nil {
		return err
	}
	id := firstPos(c.inv.positionals)
	switch c.sub {
	case "new":
		if c.inv.bools["--worktree"] {
			return &usageErr{"awf effort new: --worktree is reserved until managed worktrees are available"}
		}
		record, err := service.New(id, !c.inv.bools["--no-memory"])
		if err != nil {
			return err
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
		return &usageErr{"usage: awf effort <new|list|show|rename|memory|complete|abandon|reopen|repair>"}
	}
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
