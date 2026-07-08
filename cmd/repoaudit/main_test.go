package main

import (
	"errors"
	"fmt"
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

func TestUsageError(t *testing.T) {
	code, out := runFake([]string{"repoaudit", "no-range-here"}, fakeGit{})
	if code != 2 || !strings.Contains(out, "usage:") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestCleanNonAdopterFacing(t *testing.T) {
	// Default range (no arg) + changes outside the allowlist → clean, exit 0. The
	// blank line between the two paths also exercises changelogRule's empty-token
	// `continue` — the sole branch no other test reaches (100%-coverage gate).
	g := fakeGit{
		"merge-base origin/main HEAD":       {out: "origin/main\n"},
		"diff --name-only origin/main HEAD": {out: "docs/x.md\n\ninternal/render/render.go\n"},
	}
	code, out := runFake([]string{"repoaudit"}, g)
	if code != 0 || !strings.Contains(out, "repoaudit: clean") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestErrorMissingEntry(t *testing.T) {
	// Adopter-facing change, [Unreleased] identical across the range → Error, exit 1.
	same := changelog("\n")
	g := fakeGit{
		"merge-base b h":          {out: "b\n"},
		"diff --name-only b h":    {out: "templates/x.tmpl\n"},
		"show b:" + changelogPath: {out: same},
		"show h:" + changelogPath: {out: same},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 1 || !strings.Contains(out, "[Unreleased] is unchanged") {
		t.Fatalf("code=%d out=%q", code, out)
	}
	if !strings.Contains(out, "adopter-facing paths in b..h: templates/x.tmpl") {
		t.Fatalf("missing considered-paths log: %q", out)
	}
}

func TestCleanEntryAdded(t *testing.T) {
	// Adopter-facing change, [Unreleased] differs across the range → clean, exit 0.
	g := fakeGit{
		"merge-base b h":          {out: "b\n"},
		"diff --name-only b h":    {out: "cmd/awf/root.go\n"},
		"show b:" + changelogPath: {out: changelog("\n")},
		"show h:" + changelogPath: {out: changelog("### Features\n- new thing\n")},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 0 || !strings.Contains(out, "repoaudit: clean") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestDivergedBaseJudgesFromMergeBase(t *testing.T) {
	// Regression: base has moved past the fork point (upstream pushed). The rule must
	// diff and compare [Unreleased] from the merge base — endpoint semantics would
	// blame upstream files on the effort and let an upstream changelog entry mask the
	// effort's own missing one. The fake maps only merge-base-side keys, so any
	// endpoint-side git call fails the test as an unexpected call.
	same := changelog("\n")
	g := fakeGit{
		"merge-base b h":          {out: "m\n"},
		"diff --name-only m h":    {out: "templates/x.tmpl\n"},
		"show m:" + changelogPath: {out: same},
		"show h:" + changelogPath: {out: same},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 1 || !strings.Contains(out, "[Unreleased] is unchanged") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestMergeBaseFails(t *testing.T) {
	g := fakeGit{"merge-base b h": {err: errors.New("boom")}}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 1 || !strings.Contains(out, "git merge-base b h failed") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestDiffFails(t *testing.T) {
	g := fakeGit{
		"merge-base b h":       {out: "b\n"},
		"diff --name-only b h": {err: errors.New("boom")},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 1 || !strings.Contains(out, "git diff b..h failed") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestShowBaseFails(t *testing.T) {
	g := fakeGit{
		"merge-base b h":          {out: "b\n"},
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
		"merge-base b h":          {out: "b\n"},
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
		"merge-base b h":          {out: "b\n"},
		"diff --name-only b h":    {out: "templates/x.tmpl\n"},
		"show b:" + changelogPath: {out: "# Changelog\n\n## [0.1.0] - 2026-01-01\n- x\n"},
	}
	code, out := runFake([]string{"repoaudit", "b..h"}, g)
	if code != 1 || !strings.Contains(out, "no ## [Unreleased] section") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}
