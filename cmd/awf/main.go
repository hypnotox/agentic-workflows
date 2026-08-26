// Command awf renders standardised .claude skills, agents, and docs into a project from embedded templates plus a per-project .awf/ config tree.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/upgrade"
)

func main() { os.Exit(run(os.Args, os.Stdout, os.Stderr)) }

// gitCommandTimeout is the deadline every command boundary puts on the git work
// it starts. The value is the seam's, so awf and repoaudit cannot drift apart on
// it; the choice to apply it here is this boundary's.
const gitCommandTimeout = awfgit.CommandTimeout

var getwd = os.Getwd

var stdin io.Reader = os.Stdin

// isInteractive reports whether stdin is a terminal (so init should prompt).
var isInteractive = func() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// globalHelp renders the top-level `awf help` overview from each command's summary,
// so the overview and the per-command `awf <cmd> --help` texts share one source -
// the internal/clispec table (inv: cli-command-spec-single-source). A group's
// children are listed beneath it at a deeper indent, so no command is reachable
// only by knowing to ask its parent for help (inv: help-lists-group-children).
// The child indent is deeper than the parent's on purpose: indentation carries the
// relationship, and a child sharing a top-level name (`new topic` beside `topic`)
// therefore cannot be mistaken for the top-level entry.
func globalHelp() (presentation.Document, error) {
	introValue, err := presentation.Prose("render agentic-workflow tooling into a project from a committed .awf config tree")
	if err != nil {
		return presentation.Document{}, err
	}
	intro, err := presentation.NewField("awf", introValue)
	if err != nil {
		return presentation.Document{}, err
	}
	usage, err := presentation.NewList("usage", mustValues("awf <command> [flags]")...)
	if err != nil {
		return presentation.Document{}, err
	}
	commands, err := commandSections(clispec.Commands)
	if err != nil {
		return presentation.Document{}, err
	}
	related, err := presentation.NewList("related commands", mustValues("awf <command> --help")...)
	if err != nil {
		return presentation.Document{}, err
	}
	section, err := presentation.NewSection("help", usage, commands, related)
	if err != nil {
		return presentation.Document{}, err
	}
	return presentation.NewDocument(intro, section)
}

// commandSections preserves the command table's tree. Each hierarchy owns one
// record group containing its direct commands. Top-level groups get a Section;
// nested groups remain record groups under that Section, keeping the closed
// presentation grammar's three Section levels while retaining command depth.
func commandSections(specs []clispec.Command) (presentation.Section, error) {
	records, err := commandRecords(specs)
	if err != nil {
		return presentation.Section{}, err
	}
	group, err := presentation.NewRecordGroup("commands", []string{"command", "summary"}, records...)
	if err != nil {
		return presentation.Section{}, err
	}
	nodes := []presentation.Node{group}
	for _, spec := range specs {
		if len(spec.Children) == 0 {
			continue
		}
		section, err := commandSection(spec)
		if err != nil {
			return presentation.Section{}, err
		}
		nodes = append(nodes, section)
	}
	return presentation.NewSection("commands", nodes...)
}

func commandSection(spec clispec.Command) (presentation.Section, error) {
	records, err := commandRecords(spec.Children)
	if err != nil {
		return presentation.Section{}, err
	}
	group, err := presentation.NewRecordGroup("commands", []string{"command", "summary"}, records...)
	if err != nil {
		return presentation.Section{}, err
	}
	nodes := []presentation.Node{group}
	for _, child := range spec.Children {
		if len(child.Children) == 0 {
			continue
		}
		children, err := commandRecords(child.Children)
		if err != nil {
			return presentation.Section{}, err
		}
		nested, err := presentation.NewRecordGroup(child.Name, []string{"command", "summary"}, children...)
		if err != nil {
			return presentation.Section{}, err
		}
		nodes = append(nodes, nested)
	}
	return presentation.NewSection(spec.Name, nodes...)
}

func commandRecords(specs []clispec.Command) ([]presentation.Record, error) {
	records := make([]presentation.Record, 0, len(specs))
	for _, spec := range specs {
		record, err := commandRecord(spec)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func commandRecord(spec clispec.Command) (presentation.Record, error) {
	name, err := presentation.Literal(spec.Name)
	if err != nil {
		return presentation.Record{}, err
	}
	summary, err := presentation.Prose(spec.Summary)
	if err != nil {
		return presentation.Record{}, err
	}
	return presentation.NewRecord(name, summary)
}

func mustValues(text ...string) []presentation.Value {
	values := make([]presentation.Value, len(text))
	for i, value := range text {
		values[i], _ = presentation.Literal(value)
	}
	return values
}

func renderHelp(dst io.Writer, spec clispec.Command, path string) error {
	document, err := spec.Help.Document("awf "+path, spec.Summary)
	if err != nil {
		return err
	}
	return presentation.Render(dst, document)
}

// run is the CLI driver: it resolves args to a clispec command, prints help,
// parses the arguments once, applies the gating classification, and dispatches
// to the command's handler - a single parse-once path shared by every command.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		return dispatchFailure(stdout, stderr, &usageErr{fmt.Sprintf("usage: %s [args]; run `awf help` for command details", clispec.UsageLine())})
	}
	if a := args[1]; a == "help" || a == "--help" || a == "-h" {
		if a == "help" && len(args) >= 3 {
			spec, ok := clispec.Lookup(args[2])
			if !ok {
				return dispatchFailure(stdout, stderr, &usageErr{fmt.Sprintf("unknown command %q", args[2])})
			}
			for _, name := range args[3:] {
				child, found := spec.Child(name)
				if !found {
					return dispatchFailure(stdout, stderr, &usageErr{fmt.Sprintf("unknown command %q", name)})
				}
				spec = child
			}
			if err := renderHelp(stdout, spec, strings.Join(args[2:], " ")); err != nil {
				return dispatchFailure(stdout, stderr, err)
			}
			return 0
		}
		document, err := globalHelp()
		if err != nil {
			return dispatchFailure(stdout, stderr, err)
		}
		if err := presentation.Render(stdout, document); err != nil {
			return dispatchFailure(stdout, stderr, err)
		}
		return 0
	}
	cwd, err := getwd()
	if err != nil {
		return dispatchFailure(stdout, stderr, err)
	}
	cmd, top, sub, rest, ok := resolve(args[1:])
	if !ok {
		return dispatchFailure(stdout, stderr, &usageErr{fmt.Sprintf("unknown command %q", args[1])})
	}
	if wantsHelp(rest) { // `awf <cmd> --help`/`-h` - intercept before parseArgs rejects it
		if err := renderHelp(stdout, cmd, strings.TrimSpace(top.Name+" "+sub)); err != nil {
			return dispatchFailure(stdout, stderr, err)
		}
		return 0
	}
	inv, err := parseArgs(cmd, rest)
	if err != nil {
		if top.Name == "effort" && (sub == "memory" || strings.HasPrefix(sub, "memory ")) {
			err = &usageErr{boundedMemoryCommandError(err).Error()}
		}
		return dispatchFailure(stdout, stderr, err) // parseArgs only returns usageErr → exit 2
	}
	// Process guards own interruption before dispatch so independently invocable
	// handlers receive only operational project state. Capability interpretation
	// follows live-source admission and reads the same working or staged universe
	// as the command rather than allowing config parsing to precede schema refusal.
	guardCtx, cancel := newGitCommandContext()
	if err := guardProjectState(guardCtx, cwd, cmd, top, sub, inv); err != nil {
		cancel()
		return dispatchFailure(stdout, stderr, err)
	}
	if cmd.FullOnly || top.FullOnly {
		if err := requireCommandCapability(guardCtx, cwd, top, sub, inv); err != nil {
			cancel()
			return dispatchFailure(stdout, stderr, err)
		}
	}
	cancel()
	// The driver gates every Gated command before its handler; config/context/topic/new
	// self-gate in-handler after their static-fallback / name-validation checks.
	// Group children inherit the top-level command's classification.
	if top.Gating == clispec.Gated {
		gateFn := gate
		if top.Name == "check" && (sub == "staged" || strings.HasPrefix(sub, "staged ")) {
			gateFn = gateStaged
		}
		gateCtx, cancel := newGitCommandContext()
		if err := gateFn(gateCtx, cwd); err != nil {
			cancel()
			if !selectsStagedDrift(top, sub) || !errors.Is(err, errNoStagedLock) {
				return dispatchFailure(stdout, stderr, err)
			}
		}
		cancel()
	}
	// The registry key is the top-level command name even when resolve returned a
	// child spec - the child drives parse/help, the group's handler drives
	// dispatch via sub.
	handlerCtx, cancel := newGitCommandContext()
	defer cancel()
	result := handlers[top.Name](&cmdCtx{ctx: handlerCtx, root: cwd, sub: sub, inv: inv, stdout: stdout, stdin: stdin})
	return completeHandlerResult(stdout, stderr, result)
}

// completeHandlerResult ends a command operation only after the command has
// completed its selected output. In particular, typed errors are transformed
// into their model-owned diagnostics and written centrally before the mutation
// lease is released.
func completeHandlerResult(stdout, stderr io.Writer, result handlerResult) int {
	code := 0
	if result.err != nil {
		// A handler declares a complete report explicitly. It has already been
		// rendered to stdout and owns its failing exit; diagnostics were not
		// produced as reports and are rendered once to stderr below.
		if result.producedReport {
			code = 1
		} else {
			code = dispatchFailure(stdout, stderr, result.err)
		}
	}
	if result.release != nil {
		if err := result.release(); err != nil {
			// Output has completed, so a release error cannot be rendered without
			// a second report. Preserve the failing process result instead.
			return 1
		}
	}
	return code
}

// newGitCommandContext gives each process-command stage its own full timeout.
// Guard and gate work must not consume the handler's hang-prevention ceiling.
// It is the sole production context root.
func newGitCommandContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), gitCommandTimeout)
}

// guardProjectState enforces the current-state upgrade command-state matrix.
// Exemption is the resolved command's StateExempt property, not a name list, so
// a group child carries it independently of its parent. Only `check staged
// commit` remains exempt while bare `check` and both repo scan children are not,
// which keeps the commit-msg hook working during a committed journal (ADR-0159
// Decision 5). The read-only init descriptor query bypasses it too; outside an
// adopted tree it is a no-op so config/context/topic keep their static fallback.
// Inside a tree:
//   - a valid journal permits only `awf upgrade --recover`; every other command
//     refuses with a run-recover diagnostic;
//   - a malformed journal refuses every mode, recovery included, with the
//     Git-restoration guidance the journal loader carries;
//   - without a journal, complete permanent authority proceeds and a recovery
//     request refuses because there is no journal;
//   - a corrupt lock with no journal defers to the existing ADR-0076 refusal.
func selectsStagedDrift(top clispec.Command, sub string) bool {
	return top.Name == "check" && (sub == "staged" || sub == "staged drift")
}

func selectsStagedProjectUniverse(top clispec.Command, sub string, inv invocation) bool {
	return top.Name == "check" && (sub == "staged" || strings.HasPrefix(sub, "staged ")) ||
		top.Name == "context" && inv.bools["--staged"]
}

func requireCommandCapability(ctx context.Context, root string, top clispec.Command, sub string, inv invocation) error {
	var cfg *config.Config
	var err error
	if selectsStagedProjectUniverse(top, sub, inv) {
		tree, treeErr := stagedTree(ctx, root)
		if treeErr != nil {
			return treeErr
		}
		file, ok := tree.Lookup(config.DirName + "/config.yaml")
		if !ok {
			return nil
		}
		cfg, err = config.Parse(config.RootDir(root), file.Bytes)
	} else {
		cfg, err = config.Load(config.RootDir(root))
		if errors.Is(err, os.ErrNotExist) {
			present, presentErr := migrate.ProjectPresent(root)
			if presentErr != nil {
				return presentErr
			}
			if !present {
				return nil
			}
		}
	}
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	return project.RequireCapability(cfg.Profile, strings.TrimSpace(top.Name+" "+sub), true)
}

func validateCurrentAuthority(found, currentConfig, currentLock bool) error {
	if currentConfig != currentLock {
		return &manifest.PartialAuthorityError{Config: currentConfig, Lock: currentLock}
	}
	if !found {
		return &manifest.PartialAuthorityError{Config: currentConfig, Lock: false}
	}
	return nil
}

func guardProjectState(ctx context.Context, root string, cmd clispec.Command, top clispec.Command, sub string, inv invocation) error {
	if cmd.StateExempt {
		return nil
	}
	if top.Name == "init" && inv.bools["--describe"] {
		return nil
	}
	staged := selectsStagedProjectUniverse(top, sub, inv)
	present, journal, journalFound, lock, found, currentConfig, currentLock, loadErr, err := projectGuardState(ctx, root, staged)
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	isUpgrade := top.Name == "upgrade"
	isRecover := isUpgrade && inv.bools["--recover"]
	if journalFound {
		if staged {
			_, err = upgrade.ParseJournal(journal)
		} else {
			_, err = upgrade.LoadJournal(root)
		}
		if err != nil {
			return err // malformed journal: refuse every mode, recovery included
		}
		if isRecover {
			return nil
		}
		return errors.New("a current-state upgrade journal is present; run `awf upgrade --recover` before any other command")
	}
	// No journal: admit complete live authority before permanent authority
	// interpretation. Retired layouts and below-floor locks cannot reach
	// AuthorityState, and an incomplete current control pair is partial authority.
	if loadErr != nil {
		if errors.Is(loadErr, manifest.ErrUnsupportedLiveSource) {
			return presentLiveSourceRefusal(loadErr)
		}
		return fmt.Errorf("invalid authority: restore .awf/awf.lock from version control: %w", loadErr)
	}
	if currentConfig || currentLock {
		if err := validateCurrentAuthority(found, currentConfig, currentLock); err != nil {
			return presentUpgradeRefusal(err)
		}
	} else {
		if staged {
			return fmt.Errorf("retired project authority is below live floor %d; restore a supported .awf control pair or use a release that supports the retired layout", migrate.LiveSchemaFloor)
		}
		if _, err := migrate.CheckLive(root); err != nil {
			return presentGateRefusal(err)
		}
		return fmt.Errorf("retired project layout is unsupported at live floor %d; restore a supported .awf control pair", migrate.LiveSchemaFloor)
	}
	if _, err := lock.AuthorityState(); err != nil {
		return fmt.Errorf("invalid authority: restore .awf/awf.lock from version control: %w", err)
	}
	if isRecover {
		return errors.New("no current-state upgrade journal to recover")
	}
	return nil
}

// projectGuardState captures the presence, journal, and lock used by the
// command-state guard. A staged check derives all three from the index snapshot;
// every other command derives all three from the working project.
func projectGuardState(ctx context.Context, root string, staged bool) (present bool, journal []byte, journalFound bool, lock *manifest.Lock, lockFound, currentConfig, currentLock bool, loadErr, err error) {
	if !staged {
		present, err = migrate.ProjectPresent(root)
		if err != nil {
			return false, nil, false, nil, false, false, false, nil, err
		}
		if journalFound, err = upgrade.JournalPresent(root); err != nil {
			return false, nil, false, nil, false, false, false, nil, err
		}
		_, configErr := os.Stat(config.ConfigPath(root))
		if configErr != nil && !errors.Is(configErr, os.ErrNotExist) {
			return false, nil, false, nil, false, false, false, nil, fmt.Errorf("stat .awf/config.yaml: %w", configErr)
		}
		currentConfig = configErr == nil
		_, lockErr := os.Stat(config.LockPath(root))
		if lockErr != nil && !errors.Is(lockErr, os.ErrNotExist) {
			return false, nil, false, nil, false, false, false, nil, fmt.Errorf("stat .awf/awf.lock: %w", lockErr)
		}
		currentLock = lockErr == nil
		lock, lockFound, loadErr = manifest.LoadLiveOptional(config.LockPath(root), migrate.LiveSchemaFloor, migrate.Current())
		return
	}
	tree, err := stagedTree(ctx, root)
	if err != nil {
		return false, nil, false, nil, false, false, false, nil, err
	}
	present = migrate.ProjectPresentFromFiles(func(path string) bool {
		_, ok := tree.Lookup(path)
		return ok
	})
	_, currentConfig = tree.Lookup(config.DirName + "/config.yaml")
	_, currentLock = tree.Lookup(config.DirName + "/awf.lock")
	if file, ok := tree.Lookup(config.DirName + "/current-state-upgrade.journal"); ok {
		journal, journalFound = file.Bytes, true
	}
	if file, ok := tree.Lookup(config.DirName + "/awf.lock"); ok {
		lockFound = true
		lock, loadErr = manifest.ParseLive(file.Bytes, migrate.LiveSchemaFloor, migrate.Current())
		if loadErr != nil {
			loadErr = fmt.Errorf("parse staged lock: %w", loadErr)
		}
	}
	return
}

// commandStream selects the only destination a command outcome may write.
type commandStream uint8

const (
	commandStdout commandStream = iota
	commandStderr
)

// commandOutcome is the command presentation boundary. A produced report has
// already been completely assembled and goes to stdout even when it exits
// unsuccessfully; diagnostics are failures to produce a report and go once to
// stderr. Business errors stay attached for identity-aware callers.
type commandOutcome struct {
	document presentation.Document
	stream   commandStream
	exit     int
	err      error
}

type diagnosticError interface {
	Diagnostic() (presentation.Diagnostic, error)
}

func diagnosticOutcome(err error) commandOutcome {
	var typed diagnosticError
	if errors.As(err, &typed) {
		diagnostic, diagnosticErr := typed.Diagnostic()
		if diagnosticErr != nil {
			return genericDiagnosticOutcome(diagnosticErr, 1)
		}
		document, documentErr := diagnostic.Document()
		if documentErr != nil {
			return genericDiagnosticOutcome(documentErr, 1)
		}
		return commandOutcome{document: document, stream: commandStderr, exit: 1, err: err}
	}
	exit := 1
	var usage *usageErr
	if errors.As(err, &usage) {
		exit = 2
	}
	return genericDiagnosticOutcome(err, exit)
}

// genericDiagnosticOutcome builds the bounded fallback used when a typed
// diagnostic cannot be mapped. Prose normalizes arbitrary error text and the
// fixed label guarantees this one-field document is valid, without re-entering
// the typed mapper that just failed.
func genericDiagnosticOutcome(err error, exit int) commandOutcome {
	condition, _ := presentation.Prose("awf: " + err.Error())
	field, _ := presentation.NewField("condition", condition)
	document, _ := presentation.NewDocument(field)
	return commandOutcome{document: document, stream: commandStderr, exit: exit, err: err}
}

// writeRendererFailure is the sole terminal fallback after presentation
// rendering fails. It deliberately is not a presentation renderer or a
// successful-output bypass: it reports that the renderer could not produce a
// document at all.
func writeRendererFailure(stderr io.Writer, cause error) {
	text := strings.Join(strings.FieldsFunc(cause.Error(), unicode.IsSpace), " ")
	if text == "" {
		text = "renderer failed"
	}
	_, _ = io.WriteString(stderr, "awf: "+text+"\n")
}

// writeStatus presents a complete ordinary scalar result. Callers own the
// semantic status text; presentation owns validation and rendering.
func writeStatus(stdout io.Writer, status string) error {
	value, err := presentation.Prose(status)
	if err != nil {
		return err
	}
	field, err := presentation.NewField("status", value)
	if err != nil {
		return err
	}
	document, err := presentation.NewDocument(field)
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}

func writeOutcome(stdout, stderr io.Writer, outcome commandOutcome) int {
	dst := stdout
	if outcome.stream == commandStderr {
		dst = stderr
	}
	if err := presentation.Render(dst, outcome.document); err != nil {
		writeRendererFailure(stderr, err)
		return 1
	}
	return outcome.exit
}

func dispatchFailure(stdout, stderr io.Writer, err error) int {
	return writeOutcome(stdout, stderr, diagnosticOutcome(err))
}

// usageErr marks a CLI-misuse error (unknown flag, bad arity, unknown command),
// which the central handler maps to exit code 2 rather than the failure code 1.
type usageErr struct{ msg string }

func (e *usageErr) Error() string { return e.msg }
