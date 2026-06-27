// Command awf renders standardised .claude skills, review agents, and git hooks into a project from embedded templates plus a per-project .awf/ config tree.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

func main() { os.Exit(run(os.Args, os.Stdout, os.Stderr)) } // coverage-ignore: os.Exit wrapper; run() is unit-tested

var getwd = os.Getwd

// run dispatches a subcommand and returns a process exit code. All user-facing
// output goes to the injected writers so the dispatch is unit-testable.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: awf <init|sync|check|invariants|audit|list|add|setup|upgrade|uninstall> [args]")
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
		cmdErr = runInit(cwd, hasFlag(args, "--force"), hasFlag(args, "--force-hooks"), stdout, stderr)
	case "sync":
		cmdErr = runSync(cwd, stdout)
	case "check":
		cmdErr = runCheck(cwd, stdout)
	case "invariants":
		cmdErr = runInvariants(cwd, stdout)
	case "audit":
		cmdErr = runAudit(cwd, baseFlag(args), stdout)
	case "list":
		cmdErr = runList(cwd, stdout)
	case "add":
		if len(args) < 3 {
			fmt.Fprintln(stderr, "awf:", errors.New("usage: awf add <skill>"))
			return 1
		}
		cmdErr = runAdd(cwd, args[2], stdout)
	case "setup":
		cmdErr = runSetup(cwd, hasFlag(args, "--force-hooks"), stdout, stderr)
	case "upgrade":
		cmdErr = runUpgrade(cwd, stdout)
	case "uninstall":
		cmdErr = runUninstall(cwd, stdout)
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

// hasFlag reports whether flag appears anywhere in args[2:].
func hasFlag(args []string, flag string) bool {
	for _, a := range args[2:] {
		if a == flag {
			return true
		}
	}
	return false
}

// baseFlag returns the value after --base in args[2:], or "" if it is absent or
// has no following value.
func baseFlag(args []string) string {
	rest := args[2:]
	for i, a := range rest {
		if a == "--base" && i+1 < len(rest) {
			return rest[i+1]
		}
	}
	return ""
}

func runInit(root string, force, forceHooks bool, stdout, stderr io.Writer) error {
	cfgPath := filepath.Join(root, ".awf", "config.yaml")
	scaffolded := false
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil { // coverage-ignore: entering this block needs cfgPath absent, which precludes a parent collision making MkdirAll fail
			return err
		}
		scaffold, err := project.ScaffoldConfig(filepath.Base(root))
		if err != nil { // coverage-ignore: ScaffoldConfig renders a static template over a dir basename; cannot fail in practice
			return err
		}
		if err := os.WriteFile(cfgPath, scaffold, 0o644); err != nil { // coverage-ignore: post-MkdirAll write; fails only on a permission fault that root bypasses
			return err
		}
		scaffolded = true
		fmt.Fprintf(stdout, "scaffolded %s\n", cfgPath)
	}
	collisions, err := initCollisions(root)
	if err != nil {
		return err
	}
	if len(collisions) > 0 {
		if !force {
			if scaffolded {
				os.RemoveAll(filepath.Dir(cfgPath)) // writes nothing on abort
			}
			return fmt.Errorf("awf init: refusing to overwrite existing files (use --force):\n  %s",
				strings.Join(collisions, "\n  "))
		}
		// --force: back up each colliding non-managed file before sync overwrites it.
		// invariant: init-force-backs-up
		for _, rel := range collisions {
			bak := rel + ".awf-bak"
			if err := copyFile(filepath.Join(root, rel), filepath.Join(root, bak)); err != nil { // coverage-ignore: rel is a known-existing collision and bak is a sibling path; copyFile fails only on a permission fault root bypasses
				return fmt.Errorf("awf init: back up %s: %w", rel, err)
			}
			fmt.Fprintf(stdout, "backed up %s → %s\n", rel, bak)
		}
	}
	if err := runSync(root, stdout); err != nil {
		return err
	}
	if err := runSetup(root, forceHooks, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "awf init: hook setup skipped:", err)
	}
	return nil
}

// copyFile copies src to dst, preserving the source file's permission bits.
func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil { // coverage-ignore: src is a known-existing collision path
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil { // coverage-ignore: src was just stat'd and is readable
		return err
	}
	return os.WriteFile(dst, data, info.Mode().Perm())
}

// initCollisions returns planned output paths that already exist on disk and are
// not recorded in the prior lock (i.e. not awf-managed). An awf-managed path that
// already exists is not a collision — re-init is idempotent.
func initCollisions(root string) ([]string, error) {
	p, err := project.Open(root)
	if err != nil {
		return nil, err
	}
	planned, err := p.PlannedOutputs()
	if err != nil {
		return nil, err
	}
	managed := map[string]bool{}
	if lock, err := manifest.Load(filepath.Join(root, ".awf", "awf.lock")); err == nil {
		for path := range lock.Files {
			managed[path] = true
		}
	}
	var collisions []string
	for _, rel := range planned {
		if managed[rel] {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			collisions = append(collisions, rel)
		}
	}
	sort.Strings(collisions)
	return collisions, nil
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
