// Package clispec is the single declarative source of awf's CLI command set:
// every command's flags, positional bounds, gating, help text, and (for a group
// command) its subcommands. cmd/awf builds its runtime dispatcher by attaching
// handler funcs to these specs; internal/project reads the gated set to generate
// docs. Data only - no handler funcs and no import of cmd/awf or internal/project,
// so it stays an importable leaf.
package clispec

import (
	"strings"
)

// Gating classifies when a command runs the binary-version gate (ADR-0094 Decision 3).
type Gating int

const (
	Inherit        Gating = iota // a group child that declares nothing: inherit from the top-level group
	Ungated                      // never gates (version, changelog, upgrade, uninstall, and init)
	Gated                        // the driver gates before the handler
	GatedInHandler               // the handler gates itself (config/context/topic after their static-fallback check; new after name validation)
)

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
	// StateExempt bypasses the current-state journal/attestation guard
	// (ADR-0159 Decision 5). It is read from the resolved command, so a group
	// child carries it independently of its parent.
	StateExempt bool
	Children    []Command
}

// Commands is the ordered command table - the sole source of the command set,
// `awf help` order, the usage line, and the gated-command list.
// touches-state: tooling/cli:cli-command-spec-single-source - sole command-table source; proof in clispec_test.go
var Commands = []Command{
	{
		Name: "init", Summary: "Scaffold .awf/ and render the workflow-core set",
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
		Name: "render", Summary: "Re-render after a template or config change",
		MaxPos: 0, Gating: Gated,
		HelpBody: `Usage: awf render

Re-render every enabled target after a template or config change and update .awf/awf.lock.
`,
	},
	{
		Name: "check", Summary: "Verify the repository and staged universes",
		MaxPos: -1, Gating: Gated,
		HelpBody: `Usage: awf check
       awf check repo [subcommand]
       awf check staged [subcommand]

Bare check runs both universes. The repo universe checks drift, current state,
and the opt-in scans; the staged universe validates the HEAD-to-index transition.
Outside a Git repository bare check runs the repo universe and reports that the
staged universe is unavailable.
`,
		Children: []Command{
			{Name: "repo", Summary: "Verify repository properties", MaxPos: -1,
				HelpBody: `Usage: awf check repo [subcommand]

Run the repository universe: drift, current-state, prose, and memory checks.
`,
				Children: []Command{
					{Name: "drift", Summary: "Report stale or hand-edited rendered output", MaxPos: 0,
						HelpBody: "Usage: awf check repo drift\n\nRe-render in memory and report stale or hand-edited output.\n"},
					{Name: "state", Summary: "Report current-state authority findings", MaxPos: 0,
						HelpBody: "Usage: awf check repo state\n\nCheck current-state authority over the working tree.\n"},
					{Name: "prose", Summary: "Scan tracked text files for typographic punctuation, blocking", MaxPos: 0,
						HelpBody: "Usage: awf check repo prose\n\nScan the project's tracked text files when proseGate.enabled is true.\n"},
					{Name: "memory", Summary: "Scan staged decision records for working-memory citations, blocking", MaxPos: 0,
						HelpBody: "Usage: awf check repo memory\n\nScan tracked decision records when memoryCite.enabled is true.\n"},
				},
			},
			{Name: "staged", Summary: "Verify staged transition properties", MaxPos: -1,
				HelpBody: `Usage: awf check staged [subcommand]

Run the staged transition and rendered-output drift checks. The commit child is
directly invoked by a commit-msg hook and is not part of the aggregate.
`,
				Children: []Command{
					{Name: "state", Summary: "Report staged current-state transition findings", MaxPos: 0,
						HelpBody: "Usage: awf check staged state\n\nValidate the HEAD-to-index current-state transition.\n"},
					{Name: "drift", Summary: "Compare staged config with staged rendered output", MaxPos: 0,
						HelpBody: "Usage: awf check staged drift\n\nReport stale or hand-edited rendered output in the staged tree.\n"},
					{Name: "commit", Summary: "Validate one commit message and stale-ADR merge authorization, blocking", MaxPos: 1, StateExempt: true,
						HelpBody: `Usage: awf check staged commit [FILE]

Validate one commit message against the Conventional Commits rules and
definitively validate stale-format ADR merge authorization. Reads FILE (the
path a commit-msg hook passes as $1) or stdin and cleans it git-style. Merge and
autosquash subjects are exempt only from Conventional Commits. An older-format
ADR introduced by a real merge must qualify against an incoming MERGE_HEAD
parent and carry an adjacent AWF-Allow-Version / nonempty AWF-Allow-Reason pair
in the final trailer block; malformed reserved trailers refuse. Refusal leaves
the staged index, message, and merge state unchanged so correcting the trailers
and rerunning git commit finishes the existing merge. awf installs no hook; wire
this into your own commit-msg hook (the rendered .awf/hooks/commit-msg.sh payload
runs it when the hooks artifact is enabled).
`},
				},
			},
		},
	},

	{
		Name: "audit", Summary: "Report workflow-conformance findings over a commit range (advisory)",
		MaxPos: 1, Gating: Gated,
		HelpBody: `Usage: awf audit <base>|<a>..<b>

Report advisory workflow-conformance findings over an explicit commit range; never gates.
The range is required: a bare <base> means <base>..HEAD, or give a two-sided <a>..<b>.
There is no default range, so an audit never reports over commits nobody named.
`,
	},
	{
		Name: "effort", Summary: "Manage slugged repository-local efforts",
		MaxPos: 0, Gating: Gated,
		HelpBody: `Usage: awf effort <subcommand>

Create, inspect, finish, integrate, and remove immutable slugged efforts, which get a managed worktree by default.
`,
		Children: []Command{
			{Name: "new", Summary: "Create an effort with a managed worktree by default",
				BoolFlags: []string{"--json", "--no-worktree"}, ValueFlags: []string{"--base"},
				MinPos: 1, MaxPos: 1,
				HelpBody: `Usage: awf effort new <outcome-title> [--json] [--no-worktree] [--base <ref>]

Derive an immutable slug, publish schema-2 state with owned memory, and create the
managed .awf/worktrees/<slug> checkout on awf/<slug> (base: the invoking checkout's
HEAD, or --base <ref>). --no-worktree keeps execution in the invoking checkout and
rejects --base. On worktree failure the effort is rolled back only when managed
topology is proven absent; otherwise it is retained with recovery steps.
`},
			{Name: "list", Summary: "List efforts by slug", BoolFlags: []string{"--json"}, MaxPos: 0,
				HelpBody: `Usage: awf effort list [--json]

List every usable active effort in slug order.
`},
			{Name: "show", Summary: "Show one effort", BoolFlags: []string{"--json"}, MinPos: 1, MaxPos: 1,
				HelpBody: `Usage: awf effort show <slug> [--json]

Show one schema-2 effort and its owned memory path.
`},
			{Name: "finish", Summary: "Finish and delete one effort", MinPos: 1, MaxPos: 1,
				HelpBody: `Usage: awf effort finish <slug>

Restartably delete an effort only after all managed Git topology is absent.
`},
			{Name: "worktree", Summary: "Add or remove a managed worktree", ValueFlags: []string{"--base"}, MinPos: 2, MaxPos: 2,
				HelpBody: `Usage: awf effort worktree add <slug> [--base <ref>]
       awf effort worktree remove <slug>

Manage the fixed .awf/worktrees/<slug> checkout and awf/<slug> branch without stored attachment state.
`},
			{Name: "integrate", Summary: "Integrate a managed worktree", MinPos: 1, MaxPos: 1,
				HelpBody: "Usage: awf effort integrate <slug>\n\nIntegrate into the invoking clean target checkout without committing, reviewing, removing, or finishing.\n"},
		},
	},
	{
		Name: "adr", Summary: "ADR lifecycle operations", MaxPos: 0, Gating: Gated,
		HelpBody: `Usage: awf adr <subcommand>

Perform an ADR lifecycle operation that the corpus, not the author, owns.
`,
		Children: []Command{
			{Name: "number", Summary: "Number pending ADRs at integration", MinPos: 0, MaxPos: -1,
				HelpBody: `Usage: awf adr number [<slug>...]

Number pending ADRs after merging the integration branch in and before merging
back. Bare invocation numbers a single pending ADR; several pending ADRs require
an explicit list naming every pending slug, in the intended add-before-revise
order.
`},
		},
	},
	{
		Name: "list", Summary: "Show targets and their per-project state (all kinds, or one)",
		MaxPos: 1, Gating: Gated,
		HelpBody: `Usage: awf list [<kind>]

Show targets and their per-project enabled state, for all kinds or one (skill|agent|doc|domain|target|bootstrap|hooks).
`,
	},
	{
		Name: "config", Summary: "Describe config keys and vars (live state inside a project)",
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
		Name: "context", Summary: "Orient by request with compact current-state impact reports",
		BoolFlags: []string{"--staged", "--uncovered", "--full"}, ValueFlags: []string{"--range", "--show"}, Repeatable: []string{"--show"}, MaxPos: -1, Gating: GatedInHandler,
		HelpBody: `Usage: awf context <path>... [--show <facet>]... [--full] [--staged] [--range <a>..<b>] [--uncovered]

Report request-oriented current-state impact. Bare directories use tier 0:
census, compact groups, classification, compact provenance, domains, topics,
per-topic invariant/rule counts, and bounded pending summaries. Bare exact files
and Git-selected staged/range files add tier-1 relationships declared on that
file, rendering only non-empty State, Touches, and Proofs marker-kind sets.
Groups of at most three list every member and larger groups disclose no paths.

Repeat --show with one of relationships, invariants, all-rules, evidence,
selectors, references, pending, or artifacts. relationships expands a directory's
aggregated direct relationships; invariants and all-rules expand non-direct claim
summaries. evidence and references only enrich claims already visible through a
tier or authority facet. Only artifacts may refine directory grouping. --full is
exactly the normalized union of all eight facets. --show and --full cannot be
combined with --uncovered. JSON is not supported.

The complete human rendering is written unchanged through 8,192 bytes. Larger
results spill to an owner-only temporary file outside the repository and stdout
receives AWF_CONTEXT_SPILL_V1, its decimal byte count and text format, followed
by the absolute path. A caller that receives the notice owns deletion.

Provide paths explicitly, or resolve sorted exact files from Git with --staged
or --range <a>..<b>. With --uncovered, positional args are optional scan roots;
--range is not accepted.

Flags:
  --show <facet>        add one bounded detail facet (repeatable)
  --full                add all eight facets
  --staged              use the staged index universe
  --range <a>..<b>      use paths changed between revisions a and b
  --uncovered           report unowned and uncovered paths
`,
	},
	{
		Name: "topic", Summary: "Query current claims, history, references, and applicability",
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
		Name: "new", Summary: "Scaffold a new artifact: kind ∈ {adr, plan, topic, skill, agent, doc}",
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

Scaffold a new ADR under docs/decisions from the rendered template, with its
date and title heading filled in. The identity depends on the branch: on the
configured integrationBranch the record gets the next sequential number
(NNNN-<slug>.md), and anywhere else it is written as a pending record named
<slug>.md, which awf adr number numbers at integration time.

The title must not slugify to a reserved name (readme, index, template), to a
slug already used in the corpus, or to something opening with four digits and a
hyphen, which would read as a number.
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
		Name: "enable", Summary: "Enable an artifact: kind ∈ {skill, agent, doc, domain, target, bootstrap, hooks, runner}",
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
		Name: "disable", Summary: "Disable an artifact: kind ∈ {skill, agent, doc, domain, target, bootstrap, hooks, runner}",
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
		Name: "upgrade", Summary: "Migrate the .awf/ config tree or consume a current-state attestation",
		BoolFlags: []string{"--recover"}, MaxPos: 0, Gating: Ungated,
		HelpBody: `Usage: awf upgrade [--recover]

Migrate the .awf/ config tree to the current schema version, then sync.

When the lock carries a bridge attestation, plain upgrade instead performs the
final current-state cutover: it verifies the complete sealed attestation,
including the prepared HEAD, tree digest, and historical routing payload, then
journals deletion of the migration approval file and replacement of the
permanent lock while discarding the cutoff and gap payload. Attestation and
readiness reporting live only in the preceding bridge release; this binary
consumes seals, it never produces them.

  --recover              replay the current-state upgrade journal's recovery
                         table: roll an interrupted cutover back or clean up a
                         committed one. The only mode a journal permits.
`,
	},
	{
		Name: "uninstall", Summary: "Remove awf's generated files (keeps .awf/)",
		MaxPos: 0, Gating: Ungated,
		HelpBody: `Usage: awf uninstall

Remove every awf-generated file recorded in the lock (keeps your authored .awf/ config).
`,
	},
	{
		Name: "changelog", Summary: "Print the embedded changelog, or one version/range of it",
		ValueFlags: []string{"--version", "--since", "--range"}, MaxPos: 0, Gating: Ungated,
		StateExempt: true,
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
		Name: "version", Summary: "Print the awf version",
		MaxPos: 0, Gating: Ungated, StateExempt: true,
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

// UsageLine renders the `awf <a|b|...>` usage token list from the table.
func UsageLine() string { return "awf <" + strings.Join(Names(), "|") + ">" }

// GatedCommandNames returns, in table order, every top-level command that runs
// the binary-version gate - the driver-gated commands plus the ones that gate
// in-handler (config/context/topic after their static fallback, new after name
// validation). Ungated commands are excluded; a group contributes only its own
// token. It is the source of the doc-published gated-command list.
func GatedCommandNames() []string {
	var out []string
	for _, c := range Commands {
		if c.Gating != Ungated {
			out = append(out, c.Name)
		}
	}
	return out
}
