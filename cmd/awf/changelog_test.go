package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	changelogfs "github.com/hypnotox/agentic-workflows/changelog"
	"github.com/hypnotox/agentic-workflows/internal/changelog"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"golang.org/x/mod/semver"
)

// TestChangelogMonotonicOrder keeps the changelog ordered without pinning it to the
// version const (ADR-0078): entries must be strictly descending by semver and the
// newest entry must not exceed project.Version. Mid-cycle the const may run ahead of
// the changelog - entries accumulate under [Unreleased] until the release-prep
// promotion - and the exact match is enforced at release time by cmd/releasecheck.
// invariant: tooling/changelog-and-release:changelog-monotonic (TestChangelogMonotonicOrder)
func TestChangelogMonotonicOrder(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	entries, err := changelog.Load(changelogfs.FS)
	if err != nil {
		t.Fatalf("load embedded changelog: %v", err)
	}
	if semver.Compare("v"+entries[0].Version, "v"+project.Version) > 0 {
		t.Errorf("newest changelog entry %q is ahead of project.Version %q - the binary cannot carry notes for a version it does not know",
			entries[0].Version, project.Version)
	}
	for i := 0; i+1 < len(entries); i++ {
		if semver.Compare("v"+entries[i].Version, "v"+entries[i+1].Version) <= 0 {
			t.Errorf("changelog entries out of order: %q is not strictly newer than %q",
				entries[i].Version, entries[i+1].Version)
		}
	}
}

func TestRunChangelogNoFlags(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	var out bytes.Buffer
	if err := runChangelog("", "", "", &out); err != nil {
		t.Fatalf("runChangelog: %v", err)
	}
	got := out.String()
	want, err := changelogfs.FS.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("no-flags payload differs from authored changelog")
	}
}

func TestRunChangelogVersion(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
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
	ctx := testContext(t)
	_ = ctx
	var out bytes.Buffer
	if err := runChangelog("9.9.9", "", "", &out); err == nil {
		t.Fatal("an unmatched --version should error")
	}
}

func TestRunChangelogSinceUnmatched(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	var out bytes.Buffer
	if err := runChangelog("", "9.9.9", "", &out); err == nil {
		t.Fatal("an unmatched --since should error")
	}
}

func TestRunChangelogSince(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
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

type changelogErrorWriter struct{}

func (changelogErrorWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRunChangelogPayloadWriteFailures(t *testing.T) {
	for _, test := range []struct {
		name       string
		since, rng string
	}{
		{name: "since", since: "0.3.1"},
		{name: "range", rng: "0.2.0..0.4.0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := runChangelog("", test.since, test.rng, changelogErrorWriter{}); err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("write error = %v", err)
			}
		})
	}
}

func TestRunChangelogSinceLatest(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	entries, err := changelog.Load(changelogfs.FS)
	if err != nil {
		t.Fatalf("load embedded changelog: %v", err)
	}
	// --since the newest entry yields nothing newer; derived from the changelog
	// itself, not project.Version, which may run ahead mid-cycle (ADR-0078).
	latest := entries[0].Version
	var out bytes.Buffer
	if err := runChangelog("", latest, "", &out); err != nil {
		t.Fatalf("runChangelog: %v", err)
	}
	want := "no releases since " + latest
	if !strings.Contains(out.String(), want) {
		t.Errorf("expected %q, got:\n%s", want, out.String())
	}
}

func TestRunChangelogRange(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
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
	ctx := testContext(t)
	_ = ctx
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
	ctx := testContext(t)
	_ = ctx
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
	ctx := testContext(t)
	_ = ctx
	var out bytes.Buffer
	err := runChangelog("0.2.0", "0.3.0", "", &out)
	// invariant: tooling/changelog-and-release:changelog-flags-exclusive (TestRunChangelogFlagsExclusive)
	if err == nil {
		t.Fatal("setting both --version and --since should error")
	}
	var ue *usageErr
	if !errors.As(err, &ue) {
		t.Errorf("mutual-exclusion violation should be a usageErr, got %T: %v", err, err)
	}
}

// TestChangelogPublicPayloadContracts pins every public payload form at the
// driver boundary: authored bytes go only to stdout and successful payloads
// never add diagnostics or alter the exit status.
func TestChangelogPublicPayloadContracts(t *testing.T) {
	readGolden := func(t *testing.T, name string) []byte {
		t.Helper()
		got, err := os.ReadFile(filepath.Join("testdata", "changelog", name))
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	full, err := changelogfs.FS.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := changelog.Load(changelogfs.FS)
	if err != nil {
		t.Fatal(err)
	}
	latest := entries[0].Version
	for _, tc := range []struct {
		name string
		args []string
		want []byte
	}{
		{"full", []string{"awf", "changelog"}, full},
		{"version", []string{"awf", "changelog", "--version", "0.2.0"}, readGolden(t, "version-0.2.0.md")},
		{"since", []string{"awf", "changelog", "--since", "0.18.0"}, []byte(entries[0].Raw + "\n")},
		{"range", []string{"awf", "changelog", "--range", "0.2.0..0.4.0"}, append(readGolden(t, "range-0.2.0-0.4.0.md"), '\n')},
		{"empty-since", []string{"awf", "changelog", "--since", latest}, []byte("status: no releases since " + latest + "\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tc.args, &stdout, &stderr); code != 0 || !bytes.Equal(stdout.Bytes(), tc.want) || stderr.Len() != 0 {
				t.Fatalf("run(%v): exit=%d stdout=%q stderr=%q", tc.args, code, stdout.String(), stderr.String())
			}
		})
	}
}
