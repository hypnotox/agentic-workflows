// Command repoaudit runs repo-specific conformance checks over a git commit range,
// sharing awf audit's finding contract: it reports the same warn/error rank from
// internal/severity, and exits non-zero only on an error finding. It is deliberately
// NOT part of the shipped awf standard: it is repo-local dev tooling wired as
// `./x audit-local` and invoked by awf-reviewing-impl (ADR-0073). It never runs the
// gate. Three rules: changelog conformance - an adopter-facing change in the range with
// no CHANGELOG [Unreleased] entry is an error - and coverage-ignore re-evaluation - an
// added or touched coverage-ignore directive in a production Go file is a warn
// prompting a reachability re-check.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/coverage"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

type finding struct {
	sev    severity.Rank
	rule   string
	detail string
}

// gitReader is this consumer's narrow view of the semantic git seam.
type gitReader interface {
	MergeBase(context.Context, string, string) (string, error)
	RangeChangedPaths(context.Context, string, string) ([]string, error)
	RangeDiffText(context.Context, string, string) (string, error)
	FileText(context.Context, string, string) (string, bool, error)
}

func main() { // coverage-ignore: process-exit composition boundary; runWith has fake-backed tests
	_, _, code := parseArgs(os.Args, os.Stderr)
	if code != 0 { // coverage-ignore: process-exit composition boundary; runWith has fake-backed tests
		os.Exit(code)
	}
	ctx, cancel := context.WithTimeout(context.Background(), awfgit.CommandTimeout) // coverage-ignore: process-exit composition boundary; runWith has fake-backed tests
	repo, err := awfgit.Open(".")
	if err != nil { // coverage-ignore: process-exit composition boundary; runWith has fake-backed tests
		fmt.Fprintln(os.Stderr, "repoaudit:", err)
		cancel()
		os.Exit(1)
	}
	code = runWith(ctx, os.Args, os.Stdout, os.Stderr, repo) // coverage-ignore: process-exit composition boundary; runWith has fake-backed tests
	cancel()
	os.Exit(code) // coverage-ignore: process-exit composition boundary; runWith has fake-backed tests
} // coverage-ignore: process-exit composition boundary; runWith has fake-backed tests

const changelogPath = "changelog/CHANGELOG.md"

const coverageBaselinePath = "coverage-baseline.json"

// adopterFacingPrefixes are the path roots whose change is adopter-visible: the
// rendered templates, the shipped CLI, the config/lock schema, the artifact catalog
// (since ADR-0068 a new shipped skill/agent can land as a pure catalog entry), and
// the packages that decide what the shipped commands actually answer or write.
// The behaviour roots were added after a real miss: `awf context` reported an
// in-flight decision record as frozen, and the fix under internal/adr and
// internal/project drew no finding because only the declarative roots were listed.
// A source root belongs here when changing it alone can change what an adopter
// observes without any template or schema moving. Test files under these roots are
// excluded: tests are not adopter-visible (ADR-0073).
var adopterFacingPrefixes = []string{
	"templates/", "cmd/awf/",
	"internal/config/", "internal/manifest/", "internal/catalog/",
	"internal/adr/", "internal/project/", "internal/render/",
	"internal/effort/", "internal/worktree/",
}

// rules is the repo-local audit's rule registry (ADR-0073 Decision 1): each rule
// reports findings over the range, and another repo-local rule is a new function
// appended here plus nothing else.
var rules = []func(ctx context.Context, git gitReader, base, head string, log io.Writer) []finding{
	changelogRule,
	coverageIgnoreRule,
	coverageBaselineRule,
}

func runWith(ctx context.Context, args []string, stdout, stderr io.Writer, git gitReader) int {
	base, head, code := parseArgs(args, stderr)
	if code != 0 {
		return code
	}
	return runRange(ctx, base, head, stdout, git)
}

func parseArgs(args []string, stderr io.Writer) (base, head string, code int) {
	// No default range (ADR-0127 Decision 11): a no-argument call would report
	// over commits nobody named, which is the guess-the-base defect in
	// repo-local clothing.
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: repoaudit <base>..<head>")
		return "", "", 2
	}
	base, head, err := awfgit.ParseRange(args[1], false)
	if err != nil {
		fmt.Fprintf(stderr, "repoaudit: %v\n", err)
		return "", "", 2
	}
	return base, head, 0
}

func runRange(ctx context.Context, base, head string, stdout io.Writer, git gitReader) int {
	errs, warns := 0, 0
	for _, rule := range rules {
		for _, f := range rule(ctx, git, base, head, stdout) {
			fmt.Fprintf(stdout, "%-7s %-22s %s\n", f.sev.String(), f.rule, f.detail)
			if f.sev == severity.Error {
				errs++
			} else {
				warns++
			}
		}
	}
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

// coverageBaselineRule reports each newly admitted raw miss as review evidence.
// The gate owns policy enforcement; this range-oriented audit only makes the
// stored admission reason visible to reviewers.
func coverageBaselineRule(ctx context.Context, git gitReader, base, head string, _ io.Writer) []finding {
	mb, err := git.MergeBase(ctx, base, head)
	if err != nil {
		return []finding{{severity.Error, "coverage-baseline-added", fmt.Sprintf("git merge-base %s %s failed: %v", base, head, err)}}
	}
	from := strings.TrimSpace(mb)
	previous, previousFound, err := baselineAt(ctx, git, from)
	if err != nil {
		return []finding{{severity.Error, "coverage-baseline-added", fmt.Sprintf("reading %s at %s: %v", coverageBaselinePath, from, err)}}
	}
	current, currentFound, err := baselineAt(ctx, git, head)
	if err != nil {
		return []finding{{severity.Error, "coverage-baseline-added", fmt.Sprintf("reading %s at %s: %v", coverageBaselinePath, head, err)}}
	}
	if !previousFound && !currentFound {
		return nil
	}
	if !currentFound {
		return []finding{{severity.Error, "coverage-baseline-added", fmt.Sprintf("%s is unavailable at %s", coverageBaselinePath, head)}}
	}

	known := make(map[coverage.Identity]bool)
	if previousFound {
		for _, admission := range previous.Repository {
			known[admission.Identity] = true
		}
	}
	var added []coverage.MissAdmission
	for _, admission := range current.Repository {
		if !known[admission.Identity] {
			added = append(added, admission)
		}
	}
	slices.SortFunc(added, func(a, b coverage.MissAdmission) int {
		return strings.Compare(formatCoverageIdentity(a.Identity), formatCoverageIdentity(b.Identity))
	})
	findings := make([]finding, 0, len(added))
	for _, admission := range added {
		detail := fmt.Sprintf("%s: added raw baseline identity; reviewed reason: %s", formatCoverageIdentity(admission.Identity), admission.Reason)
		if admission.MovedFrom != nil {
			detail = fmt.Sprintf("%s: moved from %s to raw baseline identity; reviewed reason: %s", formatCoverageIdentity(admission.Identity), formatCoverageIdentity(*admission.MovedFrom), admission.Reason)
		}
		findings = append(findings, finding{severity.Warn, "coverage-baseline-added", detail})
	}
	return findings
}

// baselineAt delegates strict parsing and canonical validation to the canonical
// coverage owner after the git seam supplies endpoint content.
func baselineAt(ctx context.Context, git gitReader, rev string) (coverage.Baseline, bool, error) {
	raw, found, err := git.FileText(ctx, rev, coverageBaselinePath)
	if err != nil {
		return coverage.Baseline{}, false, err
	}
	if !found {
		return coverage.Baseline{}, false, nil
	}
	file, err := os.CreateTemp("", "repoaudit-coverage-baseline-*.json")
	if err != nil {
		return coverage.Baseline{}, false, err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err := io.WriteString(file, raw); err != nil {
		file.Close()
		return coverage.Baseline{}, false, err
	}
	if err := file.Close(); err != nil {
		return coverage.Baseline{}, false, err
	}
	baseline, err := coverage.LoadBaselineForHistoricalComparison(name)
	if err != nil {
		return coverage.Baseline{}, false, err
	}
	return baseline, true, nil
}

func formatCoverageIdentity(identity coverage.Identity) string {
	return fmt.Sprintf("%s:%d.%d,%d.%d %d", identity.File, identity.Start.Line, identity.Start.Column, identity.End.Line, identity.End.Column, identity.Statements)
}

// changelogRule flags an adopter-facing change in base..head that lacks a CHANGELOG
// [Unreleased] entry. It logs the adopter-facing files it considered. The conformance
// verdict is an advisory warn (ADR-0107) - the path heuristic cannot tell a benign
// change from a behavioral one, so it informs rather than blocks. A git or parse failure
// is an error - it cannot verify conformance, so it fails loud.
func changelogRule(ctx context.Context, git gitReader, base, head string, log io.Writer) []finding {
	// Judge from the merge base, not the base tip: once base moves past the fork
	// point, endpoint semantics would blame upstream files on the effort (false
	// error) and an upstream [Unreleased] edit would mask the effort's own missing
	// entry (false pass). Both the diff and the section comparison must use it.
	mb, err := git.MergeBase(ctx, base, head)
	if err != nil {
		return []finding{{severity.Error, "changelog-unreleased", fmt.Sprintf("git merge-base %s %s failed: %v", base, head, err)}}
	}
	from := strings.TrimSpace(mb)
	diff, err := git.RangeChangedPaths(ctx, from, head)
	if err != nil {
		return []finding{{severity.Error, "changelog-unreleased", fmt.Sprintf("git diff %s..%s failed: %v", from, head, err)}}
	}
	var touched []string
	for _, f := range diff {
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
	baseBody, err := unreleasedSection(ctx, git, from)
	if err != nil {
		return []finding{{severity.Error, "changelog-unreleased", fmt.Sprintf("reading %s at %s: %v", changelogPath, from, err)}}
	}
	headBody, err := unreleasedSection(ctx, git, head)
	if err != nil {
		return []finding{{severity.Error, "changelog-unreleased", fmt.Sprintf("reading %s at %s: %v", changelogPath, head, err)}}
	}
	if baseBody == headBody {
		// Advisory, not blocking (ADR-0107): the path heuristic cannot tell a benign
		// change (a refactor, a comment/marker relocation) from a behavioral one, so a
		// blocking error over-fires. A git/read failure above stays an error - that means
		// the rule cannot verify conformance and must fail loud.
		return []finding{{severity.Warn, "changelog-unreleased", fmt.Sprintf("adopter-facing change in %s..%s but %s [Unreleased] is unchanged: add an entry", base, head, changelogPath)}}
	}
	return nil
}

// coverageIgnoreMarker is the comment form the rule detects, assembled so this
// file's own lines never match it (the same split literal internal/coverage
// uses for its directive constant).
const coverageIgnoreMarker = "//" + " coverage-ignore"

// coverageIgnoreRule emits one warn per added-or-touched coverage-ignore
// directive in a non-test Go file over the range: every ignore states a
// reachability claim, and three factually false claims surfaced on 2026-07-08
// alone, so each new one gets a deterministic re-evaluation prompt at review
// time. A warn never affects the exit code; a git failure is an error - the
// rule cannot verify, so it fails loud like the changelog rule.
func coverageIgnoreRule(ctx context.Context, git gitReader, base, head string, log io.Writer) []finding {
	mb, err := git.MergeBase(ctx, base, head)
	if err != nil {
		return []finding{{severity.Error, "coverage-ignore-added", fmt.Sprintf("git merge-base %s %s failed: %v", base, head, err)}}
	}
	from := strings.TrimSpace(mb)
	// Pin the header format against user git config: diff.noprefix /
	// diff.mnemonicprefix would drop or change the "b/" prefix the parser keys
	// on, and an external diff driver would replace the format entirely.
	diff, err := git.RangeDiffText(ctx, from, head)
	if err != nil {
		return []finding{{severity.Error, "coverage-ignore-added", fmt.Sprintf("git diff %s..%s failed: %v", from, head, err)}}
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
			out = append(out, finding{severity.Warn, "coverage-ignore-added",
				file + ": added or touched coverage-ignore; re-evaluate: is this branch genuinely untriggerable? Try to stage the state it declares impossible"})
		}
	}
	return out
}

// unreleasedSection returns the body of the ## [Unreleased] section of the changelog at
// rev - the lines between the [Unreleased] header and the next top-level "## [" header.
func unreleasedSection(ctx context.Context, git gitReader, rev string) (string, error) {
	content, found, err := git.FileText(ctx, rev, changelogPath)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("%s not found", changelogPath)
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
