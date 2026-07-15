package main

import (
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

// cmdCtx bundles what a handler needs: the working dir, the parsed args, the
// resolved subcommand token (for a group command; "" otherwise), and the I/O.
type cmdCtx struct {
	root   string
	sub    string
	inv    invocation
	stdout io.Writer
	stdin  io.Reader
}

// handler runs one resolved command against its parsed invocation.
type handler func(*cmdCtx) error

// firstPos returns the first positional or "" — the optional-argument shape of
// list, config, and commit-gate.
func firstPos(pos []string) string {
	if len(pos) > 0 {
		return pos[0]
	}
	return ""
}

// handlers maps a top-level command name to its handler. A group command (new)
// has a single handler that dispatches on c.sub; children are NOT separate keys.
// TestHandlerRegistryParity asserts these keys match the clispec top-level names.
var handlers = map[string]handler{
	"init": func(c *cmdCtx) error {
		return runInit(c.root, c.inv.bools["--force"], c.inv.bools["--describe"], c.inv.multi["--set"], c.inv.values["--answers"], c.stdout)
	},
	"sync":        func(c *cmdCtx) error { return runSync(c.root, c.stdout) },
	"check":       func(c *cmdCtx) error { return runCheck(c.root, c.stdout) },
	"invariants":  func(c *cmdCtx) error { return runInvariants(c.root, c.stdout) },
	"audit":       func(c *cmdCtx) error { return runAudit(c.root, c.inv.values["--base"], c.stdout) },
	"commit-gate": func(c *cmdCtx) error { return runCommitGate(c.root, firstPos(c.inv.positionals), c.stdin, c.stdout) },
	"list":        func(c *cmdCtx) error { return runList(c.root, firstPos(c.inv.positionals), c.stdout) },
	"config":      func(c *cmdCtx) error { return runConfig(c.root, firstPos(c.inv.positionals), c.stdout) },
	"context": func(c *cmdCtx) error {
		return runContext(c.root, c.inv.positionals, c.inv.bools["--staged"], c.inv.values["--range"], c.inv.bools["--json"], c.inv.bools["--uncovered"], c.stdout)
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
		return runNew(c.root, kind, args, c.stdout)
	},
	"enable": func(c *cmdCtx) error {
		kind, name, err := enableDisableArgs(c.inv.positionals, true)
		if err != nil {
			return err
		}
		return runEnable(c.root, kind, name, c.inv.bools["--dry-run"], c.stdout)
	},
	"disable": func(c *cmdCtx) error {
		kind, name, err := enableDisableArgs(c.inv.positionals, false)
		if err != nil {
			return err
		}
		return runDisable(c.root, kind, name, c.inv.bools["--with-dependents"], c.inv.bools["--dry-run"], c.stdout)
	},
	"upgrade":   func(c *cmdCtx) error { return runUpgrade(c.root, c.stdout) },
	"uninstall": func(c *cmdCtx) error { return runUninstall(c.root, c.stdout) },
	"changelog": func(c *cmdCtx) error {
		return runChangelog(c.inv.values["--version"], c.inv.values["--since"], c.inv.values["--range"], c.stdout)
	},
	"version": func(c *cmdCtx) error { runVersion(c.stdout); return nil },
}

// enableDisableArgs resolves the shared positional forms of enable/disable —
// `<kind> <name>` or a nameless singleton (bootstrap/hooks) — into a kind and
// name, or a usage error. isEnable selects the verb, the enable-only "requires a
// kind" hint, and the per-command usage line. A singleton handed a name, and a
// lone kind token missing its name, are distinct usage errors — not silently
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
		return "", "", &usageErr{fmt.Sprintf("awf %s %s takes no name — it is a singleton toggle", verb, pos[0])}
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

// isKindToken reports whether s names a CLI kind that takes a <name> — a
// descriptor kind (skill/agent/doc/domain, via the one kind table that
// kind-dispatch-single-table guards) or the target adapter (which has no
// descriptor). It lets enableDisableArgs tell "forgot the name" from "forgot the
// kind". bootstrap/hooks/runner are excluded — they are nameless singletons
// handled before this check.
func isKindToken(s string) bool {
	if s == "target" {
		return true
	}
	_, ok := project.PluralKind(s)
	return ok
}

// resolve looks up args[0] as a top-level command. For a group command (new)
// whose next token names a child, it returns the child as cmd (so parseArgs
// validates against the child's flag spec and --help prints the child help),
// with sub set to the child token and rest the tokens after it. A leaf, or a
// group with no or an unknown child, returns itself as cmd with sub "" and rest
// args[1:] — the group's handler then owns the missing/unknown-child messages.
// top is always the top-level command (== cmd for a leaf, the group for a
// resolved child); the driver reads gating and the handler key from top, since
// both are top-level properties a child never overrides.
func resolve(args []string) (cmd, top clispec.Command, sub string, rest []string, ok bool) {
	top, found := clispec.Lookup(args[0])
	if !found {
		return clispec.Command{}, clispec.Command{}, "", nil, false
	}
	if len(top.Children) > 0 && len(args) > 1 {
		if child, childOK := top.Child(args[1]); childOK {
			return child, top, args[1], args[2:], true
		}
	}
	return top, top, "", args[1:], true
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
