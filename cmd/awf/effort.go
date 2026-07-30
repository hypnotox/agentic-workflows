package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

var openWorktreeManager = worktree.Open

func runEffort(c *cmdCtx) error {
	if err := validateEffortGrammar(c); err != nil {
		return err
	}
	service, err := effort.Open(c.ctx, c.root, effort.Options{})
	if err != nil {
		return err
	}
	slug := firstPos(c.inv.positionals)
	switch c.sub {
	case "new":
		record, err := service.New(slug)
		if err != nil {
			return err
		}
		return writeEffort(c.stdout, record, c.inv.bools["--json"])
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
		record, err := service.Show(slug)
		if err != nil {
			return err
		}
		return writeEffort(c.stdout, record, c.inv.bools["--json"])
	case "finish":
		result, err := service.Finish(slug)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(c.stdout, "effort %s finished; changed active rename: %s; changed cleanup: %s; next action: continue without this finished effort\n", slug, yesNo(result.Renamed), yesNo(result.Cleaned))
		return err
	case "worktree":
		manager, err := openWorktreeManager(c.ctx, c.root, worktree.Options{})
		if err != nil {
			return err
		}
		action, selected := c.inv.positionals[0], c.inv.positionals[1]
		var result worktree.Result
		switch action {
		case "add":
			result, err = manager.Add(selected, c.inv.values["--base"])
		case "remove":
			result, err = manager.Remove(selected)
		default: // coverage-ignore: validateEffortGrammar accepts only add or remove before this closed dispatch
			return &usageErr{"usage: awf effort worktree <add|remove> <slug>"}
		}
		return writeWorktreeResult(c.stdout, result, err)
	case "integrate":
		manager, err := openWorktreeManager(c.ctx, c.root, worktree.Options{})
		if err != nil {
			return err
		}
		result, err := manager.Integrate(slug)
		return writeWorktreeResult(c.stdout, result, err)
	default:
		return &usageErr{"usage: awf effort <new|list|show|finish|worktree|integrate>"}
	}
}

func validateEffortGrammar(c *cmdCtx) error {
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
