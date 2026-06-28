// Command awf renders standardised .claude skills, review agents, and git hooks into a project from embedded templates plus a per-project .awf/ config tree.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
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
const helpText = `awf — render agentic-workflow tooling into a project from a committed .awf/ config tree

Usage: awf <command> [flags]

Commands:
  init         Scaffold .awf/, render the workflow-core set, and activate git hooks
                 --force        overwrite colliding files, backing each up to <path>.awf-bak
                 --force-hooks  take over an existing core.hooksPath (husky/lefthook)
  sync         Re-render after a template or config change
  check        Fail on stale or hand-edited rendered output
  list [<kind>]        Show targets and their per-project state (all kinds, or one)
  add <kind> <name>    Enable a target — kind ∈ {skill, agent, doc, hook, domain}
  remove <kind> <name> Disable a target (a freeform domain, or a catalog target)
  setup        Activate git hooks (core.hooksPath=.githooks)
                 --force-hooks  take over an existing core.hooksPath
  audit        Report workflow-conformance findings over the branch (advisory)
                 --base <ref>   compare against <ref> instead of the configured base branch
  invariants   Report Implemented-ADR invariant slugs lacking a backing comment
  upgrade      Migrate the .awf/ config tree to the current schema
  uninstall    Remove awf's generated files and unset core.hooksPath (keeps .awf/)
  version      Print the awf version
`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: awf <init|sync|check|invariants|audit|list|add|remove|setup|upgrade|uninstall|version> [args]")
		fmt.Fprintln(stderr, "run `awf help` for command details")
		return 2
	}
	if a := args[1]; a == "help" || a == "--help" || a == "-h" {
		fmt.Fprint(stdout, helpText)
		return 0
	}
	cwd, err := getwd()
	if err != nil {
		fmt.Fprintln(stderr, "awf:", err)
		return 1
	}
	if spec, ok := argSpecs[args[1]]; ok {
		if err := checkArgs(args[1], args[2:], spec.boolFlags, spec.valueFlags, spec.minPos, spec.maxPos); err != nil {
			fmt.Fprintln(stderr, "awf:", err)
			return 2
		}
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
		kindFilter := ""
		if len(args) >= 3 {
			kindFilter = args[2]
		}
		cmdErr = runList(cwd, kindFilter, stdout)
	case "add":
		switch len(args) {
		case 4:
			cmdErr = runAdd(cwd, args[2], args[3], stdout)
		case 3:
			cmdErr = &usageErr{fmt.Sprintf("awf add requires a kind: awf add <kind> <name> (e.g. awf add skill %s)", args[2])}
		default:
			cmdErr = &usageErr{"usage: awf add <kind> <name>"}
		}
	case "remove":
		switch len(args) {
		case 4:
			cmdErr = runRemove(cwd, args[2], args[3], stdout)
		default:
			cmdErr = &usageErr{"usage: awf remove <kind> <name>"}
		}
	case "setup":
		cmdErr = runSetup(cwd, hasFlag(args, "--force-hooks"), stdout, stderr)
	case "upgrade":
		cmdErr = runUpgrade(cwd, stdout)
	case "uninstall":
		cmdErr = runUninstall(cwd, stdout)
	case "version":
		runVersion(stdout)
	default:
		cmdErr = &usageErr{fmt.Sprintf("unknown command %q", args[1])}
	}
	if cmdErr != nil {
		fmt.Fprintln(stderr, "awf:", cmdErr)
		var ue *usageErr
		if errors.As(cmdErr, &ue) {
			return 2
		}
		return 1
	}
	return 0
}

// usageErr marks a CLI-misuse error (unknown flag, bad arity, unknown command),
// which the central handler maps to exit code 2 rather than the failure code 1.
type usageErr struct{ msg string }

func (e *usageErr) Error() string { return e.msg }

// argSpec declares a subcommand's accepted flags and positional bounds. boolFlags
// take no value; valueFlags consume the following token; maxPos < 0 is unbounded
// (add/remove refine their arity in the switch to keep their specific messages).
type argSpec struct {
	boolFlags, valueFlags []string
	minPos, maxPos        int
}

var argSpecs = map[string]argSpec{
	"init":       {boolFlags: []string{"--force", "--force-hooks"}, maxPos: 0},
	"sync":       {maxPos: 0},
	"check":      {maxPos: 0},
	"invariants": {maxPos: 0},
	"audit":      {valueFlags: []string{"--base"}, maxPos: 0},
	"list":       {maxPos: 1},
	"add":        {maxPos: -1},
	"remove":     {maxPos: -1},
	"setup":      {boolFlags: []string{"--force-hooks"}, maxPos: 0},
	"upgrade":    {maxPos: 0},
	"uninstall":  {maxPos: 0},
	"version":    {maxPos: 0},
}

// checkArgs rejects unrecognized --flags and enforces the positional count for a
// subcommand. rest is args[2:]; a valueFlag consumes its following token.
func checkArgs(cmd string, rest []string, boolFlags, valueFlags []string, minPos, maxPos int) error {
	pos := 0
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case slices.Contains(valueFlags, a):
			if i+1 >= len(rest) {
				return &usageErr{fmt.Sprintf("awf %s: flag %s needs a value", cmd, a)}
			}
			i++ // consume the flag's value
		case slices.Contains(boolFlags, a):
			// recognized boolean flag
		case strings.HasPrefix(a, "-"):
			return &usageErr{fmt.Sprintf("awf %s: unknown flag %q", cmd, a)}
		default:
			pos++
		}
	}
	if pos < minPos || (maxPos >= 0 && pos > maxPos) {
		return &usageErr{fmt.Sprintf("awf %s: unexpected arguments", cmd)}
	}
	return nil
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
				_ = os.Remove(cfgPath)               // remove the config we scaffolded
				_ = os.Remove(filepath.Dir(cfgPath)) // remove .awf only if now empty
			}
			return fmt.Errorf("awf init: refusing to overwrite existing files (use --force):\n  %s",
				strings.Join(collisions, "\n  "))
		}
		// --force: back up each colliding non-managed file before sync overwrites it.
		// invariant: init-force-backs-up
		for _, rel := range collisions {
			src := filepath.Join(root, rel)
			bak := freeBackupPath(src)
			if err := copyFile(src, bak); err != nil { // coverage-ignore: rel is a known-existing collision and bak is a free sibling path; copyFile fails only on a permission fault root bypasses
				return fmt.Errorf("awf init: back up %s: %w", rel, err)
			}
			bakRel, _ := filepath.Rel(root, bak)
			fmt.Fprintf(stdout, "backed up %s → %s\n", rel, bakRel)
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

// freeBackupPath returns base+".awf-bak", or "...awf-bak.N" with the lowest N
// that does not yet exist, so a forced backup never overwrites a prior one.
func freeBackupPath(base string) string {
	p := base + ".awf-bak"
	for i := 1; fileExists(p); i++ {
		p = fmt.Sprintf("%s.awf-bak.%d", base, i)
	}
	return p
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
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
