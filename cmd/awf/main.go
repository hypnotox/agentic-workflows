// Command awf projects a small .awf source tree into agent guidance.
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	awfdocs "github.com/hypnotox/agentic-workflows/docs"
	"github.com/hypnotox/agentic-workflows/internal/artifactfs"
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
		if len(args) >= 3 && args[2] == "--coverage" {
			if len(args) == 3 {
				return usage(stderr, "usage: awf resolve --coverage <path>...")
			}
			coverage, err := projector.ResolveCoverage(root, args[3:])
			if err != nil {
				return failure(stderr, err)
			}
			printCoverage(stdout, coverage)
			return 0
		}
		matches, err := projector.Resolve(root, args[2:])
		if err != nil {
			return failure(stderr, err)
		}
		if len(matches) == 0 {
			fmt.Fprintln(stdout, "none")
			return 0
		}
		for _, match := range matches {
			fmt.Fprintf(stdout, "%s\t%s\n", match.ID, match.SourcePath)
		}
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
		return usage(stderr, "usage: awf docs [integration|topics|effort]")
	}
	name := ""
	if len(args) == 1 {
		name = args[0]
	}
	page, ok := awfdocs.Page(name)
	if !ok {
		return usage(stderr, fmt.Sprintf("unknown documentation page %q; expected integration, topics, or effort", name))
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
		label = "memory"
		created, err = effortfs.New(root, args[1])
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
		return usage(stderr, fmt.Sprintf("unknown new command %q; expected effort, plan, adr, or topic", args[0]))
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
		slugs, err := effortfs.List(root)
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
		if len(args) != 2 {
			return usage(stderr, "usage: awf effort show <slug>")
		}
		path, body, err := effortfs.Show(root, args[1])
		if err != nil {
			return failure(stderr, err)
		}
		fmt.Fprintln(stdout, "memory:", filepathSlash(path))
		fmt.Fprintln(stdout)
		if _, err := stdout.Write(body); err != nil {
			return failure(stderr, err)
		}
		if len(body) > 0 && body[len(body)-1] != '\n' {
			fmt.Fprintln(stdout)
		}
		return 0
	case "finish":
		if len(args) != 2 {
			return usage(stderr, "usage: awf effort finish <slug>")
		}
		path, err := effortfs.Finish(root, args[1])
		if err != nil {
			return failure(stderr, err)
		}
		fmt.Fprintln(stdout, "archive:", filepathSlash(path))
		return 0
	default:
		return usage(stderr, fmt.Sprintf("unknown effort command %q; expected list, show, or finish", args[0]))
	}
}

func printCoverage(stdout io.Writer, coverage projector.Coverage) {
	fmt.Fprintln(stdout, "globals:")
	if len(coverage.Globals) == 0 {
		fmt.Fprintln(stdout, "  none")
	} else {
		for _, match := range coverage.Globals {
			fmt.Fprintf(stdout, "  %s\t%s\n", match.ID, match.SourcePath)
		}
	}
	for _, entry := range coverage.Paths {
		fmt.Fprintf(stdout, "path: %s\n", strconv.Quote(entry.Path))
		if len(entry.Matches) == 0 {
			fmt.Fprintln(stdout, "  none")
			continue
		}
		for _, match := range entry.Matches {
			fmt.Fprintf(stdout, "  %s\t%s\n", match.ID, match.SourcePath)
		}
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

const globalHelp = `AWF projects agent guidance and keeps lightweight local effort memory.

Usage:
  awf <command> [arguments]

Commands:
  init       create the minimal AWF sources and projection
  render     render the fixed generated files
  check      check sources and generated files
  resolve    find global topics or topics for repository paths
  docs       read embedded adopter guides
  new        create an effort, plan, ADR, or topic
  effort     inspect or finish local effort memory
  version    print the AWF version

Run ` + "`awf help <command>`" + ` for command details or ` + "`awf docs`" + ` for the guide.
`

const initHelp = `Usage: awf init

Create .awf/project.md with editable starter guidance and render the fixed generated files.
Read ` + "`awf docs integration`" + ` before adopting AWF in an existing repository.
`

const renderHelp = `Usage: awf render

Render the fixed generated files. Retired AWF-marked files are reported but never deleted.
See ` + "`awf docs integration`" + ` for ownership and update procedures.
`

const checkHelp = `Usage: awf check

Validate working-tree AWF sources and generated files. Unmanaged AWF-marked files fail the check.
See ` + "`awf docs integration`" + ` for gate and CI integration.
`

const resolveHelp = `Usage: awf resolve [<path>...]
       awf resolve --coverage <path>...

Without paths, print explicit global topics. With paths, print globals and every topic matching a supplied lexical repository-relative path.
Coverage reports explicit globals once and matching non-global topics for each distinct normalized path, including paths with no specific match.
See ` + "`awf docs topics`" + ` for authoring, maintenance, and coverage limits.
`

const docsHelp = `Usage: awf docs [integration|topics|effort]

Print the embedded overview or one adopter guide to standard output.
`

const newHelp = `Usage: awf new <artifact> [arguments]

Commands:
  effort <slug>            create local effort memory
  plan <slug>              create docs/plans/<slug>.md
  adr <slug>               create docs/decisions/<slug>.md with pending status
  topic <id> <pattern>...  create .awf/topics/<id>.md with supplied selectors

Creation never replaces an existing destination. Quote glob patterns so the shell does not expand them.
See ` + "`awf docs effort`" + ` and ` + "`awf docs topics`" + ` for lifecycle and authoring guidance.
`

const effortHelp = `Usage: awf effort <command>

Commands:
  list           list active efforts
  show <slug>    show an effort's memory path and contents
  finish <slug>  move an effort into the local archive

Create memory with ` + "`awf new effort <slug>`" + `. See ` + "`awf docs effort`" + ` for the complete workflow.
`

const versionHelp = `Usage: awf version

Print the embedded AWF release version.
`
