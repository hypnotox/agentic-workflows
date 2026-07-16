// Command repoaudit runs repo-specific conformance checks over a git commit range,
// mirroring awf audit's finding contract (Warning/Error severity; non-zero exit only
// on an Error finding) but deliberately NOT part of the shipped awf standard: it is
// repo-local dev tooling wired as `./x audit-local` and invoked by awf-reviewing-impl
// (ADR-0073). It never runs the gate. Two rules: changelog conformance - an
// adopter-facing change in the range with no CHANGELOG [Unreleased] entry is an
// Error - and coverage-ignore re-evaluation - an added or touched coverage-ignore
// directive in a production Go file is a Warning prompting a reachability re-check.
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
// it - repoaudit is standalone repo tooling.
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

// gitFunc runs git and returns stdout - the seam that keeps runWith testable.
type gitFunc func(args ...string) (string, error)

func realGit(args ...string) (string, error) { // coverage-ignore: os/exec boundary; runWith is tested with a fake gitFunc
	// Single-block body: the trailing coverage-ignore drops the block whose start
	// line is this signature line. A multi-line body with an `if err` branch would
	// leave the branch and final-return blocks (later start lines) UNignored and
	// uncovered - realGit runs no test - failing the 100% gate. Callers check err
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
// templates, the shipped CLI, the config/lock schema, and the artifact catalog - since
// ADR-0068 a new shipped skill/agent can land as a pure catalog entry). Conservative
// and logged - a render-logic-only change under internal/render can slip it (ADR-0073).
// Test files under these roots are excluded: tests are not adopter-visible.
var adopterFacingPrefixes = []string{"templates/", "cmd/awf/", "internal/config/", "internal/manifest/", "internal/catalog/"}

// rules is the repo-local audit's rule registry (ADR-0073 Decision 1): each rule
// reports findings over the range, and another repo-local rule is a new function
// appended here plus nothing else.
var rules = []func(git gitFunc, base, head string, log io.Writer) []finding{
	changelogRule,
	coverageIgnoreRule,
}

func runWith(args []string, stdout, stderr io.Writer, git gitFunc) int {
	rng := "origin/main..HEAD"
	if len(args) >= 2 {
		rng = args[1]
	}
	base, head, ok := strings.Cut(rng, "..")
	// Cut mangles a three-dot range (head "."-prefixed) or a multi-".." input
	// (head contains ".."); both would reach git as a bogus rev. A "-"-prefixed
	// side would reach git as an option-like argument. Dots inside a rev
	// (v0.10.0) are fine - git forbids "."-leading, ".."-containing, and
	// "-"-leading refs, so no valid rev is rejected.
	if !ok || base == "" || head == "" || strings.HasPrefix(head, ".") || strings.Contains(head, "..") ||
		strings.HasPrefix(base, "-") || strings.HasPrefix(head, "-") {
		fmt.Fprintln(stderr, "usage: repoaudit [<base>..<head>]  (default origin/main..HEAD)")
		return 2
	}
	errs, warns := 0, 0
	for _, rule := range rules {
		for _, f := range rule(git, base, head, stdout) {
			fmt.Fprintf(stdout, "%-7s %-22s %s\n", f.sev.label(), f.rule, f.detail)
			if f.sev == errorSev {
				errs++
			} else {
				warns++
			}
		}
	}
	// touches-invariant: repo-audit-error-exit - error-count exit-code branch; proof in main_test.go
	if errs > 0 {
		return 1
	}
	if warns > 0 {
		fmt.Fprintf(stdout, "repoaudit: %d warning(s), no errors\n", warns)
		return 0
	}
	fmt.Fprintln(stdout, "repoaudit: clean")
	return 0
}

// changelogRule flags an adopter-facing change in base..head that lacks a CHANGELOG
// [Unreleased] entry. It logs the adopter-facing files it considered. The conformance
// verdict is an advisory Warning (ADR-0107) - the path heuristic cannot tell a benign
// change from a behavioral one, so it informs rather than blocks. A git or parse failure
// is an Error - it cannot verify conformance, so it fails loud.
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
		if f == "" || strings.HasSuffix(f, "_test.go") {
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
	fmt.Fprintf(log, "repoaudit: adopter-facing paths in %s..%s: %s\n", from, head, strings.Join(touched, ", "))
	baseBody, err := unreleasedSection(git, from)
	if err != nil {
		return []finding{{errorSev, "changelog-unreleased", fmt.Sprintf("reading %s at %s: %v", changelogPath, from, err)}}
	}
	headBody, err := unreleasedSection(git, head)
	if err != nil {
		return []finding{{errorSev, "changelog-unreleased", fmt.Sprintf("reading %s at %s: %v", changelogPath, head, err)}}
	}
	if baseBody == headBody {
		// Advisory, not blocking (ADR-0107): the path heuristic cannot tell a benign
		// change (a refactor, a comment/marker relocation) from a behavioral one, so a
		// blocking Error over-fires. A git/read failure above stays an Error - that means
		// the rule cannot verify conformance and must fail loud.
		return []finding{{warning, "changelog-unreleased", fmt.Sprintf("adopter-facing change in %s..%s but %s [Unreleased] is unchanged: add an entry", base, head, changelogPath)}}
	}
	return nil
}

// coverageIgnoreMarker is the comment form the rule detects, assembled so this
// file's own lines never match it (the same split literal internal/coverage
// uses for its directive constant).
const coverageIgnoreMarker = "//" + " coverage-ignore"

// coverageIgnoreRule emits one Warning per added-or-touched coverage-ignore
// directive in a non-test Go file over the range: every ignore states a
// reachability claim, and three factually false claims surfaced on 2026-07-08
// alone, so each new one gets a deterministic re-evaluation prompt at review
// time. Warnings never affect the exit code; a git failure is an Error - the
// rule cannot verify, so it fails loud like the changelog rule.
func coverageIgnoreRule(git gitFunc, base, head string, log io.Writer) []finding {
	mb, err := git("merge-base", base, head)
	if err != nil {
		return []finding{{errorSev, "coverage-ignore-added", fmt.Sprintf("git merge-base %s %s failed: %v", base, head, err)}}
	}
	from := strings.TrimSpace(mb)
	// Pin the header format against user git config: diff.noprefix /
	// diff.mnemonicprefix would drop or change the "b/" prefix the parser keys
	// on, and an external diff driver would replace the format entirely.
	diff, err := git("-c", "diff.noprefix=false", "-c", "diff.mnemonicprefix=false",
		"-c", "diff.dstPrefix=b/", "diff", "--no-ext-diff", "-U0", from, head, "--", "*.go")
	if err != nil {
		return []finding{{errorSev, "coverage-ignore-added", fmt.Sprintf("git diff %s..%s failed: %v", from, head, err)}}
	}
	var out []finding
	file := "" // current +++ target; "" while in a skipped (test/deleted) file
	for _, ln := range strings.Split(diff, "\n") {
		// Known limitation: an added content line that itself starts "++ "
		// (a diff fixture embedded in a raw string in production Go) renders as
		// "+++ ..." and would be misparsed as a header - contrived for *.go
		// content and warning-only, so tolerated.
		if rest, ok := strings.CutPrefix(ln, "+++ "); ok {
			file = ""
			if p, ok := strings.CutPrefix(rest, "b/"); ok && !strings.HasSuffix(p, "_test.go") {
				file = p
			}
			continue
		}
		if file == "" || !strings.HasPrefix(ln, "+") {
			continue
		}
		if strings.Contains(ln, coverageIgnoreMarker) {
			out = append(out, finding{warning, "coverage-ignore-added",
				file + ": added or touched coverage-ignore; re-evaluate: is this branch genuinely untriggerable? Try to stage the state it declares impossible"})
		}
	}
	return out
}

// unreleasedSection returns the body of the ## [Unreleased] section of the changelog at
// rev - the lines between the [Unreleased] header and the next top-level "## [" header.
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
