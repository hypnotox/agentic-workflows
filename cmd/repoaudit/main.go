// Command repoaudit runs repo-specific conformance checks over a git commit range,
// sharing awf audit's finding contract: it reports the same warn/error rank from
// internal/severity, and exits non-zero only on an error finding. It is deliberately
// NOT part of the shipped awf standard: it is repo-local dev tooling wired as
// `./x audit-local` for explicit repository-owner use (ADR-0073). It never runs the
// gate. Three rules: changelog conformance - an adopter-facing change in the range with
// no CHANGELOG [Unreleased] entry is an error.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

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
	FileText(context.Context, string, string) (string, bool, error)
}

func main() {
	_, _, code := parseArgs(os.Args, os.Stderr)
	if code != 0 {
		os.Exit(code)
	}
	ctx, cancel := context.WithTimeout(context.Background(), awfgit.CommandTimeout)
	repo, err := awfgit.Open(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "repoaudit:", err)
		cancel()
		os.Exit(1)
	}
	code = runWith(ctx, os.Args, os.Stdout, os.Stderr, repo)
	cancel()
	os.Exit(code)
}

const changelogPath = "changelog/CHANGELOG.md"

// adopterFacingPrefixes are the path roots whose change is adopter-visible: the
// rendered templates, the shipped CLI, the config/lock schema, the artifact catalog
// (since ADR-0068 a new shipped skill/agent can land as a pure catalog entry), and
// the packages that decide what the shipped commands actually answer or write.
// A source root belongs here when changing it alone can change what an adopter
// observes without any template or schema moving. Test files under these roots are
// excluded: tests are not adopter-visible (ADR-0073).
var adopterFacingPrefixes = []string{
	"templates/", "cmd/awf/",
	"internal/config/", "internal/manifest/", "internal/catalog/",
	"internal/project/", "internal/render/",
	"internal/effort/", "internal/worktree/",
}

// rules is the repo-local audit's rule registry (ADR-0073 Decision 1): each rule
// reports findings over the range, and another repo-local rule is a new function
// appended here plus nothing else.
var rules = []func(ctx context.Context, git gitReader, base, head string, log io.Writer) []finding{
	changelogRule,
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
