// Command awf renders standardised .claude skills, review agents, and docs into a project from embedded templates plus a per-project .awf/ config tree.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

func main() { os.Exit(run(os.Args, os.Stdout, os.Stderr)) } // coverage-ignore: os.Exit wrapper; run() is unit-tested

var getwd = os.Getwd

var stdin io.Reader = os.Stdin

// isInteractive reports whether stdin is a terminal (so init should prompt).
var isInteractive = func() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// globalHelp renders the top-level `awf help` overview from each command's summary,
// so the overview and the per-command `awf <cmd> --help` texts share one source —
// the internal/clispec table (inv: cli-command-spec-single-source).
func globalHelp() string {
	var b strings.Builder
	b.WriteString("awf — render agentic-workflow tooling into a project from a committed .awf/ config tree\n\n")
	b.WriteString("Usage: awf <command> [flags]\n\n")
	b.WriteString("Commands:\n")
	for _, c := range clispec.Commands {
		fmt.Fprintf(&b, "  %-12s %s\n", c.Name, c.Summary)
	}
	b.WriteString("\nRun `awf <command> --help` for details on a command.\n")
	return b.String()
}

// hasHelpFlag reports whether a --help or -h token appears among a command's args.
func hasHelpFlag(rest []string) bool {
	for _, a := range rest {
		if a == "--help" || a == "-h" {
			return true
		}
	}
	return false
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage:", clispec.UsageLine(), "[args]")
		fmt.Fprintln(stderr, "run `awf help` for command details")
		return 2
	}
	if a := args[1]; a == "help" || a == "--help" || a == "-h" {
		if a == "help" && len(args) >= 3 {
			if spec, ok := clispec.Lookup(args[2]); ok {
				fmt.Fprint(stdout, spec.HelpBody)
				return 0
			}
		}
		fmt.Fprint(stdout, globalHelp())
		return 0
	}
	cwd, err := getwd()
	if err != nil {
		fmt.Fprintln(stderr, "awf:", err)
		return 1
	}
	if spec, ok := clispec.Lookup(args[1]); ok {
		if hasHelpFlag(args[2:]) { // `awf <cmd> --help`/`-h` — intercept before checkArgs rejects it
			fmt.Fprint(stdout, spec.HelpBody)
			return 0
		}
		if err := checkArgs(spec, args[2:]); err != nil {
			fmt.Fprintln(stderr, "awf:", err)
			return 2
		}
	}
	var cmdErr error
	switch args[1] {
	case "init":
		cmdErr = runInit(cwd, hasFlag(args, "--force"),
			hasFlag(args, "--describe"), setFlags(args), valueFlag(args, "--answers"),
			stdout)
	case "sync":
		cmdErr = runSync(cwd, stdout)
	case "check":
		cmdErr = runCheck(cwd, stdout)
	case "invariants":
		cmdErr = runInvariants(cwd, stdout)
	case "audit":
		cmdErr = runAudit(cwd, baseFlag(args), stdout)
	case "commit-gate":
		msgPath := ""
		if len(args) >= 3 {
			msgPath = args[2]
		}
		cmdErr = runCommitGate(cwd, msgPath, stdin, stdout)
	case "list":
		kindFilter := ""
		if len(args) >= 3 {
			kindFilter = args[2]
		}
		cmdErr = runList(cwd, kindFilter, stdout)
	case "config":
		key := ""
		if len(args) >= 3 {
			key = args[2]
		}
		cmdErr = runConfig(cwd, key, stdout)
	case "context":
		spec, _ := clispec.Lookup("context")
		pos := positionals(args[2:], spec.BoolFlags, spec.ValueFlags)
		if len(pos) == 0 {
			staged, rng := hasFlag(args, "--staged"), valueFlag(args, "--range")
			if !staged && rng == "" {
				cmdErr = &usageErr{"usage: awf context <path>... [--json] [--staged] [--range <a>..<b>]"}
				break
			}
			var gerr error
			if pos, gerr = awfgit.ChangedPaths(cwd, staged, rng); gerr != nil {
				cmdErr = gerr
				break
			}
			if len(pos) == 0 {
				cmdErr = &usageErr{"awf context: no changed paths for the given selector"}
				break
			}
		}
		cmdErr = runContext(cwd, pos, hasFlag(args, "--json"), stdout)
	case "new":
		if len(args) < 4 {
			cmdErr = &usageErr{"usage: awf new <kind> <title>"}
		} else {
			cmdErr = runNew(cwd, args[2], args[3:], stdout)
		}
	case "enable":
		spec, _ := clispec.Lookup("enable")
		pos := positionals(args[2:], spec.BoolFlags, spec.ValueFlags)
		switch {
		case len(pos) == 2:
			cmdErr = runEnable(cwd, pos[0], pos[1], hasFlag(args, "--dry-run"), stdout)
		case len(pos) == 1 && (pos[0] == "bootstrap" || pos[0] == "hooks"): // nameless singleton forms (ADR-0040, ADR-0048)
			cmdErr = runEnable(cwd, pos[0], "", hasFlag(args, "--dry-run"), stdout)
		case len(pos) == 1:
			cmdErr = &usageErr{fmt.Sprintf("awf enable requires a kind: awf enable <kind> <name> (e.g. awf enable skill %s)", pos[0])}
		default:
			cmdErr = &usageErr{"usage: awf enable <kind> <name> [--dry-run]"}
		}
	case "disable":
		spec, _ := clispec.Lookup("disable")
		pos := positionals(args[2:], spec.BoolFlags, spec.ValueFlags)
		switch {
		case len(pos) == 2:
			cmdErr = runDisable(cwd, pos[0], pos[1], hasFlag(args, "--with-dependents"), hasFlag(args, "--dry-run"), stdout)
		case len(pos) == 1 && (pos[0] == "bootstrap" || pos[0] == "hooks"): // nameless singleton forms (ADR-0040, ADR-0048)
			cmdErr = runDisable(cwd, pos[0], "", hasFlag(args, "--with-dependents"), hasFlag(args, "--dry-run"), stdout)
		default:
			cmdErr = &usageErr{"usage: awf disable <kind> <name> [--with-dependents] [--dry-run]"}
		}
	case "upgrade":
		cmdErr = runUpgrade(cwd, stdout)
	case "uninstall":
		cmdErr = runUninstall(cwd, stdout)
	case "changelog":
		cmdErr = runChangelog(valueFlag(args, "--version"), valueFlag(args, "--since"), valueFlag(args, "--range"), stdout)
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

// checkArgs rejects unrecognized --flags and enforces the positional count for a
// subcommand against its clispec spec. rest is args[2:]; a valueFlag consumes its
// following token.
func checkArgs(cmd clispec.Command, rest []string) error {
	pos := 0
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case slices.Contains(cmd.ValueFlags, a):
			if i+1 >= len(rest) {
				return &usageErr{fmt.Sprintf("awf %s: flag %s needs a value", cmd.Name, a)}
			}
			i++ // consume the flag's value
		case slices.Contains(cmd.BoolFlags, a):
			// recognized boolean flag
		case strings.HasPrefix(a, "-"):
			return &usageErr{fmt.Sprintf("awf %s: unknown flag %q", cmd.Name, a)}
		default:
			pos++
		}
	}
	if pos < cmd.MinPos || (cmd.MaxPos >= 0 && pos > cmd.MaxPos) {
		return &usageErr{fmt.Sprintf("awf %s: unexpected arguments", cmd.Name)}
	}
	return nil
}

// positionals returns rest's non-flag tokens, skipping each valueFlag's
// consumed value — the flag-tolerant arity source for enable/disable.
func positionals(rest []string, boolFlags, valueFlags []string) []string {
	var out []string
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case slices.Contains(valueFlags, a):
			i++
		case slices.Contains(boolFlags, a):
		case strings.HasPrefix(a, "-"):
		default:
			out = append(out, a)
		}
	}
	return out
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
