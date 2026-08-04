// Package clispec is the single declarative source of awf's CLI command set:
// every command's flags, positional bounds, gating, help text, and (for a group
// command) its subcommands. cmd/awf builds its runtime dispatcher by attaching
// handler funcs to these specs; internal/project reads the gated set to generate
// docs. Data only - no handler funcs and no import of cmd/awf or internal/project,
// so it stays an importable leaf.
package clispec

import (
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
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
	Help       Help
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

// Help is structured command help. The specification owns its semantic data;
// it lowers through the common presentation tree at the command boundary.
type Help struct {
	Usage       []string
	Description string
	Details     []string
	Positionals []HelpItem
	Options     []HelpItem
	Examples    []string
	Related     []string
}

// HelpItem is a named positional or option with its description.
type HelpItem struct{ Name, Description string }

// Document lowers structured help to the shared presentation grammar.
func (h Help) Document(command, summary string) (presentation.Document, error) {
	commandValue, err := presentation.Prose(command)
	if err != nil {
		return presentation.Document{}, err
	}
	field, err := presentation.NewField("command", commandValue)
	if err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
		return presentation.Document{}, err
	}
	summaryValue, err := presentation.Prose(summary)
	if err != nil {
		return presentation.Document{}, err
	}
	summaryField, err := presentation.NewField("summary", summaryValue)
	if err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
		return presentation.Document{}, err
	}
	nodes := []presentation.Node{}
	if len(h.Usage) > 0 {
		list, err := helpList("usage", h.Usage)
		if err != nil {
			return presentation.Document{}, err
		}
		nodes = append(nodes, list)
	}
	if h.Description != "" {
		descriptionValue, err := presentation.Prose(h.Description)
		if err != nil {
			return presentation.Document{}, err
		}
		description, err := presentation.NewField("description", descriptionValue)
		if err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
			return presentation.Document{}, err
		}
		nodes = append(nodes, description)
	}
	if len(h.Details) > 0 {
		list, err := helpList("details", h.Details)
		if err != nil {
			return presentation.Document{}, err
		}
		nodes = append(nodes, list)
	}
	if len(h.Positionals) > 0 {
		group, err := helpItems("positionals", h.Positionals)
		if err != nil {
			return presentation.Document{}, err
		}
		nodes = append(nodes, group)
	}
	if len(h.Options) > 0 {
		group, err := helpItems("options", h.Options)
		if err != nil {
			return presentation.Document{}, err
		}
		nodes = append(nodes, group)
	}
	if len(h.Examples) > 0 {
		list, err := helpList("examples", h.Examples)
		if err != nil {
			return presentation.Document{}, err
		}
		nodes = append(nodes, list)
	}
	if len(h.Related) > 0 {
		list, err := helpList("related commands", h.Related)
		if err != nil {
			return presentation.Document{}, err
		}
		nodes = append(nodes, list)
	}
	section, err := presentation.NewSection("help", nodes...)
	if err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
		return presentation.Document{}, err
	}
	return presentation.NewDocument(field, summaryField, section)
}
func helpList(label string, values []string) (presentation.List, error) {
	out := make([]presentation.Value, len(values))
	for i, value := range values {
		var err error
		out[i], err = presentation.Prose(value)
		if err != nil {
			return presentation.List{}, err
		}
	}
	return presentation.NewList(label, out...)
}
func helpItems(label string, items []HelpItem) (presentation.RecordGroup, error) {
	records := make([]presentation.Record, len(items))
	for i, item := range items {
		name, err := presentation.Literal(item.Name)
		if err != nil {
			return presentation.RecordGroup{}, err
		}
		description, err := presentation.Prose(item.Description)
		if err != nil {
			return presentation.RecordGroup{}, err
		}
		records[i], err = presentation.NewRecord(name, description)
		if err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
			return presentation.RecordGroup{}, err
		}
	}
	return presentation.NewRecordGroup(label, []string{"name", "description"}, records...)
}

// Commands is the ordered command table - the sole source of the command set,
// `awf help` order, the usage line, and the gated-command list.
// touches-state: tooling/cli:cli-command-spec-single-source - sole command-table source; proof in clispec_test.go
var Commands = []Command{
	{
		Name: "init", Summary: "Scaffold .awf/ and render the workflow-core set",
		BoolFlags: []string{"--force", "--describe"}, ValueFlags: []string{"--set", "--answers"},
		Repeatable: []string{"--set"}, MaxPos: 0, Gating: Ungated,
		Help: Help{Usage: []string{"awf init [flags]"}, Description: "Scaffold a .awf/ config tree and render the workflow-core set into the project.", Options: []HelpItem{{Name: "--force", Description: "overwrite colliding files, backing each up to <path>.awf-bak"}, {Name: "--describe", Description: "print the fillable value descriptors as JSON and exit"}, {Name: "--set", Description: "k=v      set a value non-interactively (repeatable)"}, {Name: "--answers", Description: "FILE read values from a JSON/YAML answers file: a flat key→value map of descriptor keys (see --describe); multiselect answers (skills, docs) are comma-joined name lists"}}},
	},
	{
		Name: "render", Summary: "Re-render after a template or config change",
		MaxPos: 0, Gating: Gated,
		Help: Help{Usage: []string{"awf render"}, Description: "Re-render every enabled target after a template or config change and update .awf/awf.lock."},
	},
	{
		Name: "check", Summary: "Verify the repository and staged universes",
		MaxPos: -1, Gating: Gated,
		Help: Help{Usage: []string{"awf check", "awf check repo [subcommand]", "awf check staged [subcommand]", "awf check commit-policy <revision-or-range>..."}, Description: "Bare check runs both universes. The repo universe checks drift, current state,", Details: []string{"and the opt-in scans; the staged universe validates the HEAD-to-index transition.", "Outside a Git repository bare check runs the repo universe and reports that the", "staged universe is unavailable."}},
		Children: []Command{
			{Name: "commit-policy", Summary: "Verify exact commit provenance for explicit targets", MinPos: 1, MaxPos: -1,
				Help: Help{Usage: []string{"awf check commit-policy <revision-or-range>..."}, Description: "Verify every unique commit reachable from explicit targets after the configured baseline. An absent policy reports one disabled-policy note and succeeds.", Positionals: []HelpItem{{Name: "<revision-or-range>", Description: "commit revision or range to verify"}}}},
			{Name: "repo", Summary: "Verify repository properties", MaxPos: -1,
				Help: Help{Usage: []string{"awf check repo [subcommand]"}, Description: "Run the repository universe: drift, current-state, prose, and memory checks."},
				Children: []Command{
					{Name: "drift", Summary: "Report stale or hand-edited rendered output", MaxPos: 0,
						Help: Help{Usage: []string{"awf check repo drift"}, Description: "Re-render in memory and report stale or hand-edited output."}},
					{Name: "state", Summary: "Report current-state authority findings", MaxPos: 0,
						Help: Help{Usage: []string{"awf check repo state"}, Description: "Check current-state authority over the working tree."}},
					{Name: "prose", Summary: "Scan tracked text files for typographic punctuation, blocking", MaxPos: 0,
						Help: Help{Usage: []string{"awf check repo prose"}, Description: "Scan the project's tracked text files when proseGate.enabled is true."}},
					{Name: "memory", Summary: "Scan staged decision records for working-memory citations, blocking", MaxPos: 0,
						Help: Help{Usage: []string{"awf check repo memory"}, Description: "Scan tracked decision records when memoryCite.enabled is true."}},
				},
			},
			{Name: "staged", Summary: "Verify staged transition properties", MaxPos: -1,
				Help: Help{Usage: []string{"awf check staged [subcommand]"}, Description: "Run the staged transition and rendered-output drift checks. The commit child is", Details: []string{"directly invoked by a commit-msg hook and is not part of the aggregate."}},
				Children: []Command{
					{Name: "state", Summary: "Report staged current-state transition findings", MaxPos: 0,
						Help: Help{Usage: []string{"awf check staged state"}, Description: "Validate the HEAD-to-index current-state transition."}},
					{Name: "drift", Summary: "Compare staged config with staged rendered output", MaxPos: 0,
						Help: Help{Usage: []string{"awf check staged drift"}, Description: "Report stale or hand-edited rendered output in the staged tree."}},
					{Name: "commit", Summary: "Validate one commit message and stale-ADR merge authorization, blocking", MaxPos: 1, StateExempt: true,
						Help: Help{Usage: []string{"awf check staged commit [FILE]"}, Description: "Validate one commit message against the Conventional Commits rules and", Details: []string{"definitively validate stale-format ADR merge authorization. Reads FILE (the", "path a commit-msg hook passes as $1) or stdin and cleans it git-style. Merge and", "autosquash subjects are exempt only from Conventional Commits. An older-format", "ADR introduced by a real merge must qualify against an incoming MERGE_HEAD", "parent and carry an adjacent AWF-Allow-Version / nonempty AWF-Allow-Reason pair", "in the final trailer block; malformed reserved trailers refuse. Refusal leaves", "the staged index, message, and merge state unchanged so correcting the trailers", "and rerunning git commit finishes the existing merge. awf installs no hook; wire", "this into your own commit-msg hook (the rendered .awf/hooks/commit-msg.sh payload", "runs it when the hooks artifact is enabled)."}, Positionals: []HelpItem{{Name: "[FILE]", Description: "commit message file; reads stdin when omitted"}}}},
				},
			},
		},
	},

	{
		Name: "read", Summary: "Read an executable projection from a parsed artifact",
		MaxPos: 0, Gating: Gated,
		Help: Help{Usage: []string{"awf read <subcommand>"}, Description: "Read a bounded executable projection from a parsed project artifact."},
		Children: []Command{
			{Name: "plan", Summary: "Read one exact plan phase or task projection", MinPos: 2, MaxPos: 2,
				Help: Help{Usage: []string{"awf read plan <plan> <P[.T]>"}, Description: "Resolve <plan> as an exact filename or exact filename stem under the configured", Details: []string{"plans directory. P selects a complete phase; P.T selects one task plus its phase", "closure. Plan-v2 always includes task-scoped Decisions and phase outcomes; plan-v1", "retains its original closure. Selectors are canonical positive integers, and failures", "list available exact plan names or selectors."}, Positionals: []HelpItem{{Name: "<plan>", Description: "exact plan filename or filename stem"}, {Name: "<P[.T]>", Description: "canonical positive phase or phase.task selector"}}}},
		},
	},
	{
		Name: "audit", Summary: "Report workflow-conformance findings over a commit range (advisory)",
		MaxPos: 1, Gating: Gated,
		Help: Help{Usage: []string{"awf audit <base>|<a>..<b>"}, Description: "Report advisory workflow-conformance findings over an explicit commit range; never gates.", Details: []string{"The range is required: a bare <base> means <base>..HEAD, or give a two-sided <a>..<b>.", "There is no default range, so an audit never reports over commits nobody named."}, Positionals: []HelpItem{{Name: "<base>", Description: "base revision for the audit range"}, {Name: "<a>", Description: "left revision for a two-sided range"}, {Name: "<b>", Description: "right revision for a two-sided range"}}},
	},
	{
		Name: "effort", Summary: "Manage slugged repository-local efforts",
		MaxPos: 0, Gating: Gated,
		Help: Help{Usage: []string{"awf effort <subcommand>"}, Description: "Create, inspect, finish, integrate, and remove immutable slugged efforts, which get a managed worktree by default."},
		Children: []Command{
			{Name: "new", Summary: "Create an effort with a managed worktree by default",
				BoolFlags: []string{"--json", "--no-worktree"}, ValueFlags: []string{"--slug", "--base"},
				MinPos: 1, MaxPos: 1,
				Help: Help{Usage: []string{"awf effort new --slug <slug> <outcome-title> [--json] [--no-worktree] [--base <ref>]"}, Description: "Create schema-2 effort state with owned memory and a managed worktree by default.", Details: []string{"The immutable canonical slug is supplied independently of the single outcome title. Flags may appear before or after that positional.", "The worktree uses the invoking checkout HEAD by default; --no-worktree keeps execution in the invoking checkout and rejects --base. A worktree failure rolls back only when managed topology is proven absent."}, Positionals: []HelpItem{{Name: "<outcome-title>", Description: "single effort outcome title"}}, Options: []HelpItem{{Name: "--slug", Description: "<slug> immutable canonical slug of 1 through 32 bytes"}, {Name: "--base", Description: "<ref> base revision for the managed worktree"}, {Name: "--no-worktree", Description: "keep execution in the invoking checkout"}, {Name: "--json", Description: "emit the activity-compatible machine form"}}}},

			{Name: "list", Summary: "List efforts by slug", BoolFlags: []string{"--json"}, MaxPos: 0,
				Help: Help{Usage: []string{"awf effort list [--json]"}, Description: "List every usable active effort in slug order.", Options: []HelpItem{{Name: "--json", Description: "emit the activity-compatible machine form"}}}},
			{Name: "show", Summary: "Show one effort", BoolFlags: []string{"--json"}, MinPos: 1, MaxPos: 1,
				Help: Help{Usage: []string{"awf effort show <slug> [--json]"}, Description: "Show one schema-2 effort and its owned memory path.", Positionals: []HelpItem{{Name: "<slug>", Description: "immutable effort slug"}}, Options: []HelpItem{{Name: "--json", Description: "emit the activity-compatible machine form"}}}},
			{Name: "finish", Summary: "Finish and delete one effort", MinPos: 1, MaxPos: 1,
				Help: Help{Usage: []string{"awf effort finish <slug>"}, Description: "Restartably delete an effort only after all managed Git topology is absent.", Positionals: []HelpItem{{Name: "<slug>", Description: "immutable effort slug"}}}},
			{Name: "worktree", Summary: "Add or remove a managed worktree", ValueFlags: []string{"--base"}, MinPos: 2, MaxPos: 2,
				Help: Help{Usage: []string{"awf effort worktree add <slug> [--base <ref>]", "awf effort worktree remove <slug>"}, Description: "Manage the fixed .awf/worktrees/<slug> checkout and awf/<slug> branch without stored attachment state.", Positionals: []HelpItem{{Name: "<add|remove>", Description: "worktree operation"}, {Name: "<slug>", Description: "immutable effort slug"}}, Options: []HelpItem{{Name: "--base", Description: "<ref> Git revision used as the worktree base"}}}},
			{Name: "integrate", Summary: "Integrate a managed worktree", MinPos: 1, MaxPos: 1,
				Help: Help{Usage: []string{"awf effort integrate <slug>"}, Description: "Integrate into the invoking clean target checkout without committing, reviewing, removing, or finishing.", Positionals: []HelpItem{{Name: "<slug>", Description: "immutable effort slug"}}}},
			{Name: "memory", Summary: "Update bounded effort memory metadata", MaxPos: 0,
				Help: Help{Usage: []string{"awf effort memory update <slug> [--phase <text>] [--next <text>]"}, Description: "Update one or both mutable memory metadata fields. At least one of --phase and", Details: []string{"--next is required."}},
				Children: []Command{
					{Name: "update", Summary: "Update memory phase or next action", ValueFlags: []string{"--phase", "--next"}, MinPos: 1, MaxPos: 1,
						Help: Help{Usage: []string{"awf effort memory update <slug> [--phase <text>] [--next <text>]"}, Description: "Update one or both mutable memory metadata fields. At least one of --phase and", Details: []string{"--next is required."}, Positionals: []HelpItem{{Name: "<slug>", Description: "immutable effort slug"}}, Options: []HelpItem{{Name: "--phase", Description: "<text> replacement phase metadata"}, {Name: "--next", Description: "<text> replacement next-action metadata"}}}}},
			},
			{Name: "activity", Summary: "Mutate advisory Pi session activity", MaxPos: 0,
				Help: Help{Usage: []string{"awf effort activity attach <slug> --owner <uuid> --json", "awf effort activity heartbeat <slug> --owner <uuid> --json", "awf effort activity detach <slug> --owner <uuid> --json"}, Description: "Activity replies are protocol-2 JSON only."},
				Children: []Command{
					{Name: "attach", Summary: "Attach or take over an advisory activity claim", BoolFlags: []string{"--json"}, ValueFlags: []string{"--owner"}, MinPos: 1, MaxPos: 1, Help: Help{Usage: []string{"awf effort activity attach <slug> --owner <uuid> --json"}, Description: "Attach this Pi owner to an effort activity record and emit the protocol-2 JSON reply.", Positionals: []HelpItem{{Name: "<slug>", Description: "immutable effort slug"}}, Options: []HelpItem{{Name: "--owner", Description: "<uuid> Pi session owner UUID"}, {Name: "--json", Description: "require the protocol-2 JSON reply"}}}},

					{Name: "heartbeat", Summary: "Heartbeat an owned advisory activity claim", BoolFlags: []string{"--json"}, ValueFlags: []string{"--owner"}, MinPos: 1, MaxPos: 1, Help: Help{Usage: []string{"awf effort activity heartbeat <slug> --owner <uuid> --json"}, Description: "Refresh this Pi owner's advisory activity claim and emit the protocol-2 JSON reply.", Positionals: []HelpItem{{Name: "<slug>", Description: "immutable effort slug"}}, Options: []HelpItem{{Name: "--owner", Description: "<uuid> Pi session owner UUID"}, {Name: "--json", Description: "require the protocol-2 JSON reply"}}}},

					{Name: "detach", Summary: "Detach an owned advisory activity claim", BoolFlags: []string{"--json"}, ValueFlags: []string{"--owner"}, MinPos: 1, MaxPos: 1, Help: Help{Usage: []string{"awf effort activity detach <slug> --owner <uuid> --json"}, Description: "Remove this Pi owner's advisory activity claim and emit the protocol-2 JSON reply.", Positionals: []HelpItem{{Name: "<slug>", Description: "immutable effort slug"}}, Options: []HelpItem{{Name: "--owner", Description: "<uuid> Pi session owner UUID"}, {Name: "--json", Description: "require the protocol-2 JSON reply"}}}},
				},
			},
		},
	},
	{
		Name: "adr", Summary: "ADR lifecycle operations", MaxPos: 0, Gating: Gated,
		Help: Help{Usage: []string{"awf adr <subcommand>"}, Description: "Perform an ADR lifecycle operation that the corpus, not the author, owns."},
		Children: []Command{
			{Name: "number", Summary: "Number pending ADRs at integration", MinPos: 0, MaxPos: -1,
				Help: Help{Usage: []string{"awf adr number [<slug>...]"}, Description: "Number pending ADRs after merging the integration branch in and before merging", Details: []string{"back. Bare invocation numbers a single pending ADR; several pending ADRs require", "an explicit list naming every pending slug, in the intended add-before-revise", "order."}, Positionals: []HelpItem{{Name: "<slug>", Description: "ADR filename slug, without number or extension"}}}},
		},
	},
	{
		Name: "list", Summary: "Show targets and their per-project state (all kinds, or one)",
		MaxPos: 1, Gating: Gated,
		Help: Help{Usage: []string{"awf list [<kind>]"}, Description: "Show targets and their per-project enabled state, for all kinds or one (skill|agent|doc|domain|target|bootstrap|hooks).", Positionals: []HelpItem{{Name: "<kind>", Description: "artifact kind"}}},
	},
	{
		Name: "config", Summary: "Describe config keys and vars (live state inside a project)",
		MaxPos: 1, Gating: GatedInHandler,
		Help: Help{Usage: []string{"awf config [<key-or-var>]"}, Description: "Print the configuration reference: every config key, var, sidecar field, and", Details: []string{"data key with descriptions, defaults, and availability. Inside an awf project", "the output adds live state (current values; which enabled artifacts consume", "each var; dormant hints). Outside one, a static catalog-wide reference prints.", "With an argument, print just that entry (a config key path like", "audit.diffThreshold, a var name like gateCmd, a sidecar field like", "sidecar.local, or a data key name)."}, Positionals: []HelpItem{{Name: "<key-or-var>", Description: "config key, var, sidecar field, or data key"}}},
	},
	{
		Name: "context", Summary: "Orient by request with compact current-state impact reports",
		BoolFlags: []string{"--staged", "--uncovered", "--full"}, ValueFlags: []string{"--range", "--show"}, Repeatable: []string{"--show"}, MaxPos: -1, Gating: GatedInHandler,
		Help: Help{Usage: []string{"awf context <path>... [--show <facet>]... [--full] [--staged] [--range <a>..<b>] [--uncovered]"}, Description: "Report request-oriented current-state impact. Bare directories use tier 0:", Details: []string{"census, compact groups, classification, compact provenance, domains, topics,", "per-topic invariant/rule counts, and bounded pending summaries. Bare exact files", "and Git-selected staged/range files add tier-1 relationships declared on that", "file, rendering only non-empty State, Touches, and Proofs marker-kind sets.", "Groups of at most three list every member and larger groups disclose no paths.", "Repeat --show with one of relationships, invariants, all-rules, evidence,", "selectors, references, pending, or artifacts. relationships expands a directory's", "aggregated direct relationships; invariants and all-rules expand non-direct claim", "summaries. evidence and references only enrich claims already visible through a", "tier or authority facet. Only artifacts may refine directory grouping. --full is", "exactly the normalized union of all eight facets. --show and --full cannot be", "combined with --uncovered. JSON is not supported.", "The complete human rendering is written unchanged through 8,192 bytes. Larger", "results spill to an owner-only temporary file outside the repository and stdout", "receives AWF_CONTEXT_SPILL_V1, its decimal byte count and text format, followed", "by the absolute path. A caller that receives the notice owns deletion.", "Provide paths explicitly, or resolve sorted exact files from Git with --staged", "or --range <a>..<b>. With --uncovered, positional args are optional scan roots;", "--range is not accepted."}, Positionals: []HelpItem{{Name: "<path>", Description: "repository path or directory to orient against"}}, Options: []HelpItem{{Name: "--show", Description: "<facet>        add one bounded detail facet (repeatable)"}, {Name: "--full", Description: "add all eight facets"}, {Name: "--staged", Description: "use the staged index universe"}, {Name: "--range", Description: "<a>..<b>      use paths changed between revisions a and b"}, {Name: "--uncovered", Description: "report unowned and uncovered paths"}}},
	},
	{
		Name: "topic", Summary: "Query current claims, history, references, and applicability",
		BoolFlags: []string{"--history", "--references", "--coverage"}, MinPos: 1, MaxPos: 1, Gating: GatedInHandler,
		Help: Help{Usage: []string{"awf topic <domain>/<topic>[:<claim>] [flags]"}, Description: "Query one current-state topic or claim, active by default. Default output includes", Details: []string{"title and summary for a topic plus claim types, prose, and backing state. Detail", "flags are independent and direct-only. A removed claim identity resolves only", "with --history and returns operation history without an active tombstone. Outside", "an awf project, a static command reference prints without version gating."}, Positionals: []HelpItem{{Name: "<domain>/<topic>[:<claim>]", Description: "current-state topic or claim identifier"}}, Options: []HelpItem{{Name: "--history", Description: "add direct Origin, Revised-by, and Removed-by ADR details"}, {Name: "--references", Description: "add sorted direct incoming and outgoing claim IDs"}, {Name: "--coverage", Description: "add separate domain/topic scopes, current matches, and marker sites"}}},
	},
	{
		Name: "new", Summary: "Scaffold a new artifact: kind ∈ {adr, plan, topic, skill, agent, doc}",
		MaxPos: -1, Gating: GatedInHandler,
		Help: Help{
			Usage:       []string{"awf new <kind> <args>"},
			Description: "Scaffold a new artifact. The kind is adr, plan, topic, skill, agent, or doc.",
			Positionals: []HelpItem{
				{Name: "<kind>", Description: "artifact kind"},
				{Name: "<args>", Description: "arguments required by the selected kind"},
			},
			Examples: []string{
				"awf new adr \"Some Decision Title\"",
				"awf new plan \"Some Plan Title\"",
				"awf new topic <domain> \"Some Topic Title\"",
				"awf new skill <name> \"<description>\" (a project-local skill)",
				"awf new agent <name> \"<description>\" (a project-local agent)",
				"awf new doc <name> \"<description>\" (a project-local doc; name may be nested, for example guides/ci)",
			},
			Related: []string{"awf adr number"},
		},
		Children: []Command{
			{
				Name: "adr", Summary: "Scaffold a new ADR", MinPos: 0, MaxPos: -1,
				Help: Help{Usage: []string{"awf new adr <title>"}, Description: "Scaffold a new ADR under docs/decisions from the rendered template, with its", Details: []string{"date and title heading filled in. The identity depends on the branch: on the", "configured integrationBranch the record gets the next sequential number", "(NNNN-<slug>.md), and anywhere else it is written as a pending record named", "<slug>.md, which awf adr number numbers at integration time.", "The title must not slugify to a reserved name (readme, index, template), to a", "slug already used in the corpus, or to something opening with four digits and a", "hyphen, which would read as a number."}, Positionals: []HelpItem{{Name: "<title>", Description: "human-readable artifact title"}}},
			},
			{
				Name: "plan", Summary: "Scaffold a new plan", MinPos: 0, MaxPos: -1,
				Help: Help{Usage: []string{"awf new plan <title>"}, Description: "Scaffold a new plan under docs/plans, date-prefixed (no sequential number),", Details: []string{"from the rendered plans template with its date and title heading filled in."}, Positionals: []HelpItem{{Name: "<title>", Description: "human-readable artifact title"}}},
			},
			{
				Name: "topic", Summary: "Scaffold paired current-state topic inputs", MinPos: 2, MaxPos: -1,
				Help: Help{Usage: []string{"awf new topic <domain> <title>"}, Description: "Scaffold paired topic metadata and authored current-state inputs without syncing.", Details: []string{"Edit the path placeholder and author reviewed claims manually."}, Positionals: []HelpItem{{Name: "<domain>", Description: "current-state domain identifier"}, {Name: "<title>", Description: "human-readable artifact title"}}},
			},
			{
				Name: "skill", Summary: "Scaffold a project-local skill", MinPos: 0, MaxPos: -1,
				Help: Help{Usage: []string{"awf new skill <name> \"<description>\""}, Description: "Scaffold a project-local skill: a declaring sidecar carrying the description, a", Details: []string{"starter content part, the enable, and a re-render."}, Positionals: []HelpItem{{Name: "<name>", Description: "artifact name"}, {Name: "<description>", Description: "artifact description"}}},
			},
			{
				Name: "agent", Summary: "Scaffold a project-local agent", MinPos: 0, MaxPos: -1,
				Help: Help{Usage: []string{"awf new agent <name> \"<description>\""}, Description: "Scaffold a project-local agent: a declaring sidecar carrying the description, a", Details: []string{"starter content part, the enable, and a re-render."}, Positionals: []HelpItem{{Name: "<name>", Description: "artifact name"}, {Name: "<description>", Description: "artifact description"}}},
			},
			{
				Name: "doc", Summary: "Scaffold a project-local doc", MinPos: 0, MaxPos: -1,
				Help: Help{Usage: []string{"awf new doc <name> \"<description>\""}, Description: "Scaffold a project-local doc; the name may be nested, e.g. guides/ci. Writes a", Details: []string{"declaring sidecar with a derived title and the description, a starter content", "part, the enable, and a re-render."}, Positionals: []HelpItem{{Name: "<name>", Description: "artifact name"}, {Name: "<description>", Description: "artifact description"}}},
			},
		},
	},
	{
		Name: "enable", Summary: "Enable an artifact: kind ∈ {skill, agent, doc, domain, target, bootstrap, hooks, runner}",
		BoolFlags: []string{"--dry-run"}, MaxPos: -1, Gating: Gated,
		Help: Help{Usage: []string{"awf enable <kind> <name> [--dry-run]"}, Description: "Enable an artifact in this project. <kind> is skill, agent, doc, domain, target,", Details: []string{"bootstrap, hooks, or runner. For skill/agent/doc, the full requirement closure is enabled", "in one edit, printed as a plan (ADR-0081)."}, Positionals: []HelpItem{{Name: "<kind>", Description: "artifact kind"}, {Name: "<name>", Description: "artifact name"}}, Options: []HelpItem{{Name: "--dry-run", Description: "print the closure plan without changing the config"}}},
	},
	{
		Name: "disable", Summary: "Disable an artifact: kind ∈ {skill, agent, doc, domain, target, bootstrap, hooks, runner}",
		BoolFlags: []string{"--with-dependents", "--dry-run"}, MaxPos: -1, Gating: Gated,
		Help: Help{Usage: []string{"awf disable <kind> <name> [--with-dependents] [--dry-run]"}, Description: "Disable an artifact: a catalog skill/agent/doc, a freeform domain, an adapter target, the bootstrap, the hooks, or the runner.", Details: []string{"For skill/agent/doc, disabling refuses while enabled artifacts still require", "<name>, printing the dependent plan (ADR-0081)."}, Positionals: []HelpItem{{Name: "<kind>", Description: "artifact kind"}, {Name: "<name>", Description: "artifact name"}}, Options: []HelpItem{{Name: "--with-dependents", Description: "also disable every enabled artifact that transitively requires <name>"}, {Name: "--dry-run", Description: "print the plan without changing the config"}}},
	},
	{
		Name: "upgrade", Summary: "Migrate the .awf/ config tree or consume a current-state attestation",
		BoolFlags: []string{"--recover"}, MaxPos: 0, Gating: Ungated,
		Help: Help{Usage: []string{"awf upgrade [--recover]"}, Description: "Migrate the .awf/ config tree to the current schema version, then sync.", Options: []HelpItem{{Name: "--recover", Description: "replay the current-state upgrade journal's recovery table"}}, Details: []string{"When the lock carries a bridge attestation, plain upgrade instead performs the", "final current-state cutover: it verifies the complete sealed attestation,", "including the prepared HEAD, tree digest, and historical routing payload, then", "journals deletion of the migration approval file and replacement of the", "permanent lock while discarding the cutoff and gap payload. Attestation and", "readiness reporting live only in the preceding bridge release; this binary", "consumes seals, it never produces them.", "--recover              replay the current-state upgrade journal's recovery", "table: roll an interrupted cutover back or clean up a", "committed one. The only mode a journal permits."}},
	},
	{
		Name: "uninstall", Summary: "Remove awf's generated files (keeps .awf/)",
		MaxPos: 0, Gating: Ungated,
		Help: Help{Usage: []string{"awf uninstall"}, Description: "Remove every awf-generated file recorded in the lock (keeps your authored .awf/ config)."},
	},
	{
		Name: "changelog", Summary: "Print the embedded changelog, or one version/range of it",
		ValueFlags: []string{"--version", "--since", "--range"}, MaxPos: 0, Gating: Ungated,
		StateExempt: true,
		Help:        Help{Usage: []string{"awf changelog [--version <v> | --since <v> | --range <from>..<to>]"}, Description: "Print the embedded awf changelog. With no flags, print the whole file. The three", Details: []string{"flags are mutually exclusive."}, Options: []HelpItem{{Name: "--version", Description: "<v>          print only version v's entry"}, {Name: "--since", Description: "<v>            print every version released after v (exclusive)"}, {Name: "--range", Description: "<from>..<to>   print every version in [from, to] (inclusive both ends)"}}},
	},
	{
		Name: "version", Summary: "Print the awf version",
		MaxPos: 0, Gating: Ungated, StateExempt: true,
		Help: Help{Usage: []string{"awf version"}, Description: "Print the awf version."},
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
