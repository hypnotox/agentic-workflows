package main

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

// invocation is a command's arguments parsed once. Handlers read this; they never
// re-scan the raw args slice.
type invocation struct {
	positionals []string
	bools       map[string]bool     // every declared BoolFlag → present
	values      map[string]string   // a present value flag → its value; an absent flag is not keyed (handlers read the "" zero value)
	multi       map[string][]string // every declared Repeatable flag → all values
}

// cmdCtx bundles what a handler needs: the invocation's deadlined context, the
// working dir, the parsed args, the resolved subcommand token (for a group
// command; "" otherwise), and the I/O.
type cmdCtx struct {
	// ctx is the invocation's deadlined context, created once at the command
	// boundary in run and read by every handler that reaches git.
	ctx    context.Context
	root   string
	sub    string
	inv    invocation
	stdout io.Writer
	stdin  io.Reader
}

// handler runs one resolved command against its parsed invocation.
type handler func(*cmdCtx) error

// firstPos returns the first positional or "" - the optional-argument shape of
// list, config, and `check staged commit`.
func firstPos(pos []string) string {
	if len(pos) > 0 {
		return pos[0]
	}
	return ""
}

// checkSubcommands lists the immediate children of the group addressed by path.
func checkSubcommands(path string) string {
	spec, _ := clispec.Lookup("check")
	for _, name := range strings.Fields(path) {
		child, ok := spec.Child(name)
		if !ok {
			break
		}
		spec = child
	}
	names := make([]string, len(spec.Children))
	for i, child := range spec.Children {
		names[i] = child.Name
	}
	return strings.Join(names, ", ")
}

func runCheckGroup(c *cmdCtx) error {
	if c.sub == "" {
		if pos := firstPos(c.inv.positionals); pos != "" {
			return &usageErr{fmt.Sprintf("awf check: unknown subcommand %q: expected one of %s", pos, checkSubcommands(""))}
		}
		return runCheck(c.ctx, c.root, c.stdout)
	}
	if (c.sub == "repo" || c.sub == "staged") && len(c.inv.positionals) > 0 {
		return &usageErr{fmt.Sprintf("awf check %s: unknown subcommand %q: expected one of %s", c.sub, c.inv.positionals[0], checkSubcommands(c.sub))}
	}
	switch c.sub {
	case "commit-policy":
		return runCommitPolicy(c.ctx, c.root, c.inv.positionals, c.stdout)
	case "repo":
		return runCheckRepo(c.ctx, c.root, c.stdout)
	case "repo drift":
		return runCheckDrift(c.ctx, c.root, c.stdout)
	case "repo state":
		return runCheckState(c.ctx, c.root, c.stdout)
	case "repo prose":
		return runProseGate(c.ctx, c.root, c.stdout)
	case "repo memory":
		return runMemoryGate(c.ctx, c.root, c.stdout)
	case "staged":
		return runCheckStaged(c.ctx, c.root, c.stdout)
	case "staged state":
		return runCheckStagedState(c.ctx, c.root, c.stdout)
	case "staged drift":
		return runCheckStagedDrift(c.ctx, c.root, c.stdout)
	case "staged commit":
		return runCommitGate(c.ctx, c.root, firstPos(c.inv.positionals), c.stdin, c.stdout)
	default:
		return &usageErr{fmt.Sprintf("awf check: unknown subcommand %q: expected one of %s", c.sub, checkSubcommands(""))}
	}
}

// handlers maps a top-level command name to its handler. A group command (new)
// has a single handler that dispatches on c.sub; children are NOT separate keys.
// TestHandlerRegistryParity asserts these keys match the clispec top-level names.
var handlers = map[string]handler{
	"init": func(c *cmdCtx) error {
		return runInit(c.ctx, c.root, c.inv.bools["--force"], c.inv.bools["--describe"], c.inv.multi["--set"], c.inv.values["--answers"], c.stdout)
	},
	"render": func(c *cmdCtx) error { return runSync(c.ctx, c.root, c.stdout) },
	"check":  runCheckGroup,
	"read": func(c *cmdCtx) error {
		if c.sub != "plan" { // coverage-ignore: clispec admits only the declared plan child before dispatch
			return &usageErr{"usage: awf read plan <plan> <P[.T]>"}
		}
		return runReadPlan(c.ctx, c.root, c.inv.positionals, c.stdout)
	},
	"audit":  func(c *cmdCtx) error { return runAudit(c.ctx, c.root, firstPos(c.inv.positionals), c.stdout) },
	"effort": func(c *cmdCtx) error { return runEffort(c, openEffortComposition) },
	"adr":    runADR,
	"list":   func(c *cmdCtx) error { return runList(c.ctx, c.root, firstPos(c.inv.positionals), c.stdout) },
	"config": func(c *cmdCtx) error { return runConfig(c.ctx, c.root, firstPos(c.inv.positionals), c.stdout) },
	"context": func(c *cmdCtx) error {
		return runContext(c.ctx, c.root, c.inv.positionals, c.inv.bools["--staged"], c.inv.values["--range"], c.inv.bools["--uncovered"], c.inv.bools["--full"], c.inv.multi["--show"], c.stdout)
	},
	"topic": func(c *cmdCtx) error {
		return runTopic(c.ctx, c.root, firstPos(c.inv.positionals), c.inv.bools["--history"], c.inv.bools["--references"], c.inv.bools["--coverage"], c.stdout)
	},
	"new": func(c *cmdCtx) error {
		// For a recognized child, sub is the kind and positionals are the child's
		// args; for an absent or unrecognized child, the typed token (if any) is
		// the first positional. Reunite them so runNew's kind switch owns every
		// usage / unknown-kind message.
		kind, args := c.sub, c.inv.positionals
		if kind == "" && len(args) > 0 {
			kind, args = args[0], args[1:]
		}
		return runNew(c.ctx, c.root, kind, args, c.stdout)
	},
	"enable": func(c *cmdCtx) error {
		kind, name, err := enableDisableArgs(c.inv.positionals, true)
		if err != nil {
			return err
		}
		return runEnable(c.ctx, c.root, kind, name, c.inv.bools["--dry-run"], c.stdout)
	},
	"disable": func(c *cmdCtx) error {
		kind, name, err := enableDisableArgs(c.inv.positionals, false)
		if err != nil {
			return err
		}
		return runDisable(c.ctx, c.root, kind, name, c.inv.bools["--with-dependents"], c.inv.bools["--dry-run"], c.stdout)
	},
	"upgrade": func(c *cmdCtx) error {
		return runUpgradeFlags(c.ctx, c.root, c.inv.bools["--recover"], c.stdout)
	},
	"uninstall": func(c *cmdCtx) error { return runUninstall(c.ctx, c.root, c.stdout) },
	"changelog": func(c *cmdCtx) error {
		return runChangelog(c.inv.values["--version"], c.inv.values["--since"], c.inv.values["--range"], c.stdout)
	},
	"version": func(c *cmdCtx) error { runVersion(c.stdout); return nil },
}

// enableDisableArgs resolves the shared positional forms of enable/disable -
// `<kind> <name>` or a nameless singleton (bootstrap/hooks) - into a kind and
// name, or a usage error. isEnable selects the verb, the enable-only "requires a
// kind" hint, and the per-command usage line. A singleton handed a name, and a
// lone kind token missing its name, are distinct usage errors - not silently
// dropped input (the singleton) or a misattributed "requires a kind" hint (the
// kind token).
func enableDisableArgs(pos []string, isEnable bool) (kind, name string, err error) {
	verb, usage := "disable", "usage: awf disable <kind> <name> [--with-dependents] [--dry-run]"
	if isEnable {
		verb, usage = "enable", "usage: awf enable <kind> <name> [--dry-run]"
	}
	isSingleton := len(pos) >= 1 && (pos[0] == "bootstrap" || pos[0] == "hooks" || pos[0] == "runner") // nameless singleton forms (ADR-0040, ADR-0048, ADR-0101)
	switch {
	case len(pos) == 1 && isSingleton:
		return pos[0], "", nil
	case len(pos) == 2 && isSingleton:
		return "", "", &usageErr{fmt.Sprintf("awf %s %s takes no name: it is a singleton toggle", verb, pos[0])}
	case len(pos) == 2:
		return pos[0], pos[1], nil
	case len(pos) == 1 && isKindToken(pos[0]):
		return "", "", &usageErr{fmt.Sprintf("awf %s %s requires a name: awf %s %s <name>", verb, pos[0], verb, pos[0])}
	case len(pos) == 1 && isEnable:
		return "", "", &usageErr{fmt.Sprintf("awf enable requires a kind: awf enable <kind> <name> (e.g. awf enable skill %s)", pos[0])}
	default:
		return "", "", &usageErr{usage}
	}
}

// isKindToken reports whether s names a CLI kind that takes a <name> - a
// descriptor kind (skill/agent/doc/domain, via the one kind table that
// kind-dispatch-single-table guards) or the target adapter (which has no
// descriptor). It lets enableDisableArgs tell "forgot the name" from "forgot the
// kind". bootstrap/hooks/runner are excluded - they are nameless singletons
// handled before this check.
func isKindToken(s string) bool {
	if s == "target" {
		return true
	}
	_, ok := project.PluralKind(s)
	return ok
}

// resolve descends through named children, returning the deepest leaf and its
// joined child path. Unknown child tokens remain positional arguments for the
// addressed group's handler to diagnose.
func resolve(args []string) (cmd, top clispec.Command, sub string, rest []string, ok bool) {
	top, ok = clispec.Lookup(args[0])
	if !ok {
		return clispec.Command{}, clispec.Command{}, "", nil, false
	}
	cmd = top
	i := 1
	var path []string
	for i < len(args) {
		child, found := cmd.Child(args[i])
		if !found {
			break
		}
		cmd = child
		path = append(path, args[i])
		i++
	}
	return cmd, top, strings.Join(path, " "), args[i:], true
}

// wantsHelp reports whether a --help or -h token appears among a command's args,
// so the driver can print help before parseArgs would reject the flag.
func wantsHelp(rest []string) bool {
	return slices.Contains(rest, "--help") || slices.Contains(rest, "-h")
}

// parseArgs validates rest against cmd's flag/positional spec and builds the
// invocation in one pass (folding the former checkArgs/positionals/valueFlag/
// setFlags/hasFlag/baseFlag scans). A value flag consumes its following token.
func parseArgs(cmd clispec.Command, rest []string) (invocation, error) {
	inv := invocation{bools: map[string]bool{}, values: map[string]string{}, multi: map[string][]string{}}
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case slices.Contains(cmd.ValueFlags, a):
			if i+1 >= len(rest) {
				return invocation{}, &usageErr{fmt.Sprintf("awf %s: flag %s needs a value", cmd.Name, a)}
			}
			i++
			if slices.Contains(cmd.Repeatable, a) {
				inv.multi[a] = append(inv.multi[a], rest[i])
			} else if _, dup := inv.values[a]; dup {
				return invocation{}, &usageErr{fmt.Sprintf("awf %s: flag %s given more than once", cmd.Name, a)}
			} else {
				inv.values[a] = rest[i]
			}
		case slices.Contains(cmd.BoolFlags, a):
			if inv.bools[a] {
				return invocation{}, &usageErr{fmt.Sprintf("awf %s: flag %s given more than once", cmd.Name, a)}
			}
			inv.bools[a] = true
		case strings.HasPrefix(a, "-"):
			return invocation{}, &usageErr{fmt.Sprintf("awf %s: unknown flag %q", cmd.Name, a)}
		default:
			inv.positionals = append(inv.positionals, a)
		}
	}
	if len(inv.positionals) < cmd.MinPos || (cmd.MaxPos >= 0 && len(inv.positionals) > cmd.MaxPos) {
		return invocation{}, &usageErr{fmt.Sprintf("awf %s: unexpected arguments", cmd.Name)}
	}
	return inv, nil
}
