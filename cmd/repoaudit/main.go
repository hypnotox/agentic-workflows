// Command repoaudit runs repo-specific conformance checks over a git commit range,
// mirroring awf audit's finding contract (Warning/Error severity; non-zero exit only
// on an Error finding) but deliberately NOT part of the shipped awf standard: it is
// repo-local dev tooling wired as `./x audit-local` and invoked by awf-reviewing-impl
// (ADR-0073). It never runs the gate. Its one rule is changelog conformance — an
// adopter-facing change in the range with no CHANGELOG [Unreleased] entry is an Error.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// severity mirrors internal/audit's contract (Warning=0, Error=1) without importing
// it — repoaudit is standalone repo tooling.
type severity int

const (
	warning severity = iota
	errorSev
)

func (s severity) label() string {
	if s == errorSev {
		return "error"
	}
	return "warning"
}

type finding struct {
	sev    severity
	rule   string
	detail string
}

// gitFunc runs git and returns stdout — the seam that keeps runWith testable.
type gitFunc func(args ...string) (string, error)

func realGit(args ...string) (string, error) { // coverage-ignore: os/exec boundary; runWith is tested with a fake gitFunc
	// Single-block body: the trailing coverage-ignore drops the block whose start
	// line is this signature line. A multi-line body with an `if err` branch would
	// leave the branch and final-return blocks (later start lines) UNignored and
	// uncovered — realGit runs no test — failing the 100% gate. Callers check err
	// before using the string, so returning it unconditionally is equivalent.
	out, err := exec.Command("git", args...).Output()
	return string(out), gitError(err)
}

// gitError appends git's captured stderr to an *exec.ExitError: .Output() stores
// it, but %v prints only "exit status N", leaving a git-failure finding
// undiagnosable (e.g. no hint of "unknown revision 'origin/main'").
func gitError(err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(ee.Stderr) > 0 {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
	}
	return err
}

func main() { os.Exit(runWith(os.Args, os.Stdout, os.Stderr, realGit)) } // coverage-ignore: os.Exit + real-git boundary; runWith is unit-tested

const changelogPath = "changelog/CHANGELOG.md"

// adopterFacingPrefixes are the path roots whose change is adopter-visible (rendered
// templates, the shipped CLI, the config/lock schema). Conservative and logged — a
// render-logic-only change under internal/render can slip it (ADR-0073).
var adopterFacingPrefixes = []string{"templates/", "cmd/awf/", "internal/config/", "internal/manifest/"}

// rules is the repo-local audit's rule registry (ADR-0073 Decision 1): each rule
// reports findings over the range, and a second repo-local rule is a new function
// appended here plus nothing else. Today it holds the one changelog rule.
var rules = []func(git gitFunc, base, head string, log io.Writer) []finding{
	changelogRule,
}

func runWith(args []string, stdout, stderr io.Writer, git gitFunc) int {
	rng := "origin/main..HEAD"
	if len(args) >= 2 {
		rng = args[1]
	}
	base, head, ok := strings.Cut(rng, "..")
	// Cut mangles a three-dot range (head "."-prefixed) or a multi-".." input
	// (head contains ".."); both would reach git as a bogus rev. Dots inside a
	// rev (v0.10.0) are fine — git forbids "."-leading and ".."-containing refs.
	if !ok || base == "" || head == "" || strings.HasPrefix(head, ".") || strings.Contains(head, "..") {
		fmt.Fprintln(stderr, "usage: repoaudit [<base>..<head>]  (default origin/main..HEAD)")
		return 2
	}
	errs := 0
	for _, rule := range rules {
		for _, f := range rule(git, base, head, stdout) {
			fmt.Fprintf(stdout, "%-7s %-22s %s\n", f.sev.label(), f.rule, f.detail)
			if f.sev == errorSev {
				errs++
			}
		}
	}
	// invariant: repo-audit-error-exit
	if errs > 0 {
		return 1
	}
	fmt.Fprintln(stdout, "repoaudit: clean")
	return 0
}

// changelogRule flags an adopter-facing change in base..head that lacks a CHANGELOG
// [Unreleased] entry. It logs the adopter-facing files it considered. A git or parse
// failure becomes an Error finding — it cannot verify conformance, so it fails loud.
func changelogRule(git gitFunc, base, head string, log io.Writer) []finding {
	// Judge from the merge base, not the base tip: once base moves past the fork
	// point, endpoint semantics would blame upstream files on the effort (false
	// Error) and an upstream [Unreleased] edit would mask the effort's own missing
	// entry (false pass). Both the diff and the section comparison must use it.
	mb, err := git("merge-base", base, head)
	if err != nil {
		return []finding{{errorSev, "changelog-unreleased", fmt.Sprintf("git merge-base %s %s failed: %v", base, head, err)}}
	}
	from := strings.TrimSpace(mb)
	diff, err := git("diff", "--name-only", from, head)
	if err != nil {
		return []finding{{errorSev, "changelog-unreleased", fmt.Sprintf("git diff %s..%s failed: %v", from, head, err)}}
	}
	var touched []string
	for _, f := range strings.Split(strings.TrimSpace(diff), "\n") {
		if f == "" {
			continue
		}
		for _, p := range adopterFacingPrefixes {
			if strings.HasPrefix(f, p) {
				touched = append(touched, f)
				break
			}
		}
	}
	if len(touched) == 0 {
		return nil
	}
	fmt.Fprintf(log, "repoaudit: adopter-facing paths in %s..%s: %s\n", base, head, strings.Join(touched, ", "))
	baseBody, err := unreleasedSection(git, from)
	if err != nil {
		return []finding{{errorSev, "changelog-unreleased", fmt.Sprintf("reading %s at %s: %v", changelogPath, from, err)}}
	}
	headBody, err := unreleasedSection(git, head)
	if err != nil {
		return []finding{{errorSev, "changelog-unreleased", fmt.Sprintf("reading %s at %s: %v", changelogPath, head, err)}}
	}
	if baseBody == headBody {
		return []finding{{errorSev, "changelog-unreleased", fmt.Sprintf("adopter-facing change in %s..%s but %s [Unreleased] is unchanged — add an entry", base, head, changelogPath)}}
	}
	return nil
}

// unreleasedSection returns the body of the ## [Unreleased] section of the changelog at
// rev — the lines between the [Unreleased] header and the next top-level "## [" header.
func unreleasedSection(git gitFunc, rev string) (string, error) {
	content, err := git("show", rev+":"+changelogPath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(content, "\n")
	start := -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, "## [Unreleased]") {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return "", fmt.Errorf("no ## [Unreleased] section in %s", changelogPath)
	}
	var body []string
	for _, ln := range lines[start:] {
		if strings.HasPrefix(ln, "## [") {
			break
		}
		body = append(body, ln)
	}
	return strings.Join(body, "\n"), nil
}
