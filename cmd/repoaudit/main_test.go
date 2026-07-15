package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// fakeGit returns canned stdout/err keyed by the joined args; a key mapped to a
// non-nil error simulates a git failure.
type fakeGit map[string]struct {
	out string
	err error
}

func (f fakeGit) run(args ...string) (string, error) {
	key := strings.Join(args, " ")
	if r, ok := f[key]; ok {
		return r.out, r.err
	}
	return "", fmt.Errorf("unexpected git call: %s", key)
}

func changelog(unreleased string) string {
	return "# Changelog\n\n## [Unreleased]\n" + unreleased + "## [0.1.0] - 2026-01-01\n### Others\n- x\n"
}

// runFake runs runWith with a fake git and returns exit code + combined stdout.
func runFake(args []string, g fakeGit) (int, string) {
	var out, errOut strings.Builder
	code := runWith(args, &out, &errOut, g.run)
	return code, out.String() + errOut.String()
}

func TestSeverityLabel(t *testing.T) {
	if warning.label() != "warning" || errorSev.label() != "error" {
		t.Fatalf("labels: %q %q", warning.label(), errorSev.label())
	}
}

func TestGitErrorSurfacesStderr(t *testing.T) {
	// .Output() captures stderr on *exec.ExitError, but %v prints only
	// "exit status N" - the decoration is what makes a git-failure finding
	// diagnosable (e.g. "unknown revision 'origin/main'").
	_, err := exec.Command("sh", "-c", "echo bad rev >&2; exit 3").Output()
	if got := gitError(err); got == nil || !strings.Contains(got.Error(), "bad rev") {
		t.Fatalf("stderr not surfaced: %v", got)
	}
	if gitError(nil) != nil {
		t.Fatal("nil must stay nil")
	}
	plain := errors.New("boom")
	if got := gitError(plain); !errors.Is(got, plain) || got.Error() != plain.Error() {
		t.Fatalf("non-exec error must pass through undecorated, got %v", got)
	}
	_, err = exec.Command("sh", "-c", "exit 3").Output()
	if got := gitError(err); !errors.Is(got, err) || got.Error() != err.Error() {
		t.Fatalf("empty stderr must pass through undecorated, got %v", got)
	}
}

func TestUsageError(t *testing.T) {
	code, out := runFake([]string{"repoaudit", "no-range-here"}, fakeGit{})
	if code != 2 || !strings.Contains(out, "usage:") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestRejectsMalformedRanges(t *testing.T) {
	// strings.Cut on ".." would silently mangle these (b...h → head ".h";
	// a..b..c → head "b..c") and hand git a bogus rev, and a "-"-prefixed
	// side would reach git as an option-like argument; all must hit the
	// usage path instead. Dots inside a rev (v0.10.0..HEAD) stay legal.
	for _, rng := range []string{"b...h", "a..b..c", "-foo..HEAD", "b..--all"} {
		code, out := runFake([]string{"repoaudit", rng}, fakeGit{})
		if code != 2 || !strings.Contains(out, "usage:") {
			t.Fatalf("%s: code=%d out=%q", rng, code, out)
		}
	}
	g := fakeGit{
		"merge-base v0.10.0 HEAD": {out: "v0.10.0\n"},
		"-c diff.noprefix=false -c diff.mnemonicprefix=false -c diff.dstPrefix=b/ diff --no-ext-diff -U0 v0.10.0 HEAD -- *.go": {out: ""},
		"diff --name-only v0.10.0 HEAD": {out: "docs/x.md\n"},
	}
	code, out := runFake([]string{"repoaudit", "v0.10.0..HEAD"}, g)
	if code != 0 || !strings.Contains(out, "repoaudit: clean") {
		t.Fatalf("dotted rev: code=%d out=%q", code, out)
	}
}

func TestCleanNonAdopterFacing(t *testing.T) {
	// Default range (no arg) + changes outside the allowlist → clean, exit 0. The
	// blank line between the two paths also exercises changelogRule's empty-token
	// `continue` - the sole branch no other test reaches (100%-coverage gate).
	g := fakeGit{
		"merge-base origin/main HEAD": {out: "origin/main\n"},
		"-c diff.noprefix=false -c diff.mnemonicprefix=false -c diff.dstPrefix=b/ diff --no-ext-diff -U0 origin/main HEAD -- *.go": {out: ""},
		"diff --name-only origin/main HEAD": {out: "docs/x.md\n\ninternal/render/render.go\n"},
	}
	code, out := runFake([]string{"repoaudit"}, g)
	if code != 0 || !strings.Contains(out, "repoaudit: clean") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestWarningMissingEntry(t *testing.T) {
	// Adopter-facing change, [Unreleased] identical across the range → advisory Warning,
	// exit 0 (ADR-0107): the conformance verdict no longer blocks, only informs.
	same := changelog("\n")
	g := fakeGit{
		"merge-base b h": {out: "b\n"},
		"-c diff.noprefix=false -c diff.mnemonicprefix=false -c diff.dstPrefix=b/ diff --no-ext-diff -U0 b h -- *.go": {out: ""},
		"diff --name-only b h":    {out: "templates/x.tmpl\n"},
		"show b:" + changelogPath: {out: same},
		"show h:" + changelogPath: {out: same},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	// invariant: changelog-rule-advisory
	if code != 0 || !strings.Contains(out, "warning") || !strings.Contains(out, "[Unreleased] is unchanged") {
		t.Fatalf("code=%d out=%q", code, out)
	}
	if !strings.Contains(out, "adopter-facing paths in b..h: templates/x.tmpl") {
		t.Fatalf("missing considered-paths log: %q", out)
	}
}

func TestCleanEntryAdded(t *testing.T) {
	// Adopter-facing change, [Unreleased] differs across the range → clean, exit 0.
	g := fakeGit{
		"merge-base b h": {out: "b\n"},
		"-c diff.noprefix=false -c diff.mnemonicprefix=false -c diff.dstPrefix=b/ diff --no-ext-diff -U0 b h -- *.go": {out: ""},
		"diff --name-only b h":    {out: "cmd/awf/root.go\n"},
		"show b:" + changelogPath: {out: changelog("\n")},
		"show h:" + changelogPath: {out: changelog("### Features\n- new thing\n")},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 0 || !strings.Contains(out, "repoaudit: clean") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestTestFilesAreNotAdopterFacing(t *testing.T) {
	// A test-only change under an allowlisted root is not adopter-visible; it
	// must not demand a changelog entry.
	g := fakeGit{
		"merge-base b h": {out: "b\n"},
		"-c diff.noprefix=false -c diff.mnemonicprefix=false -c diff.dstPrefix=b/ diff --no-ext-diff -U0 b h -- *.go": {out: ""},
		"diff --name-only b h": {out: "internal/config/config_test.go\ncmd/awf/root_test.go\n"},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 0 || !strings.Contains(out, "repoaudit: clean") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestCatalogIsAdopterFacing(t *testing.T) {
	// Since ADR-0068 a new shipped skill/agent can land as a pure catalog entry,
	// with no diff under templates/ - the allowlist must catch it.
	same := changelog("\n")
	g := fakeGit{
		"merge-base b h": {out: "b\n"},
		"-c diff.noprefix=false -c diff.mnemonicprefix=false -c diff.dstPrefix=b/ diff --no-ext-diff -U0 b h -- *.go": {out: ""},
		"diff --name-only b h":    {out: "internal/catalog/catalog.go\n"},
		"show b:" + changelogPath: {out: same},
		"show h:" + changelogPath: {out: same},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 0 || !strings.Contains(out, "warning") || !strings.Contains(out, "[Unreleased] is unchanged") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestDivergedBaseJudgesFromMergeBase(t *testing.T) {
	// Regression: base has moved past the fork point (upstream pushed). The rule must
	// diff and compare [Unreleased] from the merge base - endpoint semantics would
	// blame upstream files on the effort and let an upstream changelog entry mask the
	// effort's own missing one. The fake maps only merge-base-side keys, so any
	// endpoint-side git call fails the test as an unexpected call.
	same := changelog("\n")
	g := fakeGit{
		"merge-base b h": {out: "m\n"},
		"-c diff.noprefix=false -c diff.mnemonicprefix=false -c diff.dstPrefix=b/ diff --no-ext-diff -U0 m h -- *.go": {out: ""},
		"diff --name-only m h":    {out: "templates/x.tmpl\n"},
		"show m:" + changelogPath: {out: same},
		"show h:" + changelogPath: {out: same},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 0 || !strings.Contains(out, "warning") || !strings.Contains(out, "[Unreleased] is unchanged") {
		t.Fatalf("code=%d out=%q", code, out)
	}
	if !strings.Contains(out, "adopter-facing paths in m..h:") {
		t.Fatalf("considered-paths log must state the merge-base diff basis: %q", out)
	}
}

func TestMergeBaseFails(t *testing.T) {
	g := fakeGit{"merge-base b h": {err: errors.New("boom")}}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	// invariant: repo-audit-error-exit
	if code != 1 || !strings.Contains(out, "git merge-base b h failed") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestDiffFails(t *testing.T) {
	g := fakeGit{
		"merge-base b h": {out: "b\n"},
		"-c diff.noprefix=false -c diff.mnemonicprefix=false -c diff.dstPrefix=b/ diff --no-ext-diff -U0 b h -- *.go": {out: ""},
		"diff --name-only b h": {err: errors.New("boom")},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 1 || !strings.Contains(out, "git diff b..h failed") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestShowBaseFails(t *testing.T) {
	g := fakeGit{
		"merge-base b h": {out: "b\n"},
		"-c diff.noprefix=false -c diff.mnemonicprefix=false -c diff.dstPrefix=b/ diff --no-ext-diff -U0 b h -- *.go": {out: ""},
		"diff --name-only b h":    {out: "templates/x.tmpl\n"},
		"show b:" + changelogPath: {err: errors.New("no file")},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 1 || !strings.Contains(out, "reading "+changelogPath+" at b") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestShowHeadFails(t *testing.T) {
	g := fakeGit{
		"merge-base b h": {out: "b\n"},
		"-c diff.noprefix=false -c diff.mnemonicprefix=false -c diff.dstPrefix=b/ diff --no-ext-diff -U0 b h -- *.go": {out: ""},
		"diff --name-only b h":    {out: "templates/x.tmpl\n"},
		"show b:" + changelogPath: {out: changelog("\n")},
		"show h:" + changelogPath: {err: errors.New("no file")},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 1 || !strings.Contains(out, "reading "+changelogPath+" at h") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestNoUnreleasedSection(t *testing.T) {
	// Base changelog has no [Unreleased] header → extractor error → Error finding.
	g := fakeGit{
		"merge-base b h": {out: "b\n"},
		"-c diff.noprefix=false -c diff.mnemonicprefix=false -c diff.dstPrefix=b/ diff --no-ext-diff -U0 b h -- *.go": {out: ""},
		"diff --name-only b h":    {out: "templates/x.tmpl\n"},
		"show b:" + changelogPath: {out: "# Changelog\n\n## [0.1.0] - 2026-01-01\n- x\n"},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 1 || !strings.Contains(out, "no ## [Unreleased] section") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

// testMarker assembles the directive form without this test file itself
// carrying a directive-shaped line (mirrors the rule's own split literal).
const testMarker = "//" + " coverage-ignore"

func TestCoverageIgnoreAddedWarns(t *testing.T) {
	// An added directive in a production file warns (exit stays 0 - Warning
	// only); an added directive in a _test.go and a bare prose mention do not.
	diff := "+++ b/internal/foo/foo.go\n" +
		"+\tif err != nil { " + testMarker + ": impossible per X\n" +
		"+// the trailing coverage-ignore drops the block\n" +
		"+++ /dev/null\n" +
		"+\tdeleted := 1 " + testMarker + ": must not attribute to a deleted file\n" +
		"+++ b/internal/foo/foo_test.go\n" +
		"+\tx := 1 " + testMarker + ": fixture\n"
	g := fakeGit{
		"merge-base b h":       {out: "b\n"},
		"diff --name-only b h": {out: "docs/x.md\n"},
		"-c diff.noprefix=false -c diff.mnemonicprefix=false -c diff.dstPrefix=b/ diff --no-ext-diff -U0 b h -- *.go": {out: diff},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 0 {
		t.Fatalf("warning-only run must exit 0, got %d: %q", code, out)
	}
	if !strings.Contains(out, "warning") || !strings.Contains(out, "coverage-ignore-added") || !strings.Contains(out, "internal/foo/foo.go") {
		t.Fatalf("missing warning finding: %q", out)
	}
	if !strings.Contains(out, "genuinely untriggerable") {
		t.Fatalf("missing re-evaluation prompt: %q", out)
	}
	if strings.Contains(out, "foo_test.go") {
		t.Fatalf("test-file directive must not fire: %q", out)
	}
	if strings.Count(out, "coverage-ignore-added") != 1 {
		t.Fatalf("prose mention or test file fired: %q", out)
	}
	if strings.Contains(out, "repoaudit: clean") || !strings.Contains(out, "1 warning(s), no errors") {
		t.Fatalf("warning-only run must summarize warnings, not claim clean: %q", out)
	}
}

func TestCoverageIgnoreDiffFails(t *testing.T) {
	// The rule cannot verify on a git failure - loud Error, like the changelog rule.
	g := fakeGit{
		"merge-base b h":       {out: "b\n"},
		"diff --name-only b h": {out: "docs/x.md\n"},
		"-c diff.noprefix=false -c diff.mnemonicprefix=false -c diff.dstPrefix=b/ diff --no-ext-diff -U0 b h -- *.go": {err: errors.New("boom")},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 1 || !strings.Contains(out, "coverage-ignore-added") || !strings.Contains(out, "git diff b..h failed") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestCoverageIgnoreCleanRange(t *testing.T) {
	g := fakeGit{
		"merge-base b h":       {out: "b\n"},
		"diff --name-only b h": {out: "docs/x.md\n"},
		"-c diff.noprefix=false -c diff.mnemonicprefix=false -c diff.dstPrefix=b/ diff --no-ext-diff -U0 b h -- *.go": {out: "+++ b/internal/foo/foo.go\n+\tplain := code()\n"},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 0 || !strings.Contains(out, "repoaudit: clean") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}
