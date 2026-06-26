// Command awf renders standardised .claude skills, review agents, and git hooks into a project from embedded templates plus a per-project .claude/awf/ config tree.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

func main() { os.Exit(run(os.Args, os.Stdout, os.Stderr)) } // coverage-ignore: os.Exit wrapper; run() is unit-tested

var getwd = os.Getwd

// run dispatches a subcommand and returns a process exit code. All user-facing
// output goes to the injected writers so the dispatch is unit-testable.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: awf <init|sync|check|invariants|list|add|setup|upgrade> [args]")
		return 2
	}
	cwd, err := getwd()
	if err != nil {
		fmt.Fprintln(stderr, "awf:", err)
		return 1
	}
	var cmdErr error
	switch args[1] {
	case "init":
		cmdErr = runInit(cwd, stdout, stderr)
	case "sync":
		cmdErr = runSync(cwd, stdout)
	case "check":
		cmdErr = runCheck(cwd, stdout)
	case "invariants":
		cmdErr = runInvariants(cwd, stdout)
	case "list":
		cmdErr = runList(cwd, stdout)
	case "add":
		if len(args) < 3 {
			fmt.Fprintln(stderr, "awf:", errors.New("usage: awf add <skill>"))
			return 1
		}
		cmdErr = runAdd(cwd, args[2], stdout)
	case "setup":
		cmdErr = runSetup(cwd, stdout, stderr)
	case "upgrade":
		cmdErr = runUpgrade(cwd, stdout)
	default:
		fmt.Fprintln(stderr, "awf:", fmt.Errorf("unknown command %q", args[1]))
		return 1
	}
	if cmdErr != nil {
		fmt.Fprintln(stderr, "awf:", cmdErr)
		return 1
	}
	return 0
}

func runInit(root string, stdout, stderr io.Writer) error {
	cfgPath := filepath.Join(root, ".claude", "awf", "config.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
			return err
		}
		scaffold, err := project.ScaffoldConfig(filepath.Base(root))
		if err != nil {
			return err
		}
		if err := os.WriteFile(cfgPath, scaffold, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "scaffolded %s\n", cfgPath)
	}
	if err := runSync(root, stdout); err != nil {
		return err
	}
	if err := runSetup(root, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "awf init: hook setup skipped:", err)
	}
	return nil
}

// gate refuses to operate against a stale config layout. It runs before
// project.Open (which cannot open a pre-tree project): on a covered schema gap it
// errors with a "run awf upgrade" message; an uncovered gap ("autobump") proceeds
// and the subsequent sync stamps the current schema version.
func gate(root string) error {
	if migrate.GateState(root) == "gate" {
		return fmt.Errorf("config schema is behind (generation %d < %d); run awf upgrade",
			migrate.Generation(root), migrate.Current())
	}
	return nil
}

func runSync(root string, stdout io.Writer) error {
	if err := gate(root); err != nil {
		return err
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	if err := p.Sync(); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "awf sync: done")
	return nil
}
