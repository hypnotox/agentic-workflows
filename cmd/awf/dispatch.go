package main

import (
	"context"
	"errors"
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
	// retainLease is set only by the process dispatcher for operations whose
	// outcome diagnostic must remain within the mutation lease.
	retainLease func(func() error)
}

// handlerResult states whether a handler produced a complete failing report.
// A report is a terminal command outcome, not an observation of output bytes.
type handlerResult struct {
	err            error
	producedReport bool
	// release is retained only by operations whose diagnostic presentation must
	// remain within their mutation lease.
	release func() error
}

func handlerFailure(err error) handlerResult { return handlerResult{err: err} }
func handlerFailureHeld(err error, release func() error) handlerResult {
	return handlerResult{err: err, release: release}
}

type leaseRetainingWriter struct {
	io.Writer
	retain func(func() error)
}

func (w leaseRetainingWriter) retainLease(release func() error) { w.retain(release) }
func handlerReport(err error) handlerResult {
	return handlerResult{err: err, producedReport: completeProducedReport(err)}
}

type producedReportError struct{ error }

func (e *producedReportError) Unwrap() error { return e.error }

func completeProducedReport(err error) bool {
	var report *producedReportError
	return errors.As(err, &report)
}

// handler runs one resolved command against its parsed invocation.
type handler func(*cmdCtx) handlerResult

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

// newHandlers composes a runner's top-level handlers. A group command (new)
// has a single handler that dispatches on c.sub; children are NOT separate keys.
// Its process inputs are explicit so no command operation shares a mutable seam.
func newHandlers(promptInput io.Reader, isInteractive func() bool) map[string]handler {
	return map[string]handler{
		"init": func(c *cmdCtx) handlerResult {
			return handlerFailure(runInitWithProjectLoader(c.ctx, c.root, c.inv.bools["--force"], c.inv.bools["--describe"], c.inv.multi["--set"], c.inv.values["--answers"], promptInput, isInteractive(), c.stdout, newProjectLoader, gate))
		},
		"render": func(c *cmdCtx) handlerResult { return handlerFailure(runSync(c.ctx, c.root, c.stdout)) },
		"check": func(c *cmdCtx) handlerResult {
			result := runCheckGroup(c)
			if c.sub == "staged commit" {
				return handlerFailure(result)
			}
			return handlerReport(result)
		},
		"read": func(c *cmdCtx) handlerResult {
			switch c.sub {
			case "plan":
				if err := gate(c.ctx, c.root); err != nil {
					return handlerFailure(err)
				}
				return handlerFailure(runReadPlan(c.ctx, c.root, c.inv.positionals, c.stdout))
			case "topic":
				return handlerFailure(runReadTopic(c.ctx, c.root, firstPos(c.inv.positionals), c.inv.bools["--history"], c.inv.bools["--references"], c.inv.bools["--coverage"], c.stdout))
			case "adr":
				if err := gate(c.ctx, c.root); err != nil {
					return handlerFailure(err)
				}
				return handlerFailure(runReadADR(c.ctx, c.root, firstPos(c.inv.positionals), c.stdout))
			default:
				return handlerFailure(&usageErr{"usage: awf read <plan|topic|adr>"})
			}
		},
		"resolve": func(c *cmdCtx) handlerResult {
			if c.sub != "topic" {
				return handlerFailure(&usageErr{"usage: awf resolve topic <path>..."})
			}
			return handlerFailure(runResolveTopic(c.ctx, c.root, c.inv.positionals, c.inv.bools["--uncovered"], c.stdout))
		},
		"audit": func(c *cmdCtx) handlerResult {
			return handlerReport(runAudit(c.ctx, c.root, firstPos(c.inv.positionals), c.stdout))
		},
		"effort": func(c *cmdCtx) handlerResult {
			var release func() error
			c.retainLease = func(value func() error) { release = value }
			return handlerFailureHeld(runEffort(c, openEffortComposition), release)
		},
		"adr": func(c *cmdCtx) handlerResult { return handlerFailure(runADR(c)) },
		"list": func(c *cmdCtx) handlerResult {
			return handlerFailure(runList(c.ctx, c.root, firstPos(c.inv.positionals), c.stdout))
		},
		"config": func(c *cmdCtx) handlerResult {
			return handlerFailure(runConfig(c.ctx, c.root, firstPos(c.inv.positionals), c.stdout))
		},
		"new": func(c *cmdCtx) handlerResult {
			// For a recognized child, sub is the kind and positionals are the child's
			// args; for an absent or unrecognized child, the typed token (if any) is
			// the first positional. Reunite them so runNew's kind switch owns every
			// usage / unknown-kind message.
			kind, args := c.sub, c.inv.positionals
			if kind == "" && len(args) > 0 {
				kind, args = args[0], args[1:]
			}
			if kind == localDocumentKind {
				var title *string
				if value, present := c.inv.values["--title"]; present {
					title = &value
				}
				return handlerFailure(newDoc(c.ctx, c.root, args, title, c.stdout))
			}
			return handlerFailure(runNew(c.ctx, c.root, kind, args, c.stdout))
		},
		"remove": func(c *cmdCtx) handlerResult {
			if !project.IsFreeformDomainKind(c.sub) || len(c.inv.positionals) != 1 {
				return handlerFailure(&usageErr{"usage: awf remove domain <name>"})
			}
			return handlerFailure(runRemoveDomain(c.ctx, c.root, c.inv.positionals[0], c.stdout))
		},
		"upgrade": func(c *cmdCtx) handlerResult {
			var release func() error
			stdout := leaseRetainingWriter{Writer: c.stdout, retain: func(value func() error) { release = value }}
			return handlerFailureHeld(runUpgradeFlags(c.ctx, c.root, c.inv.bools["--recover"], stdout), release)
		},
		"uninstall": func(c *cmdCtx) handlerResult { return handlerFailure(runUninstall(c.ctx, c.root, c.stdout)) },
		"changelog": func(c *cmdCtx) handlerResult {
			return handlerFailure(runChangelog(c.inv.values["--version"], c.inv.values["--since"], c.inv.values["--range"], c.stdout))
		},
		"version": func(c *cmdCtx) handlerResult { return handlerFailure(runVersion(c.stdout)) },
	}
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

// parseArgs validates rest against cmd's flag and positional spec, builds the
// invocation in one pass, and consumes each value flag with its following
// token.
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
