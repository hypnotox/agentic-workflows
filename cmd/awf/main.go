// Command awf delivers fixed workflows and routes authored project knowledge.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/artifactfs"
	awfdocs "github.com/hypnotox/agentic-workflows/internal/docs"
	"github.com/hypnotox/agentic-workflows/internal/effortfs"
	"github.com/hypnotox/agentic-workflows/internal/projector"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "awf:", err)
		os.Exit(1)
	}
	os.Exit(run(root, os.Args, os.Stdout, os.Stderr))
}

func run(root string, args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 || args[1] == "help" || args[1] == "--help" || args[1] == "-h" {
		return runHelp(args, stdout, stderr)
	}

	switch args[1] {
	case "init":
		if helpRequested(args[2:]) {
			writeText(stdout, initHelp)
			return 0
		}
		if len(args) != 2 {
			return usage(stderr, "usage: awf init")
		}
		result, err := projector.Init(root)
		if err != nil {
			return failure(stderr, err)
		}
		printRenderResult(stdout, result)
		return 0
	case "render":
		if helpRequested(args[2:]) {
			writeText(stdout, renderHelp)
			return 0
		}
		if len(args) != 2 {
			return usage(stderr, "usage: awf render")
		}
		result, err := projector.Render(root)
		if err != nil {
			return failure(stderr, err)
		}
		printRenderResult(stdout, result)
		return 0
	case "check":
		if helpRequested(args[2:]) {
			writeText(stdout, checkHelp)
			return 0
		}
		if len(args) != 2 {
			return usage(stderr, "usage: awf check")
		}
		findings, err := projector.Check(root)
		if err != nil {
			return failure(stderr, err)
		}
		if len(findings) == 0 {
			fmt.Fprintln(stdout, "check: ok")
			return 0
		}
		fmt.Fprintln(stdout, "check: failed")
		for _, finding := range findings {
			fmt.Fprintf(stdout, "%s: %s\n", finding.Path, finding.Message)
		}
		return 1
	case "resolve":
		if helpRequested(args[2:]) {
			writeText(stdout, resolveHelp)
			return 0
		}
		for _, value := range args[2:] {
			if strings.HasPrefix(value, "-") {
				return usage(stderr, fmt.Sprintf("unknown resolve option %q; usage: awf resolve [<path>...]", value))
			}
		}
		coverage, err := projector.Resolve(root, args[2:])
		if err != nil {
			return failure(stderr, err)
		}
		printResolution(stdout, coverage)
		return 0
	case "docs":
		return runDocs(args[2:], stdout, stderr)
	case "new":
		return runNew(root, args[2:], stdout, stderr)
	case "effort":
		return runEffort(root, args[2:], stdout, stderr)
	case "version":
		if helpRequested(args[2:]) {
			writeText(stdout, versionHelp)
			return 0
		}
		if len(args) != 2 {
			return usage(stderr, "usage: awf version")
		}
		fmt.Fprintln(stdout, "version:", projector.Version)
		return 0
	default:
		return usage(stderr, fmt.Sprintf("unknown command %q; run `awf --help`", args[1]))
	}
}

func runHelp(args []string, stdout, stderr io.Writer) int {
	if len(args) == 2 || len(args) < 2 || args[1] == "--help" || args[1] == "-h" {
		writeText(stdout, globalHelp)
		return 0
	}
	if len(args) != 3 || args[1] != "help" {
		return usage(stderr, "usage: awf help [command]")
	}
	help, ok := commandHelp(args[2])
	if !ok {
		return usage(stderr, fmt.Sprintf("unknown command %q", args[2]))
	}
	writeText(stdout, help)
	return 0
}

func runDocs(args []string, stdout, stderr io.Writer) int {
	if helpRequested(args) {
		writeText(stdout, docsHelp)
		return 0
	}
	if len(args) > 1 {
		return usage(stderr, "usage: awf docs [integration|knowledge|agents|topics|effort|changes|adr|completion]")
	}
	name := ""
	if len(args) == 1 {
		name = args[0]
	}
	page, ok := awfdocs.Page(name)
	if !ok {
		return usage(stderr, fmt.Sprintf("unknown documentation page %q; expected integration, knowledge, agents, topics, effort, changes, adr, or completion", name))
	}
	if _, err := stdout.Write(page); err != nil {
		return failure(stderr, err)
	}
	if len(page) > 0 && page[len(page)-1] != '\n' {
		fmt.Fprintln(stdout)
	}
	return 0
}

func runNew(root string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || helpRequested(args) {
		writeText(stdout, newHelp)
		return 0
	}

	var (
		created string
		label   string
		err     error
	)
	switch args[0] {
	case "effort":
		if len(args) != 2 {
			return usage(stderr, "usage: awf new effort <slug>")
		}
		primary, rootErr := effortfs.Root(root)
		if rootErr != nil {
			return failure(stderr, rootErr)
		}
		label = "memory"
		created, err = effortfs.New(primary, args[1])
		created = effortDisplayPath(root, primary, created)
	case "intent":
		if len(args) != 2 {
			return usage(stderr, "usage: awf new intent <slug>")
		}
		label = "intent"
		created, err = artifactfs.NewIntent(root, args[1])
	case "spec":
		if len(args) != 2 {
			return usage(stderr, "usage: awf new spec <slug>")
		}
		label = "spec"
		created, err = artifactfs.NewSpec(root, args[1])
	case "plan":
		if len(args) != 2 {
			return usage(stderr, "usage: awf new plan <slug>")
		}
		label = "plan"
		created, err = artifactfs.NewPlan(root, args[1])
	case "adr":
		if len(args) != 2 {
			return usage(stderr, "usage: awf new adr <slug>")
		}
		label = "adr"
		created, err = artifactfs.NewADR(root, args[1])
	case "topic":
		if len(args) < 3 {
			return usage(stderr, "usage: awf new topic <id> <pattern>...")
		}
		label = "topic"
		created, err = artifactfs.NewTopic(root, args[1], args[2:])
	default:
		return usage(stderr, fmt.Sprintf("unknown new command %q; expected effort, intent, spec, plan, adr, or topic", args[0]))
	}
	if err != nil {
		return failure(stderr, err)
	}
	fmt.Fprintln(stdout, label+":", filepathSlash(created))
	return 0
}

func runEffort(root string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || helpRequested(args) {
		writeText(stdout, effortHelp)
		return 0
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return usage(stderr, "usage: awf effort list")
		}
	case "show", "finish":
		if len(args) != 2 {
			return usage(stderr, "usage: awf effort "+args[0]+" <slug>")
		}
	case "worktree":
		return runEffortWorktree(root, args[1:], stdout, stderr)
	default:
		return usage(stderr, fmt.Sprintf("unknown effort command %q; expected list, show, finish, or worktree", args[0]))
	}
	primary, err := effortfs.Root(root)
	if err != nil {
		return failure(stderr, err)
	}
	switch args[0] {
	case "list":
		slugs, err := effortfs.List(primary)
		if err != nil {
			return failure(stderr, err)
		}
		if len(slugs) == 0 {
			fmt.Fprintln(stdout, "none")
			return 0
		}
		for _, slug := range slugs {
			fmt.Fprintln(stdout, slug)
		}
		return 0
	case "show":
		path, body, err := effortfs.Show(primary, args[1])
		if err != nil {
			return failure(stderr, err)
		}
		fmt.Fprintln(stdout, "memory:", filepathSlash(effortDisplayPath(root, primary, path)))
		fmt.Fprintln(stdout)
		if _, err := stdout.Write(body); err != nil {
			return failure(stderr, err)
		}
		if len(body) > 0 && body[len(body)-1] != '\n' {
			fmt.Fprintln(stdout)
		}
		return 0
	default: // finish
		path, err := effortfs.Finish(primary, args[1])
		if err != nil {
			return failure(stderr, err)
		}
		fmt.Fprintln(stdout, "archive:", filepathSlash(effortDisplayPath(root, primary, path)))
		return 0
	}
}

func runEffortWorktree(root string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || helpRequested(args) || (args[0] == "add" && helpRequested(args[1:])) {
		writeText(stdout, worktreeHelp)
		return 0
	}
	if args[0] != "add" || (len(args) != 2 && (len(args) != 4 || args[2] != "--suffix")) {
		return usage(stderr, "usage: awf effort worktree add <effort-slug> [--suffix <suffix>]")
	}
	suffix := ""
	if len(args) == 4 {
		suffix = args[3]
		if suffix == "" {
			return usage(stderr, "worktree suffix cannot be empty")
		}
	}
	primary, err := effortfs.Root(root)
	if err != nil {
		return failure(stderr, err)
	}
	worktree, err := effortfs.AddWorktree(primary, args[1], suffix)
	if err != nil {
		return failure(stderr, err)
	}
	fmt.Fprintln(stdout, "worktree:", filepathSlash(worktree.Path))
	fmt.Fprintln(stdout, "branch:", worktree.Branch)
	fmt.Fprintln(stdout, "memory:", filepathSlash(worktree.MemoryPath))
	return 0
}

func effortDisplayPath(caller, primary, path string) string {
	if caller != primary {
		return filepath.Join(primary, path)
	}
	return path
}

func printResolution(stdout io.Writer, coverage projector.Coverage) {
	numbers := make(map[string]int)
	var references []projector.TopicMatch
	printMatches := func(matches []projector.TopicMatch) {
		if len(matches) == 0 {
			fmt.Fprintln(stdout, "none")
			return
		}
		for i, match := range matches {
			number := numbers[match.ID]
			if number == 0 {
				references = append(references, match)
				number = len(references)
				numbers[match.ID] = number
			}
			if i > 0 {
				fmt.Fprint(stdout, ", ")
			}
			fmt.Fprintf(stdout, "[%d]", number)
		}
		fmt.Fprintln(stdout)
	}

	fmt.Fprintln(stdout, "globals:")
	fmt.Fprint(stdout, "  ")
	printMatches(coverage.Globals)
	if len(coverage.Paths) > 0 {
		fmt.Fprintln(stdout, "\npaths:")
		for _, entry := range coverage.Paths {
			fmt.Fprintf(stdout, "  %q: ", entry.Path)
			printMatches(entry.Matches)
		}
	}
	fmt.Fprintln(stdout, "\nreferences:")
	if len(references) == 0 {
		fmt.Fprintln(stdout, "  none")
		return
	}
	for i, match := range references {
		fmt.Fprintf(stdout, "  [%d] %s — %s\n", i+1, match.ID, match.SourcePath)
	}
}

func printRenderResult(stdout io.Writer, result projector.RenderResult) {
	if len(result.Changed) == 0 {
		fmt.Fprintln(stdout, "render: up to date")
	} else {
		for _, path := range result.Changed {
			fmt.Fprintln(stdout, "rendered:", path)
		}
	}
	for _, path := range result.Unmanaged {
		fmt.Fprintln(stdout, "unmanaged AWF-marked file:", path)
	}
}

func helpRequested(args []string) bool {
	return len(args) == 1 && (args[0] == "--help" || args[0] == "-h")
}

func commandHelp(command string) (string, bool) {
	switch command {
	case "init":
		return initHelp, true
	case "render":
		return renderHelp, true
	case "check":
		return checkHelp, true
	case "resolve":
		return resolveHelp, true
	case "docs":
		return docsHelp, true
	case "new":
		return newHelp, true
	case "effort":
		return effortHelp, true
	case "version":
		return versionHelp, true
	default:
		return "", false
	}
}

func filepathSlash(path string) string {
	return strings.ReplaceAll(path, `\`, "/")
}

func writeText(writer io.Writer, text string) {
	_, _ = io.WriteString(writer, text)
}

func usage(stderr io.Writer, message string) int {
	fmt.Fprintln(stderr, "awf:", message)
	return 2
}

func failure(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, "awf:", err)
	return 1
}

const globalHelp = `AWF delivers fixed workflow skills and keeps lightweight local effort memory.

Usage:
  awf <command> [arguments]

Commands:
  init       install the fixed generated files
  render     render the fixed generated files
  check      check sources and generated files
  resolve    find global topics or topics for repository paths
  docs       read embedded adopter guides
  new        create effort memory or a tracked document
  effort     inspect effort memory or create an implementation worktree
  version    print the AWF version

Run ` + "`awf help <command>`" + ` for command details or ` + "`awf docs`" + ` for the guide.
`

const initHelp = `Usage: awf init

Install the same fixed files as render. Agent instructions remain author-owned; no project descriptor is created.
Read ` + "`awf docs integration`" + ` before adopting AWF in an existing repository.
`

const renderHelp = `Usage: awf render

Render the fixed generated files. Retired AWF-marked files are reported but never deleted.
See ` + "`awf docs integration`" + ` for ownership and update procedures.
`

const checkHelp = `Usage: awf check

Validate the docs/ knowledge bundle, topic selectors, and generated files. Unmanaged AWF-marked files fail the check.
See ` + "`awf docs integration`" + ` for gate and CI integration.
`

const resolveHelp = `Usage: awf resolve [<path>...]

Report explicit globals separately and non-global topic matches for every supplied lexical repository-relative path, in argument order, including duplicates.
Numbered references point to one deduplicated source list at the end. Numbers are assigned in first-use order and are local to this response.
Without paths, report globals only. Unmapped paths print none and succeed.
See ` + "`awf docs topics`" + ` for authoring, maintenance, and routing limits.
`

const docsHelp = `Usage: awf docs [integration|knowledge|agents|topics|effort|changes|adr|completion]

Print the embedded overview or one adopter guide to standard output.
`

const newHelp = `Usage: awf new <artifact> [arguments]

Commands:
  effort <slug>            create local effort memory
  intent <slug>            create docs/changes/<slug>/intent.md
  spec <slug>              create docs/changes/<slug>/spec.md
  plan <slug>              create docs/plans/<slug>.md
  adr <slug>               create docs/decisions/<slug>.md with pending/draft decision metadata
  topic <id> <pattern>...  create ` + projector.TopicsPath + `/<id>.md with supplied selectors

Creation never replaces an existing destination. Quote glob patterns so the shell does not expand them.
See ` + "`awf docs changes`" + ` for change documents, ` + "`awf docs effort`" + ` for continuity, and ` + "`awf docs topics`" + ` for topic authoring.
`

const effortHelp = `Usage: awf effort <command>

Commands:
  list           list active efforts
  show <slug>    show an effort's memory path and contents
  finish <slug>  move an effort into the local archive
  worktree add <effort-slug> [--suffix <suffix>]
                 create a default or suffixed implementation worktree

Create memory with ` + "`awf new effort <slug>`" + `. See ` + "`awf docs effort`" + ` for the complete workflow.
`

const worktreeHelp = `Usage: awf effort worktree add <effort-slug> [--suffix <suffix>]

Create .awf/worktrees/<effort-slug>/<default-or-suffix> on awf/<effort-slug>/<default-or-suffix> from the primary checkout's committed HEAD.
Requires an active effort. Suffixes use letters, numbers, hyphens, or underscores; default is reserved.
Existing destinations and conflicting branches are never replaced. Integration and cleanup remain native Git operations.
See ` + "`awf docs effort`" + ` for shared memory and checkout conventions.
`

const versionHelp = `Usage: awf version

Print the embedded AWF release version.
`
