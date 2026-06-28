// Command awf renders standardised .claude skills, review agents, and git hooks into a project from embedded templates plus a per-project .awf/ config tree.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/initspec"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/templates"
)

func main() { os.Exit(run(os.Args, os.Stdout, os.Stderr)) } // coverage-ignore: os.Exit wrapper; run() is unit-tested

var getwd = os.Getwd

var stdin io.Reader = os.Stdin

// isInteractive reports whether stdin is a terminal (so init should prompt).
var isInteractive = func() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// run dispatches a subcommand and returns a process exit code. All user-facing
// output goes to the injected writers so the dispatch is unit-testable.
const helpText = `awf — render agentic-workflow tooling into a project from a committed .awf/ config tree

Usage: awf <command> [flags]

Commands:
  init         Scaffold .awf/, render the workflow-core set, and activate git hooks
                 --force        overwrite colliding files, backing each up to <path>.awf-bak
                 --force-hooks  take over an existing core.hooksPath (husky/lefthook)
                 --describe     print the fillable value descriptors as JSON and exit
                 --set k=v      set a value non-interactively (repeatable)
                 --answers FILE read values from a JSON/YAML answers file
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
		cmdErr = runInit(cwd, hasFlag(args, "--force"), hasFlag(args, "--force-hooks"),
			hasFlag(args, "--describe"), setFlags(args), valueFlag(args, "--answers"),
			stdout, stderr)
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
	"init":       {boolFlags: []string{"--force", "--force-hooks", "--describe"}, valueFlags: []string{"--set", "--answers"}, maxPos: 0},
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
	return valueFlag(args, "--base")
}

// valueFlag returns the value after the first occurrence of flag in args[2:], or
// "" if it is absent or has no following value.
func valueFlag(args []string, flag string) string {
	rest := args[2:]
	for i, a := range rest {
		if a == flag && i+1 < len(rest) {
			return rest[i+1]
		}
	}
	return ""
}

// setFlags returns every value following a --set occurrence in args[2:].
func setFlags(args []string) []string {
	var out []string
	rest := args[2:]
	for i, a := range rest {
		if a == "--set" && i+1 < len(rest) {
			out = append(out, rest[i+1])
		}
	}
	return out
}

func runInit(root string, force, forceHooks, describe bool, sets []string, answersFile string, stdout, stderr io.Writer) error {
	cat, err := catalog.Load(templates.FS)
	if err != nil { // coverage-ignore: catalog.Load over the embedded FS cannot fail at runtime
		return err
	}
	if describe {
		out, err := initspec.Describe(cat.Vars)
		if err != nil { // coverage-ignore: descriptors marshal to JSON; cannot fail
			return err
		}
		fmt.Fprintln(stdout, string(out))
		return nil
	}
	answers := map[string]string{}
	if answersFile != "" {
		b, err := os.ReadFile(answersFile)
		if err != nil {
			return fmt.Errorf("awf init: read --answers: %w", err)
		}
		if answers, err = initspec.ParseAnswersFile(b); err != nil {
			return err
		}
	}
	if err := initspec.MergeSetFlags(answers, sets); err != nil {
		return err
	}
	vars, inv, err := initspec.Resolve(cat.Vars, answers, stdin, stdout, isInteractive())
	if err != nil {
		return err
	}

	cfgPath := filepath.Join(root, ".awf", "config.yaml")
	scaffolded := false
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil { // coverage-ignore: entering this block needs cfgPath absent, which precludes a parent collision making MkdirAll fail
			return err
		}
		scaffold, err := project.ScaffoldConfig(filepath.Base(root), vars, inv, nil)
		if err != nil { // coverage-ignore: ScaffoldConfig renders a static template over a dir basename; cannot fail in practice
			return err
		}
		if err := os.WriteFile(cfgPath, scaffold, 0o644); err != nil { // coverage-ignore: post-MkdirAll write; fails only on a permission fault that root bypasses
			return err
		}
		scaffolded = true
		fmt.Fprintf(stdout, "scaffolded %s\n", cfgPath)
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	collisions, err := p.InitCollisions()
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
		for _, rel := range collisions {
			bakRel, err := p.BackupFile(rel)
			if err != nil { // coverage-ignore: p.BackupFile only fails on a copyFile permission fault that root bypasses
				return fmt.Errorf("awf init: back up %s: %w", rel, err)
			}
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
