// cmd/awf/main.go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"agentic-workflows/internal/project"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: awf <init|sync|check|list|add> [args]")
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
	case "list":
		fatalIf(runList(cwd))
	case "add":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("usage: awf add <skill>"))
		}
		fatalIf(runAdd(cwd, os.Args[2]))
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
		scaffold := fmt.Sprintf("prefix: %s\nvars: {}\nskills: {}\nagents: {}\nhooks: []\n", filepath.Base(root))
		if err := os.WriteFile(cfgPath, []byte(scaffold), 0o644); err != nil {
			return err
		}
		fmt.Printf("scaffolded %s\n", cfgPath)
	}
	return runSync(root)
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
