// Command awf renders standardised .claude skills, review agents, and docs into a project from embedded templates plus a per-project .awf/ config tree.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
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
// so the overview and the per-command `awf <cmd> --help` texts share one source -
// the internal/clispec table (inv: cli-command-spec-single-source).
func globalHelp() string {
	var b strings.Builder
	b.WriteString("awf: render agentic-workflow tooling into a project from a committed .awf/ config tree\n\n")
	b.WriteString("Usage: awf <command> [flags]\n\n")
	b.WriteString("Commands:\n")
	for _, c := range clispec.Commands {
		fmt.Fprintf(&b, "  %-12s %s\n", c.Name, c.Summary)
	}
	b.WriteString("\nRun `awf <command> --help` for details on a command.\n")
	return b.String()
}

// run is the CLI driver: it resolves args to a clispec command, prints help,
// parses the arguments once, applies the gating classification, and dispatches
// to the command's handler - a single parse-once path shared by every command.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage:", clispec.UsageLine(), "[args]")
		fmt.Fprintln(stderr, "run `awf help` for command details")
		return 2
	}
	if a := args[1]; a == "help" || a == "--help" || a == "-h" {
		if a == "help" && len(args) >= 3 {
			if spec, ok := clispec.Lookup(args[2]); ok {
				// `awf help <group> <child>` prints the child's body; an absent
				// or unknown child falls back to the group's own help.
				if len(args) >= 4 {
					if child, childOK := spec.Child(args[3]); childOK {
						fmt.Fprint(stdout, child.HelpBody)
						return 0
					}
				}
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
	cmd, top, sub, rest, ok := resolve(args[1:])
	if !ok {
		return dispatchErr(stderr, &usageErr{fmt.Sprintf("unknown command %q", args[1])})
	}
	if wantsHelp(rest) { // `awf <cmd> --help`/`-h` - intercept before parseArgs rejects it
		fmt.Fprint(stdout, cmd.HelpBody)
		return 0
	}
	inv, err := parseArgs(cmd, rest)
	if err != nil {
		return dispatchErr(stderr, err) // parseArgs only returns usageErr → exit 2
	}
	// The driver gates every Gated command before its handler; config/context/topic/new
	// self-gate in-handler after their static-fallback / name-validation checks.
	// Gating is read from top (the top-level command), not the resolved child: a
	// group's children never set Gating, so a future Gated group must gate from
	// its top-level node rather than silently inherit a child's Ungated zero value.
	if top.Gating == clispec.Gated {
		if err := gate(cwd); err != nil {
			return dispatchErr(stderr, err)
		}
	}
	// The registry key is the top-level command name even when resolve returned a
	// child spec - the child drives parse/help, the group's handler drives
	// dispatch via sub.
	if err := handlers[top.Name](&cmdCtx{root: cwd, sub: sub, inv: inv, stdout: stdout, stdin: stdin}); err != nil {
		return dispatchErr(stderr, err)
	}
	return 0
}

// dispatchErr prints err and maps it to an exit code: a usageErr (CLI misuse)
// is exit 2, any other failure is exit 1.
func dispatchErr(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, "awf:", err)
	var ue *usageErr
	if errors.As(err, &ue) {
		return 2
	}
	return 1
}

// usageErr marks a CLI-misuse error (unknown flag, bad arity, unknown command),
// which the central handler maps to exit code 2 rather than the failure code 1.
type usageErr struct{ msg string }

func (e *usageErr) Error() string { return e.msg }
