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
	// FullOnly declares governance capability at the dispatch boundary.
	FullOnly bool
	Children []Command
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
// `awf help` order, the usage line, gated-command list, and bounded README command projection.
var Commands = []Command{
	{
		Name: "init", Summary: "Scaffold .awf/ and render the selected governance footprint",
		BoolFlags: []string{"--force", "--describe"}, ValueFlags: []string{"--set", "--answers"},
		Repeatable: []string{"--set"}, MaxPos: 0, Gating: Ungated,
		Help: Help{Usage: []string{"awf init [flags]"}, Description: "Scaffold a .awf/ config tree and render the selected governance footprint into the project.", Options: []HelpItem{{Name: "--force", Description: "overwrite colliding files, backing each up to <path>.awf-bak"}, {Name: "--describe", Description: "print the fillable value descriptors as JSON and exit"}, {Name: "--set", Description: "k=v      set a value non-interactively (repeatable)"}, {Name: "--answers", Description: "FILE read values from a JSON/YAML answers file: a flat key→value map of descriptor keys (see --describe)"}}},
	},
	{
		Name: "render", Summary: "Re-render after a template or config change",
		MaxPos: 0, Gating: Gated,
		Help: Help{Usage: []string{"awf render"}, Description: "Re-render both targets after a template or config change and update .awf/awf.lock."},
	},
	{
		Name: "edit", Summary: "Replace one semantically identified artifact part",
		BoolFlags: []string{"--stdin"}, ValueFlags: []string{"--content"}, MinPos: 3, MaxPos: 3, Gating: Gated,
		Children: []Command{{Name: "sidecar", Summary: "Edit one semantically identified sidecar leaf", ValueFlags: []string{"--value", "--json-value", "--add", "--add-json", "--remove", "--remove-json"}, MinPos: 3, MaxPos: 3,
			Help: Help{Usage: []string{"awf edit sidecar <kind> <name> <field> --value <text>", "awf edit sidecar <kind> <name> <field> --json-value <json>", "awf edit sidecar <kind> <name> <field> --add <text>", "awf edit sidecar <kind> <name> <field> --add-json <json>", "awf edit sidecar <kind> <name> <field> --remove <text>", "awf edit sidecar <kind> <name> <field> --remove-json <json>"}, Description: "Edit one capability-valid sidecar leaf and synchronize the project.", Positionals: []HelpItem{{Name: "<kind>", Description: "artifact kind: doc, skill, agent, or domain"}, {Name: "<name>", Description: "semantic artifact name"}, {Name: "<field>", Description: "leaf-only dotted sidecar field"}}, Options: []HelpItem{{Name: "--value", Description: "replace with a string"}, {Name: "--json-value", Description: "replace with one JSON value"}, {Name: "--add", Description: "add one string list member"}, {Name: "--add-json", Description: "add one JSON list member"}, {Name: "--remove", Description: "remove one string list member"}, {Name: "--remove-json", Description: "remove one complete JSON list member"}}}}},
		Help: Help{
			Usage:       []string{"awf edit <kind> <name> <part> --content <text>", "awf edit <kind> <name> <part> --stdin"},
			Description: "Replace one declared convention part or a configured local document's body, validate the complete candidate project, then render and update the lock.",
			Positionals: []HelpItem{{Name: "<kind>", Description: "artifact kind: doc, skill, agent, or domain"}, {Name: "<name>", Description: "catalog, configured-domain, or configured-local-document name"}, {Name: "<part>", Description: "declared part name; local documents expose only body"}},
			Options:     []HelpItem{{Name: "--content", Description: "<text> exact replacement content, including an explicit empty value"}, {Name: "--stdin", Description: "read exact replacement content non-interactively from stdin"}},
			Examples:    []string{"awf edit doc architecture components --stdin"},
			Related:     []string{"awf reset <kind> <name> <part>"},
		},
	},
	{
		Name: "reset", Summary: "Restore one semantically identified artifact part",
		MinPos: 3, MaxPos: 3, Gating: Gated,
		Children: []Command{{Name: "sidecar", Summary: "Reset one semantically identified sidecar leaf", MinPos: 3, MaxPos: 3, Help: Help{Usage: []string{"awf reset sidecar <kind> <name> <field>"}, Description: "Remove one authored sidecar leaf, prune empty parents, and synchronize the project.", Positionals: []HelpItem{{Name: "<kind>", Description: "artifact kind: doc, skill, agent, or domain"}, {Name: "<name>", Description: "semantic artifact name"}, {Name: "<field>", Description: "leaf-only dotted sidecar field"}}}}},
		Help: Help{
			Usage:       []string{"awf reset <kind> <name> <part>"},
			Description: "Remove one authored convention-part override, or empty a configured local document body, then render inherited content and update the lock.",
			Positionals: []HelpItem{{Name: "<kind>", Description: "artifact kind: doc, skill, agent, or domain"}, {Name: "<name>", Description: "catalog, configured-domain, or configured-local-document name"}, {Name: "<part>", Description: "declared part name; local documents expose only body"}},
			Examples:    []string{"awf reset doc runbooks/incident body"},
			Related:     []string{"awf edit <kind> <name> <part>"},
		},
	},
	{
		Name: "check", Summary: "Verify the repository and staged universes",
		MaxPos: -1, Gating: Gated,
		Help: Help{Usage: []string{"awf check", "awf check repo [subcommand]", "awf check staged [subcommand]", "awf check commit-policy <revision-or-range>..."}, Description: "Bare check runs both universes. The repo universe checks drift, profile-applicable authority,", Details: []string{"and the opt-in scans; the staged universe validates the HEAD-to-index transition.", "Outside a Git repository bare check runs the repo universe and reports that the", "staged universe is unavailable."}},
		Children: []Command{
			{Name: "commit-policy", Summary: "Verify exact commit provenance for explicit targets", MinPos: 1, MaxPos: -1,
				Help: Help{Usage: []string{"awf check commit-policy <revision-or-range>..."}, Description: "Verify every unique commit reachable from explicit targets after the configured baseline. An absent policy reports one disabled-policy note and succeeds.", Positionals: []HelpItem{{Name: "<revision-or-range>", Description: "commit revision or range to verify"}}}},
			{Name: "repo", Summary: "Verify repository properties", MaxPos: -1,
				Help: Help{Usage: []string{"awf check repo [subcommand]"}, Description: "Run the repository universe: drift, profile-applicable authority, prose, and memory checks."},
				Children: []Command{
					{Name: "drift", Summary: "Report stale or hand-edited rendered output", MaxPos: 0,
						Help: Help{Usage: []string{"awf check repo drift"}, Description: "Re-render in memory and report stale or hand-edited output."}},
					{Name: "state", Summary: "Report current-state authority findings", MaxPos: 0, FullOnly: true,
						Help: Help{Usage: []string{"awf check repo state"}, Description: "Check current-state authority over the working tree."}},
					{Name: "prose", Summary: "Report punctuation restraint warnings in tracked text (advisory, zero exit)", MaxPos: 0,
						Help: Help{Usage: []string{"awf check repo prose"}, Description: "Report en dashes and paragraphs with more than two em dashes in tracked text as advisory Warnings with zero exit; binary files are skipped."}},
					{Name: "memory", Summary: "Scan staged decision records for working-memory citations, blocking", MaxPos: 0,
						Help: Help{Usage: []string{"awf check repo memory"}, Description: "Scan tracked decision records for working-memory citations."}},
				},
			},
			{Name: "staged", Summary: "Verify staged transition properties", MaxPos: -1,
				Help: Help{Usage: []string{"awf check staged [subcommand]"}, Description: "Run the staged transition and rendered-output drift checks. The commit child is", Details: []string{"directly invoked by a commit-msg hook and is not part of the aggregate."}},
				Children: []Command{
					{Name: "state", Summary: "Report staged current-state transition findings", MaxPos: 0, FullOnly: true,
						Help: Help{Usage: []string{"awf check staged state"}, Description: "Validate the HEAD-to-index current-state transition."}},
					{Name: "drift", Summary: "Compare staged config with staged rendered output", MaxPos: 0,
						Help: Help{Usage: []string{"awf check staged drift"}, Description: "Report stale or hand-edited rendered output in the staged tree."}},
					{Name: "commit", Summary: "Validate one commit message against shared commit rules, blocking", MaxPos: 1, StateExempt: true,
						Help: Help{Usage: []string{"awf check staged commit [<FILE>]"}, Description: "Validate one commit message against shared commit rules. Reads FILE (the path a commit-msg hook passes as $1) or stdin and cleans it git-style.", Details: []string{"Merge and autosquash subjects are exempt only from Conventional Commits. awf installs no hook; wire this into your own commit-msg hook", "(the always-rendered inert .awf/hooks/commit-msg.sh payload runs it once wired)."}, Positionals: []HelpItem{{Name: "[<FILE>]", Description: "commit message file; reads stdin when omitted"}}}},
				},
			},
		},
	},

	{
		Name: "read", Summary: "Read a focused current-state authority projection",
		MaxPos: 0, Gating: Gated,
		Help: Help{Usage: []string{"awf read <subcommand>"}, Description: "Read a bounded projection from parsed current-state authority."},
		Children: []Command{
			{Name: "plan", Summary: "Read one exact plan phase or task projection", MinPos: 2, MaxPos: 2, FullOnly: true,
				Help: Help{Usage: []string{"awf read plan <plan> <P[.T]>"}, Description: "Resolve <plan> as an exact filename or exact filename stem under the configured", Details: []string{"plans directory. P selects a complete phase; P.T selects one task plus its phase", "closure. Plan-v2 always includes task-scoped Decisions and phase outcomes; plan-v1", "retains its original closure. Selectors are canonical positive integers, and failures", "list available exact plan names or selectors."}, Positionals: []HelpItem{{Name: "<plan>", Description: "exact plan filename or filename stem"}, {Name: "<P[.T]>", Description: "canonical positive phase or phase.task selector"}}}},
			{Name: "topic", Summary: "Read one topic or claim authority projection", BoolFlags: []string{"--references", "--coverage"}, MinPos: 1, MaxPos: 1, FullOnly: true,
				Help: Help{Usage: []string{"awf read topic <domain>/<topic>[:<claim>] [flags]"}, Description: "Read active topic or claim authority with optional direct references and coverage.", Positionals: []HelpItem{{Name: "<domain>/<topic>[:<claim>]", Description: "current-state topic or claim identifier"}}, Options: []HelpItem{{Name: "--references", Description: "add direct claim references"}, {Name: "--coverage", Description: "add ownership and marker coverage"}}}},
		},
	},
	{
		Name: "resolve", Summary: "Resolve lexical paths to current-state authority", MaxPos: 0, Gating: Gated,
		Help: Help{Usage: []string{"awf resolve topic <path>...", "awf resolve topic --uncovered"}, Description: "Resolve repository-relative paths to owning domains and applicable topics, or census unowned paths."},
		Children: []Command{{Name: "topic", Summary: "Resolve topic authority for paths or the whole repository", BoolFlags: []string{"--uncovered"}, MinPos: 0, MaxPos: -1, FullOnly: true,
			Help: Help{Usage: []string{"awf resolve topic <path>...", "awf resolve topic --uncovered"}, Description: "Resolve lexical proposed or existing paths without mutation. The uncovered census accepts no positional paths.", Positionals: []HelpItem{{Name: "<path>", Description: "repository-relative path"}}, Options: []HelpItem{{Name: "--uncovered", Description: "report the whole-repository unowned census"}}}}},
	},
	{
		Name: "audit", Summary: "Report workflow-conformance findings over a commit range (advisory)",
		FullOnly: true, MinPos: 1, MaxPos: 1, Gating: Gated,
		Help: Help{Usage: []string{"awf audit <base>|<a>..<b>"}, Description: "Report advisory workflow-conformance findings over an explicit commit range; never gates.", Details: []string{"The range is required: a bare <base> means <base>..HEAD, or give a two-sided <a>..<b>.", "There is no default range, so an audit never reports over commits nobody named."}, Positionals: []HelpItem{{Name: "<base>", Description: "base revision for the audit range"}, {Name: "<a>", Description: "left revision for a two-sided range"}, {Name: "<b>", Description: "right revision for a two-sided range"}}},
	},
	{
		Name: "effort", Summary: "Manage slugged repository-local efforts",
		MaxPos: 0, Gating: Gated,
		Help: Help{Usage: []string{"awf effort <subcommand>"}, Description: "Create, inspect, archive, integrate, and remove immutable slugged efforts, which get a managed worktree by default."},
		Children: []Command{
			{Name: "new", Summary: "Create an effort with a managed worktree by default",
				BoolFlags: []string{"--no-worktree"}, ValueFlags: []string{"--slug", "--base"},
				MinPos: 1, MaxPos: 1,
				Help: Help{Usage: []string{"awf effort new --slug <slug> <outcome-title> [--no-worktree] [--base <ref>]"}, Description: "Create schema-2 effort state with owned memory and a managed worktree by default.", Details: []string{"The immutable canonical slug is supplied independently of the single outcome title. Flags may appear before or after that positional. An optional scratch directory is opaque and never scaffolded or managed.", "The worktree uses the invoking checkout HEAD by default; --no-worktree keeps execution in the invoking checkout and rejects --base. A worktree failure deletes only its identity-matched resident when managed topology is proven absent."}, Positionals: []HelpItem{{Name: "<outcome-title>", Description: "single effort outcome title"}}, Options: []HelpItem{{Name: "--slug", Description: "<slug> immutable canonical slug of 1 through 32 bytes"}, {Name: "--base", Description: "<ref> base revision for the managed worktree"}, {Name: "--no-worktree", Description: "keep execution in the invoking checkout"}}}},

			{Name: "list", Summary: "List efforts by slug", MaxPos: 0,
				Help: Help{Usage: []string{"awf effort list"}, Description: "List every usable active effort in slug order."}},
			{Name: "show", Summary: "Show one effort", MinPos: 1, MaxPos: 1,
				Help: Help{Usage: []string{"awf effort show <slug>"}, Description: "Show one schema-2 effort and its owned memory path.", Positionals: []HelpItem{{Name: "<slug>", Description: "immutable effort slug"}}}},
			{Name: "finish", Summary: "Finish and archive one effort", MinPos: 1, MaxPos: 1,
				Help: Help{Usage: []string{"awf effort finish <slug>"}, Description: "Archive the complete effort at .awf/effort-archive/<uuid>-<slug> only after all managed Git topology is absent.", Details: []string{"The ignored archive is unmanaged and manually disposable. Retry before the archive move; after it, inspect reported paths on durability uncertainty."}, Positionals: []HelpItem{{Name: "<slug>", Description: "immutable effort slug"}}}},
			{Name: "worktree", Summary: "Add or remove a managed worktree", ValueFlags: []string{"--base"}, MinPos: 2, MaxPos: 2,
				Help: Help{Usage: []string{"awf effort worktree add <slug> [--base <ref>]", "awf effort worktree remove <slug>"}, Description: "Manage the fixed .awf/worktrees/<slug> checkout and awf/<slug> branch without stored attachment state.", Positionals: []HelpItem{{Name: "<add|remove>", Description: "worktree operation"}, {Name: "<slug>", Description: "immutable effort slug"}}, Options: []HelpItem{{Name: "--base", Description: "<ref> Git revision used as the worktree base"}}}},
			{Name: "integrate", Summary: "Integrate a managed worktree", MinPos: 1, MaxPos: 1,
				Help: Help{Usage: []string{"awf effort integrate <slug>"}, Description: "Integrate into the invoking clean target checkout without committing, reviewing, removing, or finishing.", Positionals: []HelpItem{{Name: "<slug>", Description: "immutable effort slug"}}}},
			{Name: "memory", Summary: "Read and update bounded effort memory", MaxPos: 0,
				Help: Help{Usage: []string{"awf effort memory <read|edit|update>"}, Description: "Read, exactly edit, or update bounded effort memory. Owner-scoped forms require", Details: []string{"--owner and --json together and emit the closed protocol-1 reply."}},
				Children: []Command{
					{Name: "read", Summary: "Read complete memory lines", BoolFlags: []string{"--json"}, ValueFlags: []string{"--offset", "--limit", "--owner"}, MinPos: 1, MaxPos: 1,
						Help: Help{Usage: []string{"awf effort memory read <slug> [--offset <positive-line>] [--limit <positive-lines>]", "awf effort memory read <slug> [--offset <positive-line>] [--limit <positive-lines>] --owner <uuid> --json"}, Description: "Read a bounded range of complete memory document lines.", Positionals: []HelpItem{{Name: "<slug>", Description: "immutable effort slug"}}, Options: []HelpItem{{Name: "--offset", Description: "<positive-line> first document line"}, {Name: "--limit", Description: "<positive-lines> requested line count"}, {Name: "--owner", Description: "<uuid> advisory activity owner"}, {Name: "--json", Description: "require the owner-scoped protocol-1 reply"}}}},
					{Name: "edit", Summary: "Apply exact body replacements", BoolFlags: []string{"--json", "--preview"}, ValueFlags: []string{"--owner"}, MinPos: 1, MaxPos: 1,
						Help: Help{Usage: []string{"awf effort memory edit <slug>", "awf effort memory edit <slug> --owner <uuid> --json", "awf effort memory edit <slug> --preview --owner <uuid> --json"}, Description: "Read one closed JSON edit request from stdin and atomically replace exact original-body text.", Positionals: []HelpItem{{Name: "<slug>", Description: "immutable effort slug"}}, Options: []HelpItem{{Name: "--owner", Description: "<uuid> advisory activity owner"}, {Name: "--json", Description: "require the owner-scoped protocol-1 reply"}, {Name: "--preview", Description: "compute an owner-scoped read-only diff"}}}},
					{Name: "update", Summary: "Update memory phase or next action", BoolFlags: []string{"--json", "--preview"}, ValueFlags: []string{"--phase", "--next", "--owner"}, MinPos: 1, MaxPos: 1,
						Help: Help{Usage: []string{"awf effort memory update <slug> [--phase <text>] [--next <text>]", "awf effort memory update <slug> [--phase <text>] [--next <text>] --owner <uuid> --json", "awf effort memory update <slug> [--phase <text>] [--next <text>] --preview --owner <uuid> --json"}, Description: "Update one or both mutable memory metadata fields. At least one of --phase and", Details: []string{"--next is required."}, Positionals: []HelpItem{{Name: "<slug>", Description: "immutable effort slug"}}, Options: []HelpItem{{Name: "--phase", Description: "<text> replacement phase metadata"}, {Name: "--next", Description: "<text> replacement next-action metadata"}, {Name: "--owner", Description: "<uuid> advisory activity owner"}, {Name: "--json", Description: "require the owner-scoped protocol-1 reply"}, {Name: "--preview", Description: "compute an owner-scoped read-only diff"}}}},
				},
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
		Name: "list", Summary: "Show the catalog and configured domain inventory",
		MaxPos: 1, Gating: Gated,
		Help: Help{Usage: []string{"awf list [<kind>]"}, Description: "Show catalog artifacts and configured domains without selection state.", Positionals: []HelpItem{{Name: "<kind>", Description: "artifact kind"}}},
	},
	{
		Name: "config", Summary: "Describe config keys and vars (live state inside a project)",
		MaxPos: 1, Gating: GatedInHandler,
		Help: Help{Usage: []string{"awf config [<key-or-var>]"}, Description: "Print the configuration reference: every config key, var, sidecar field, and", Details: []string{"data key with descriptions, defaults, and availability. Inside an awf project", "the output adds live state (current values and which catalog artifacts consume", "each var). Outside one, a static catalog-wide reference prints.", "With an argument, print just that entry (a config key path like", "audit.allowedScopes, a var name like gateCmd, a sidecar field like", "sidecar.dataDefaults, or a data key name)."}, Positionals: []HelpItem{{Name: "<key-or-var>", Description: "config key, var, sidecar field, or data key"}}},
	},
	{
		Name: "new", Summary: "Scaffold a new artifact: kind in {plan, topic, domain, pitfall, doc}",
		MaxPos: -1, Gating: GatedInHandler,
		Help: Help{
			Usage:       []string{"awf new <kind> <args>"},
			Description: "Scaffold a new artifact. The kind is plan, topic, domain, pitfall, or doc.",
			Positionals: []HelpItem{
				{Name: "<kind>", Description: "artifact kind"},
				{Name: "<args>", Description: "arguments required by the selected kind"},
			},
			Examples: []string{
				"awf new plan \"Some Plan Title\"",
				"awf new topic <domain> \"Some Topic Title\"",
				"awf new domain <name>",
				"awf new pitfall \"Some Durable Hazard\"",
				"awf new doc runbooks/api-v2 \"How to operate API v2\" --title \"API v2\"",
			},
		},
		Children: []Command{
			{
				Name: "plan", Summary: "Scaffold a new plan", MinPos: 1, MaxPos: -1, FullOnly: true,
				Help: Help{Usage: []string{"awf new plan <title>..."}, Description: "Scaffold a new plan under docs/plans, date-prefixed (no sequential number),", Details: []string{"from the rendered plans template with its date and title heading filled in."}, Positionals: []HelpItem{{Name: "<title>", Description: "human-readable artifact title"}}},
			},
			{
				Name: "topic", Summary: "Scaffold paired current-state topic inputs", MinPos: 2, MaxPos: -1, FullOnly: true,
				Help: Help{Usage: []string{"awf new topic <domain> <title>..."}, Description: "Scaffold paired topic metadata and authored current-state inputs without syncing.", Details: []string{"Edit the path placeholder and author reviewed claims manually."}, Positionals: []HelpItem{{Name: "<domain>", Description: "current-state domain identifier"}, {Name: "<title>", Description: "human-readable artifact title"}}},
			},
			{
				Name: "domain", Summary: "Create a configured domain", MinPos: 1, MaxPos: 1, FullOnly: true,
				Help: Help{Usage: []string{"awf new domain <name>"}, Description: "Add a domain and scaffold its current-state convention part."},
			},
			{
				Name: "doc", Summary: "Scaffold one project-local document", ValueFlags: []string{"--title"}, MinPos: 2, MaxPos: 2,
				Help: Help{Usage: []string{"awf new doc <name> <description> [--title <title>]"}, Description: "Declare, render, and report one project-local document. Without --title, derive it from the final kebab-case name segment.", Positionals: []HelpItem{{Name: "<name>", Description: "lowercase kebab-case path below docs"}, {Name: "<description>", Description: "one-line document description"}}, Options: []HelpItem{{Name: "--title", Description: "<title> explicit document title"}}},
			},
			{
				Name: "pitfall", Summary: "Scaffold one authored pitfall", MinPos: 1, MaxPos: 1,
				Help: Help{Usage: []string{"awf new pitfall <title>"}, Description: "Create one canonical source exclusively under .awf/docs/pitfalls without rendering.", Details: []string{"The title is one complete positional, so quote titles containing spaces. Duplicate titles and a selected-path race are refused."}, Positionals: []HelpItem{{Name: "<title>", Description: "complete human-readable pitfall title"}}},
			},
		},
	},
	{
		Name: "remove", Summary: "Remove a configured domain",
		MaxPos: -1, Gating: GatedInHandler,
		Help: Help{Usage: []string{"awf remove domain <name>"}, Description: "Remove a configured domain, prune its rendered output, and report authored files left orphaned."},
		Children: []Command{{Name: "domain", Summary: "Remove a configured domain", MinPos: 1, MaxPos: 1, FullOnly: true,
			Help: Help{Usage: []string{"awf remove domain <name>"}, Description: "Remove a configured domain."}}},
	},
	{
		Name: "upgrade", Summary: "Migrate the .awf/ config tree or recover an interrupted upgrade",
		BoolFlags: []string{"--recover"}, MaxPos: 0, Gating: Ungated,
		Help: Help{Usage: []string{"awf upgrade [--recover]"}, Description: "Migrate the .awf/ config tree to the current schema version, then sync.", Options: []HelpItem{{Name: "--recover", Description: "replay the current-state upgrade journal's recovery table"}}, Details: []string{"A current-state upgrade journal blocks ordinary commands until `awf upgrade --recover` rolls a precommit transaction back or cleans postcommit residue. The permanent lock remains the transaction commit point."}},
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
