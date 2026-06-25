// Command awf renders standardised .claude skills, review agents, and git hooks into a project from embedded templates plus a per-project .claude/awf.yaml.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: awf <init|sync|check|invariants|list|add|setup> [args]")
		os.Exit(2)
	}
	cwd, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	switch os.Args[1] {
	case "init":
		fatalIf(runInit(cwd))
	case "sync":
		fatalIf(runSync(cwd))
	case "check":
		fatalIf(runCheck(cwd))
	case "invariants":
		fatalIf(runInvariants(cwd))
	case "list":
		fatalIf(runList(cwd))
	case "add":
		if len(os.Args) < 3 {
			fatal(errors.New("usage: awf add <skill>"))
		}
		fatalIf(runAdd(cwd, os.Args[2]))
	case "setup":
		fatalIf(runSetup(cwd))
	default:
		fatal(fmt.Errorf("unknown command %q", os.Args[1]))
	}
}

func runInit(root string) error {
	cfgPath := filepath.Join(root, ".claude", "awf.yaml")
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
		fmt.Printf("scaffolded %s\n", cfgPath)
	}
	if err := runSync(root); err != nil {
		return err
	}
	if err := runSetup(root); err != nil {
		fmt.Fprintln(os.Stderr, "awf init: hook setup skipped:", err)
	}
	return nil
}

func runSync(root string) error {
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	if err := p.Sync(); err != nil {
		return err
	}
	fmt.Println("awf sync: done")
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "awf:", err)
	os.Exit(1)
}

func fatalIf(err error) {
	if err != nil {
		fatal(err)
	}
}
