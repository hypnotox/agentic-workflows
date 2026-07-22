// Package clispec is the single declarative source of awf's CLI command set:
// every command's flags, positional bounds, gating, help text, and (for a group
// command) its subcommands. cmd/awf builds its runtime dispatcher by attaching
// handler funcs to these specs; internal/project reads the gated set to generate
// docs. Data only - no handler funcs and no import of cmd/awf or internal/project,
// so it stays an importable leaf.
package clispec

import (
	"fmt"
	"strings"
)

// Gating classifies when a command runs the binary-version gate (ADR-0094 Decision 3).
type Gating int

const (
	Ungated        Gating = iota // never gates (version, changelog, upgrade, uninstall, commit-gate, init)
	Gated                        // the driver gates before the handler
	GatedInHandler               // the handler gates itself (config/context/topic after their static-fallback check; new after name validation)
)

// RunnerDisposition declares whether a top-level awf command is forwarded by
// the managed project runner. Excluded commands carry the reason users must
// invoke them directly; children inherit their top-level command's disposition.
type RunnerDisposition struct {
	Forward bool
	Reason  string
}

// Command is one CLI command (or subcommand). A command with Children is a group:
// the driver dispatches on the next positional to a child; a leaf carries no
// Children and is run by its attached handler. MaxPos < 0 means unbounded.
type Command struct {
	Name       string
	Summary    string // one-line, for `awf help`
	HelpBody   string // full `awf <cmd> --help` text
	BoolFlags  []string
	ValueFlags []string // includes repeatables
	Repeatable []string // subset of ValueFlags collected into invocation.Multi
	MinPos     int
	MaxPos     int
	Gating     Gating
	Runner     RunnerDisposition
	Children   []Command
}

// Commands is the ordered command table - the sole source of the command set,
// `awf help` order, the usage line, and the gated-command list.
// touches-state: tooling/cli:cli-command-spec-single-source - sole command-table source; proof in clispec_test.go
var Commands = []Command{
	{
		Name: "init", Summary: "Scaffold .awf/ and render the workflow-core set", Runner: RunnerDisposition{Reason: "requires a pre-adoption invocation"},
		BoolFlags: []string{"--force", "--describe"}, ValueFlags: []string{"--set", "--answers"},
		Repeatable: []string{"--set"}, MaxPos: 0, Gating: Ungated,
		HelpBody: `Usage: awf init [flags]

Scaffold a .awf/ config tree and render the workflow-core set into the project.

Flags:
  --force        overwrite colliding files, backing each up to <path>.awf-bak
  --describe     print the fillable value descriptors as JSON and exit
  --set k=v      set a value non-interactively (repeatable)
  --answers FILE read values from a JSON/YAML answers file: a flat key→value map
                 of descriptor keys (see --describe); multiselect answers
                 (skills, docs) are comma-joined name lists
`,
	},
	{
		Name: "sync", Summary: "Re-render after a template or config change", Runner: RunnerDisposition{Forward: true},
		MaxPos: 0, Gating: Gated,
		HelpBody: `Usage: awf sync

Re-render every enabled target after a template or config change and update .awf/awf.lock.
`,
	},
	{
		Name: "check", Summary: "Fail on stale or hand-edited rendered output", Runner: RunnerDisposition{Forward: true},
		BoolFlags: []string{"--staged"}, MaxPos: 0, Gating: Gated,
		HelpBody: `Usage: awf check [--staged]

Re-render in memory and fail if any rendered file is stale or hand-edited (drift),
then check current-state authority over the working tree.

With --staged, skip the drift check and instead validate the staged transition:
the HEAD-to-index ADR status changes and claim add/update/remove mutations must
correspond, and the index is checked for topic coverage. It reads only committed
and staged content, never the working tree, so a pre-commit hook can invoke it.
`,
	},
	{
		Name: "invariants", Summary: "Report Implemented-ADR invariant slugs lacking a backing comment", Runner: RunnerDisposition{Forward: true},
		MaxPos: 0, Gating: Gated,
		HelpBody: `Usage: awf invariants

Report each Implemented-ADR ` + "`inv:`" + ` slug lacking a backing ` + "`<marker> invariant:`" + ` comment.
`,
	},
	{
		Name: "audit", Summary: "Report workflow-conformance findings over a commit range (advisory)", Runner: RunnerDisposition{Forward: true},
		MaxPos: 1, Gating: Gated,
		HelpBody: `Usage: awf audit <base>|<a>..<b>

Report advisory workflow-conformance findings over an explicit commit range; never gates.
The range is required: a bare <base> means <base>..HEAD, or give a two-sided <a>..<b>.
There is no default range, so an audit never reports over commits nobody named.
`,
	},
	{
		Name: "metrics", Summary: "Record and maintain workflow telemetry", Runner: RunnerDisposition{Forward: true},
		MaxPos: 0, Gating: Gated,
		HelpBody: `Usage: awf metrics <protocol|lifecycle|retain|purge>

Record explicit lifecycle mutations, inspect the protocol handshake, and run
confirmed resident-data maintenance. Query and export selectors are not available yet.
`,
		Children: []Command{
			{Name: "protocol", Summary: "Print the telemetry protocol handshake", BoolFlags: []string{"--json"}, MaxPos: 0,
				HelpBody: `Usage: awf metrics protocol --json

Print the machine-readable telemetry protocol handshake. --json is required.
`},
			{Name: "lifecycle", Summary: "Durably record one lifecycle request", BoolFlags: []string{"--json"}, ValueFlags: []string{"--request"}, MaxPos: 0,
				HelpBody: `Usage: awf metrics lifecycle --request <FILE|-> [--json]

Validate and durably record exactly one lifecycle request from a file or stdin.
`},
			{Name: "retain", Summary: "Apply configured telemetry retention", BoolFlags: []string{"--dry-run", "--json"}, MaxPos: 0,
				HelpBody: `Usage: awf metrics retain [--dry-run] [--json]

Recover interrupted maintenance and apply configured deterministic retention.
`},
			{Name: "purge", Summary: "Explicitly purge one terminal effort", BoolFlags: []string{"--confirm", "--json"}, ValueFlags: []string{"--effort"}, MaxPos: 0,
				HelpBody: `Usage: awf metrics purge --effort <ID> --confirm [--json]

Recursively purge one named terminal effort only after explicit confirmation.
`},
		},
	},
	{
		Name: "commit-gate", Summary: "Validate one commit message (Conventional Commits), blocking", Runner: RunnerDisposition{Forward: true},
		MaxPos: 1, Gating: Ungated,
		HelpBody: `Usage: awf commit-gate [FILE]

Validate one commit message against the Conventional Commits rules (type, scope,
72-char subject) and exit non-zero on a violation: the commit-side analog of the
gate. Reads FILE (the path a commit-msg hook passes as $1) or stdin; cleans the
message git-style and exempts merge/autosquash subjects. awf installs no hook;
wire this into your own commit-msg hook (the rendered .awf/hooks/commit-msg.sh
payload runs it when the hooks artifact is enabled).
`,
	},
	{
		Name: "prose-gate", Summary: "Scan tracked text files for typographic punctuation, blocking", Runner: RunnerDisposition{Forward: true},
		Gating: Ungated,
		HelpBody: `Usage: awf prose-gate

Report every typographic punctuation substitute in the project's tracked text
files and exit non-zero on any finding: the presence-level analog of the audit
rule, which only warns when a commit adds one. Exits zero without scanning
unless proseGate.enabled is true, so a hook or a runner may invoke it
unconditionally. Permit a character that is genuinely being written about with
proseGate.exemptions. awf installs no hook; wire this into your own pre-commit
hook (the rendered .awf/hooks/pre-commit.sh payload runs it when the hooks
artifact is enabled).
`,
	},
	{
		Name: "list", Summary: "Show targets and their per-project state (all kinds, or one)", Runner: RunnerDisposition{Forward: true},
		MaxPos: 1, Gating: Gated,
		HelpBody: `Usage: awf list [<kind>]

Show targets and their per-project enabled state, for all kinds or one (skill|agent|doc|domain|target|bootstrap|hooks).
`,
	},
	{
		Name: "config", Summary: "Describe config keys and vars (live state inside a project)", Runner: RunnerDisposition{Forward: true},
		MaxPos: 1, Gating: GatedInHandler,
		HelpBody: `Usage: awf config [<key-or-var>]

Print the configuration reference: every config key, var, sidecar field, and
data key with descriptions, defaults, and availability. Inside an awf project
the output adds live state (current values; which enabled artifacts consume
each var; dormant hints). Outside one, a static catalog-wide reference prints.
With an argument, print just that entry (a config key path like
audit.diffThreshold, a var name like gateCmd, a sidecar field like
sidecar.local, or a data key name).
`,
	},
	{
		Name: "context", Summary: "Orient by path with concise or full current-state authority", Runner: RunnerDisposition{Forward: true},
		BoolFlags: []string{"--json", "--staged", "--uncovered", "--full"}, ValueFlags: []string{"--range"}, MaxPos: -1, Gating: GatedInHandler,
		HelpBody: `Usage: awf context <path>... [--json] [--full] [--staged] [--range <a>..<b>] [--uncovered]

Report concise current-state orientation for normalized request paths and their
classified effective paths. Direct state, touches, and proof markers select the
default claims; omitted topic-wide claims carry drilldowns. Use --full for every
applicable current claim, backing and direct reference IDs, without truncation.
Explicit ADR paths report lifecycle operation progress without treating ADR prose
as current authority. The command is read-only. Outside a project, both projections
print the same static pre-adoption reference.

Provide paths explicitly, or resolve them from git with --staged (the staged
changes) or --range <a>..<b> (the diff between two revisions). Explicit paths
take precedence over the git selectors.

With --uncovered, report eligible paths that are unowned or owned by a domain
but covered by no scoped topic. Positional args become optional scan roots.
Combine with --staged to read the index universe; --range is not accepted.

Flags:
  --json               emit the selected projection as JSON
  --full               emit the complete applicable authority packet
  --staged             use the staged index universe
  --range <a>..<b>     use the paths changed between revisions a and b
  --uncovered          report unowned and uncovered paths (cannot use --full)
`,
	},
	{
		Name: "topic", Summary: "Query current claims, history, references, and applicability", Runner: RunnerDisposition{Forward: true},
		BoolFlags: []string{"--history", "--references", "--coverage", "--json"}, MinPos: 1, MaxPos: 1, Gating: GatedInHandler,
		HelpBody: `Usage: awf topic <domain>/<topic>[:<claim>] [flags]

Query one current-state topic or claim, active by default. Default output includes
title and summary for a topic plus claim types, prose, and backing state. Detail
flags are independent and direct-only. A removed claim identity resolves only
with --history and returns operation history without an active tombstone. Outside
an awf project, a static command reference prints without version gating.

Flags:
  --history       add direct Origin, Revised-by, and Removed-by ADR details
  --references    add sorted direct incoming and outgoing claim IDs
  --coverage      add separate domain/topic scopes, current matches, and marker sites
  --json          emit the same query result as deterministic JSON
`,
	},
	{
		Name: "new", Summary: "Scaffold a new artifact: kind ∈ {adr, plan, topic, skill, agent, doc}", Runner: RunnerDisposition{Forward: true},
		MaxPos: -1, Gating: GatedInHandler,
		HelpBody: `Usage: awf new <kind> <args>

Scaffold a new artifact. <kind> is adr, plan, topic, skill, agent, or doc.

- awf new adr "Some Decision Title"
- awf new plan "Some Plan Title"
- awf new topic <domain> "Some Topic Title"
- awf new skill <name> "<description>"   (a project-local skill)
- awf new agent <name> "<description>"   (a project-local agent)
- awf new doc <name> "<description>"     (a project-local doc; name may be nested, e.g. guides/ci)
`,
		Children: []Command{
			{
				Name: "adr", Summary: "Scaffold a new ADR", MinPos: 0, MaxPos: -1,
				HelpBody: `Usage: awf new adr <title>

Scaffold a new ADR under docs/decisions with the next sequential number, from
the rendered template with its date and title heading filled in.
`,
			},
			{
				Name: "plan", Summary: "Scaffold a new plan", MinPos: 0, MaxPos: -1,
				HelpBody: `Usage: awf new plan <title>

Scaffold a new plan under docs/plans, date-prefixed (no sequential number),
from the rendered plans template with its date and title heading filled in.
`,
			},
			{
				Name: "topic", Summary: "Scaffold paired current-state topic inputs", MinPos: 2, MaxPos: -1,
				HelpBody: `Usage: awf new topic <domain> <title>

Scaffold paired topic metadata and authored current-state inputs without syncing.
Edit the path placeholder and author reviewed claims manually.
`,
			},
			{
				Name: "skill", Summary: "Scaffold a project-local skill", MinPos: 0, MaxPos: -1,
				HelpBody: `Usage: awf new skill <name> "<description>"

Scaffold a project-local skill: a declaring sidecar carrying the description, a
starter content part, the enable, and a re-render.
`,
			},
			{
				Name: "agent", Summary: "Scaffold a project-local agent", MinPos: 0, MaxPos: -1,
				HelpBody: `Usage: awf new agent <name> "<description>"

Scaffold a project-local agent: a declaring sidecar carrying the description, a
starter content part, the enable, and a re-render.
`,
			},
			{
				Name: "doc", Summary: "Scaffold a project-local doc", MinPos: 0, MaxPos: -1,
				HelpBody: `Usage: awf new doc <name> "<description>"

Scaffold a project-local doc; the name may be nested, e.g. guides/ci. Writes a
declaring sidecar with a derived title and the description, a starter content
part, the enable, and a re-render.
`,
			},
		},
	},
	{
		Name: "enable", Summary: "Enable an artifact: kind ∈ {skill, agent, doc, domain, target, bootstrap, hooks, runner}", Runner: RunnerDisposition{Forward: true},
		BoolFlags: []string{"--dry-run"}, MaxPos: -1, Gating: Gated,
		HelpBody: `Usage: awf enable <kind> <name> [--dry-run]

Enable an artifact in this project. <kind> is skill, agent, doc, domain, target,
bootstrap, hooks, or runner. For skill/agent/doc, the full requirement closure is enabled
in one edit, printed as a plan (ADR-0081).

Flags:
  --dry-run    print the closure plan without changing the config
`,
	},
	{
		Name: "disable", Summary: "Disable an artifact: kind ∈ {skill, agent, doc, domain, target, bootstrap, hooks, runner}", Runner: RunnerDisposition{Forward: true},
		BoolFlags: []string{"--with-dependents", "--dry-run"}, MaxPos: -1, Gating: Gated,
		HelpBody: `Usage: awf disable <kind> <name> [--with-dependents] [--dry-run]

Disable an artifact: a catalog skill/agent/doc, a freeform domain, an adapter target, the bootstrap, the hooks, or the runner.
For skill/agent/doc, disabling refuses while enabled artifacts still require
<name>, printing the dependent plan (ADR-0081).

Flags:
  --with-dependents    also disable every enabled artifact that transitively requires <name>
  --dry-run            print the plan without changing the config
`,
	},
	{
		Name: "upgrade", Summary: "Migrate the .awf/ config tree or consume a current-state attestation", Runner: RunnerDisposition{Reason: "must cross the pinned bootstrap boundary"},
		BoolFlags: []string{"--recover"}, MaxPos: 0, Gating: Ungated,
		HelpBody: `Usage: awf upgrade [--recover]

Migrate the .awf/ config tree to the current schema version, then sync.

When the lock carries a bridge attestation, plain upgrade instead performs the
final current-state cutover: it verifies only the sealed facts (the prepared
HEAD and tree digest), then journals the deletion of the migration approval file
and the permanent lock, promoting the sealed format cutoff and gaps. Attestation
and readiness reporting live only in the preceding bridge release; this binary
consumes seals, it never produces them.

  --recover              replay the current-state upgrade journal's recovery
                         table: roll an interrupted cutover back or clean up a
                         committed one. The only mode a journal permits.
`,
	},
	{
		Name: "uninstall", Summary: "Remove awf's generated files (keeps .awf/)", Runner: RunnerDisposition{Reason: "runner-mediated self-removal is unsafe"},
		MaxPos: 0, Gating: Ungated,
		HelpBody: `Usage: awf uninstall

Remove every awf-generated file recorded in the lock (keeps your authored .awf/ config).
`,
	},
	{
		Name: "changelog", Summary: "Print the embedded changelog, or one version/range of it", Runner: RunnerDisposition{Forward: true},
		ValueFlags: []string{"--version", "--since", "--range"}, MaxPos: 0, Gating: Ungated,
		HelpBody: `Usage: awf changelog [--version <v> | --since <v> | --range <from>..<to>]

Print the embedded awf changelog. With no flags, print the whole file. The three
flags are mutually exclusive.

Flags:
  --version <v>          print only version v's entry
  --since <v>            print every version released after v (exclusive)
  --range <from>..<to>   print every version in [from, to] (inclusive both ends)
`,
	},
	{
		Name: "version", Summary: "Print the awf version", Runner: RunnerDisposition{Forward: true},
		MaxPos: 0, Gating: Ungated,
		HelpBody: `Usage: awf version

Print the awf version.
`,
	},
}

// Lookup returns the top-level command named name.
func Lookup(name string) (Command, bool) {
	for _, c := range Commands {
		if c.Name == name {
			return c, true
		}
	}
	return Command{}, false
}

// Child returns c's subcommand named name (for a group command like new).
func (c Command) Child(name string) (Command, bool) {
	for _, ch := range c.Children {
		if ch.Name == name {
			return ch, true
		}
	}
	return Command{}, false
}

// Names returns every top-level command name in table order.
func Names() []string {
	out := make([]string, len(Commands))
	for i, c := range Commands {
		out[i] = c.Name
	}
	return out
}

// ValidateRunnerDispositions rejects an unclassified or contradictory runner
// disposition and any child-level declaration.
func ValidateRunnerDispositions() error { return validateRunnerDispositions(Commands) }

func validateRunnerDispositions(commands []Command) error {
	for _, c := range commands {
		if c.Runner.Forward == (c.Runner.Reason != "") {
			return fmt.Errorf("command %q must be forwarded without a reason or excluded with a reason", c.Name)
		}
		for _, child := range c.Children {
			if child.Runner.Forward || child.Runner.Reason != "" {
				return fmt.Errorf("command %q child %q declares runner disposition; children inherit their top-level command", c.Name, child.Name)
			}
		}
	}
	return nil
}

// Forwarded returns the top-level commands the managed runner forwards, in CLI
// table order.
func Forwarded() []Command {
	var out []Command
	for _, c := range Commands {
		if c.Runner.Forward {
			out = append(out, c)
		}
	}
	return out
}

// ForwardedNames returns the managed-runner command names in CLI table order.
func ForwardedNames() []string {
	commands := Forwarded()
	out := make([]string, len(commands))
	for i, c := range commands {
		out[i] = c.Name
	}
	return out
}

// UsageLine renders the `awf <a|b|...>` usage token list from the table.
func UsageLine() string { return "awf <" + strings.Join(Names(), "|") + ">" }

// GatedCommandNames returns, in table order, every top-level command that runs
// the binary-version gate - the driver-gated commands plus the ones that gate
// in-handler (config/context/topic after their static fallback, new after name
// validation). Ungated commands are excluded; a group contributes only its own
// token. This is the single source of the doc-published gated-command list.
func GatedCommandNames() []string {
	var out []string
	for _, c := range Commands {
		if c.Gating != Ungated {
			out = append(out, c.Name)
		}
	}
	return out
}
