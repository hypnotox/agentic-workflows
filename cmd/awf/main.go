// Command awf renders standardised .claude skills, review agents, and docs into a project from embedded templates plus a per-project .awf/ config tree.
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
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/initspec"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/templates"
	"golang.org/x/mod/semver"
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
// commandOrder is the display order for `awf help`; every entry is a key in argSpecs.
var commandOrder = []string{
	"init", "sync", "check", "invariants", "audit", "commit-gate",
	"list", "new", "add", "remove", "upgrade", "uninstall", "changelog", "version",
}

// globalHelp renders the top-level `awf help` overview from each command's summary,
// so the overview and the per-command `awf <cmd> --help` texts share one source.
func globalHelp() string {
	var b strings.Builder
	b.WriteString("awf — render agentic-workflow tooling into a project from a committed .awf/ config tree\n\n")
	b.WriteString("Usage: awf <command> [flags]\n\n")
	b.WriteString("Commands:\n")
	for _, name := range commandOrder {
		fmt.Fprintf(&b, "  %-12s %s\n", name, argSpecs[name].summary)
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
		fmt.Fprintln(stderr, "usage: awf <init|sync|check|invariants|audit|commit-gate|list|new|add|remove|upgrade|uninstall|changelog|version> [args]")
		fmt.Fprintln(stderr, "run `awf help` for command details")
		return 2
	}
	if a := args[1]; a == "help" || a == "--help" || a == "-h" {
		fmt.Fprint(stdout, globalHelp())
		return 0
	}
	cwd, err := getwd()
	if err != nil {
		fmt.Fprintln(stderr, "awf:", err)
		return 1
	}
	if spec, ok := argSpecs[args[1]]; ok {
		if hasHelpFlag(args[2:]) { // `awf <cmd> --help`/`-h` — intercept before checkArgs rejects it
			fmt.Fprint(stdout, spec.help)
			return 0
		}
		if err := checkArgs(args[1], args[2:], spec.boolFlags, spec.valueFlags, spec.minPos, spec.maxPos); err != nil {
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
	case "new":
		if len(args) < 4 {
			cmdErr = &usageErr{"usage: awf new <kind> <title>"}
		} else {
			cmdErr = runNew(cwd, args[2], args[3:], stdout)
		}
	case "add":
		switch {
		case len(args) == 4:
			cmdErr = runAdd(cwd, args[2], args[3], stdout)
		case len(args) == 3 && args[2] == "bootstrap": // nameless bootstrap form (ADR-0040)
			cmdErr = runAdd(cwd, "bootstrap", "", stdout)
		case len(args) == 3:
			cmdErr = &usageErr{fmt.Sprintf("awf add requires a kind: awf add <kind> <name> (e.g. awf add skill %s)", args[2])}
		default:
			cmdErr = &usageErr{"usage: awf add <kind> <name>"}
		}
	case "remove":
		switch {
		case len(args) == 4:
			cmdErr = runRemove(cwd, args[2], args[3], stdout)
		case len(args) == 3 && args[2] == "bootstrap": // nameless bootstrap form (ADR-0040)
			cmdErr = runRemove(cwd, "bootstrap", "", stdout)
		default:
			cmdErr = &usageErr{"usage: awf remove <kind> <name>"}
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

// argSpec declares a subcommand's accepted flags and positional bounds. boolFlags
// take no value; valueFlags consume the following token; maxPos < 0 is unbounded
// (new/add/remove refine their arity in the switch to keep their specific messages).
type argSpec struct {
	boolFlags, valueFlags []string
	minPos, maxPos        int
	summary               string // one-line description for `awf help`
	help                  string // full `awf <cmd> --help` text (usage + description + flags)
}

var argSpecs = map[string]argSpec{
	"init": {
		boolFlags: []string{"--force", "--describe"}, valueFlags: []string{"--set", "--answers"}, maxPos: 0,
		summary: "Scaffold .awf/ and render the workflow-core set",
		help: `Usage: awf init [flags]

Scaffold a .awf/ config tree and render the workflow-core set into the project.

Flags:
  --force        overwrite colliding files, backing each up to <path>.awf-bak
  --describe     print the fillable value descriptors as JSON and exit
  --set k=v      set a value non-interactively (repeatable)
  --answers FILE read values from a JSON/YAML answers file
`,
	},
	"sync": {
		maxPos: 0, summary: "Re-render after a template or config change",
		help: `Usage: awf sync

Re-render every enabled target after a template or config change and update .awf/awf.lock.
`,
	},
	"check": {
		maxPos: 0, summary: "Fail on stale or hand-edited rendered output",
		help: `Usage: awf check

Re-render in memory and fail if any rendered file is stale or hand-edited (drift).
`,
	},
	"invariants": {
		maxPos: 0, summary: "Report Implemented-ADR invariant slugs lacking a backing comment",
		help: `Usage: awf invariants

Report each Implemented-ADR ` + "`inv:`" + ` slug lacking a backing ` + "`<marker> invariant:`" + ` comment.
`,
	},
	"audit": {
		valueFlags: []string{"--base"}, maxPos: 0,
		summary: "Report workflow-conformance findings over the branch (advisory)",
		help: `Usage: awf audit [--base <ref>]

Report advisory workflow-conformance findings over the branch's commits; never gates.

Flags:
  --base <ref>   compare against <ref> instead of the configured base branch
`,
	},
	"commit-gate": {
		maxPos: 1, summary: "Validate one commit message (Conventional Commits), blocking",
		help: `Usage: awf commit-gate [FILE]

Validate one commit message against the Conventional Commits rules (type, scope,
72-char subject) and exit non-zero on a violation — the commit-side analog of the
gate. Reads FILE (the path a commit-msg hook passes as $1) or stdin; cleans the
message git-style and exempts merge/autosquash subjects. awf installs no hook —
wire this into your own commit-msg hook.
`,
	},
	"list": {
		maxPos: 1, summary: "Show targets and their per-project state (all kinds, or one)",
		help: `Usage: awf list [<kind>]

Show targets and their per-project enabled state, for all kinds or one (skill|agent|doc|domain).
`,
	},
	"new": {
		maxPos: -1, summary: "Scaffold a new templated artifact — kind ∈ {adr}",
		help: `Usage: awf new <kind> <title>

Scaffold a new templated artifact. <kind> is adr.

Example: awf new adr "Some Decision Title"
`,
	},
	"add": {
		maxPos: -1, summary: "Enable a target — kind ∈ {skill, agent, doc, domain, bootstrap}",
		help: `Usage: awf add <kind> <name>

Enable a target. <kind> is skill, agent, doc, domain, or bootstrap.
`,
	},
	"remove": {
		maxPos: -1, summary: "Disable a target (a freeform domain, or a catalog target)",
		help: `Usage: awf remove <kind> <name>

Disable a target — a catalog skill/agent/doc, a freeform domain, or the bootstrap.
`,
	},
	"upgrade": {
		maxPos: 0, summary: "Migrate the .awf/ config tree to the current schema",
		help: `Usage: awf upgrade

Migrate the .awf/ config tree to the current schema version.
`,
	},
	"uninstall": {
		maxPos: 0, summary: "Remove awf's generated files (keeps .awf/)",
		help: `Usage: awf uninstall

Remove every awf-generated file recorded in the lock (keeps your authored .awf/ config).
`,
	},
	"changelog": {
		valueFlags: []string{"--version", "--since", "--range"}, maxPos: 0,
		summary: "Print the embedded changelog, or one version/range of it",
		help: `Usage: awf changelog [--version <v> | --since <v> | --range <from>..<to>]

Print the embedded awf changelog. With no flags, print the whole file. The three
flags are mutually exclusive.

Flags:
  --version <v>          print only version v's entry
  --since <v>            print every version released after v (exclusive)
  --range <from>..<to>   print every version in [from, to] (inclusive both ends)
`,
	},
	"version": {
		maxPos: 0, summary: "Print the awf version",
		help: `Usage: awf version

Print the awf version.
`,
	},
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

func runInit(root string, force, describe bool, sets []string, answersFile string, stdout io.Writer) error {
	cat, err := catalog.Load(templates.FS)
	if err != nil { // coverage-ignore: catalog.Load over the embedded FS cannot fail at runtime
		return err
	}
	descs := initspec.CatalogVars(cat)
	if describe {
		out, err := initspec.Describe(descs)
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
	vars, inv, trim, err := initspec.Resolve(descs, answers, stdin, stdout, isInteractive())
	if err != nil {
		return err
	}

	cfgPath := config.ConfigPath(root)
	scaffolded := false
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil { // coverage-ignore: entering this block needs cfgPath absent, which precludes a parent collision making MkdirAll fail
			return err
		}
		scaffold, err := project.ScaffoldConfig(filepath.Base(root), vars, inv, trim)
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
	if len(collisions) > 0 && !force {
		if scaffolded {
			_ = os.Remove(cfgPath)               // remove the config we scaffolded
			_ = os.Remove(filepath.Dir(cfgPath)) // remove .awf only if now empty
		}
		return fmt.Errorf("awf init: refusing to overwrite existing files (use --force):\n  %s",
			strings.Join(collisions, "\n  "))
	}
	// Under --force, the chained runSync backs up every foreign file via the shared
	// BackupFile mechanism (ADR-0035) — one backup path for init and sync alike.
	if err := runSync(root, stdout); err != nil {
		return err
	}
	return nil
}

// normalizeSemver returns s in the single-leading-v form x/mod/semver requires.
// awfVersion() already returns the v-form for `go install` builds, so a naive
// prefix would yield "vv0.4.0" and fail semver.IsValid; trimming any existing v
// first makes the normalization idempotent (ADR-0039 Decision 3).
func normalizeSemver(s string) (string, bool) {
	v := "v" + strings.TrimPrefix(s, "v")
	if !semver.IsValid(v) {
		return "", false
	}
	return v, true
}

// gate refuses to operate against a config the running binary cannot correctly
// interpret. It runs before project.Open. On the schema axis: "gate" (config
// behind binary) → "run awf upgrade"; "ahead" (config ahead of binary) → "update
// your pinned awf" (ADR-0039); "autobump" proceeds and the subsequent sync stamps
// the current schema. On the release-version axis: after the schema check it loads
// .awf/awf.lock and compares lock.AWFVersion vs awfVersion() — a lock semver-newer
// than the binary (binary behind) errors; a binary at-or-ahead is the permitted
// pre-upgrade state. The version sub-check is skipped (never errors) on an absent,
// unparseable, empty, or non-normalizable version, mirroring Generation's no-lock
// tolerance.
// invariant: version-compat-gate
func gate(root string) error {
	switch migrate.GateState(root) {
	case "gate":
		return fmt.Errorf("config schema is behind (generation %d < %d); run awf upgrade",
			migrate.Generation(root), migrate.Current())
	case "ahead":
		return fmt.Errorf("awf %s is behind this project's config (schema generation %d > %d); update your pinned awf",
			awfVersion(), migrate.Generation(root), migrate.Current())
	}
	lockV, binV, ok := lockVsBinary(root)
	if !ok {
		return nil // version sub-check not computable; schema check already applied
	}
	if semver.Compare(lockV, binV) > 0 {
		return fmt.Errorf("awf %s is behind this project (rendered by %s); update your pinned awf",
			awfVersion(), strings.TrimPrefix(lockV, "v"))
	}
	return nil
}

// lockVsBinary returns the normalized lock awfVersion and binary version for the
// release-version sub-check, with ok=false whenever the comparison cannot be
// computed (no/unloadable lock, empty AWFVersion, or a version that fails semver
// normalization) so the caller skips rather than errors (ADR-0039 Decision 5).
func lockVsBinary(root string) (lockV, binV string, ok bool) {
	l, err := manifest.Load(config.LockPath(root))
	if err != nil || l.AWFVersion == "" {
		return "", "", false
	}
	lockV, lok := normalizeSemver(l.AWFVersion)
	binV, bok := normalizeSemver(awfVersion())
	if !lok || !bok {
		return "", "", false
	}
	return lockV, binV, true
}

func runSync(root string, stdout io.Writer) error {
	if err := gate(root); err != nil {
		return err
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	backups, err := p.SyncReport()
	if err != nil {
		return err
	}
	for _, b := range backups {
		fmt.Fprintf(stdout, "backed up %s → %s\n", b.Path, b.Bak)
		if b.Index {
			fmt.Fprintf(stdout, "  note: awf now generates %s; retire any external generator for it\n", b.Path)
		}
	}
	fmt.Fprintln(stdout, "awf sync: done")
	return nil
}
