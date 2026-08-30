package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGit supplies the repoaudit consumer contract.
type fakeGit map[string]struct {
	out string
	err error
}

func (f fakeGit) result(key string) (string, error) {
	if r, ok := f[key]; ok {
		return r.out, r.err
	}
	return "", fmt.Errorf("unexpected git call: %s", key)
}
func (f fakeGit) MergeBase(_ context.Context, a, b string) (string, error) {
	return f.result("merge-base " + a + " " + b)
}
func (f fakeGit) RangeChangedPaths(_ context.Context, a, b string) ([]string, error) {
	out, err := f.result("diff --name-only " + a + " " + b)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Fields(out), nil
}
func (f fakeGit) FileText(_ context.Context, rev, path string) (string, bool, error) {
	key := "show " + rev + ":" + path
	if result, ok := f[key]; ok {
		return result.out, result.err == nil, result.err
	}
	return "", false, fmt.Errorf("unexpected git call: %s", key)
}

func changelog(unreleased string) string {
	return "# Changelog\n\n## [Unreleased]\n" + unreleased + "## [0.1.0] - 2026-01-01\n### Others\n- x\n"
}

func runFake(args []string, g fakeGit) (int, string) {
	var out, errOut strings.Builder
	code := runWith(context.Background(), args, &out, &errOut, g)
	return code, out.String() + errOut.String()
}

func TestUnreleasedSectionMissingFile(t *testing.T) {
	if _, err := unreleasedSection(context.Background(), missingFileGit{}, "head"); err == nil || !strings.Contains(err.Error(), changelogPath+" not found") {
		t.Fatalf("missing changelog error = %v", err)
	}
}

type missingFileGit struct{ fakeGit }

func (missingFileGit) FileText(context.Context, string, string) (string, bool, error) {
	return "", false, nil
}

// invariant: tooling/audit-commands:repoaudit-requires-explicit-range (TestUsageError)
func TestUsageError(t *testing.T) {
	// No argument at all: there is no default range (ADR-0127 Decision 11), so
	// this is a refusal with the usage line rather than a report over
	// origin/main..HEAD.
	code, out := runFake([]string{"repoaudit"}, fakeGit{})
	if code != 2 || !strings.Contains(out, "usage: repoaudit <base>..<head>") {
		t.Fatalf("no-arg: code=%d out=%q", code, out)
	}
	// A supplied bare base is rejected too: repoaudit does not opt into
	// ParseRange's bare-base form.
	code, out = runFake([]string{"repoaudit", "no-range-here"}, fakeGit{})
	if code != 2 || !strings.Contains(out, "must be <a>..<b>") {
		t.Fatalf("bare base: code=%d out=%q", code, out)
	}
}

func TestMainValidatesRangeBeforeOpeningRepository(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "repoaudit")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", exe, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build repoaudit: %v\n%s", err, out)
	}
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "missing", want: "usage: repoaudit <base>..<head>"},
		{name: "malformed", args: []string{"no-range-here"}, want: "must be <a>..<b>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), exe, tc.args...)
			cmd.Dir = t.TempDir()
			out, err := cmd.CombinedOutput()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 || !strings.Contains(string(out), tc.want) {
				t.Fatalf("code/output = %v, %q; want exit 2 containing %q", err, out, tc.want)
			}
		})
	}
}

func TestRejectsMalformedRanges(t *testing.T) {
	// strings.Cut on ".." would silently mangle these (b...h → head ".h";
	// a..b..c → head "b..c") and hand git a bogus rev, and a "-"-prefixed
	// side would reach git as an option-like argument; all must be refused.
	// The guards now live in internal/git.ParseRange (ADR-0127 Decision 5), so
	// the refusal reads "repoaudit: range ..." rather than the usage line.
	// Dots inside a rev (v0.10.0..HEAD) stay legal.
	for _, rng := range []string{"b...h", "a..b..c", "-foo..HEAD", "b..--all"} {
		code, out := runFake([]string{"repoaudit", rng}, fakeGit{})
		if code != 2 || !strings.Contains(out, "repoaudit: range") {
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
	// Explicit range + changes outside the allowlist → clean, exit 0. The
	// blank line between the two paths also exercises changelogRule's empty-token
	// `continue` - the sole branch no other test reaches (100%-coverage gate).
	g := fakeGit{
		"merge-base origin/main HEAD": {out: "origin/main\n"},
		"-c diff.noprefix=false -c diff.mnemonicprefix=false -c diff.dstPrefix=b/ diff --no-ext-diff -U0 origin/main HEAD -- *.go": {out: ""},
		// internal/testsupport/ is a source root that ships no adopter-visible
		// behaviour, so it stays outside the allowlist that internal/render/
		// joined once render-logic changes were recognised as adopter-visible.
		"diff --name-only origin/main HEAD": {out: "docs/x.md\n\ninternal/testsupport/fixture.go\n"},
	}
	code, out := runFake([]string{"repoaudit", "origin/main..HEAD"}, g)
	if code != 0 || !strings.Contains(out, "repoaudit: clean") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

// The rendered rank column is asserted here, not only in internal/severity: that
// test cannot reach this surface, because repoaudit's finding type is unexported
// in package main. Without this marker the claim would name a surface no proof
// reads.
// invariant: tooling/audit-commands:severity-single-spelling (TestWarnMissingEntry)
func TestWarnMissingEntry(t *testing.T) {
	// Adopter-facing change, [Unreleased] identical across the range -> advisory warn,
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
	// invariant: tooling/changelog-and-release:changelog-rule-advisory (TestWarnMissingEntry)
	if code != 0 || !strings.Contains(out, "warn    changelog-unreleased") || !strings.Contains(out, "[Unreleased] is unchanged") {
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
	if code != 0 || !strings.Contains(out, "warn    changelog-unreleased") || !strings.Contains(out, "[Unreleased] is unchanged") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestBehaviourPackagesAreAdopterFacing(t *testing.T) {
	// Regression: the allowlist covered the catalog and the schema but not the
	// packages that decide what the shipped commands answer, so a real
	// adopter-visible change slipped it. An authority query began reporting an
	// in-flight decision record as frozen, fixed in internal/adr and
	// internal/project, and this rule stayed silent because neither root was
	// listed. Each root is asserted separately: one shared case would pass
	// while the others stayed missing.
	for _, path := range []string{
		"internal/adr/status.go",
		"internal/project/context_adr.go",
		"internal/render/render.go",
		"internal/effort/service.go",
		"internal/worktree/manager.go",
	} {
		t.Run(path, func(t *testing.T) {
			same := changelog("\n")
			g := fakeGit{
				"merge-base b h": {out: "b\n"},
				"-c diff.noprefix=false -c diff.mnemonicprefix=false -c diff.dstPrefix=b/ diff --no-ext-diff -U0 b h -- *.go": {out: ""},
				"diff --name-only b h":    {out: path + "\n"},
				"show b:" + changelogPath: {out: same},
				"show h:" + changelogPath: {out: same},
			}
			code, out := runFake([]string{"repoaudit", "b..h"}, g)
			if code != 0 || !strings.Contains(out, "warn    changelog-unreleased") {
				t.Fatalf("%s: code=%d out=%q", path, code, out)
			}
		})
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
	if code != 0 || !strings.Contains(out, "warn    changelog-unreleased") || !strings.Contains(out, "[Unreleased] is unchanged") {
		t.Fatalf("code=%d out=%q", code, out)
	}
	if !strings.Contains(out, "adopter-facing paths in m..h:") {
		t.Fatalf("considered-paths log must state the merge-base diff basis: %q", out)
	}
}

func TestMergeBaseFails(t *testing.T) {
	g := fakeGit{"merge-base b h": {err: errors.New("boom")}}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	// invariant: tooling/audit-and-snapshots:repo-audit-error-exit (TestMergeBaseFails)
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
