package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	changelogfs "github.com/hypnotox/agentic-workflows/changelog"
	"github.com/hypnotox/agentic-workflows/internal/changelog"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

// TestChangelogLatestMatchesVersion pins the binary and its changelog together:
// the newest embedded entry must be project.Version, so a version bump without a
// matching changelog entry (or vice versa) fails the gate.
func TestChangelogLatestMatchesVersion(t *testing.T) {
	entries, err := changelog.Load(changelogfs.FS)
	if err != nil {
		t.Fatalf("load embedded changelog: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("embedded changelog has no entries")
	}
	if entries[0].Version != project.Version {
		t.Errorf("newest changelog entry %q != project.Version %q — bump one to match the other",
			entries[0].Version, project.Version)
	}
}

func TestRunChangelogNoFlags(t *testing.T) {
	var out bytes.Buffer
	if err := runChangelog("", "", "", &out); err != nil {
		t.Fatalf("runChangelog: %v", err)
	}
	got := out.String()
	if !strings.HasPrefix(got, "# Changelog") {
		t.Errorf("no-flags output should start with the file title, got:\n%s", got[:min(40, len(got))])
	}
	if !strings.Contains(got, "[0.1.0]") || !strings.Contains(got, "[0.5.1]") {
		t.Errorf("no-flags output should contain every backfilled version, got:\n%s", got)
	}
}

func TestRunChangelogVersion(t *testing.T) {
	var out bytes.Buffer
	if err := runChangelog("0.2.0", "", "", &out); err != nil {
		t.Fatalf("runChangelog: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "[0.2.0]") {
		t.Errorf("expected the 0.2.0 entry, got:\n%s", got)
	}
	if strings.Contains(got, "[0.3.0]") {
		t.Errorf("--version 0.2.0 should not include a neighboring version, got:\n%s", got)
	}
}

func TestRunChangelogVersionUnmatched(t *testing.T) {
	var out bytes.Buffer
	if err := runChangelog("9.9.9", "", "", &out); err == nil {
		t.Fatal("an unmatched --version should error")
	}
}

func TestRunChangelogSinceUnmatched(t *testing.T) {
	var out bytes.Buffer
	if err := runChangelog("", "9.9.9", "", &out); err == nil {
		t.Fatal("an unmatched --since should error")
	}
}

func TestRunChangelogSince(t *testing.T) {
	var out bytes.Buffer
	if err := runChangelog("", "0.3.1", "", &out); err != nil {
		t.Fatalf("runChangelog: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "[0.3.1]") {
		t.Errorf("--since is exclusive of its own version, got:\n%s", got)
	}
	if !strings.Contains(got, "[0.4.0]") || !strings.Contains(got, "[0.5.1]") {
		t.Errorf("expected every version after 0.3.1, got:\n%s", got)
	}
}

func TestRunChangelogSinceLatest(t *testing.T) {
	var out bytes.Buffer
	// --since the latest release (project.Version, per the agreement pinned above)
	// yields nothing newer; deriving it avoids a hand-updated version literal.
	if err := runChangelog("", project.Version, "", &out); err != nil {
		t.Fatalf("runChangelog: %v", err)
	}
	want := "no releases since " + project.Version
	if !strings.Contains(out.String(), want) {
		t.Errorf("expected %q, got:\n%s", want, out.String())
	}
}

func TestRunChangelogRange(t *testing.T) {
	var out bytes.Buffer
	if err := runChangelog("", "", "0.2.0..0.4.0", &out); err != nil {
		t.Fatalf("runChangelog: %v", err)
	}
	got := out.String()
	for _, v := range []string{"[0.2.0]", "[0.3.0]", "[0.3.1]", "[0.4.0]"} {
		if !strings.Contains(got, v) {
			t.Errorf("range output missing %s, got:\n%s", v, got)
		}
	}
	if strings.Contains(got, "[0.5.0]") {
		t.Errorf("range output should stop at 0.4.0, got:\n%s", got)
	}
}

func TestRunChangelogRangeMissingSeparator(t *testing.T) {
	var out bytes.Buffer
	err := runChangelog("", "", "0.2.0", &out)
	if err == nil {
		t.Fatal("a --range without \"..\" should error")
	}
	var ue *usageErr
	if !errors.As(err, &ue) {
		t.Errorf("missing-separator --range should be a usageErr, got %T: %v", err, err)
	}
}

func TestRunChangelogRangeReversed(t *testing.T) {
	var out bytes.Buffer
	err := runChangelog("", "", "0.4.0..0.2.0", &out)
	if err == nil {
		t.Fatal("a reversed --range should error")
	}
	var ue *usageErr
	if errors.As(err, &ue) {
		t.Error("a reversed --range is a runtime error, not a usageErr")
	}
}

func TestRunChangelogFlagsExclusive(t *testing.T) {
	var out bytes.Buffer
	err := runChangelog("0.2.0", "0.3.0", "", &out)
	if err == nil {
		t.Fatal("setting both --version and --since should error")
	}
	var ue *usageErr
	if !errors.As(err, &ue) {
		t.Errorf("mutual-exclusion violation should be a usageErr, got %T: %v", err, err)
	}
}

func TestDispatchChangelog(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "changelog", "--version", "0.1.0"}, &out, &errb); code != 0 {
		t.Fatalf("dispatch changelog: code=%d err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "[0.1.0]") {
		t.Errorf("dispatch changelog --version 0.1.0 missing its entry, got:\n%s", out.String())
	}
}
