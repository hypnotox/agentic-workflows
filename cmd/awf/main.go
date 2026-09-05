// Command awf projects a small .awf source tree into agent guidance.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

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
		if len(args) < 3 {
			return usage(stderr, "usage: awf resolve <path>...")
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

func runEffort(root string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || helpRequested(args) {
		writeText(stdout, effortHelp)
		return 0
	}
	switch args[0] {
	case "new":
		if len(args) != 2 {
			return usage(stderr, "usage: awf effort new <slug>")
		}
		path, err := effortfs.New(root, args[1])
		if err != nil {
			return failure(stderr, err)
		}
		fmt.Fprintln(stdout, "memory:", filepathSlash(path))
		return 0
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
		return usage(stderr, fmt.Sprintf("unknown effort command %q; expected new, list, show, or finish", args[0]))
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
  resolve    find topics for repository paths
  effort     manage local effort memory
  version    print the AWF version

Run ` + "`awf help <command>`" + ` for command details.
`

const initHelp = `Usage: awf init

Create .awf/project.md with editable starter guidance and render the fixed generated files.
`

const renderHelp = `Usage: awf render

Render the fixed generated files. Retired AWF-marked files are reported but never deleted.
`

const checkHelp = `Usage: awf check

Validate AWF sources and generated files. Unmanaged AWF-marked files fail the check.
`

const resolveHelp = `Usage: awf resolve <path>...

Print every topic matching the supplied lexical repository-relative paths.
`

const effortHelp = `Usage: awf effort <command>

Commands:
  new <slug>     create local effort memory
  list           list active efforts
  show <slug>    show an effort's memory path and contents
  finish <slug>  move an effort into the local archive
`

const versionHelp = `Usage: awf version

Print the embedded AWF release version.
`
