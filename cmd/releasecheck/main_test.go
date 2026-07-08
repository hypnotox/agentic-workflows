package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

func changelogFS(content string) fstest.MapFS {
	return fstest.MapFS{"CHANGELOG.md": &fstest.MapFile{Data: []byte(content)}}
}

func runOn(t *testing.T, fsys fstest.MapFS) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(fsys, &out, &errb)
	return code, out.String(), errb.String()
}

func TestRunPasses(t *testing.T) {
	fsys := changelogFS("# Changelog\n\n## [Unreleased]\n\n## [" + project.Version + "] - 2026-07-08\n### Features\n- something\n")
	code, out, errb := runOn(t, fsys)
	if code != 0 {
		t.Fatalf("want exit 0, got %d, stderr:\n%s", code, errb)
	}
	if !strings.Contains(out, "changelog pins "+project.Version) {
		t.Errorf("expected pin confirmation on stdout, got:\n%s", out)
	}
}

func TestRunPassesWhitespaceOnlyUnreleased(t *testing.T) {
	fsys := changelogFS("# Changelog\n\n## [Unreleased]\n\n   \n\n## [" + project.Version + "] - 2026-07-08\n- x\n")
	if code, _, errb := runOn(t, fsys); code != 0 {
		t.Fatalf("blank-line [Unreleased] must count as empty, got exit %d, stderr:\n%s", code, errb)
	}
}

func TestRunFailsMissingFile(t *testing.T) {
	code, _, errb := runOn(t, fstest.MapFS{})
	if code != 1 || !strings.Contains(errb, "read CHANGELOG.md") {
		t.Fatalf("want exit 1 with read error, got %d:\n%s", code, errb)
	}
}

func TestRunFailsUnparseable(t *testing.T) {
	code, _, errb := runOn(t, changelogFS("# Changelog\n\nno version headers here\n"))
	if code != 1 || !strings.Contains(errb, "no version entries") {
		t.Fatalf("want exit 1 with parse error, got %d:\n%s", code, errb)
	}
}

func TestRunFailsStaleNewestEntry(t *testing.T) {
	code, _, errb := runOn(t, changelogFS("# Changelog\n\n## [Unreleased]\n\n## [0.0.1] - 2026-01-01\n- old\n"))
	if code != 1 || !strings.Contains(errb, "promote [Unreleased] before tagging") {
		t.Fatalf("want exit 1 with stale-entry error, got %d:\n%s", code, errb)
	}
}

func TestRunFailsOutOfOrder(t *testing.T) {
	fsys := changelogFS("# Changelog\n\n## [Unreleased]\n\n## [" + project.Version + "] - 2026-07-08\n- x\n\n## [9.9.9] - 2026-01-01\n- misplaced\n")
	code, _, errb := runOn(t, fsys)
	if code != 1 || !strings.Contains(errb, "out of order") {
		t.Fatalf("want exit 1 with ordering error, got %d:\n%s", code, errb)
	}
}

func TestRunFailsMissingUnreleasedHeader(t *testing.T) {
	code, _, errb := runOn(t, changelogFS("# Changelog\n\n## ["+project.Version+"] - 2026-07-08\n- x\n"))
	if code != 1 || !strings.Contains(errb, "no ## [Unreleased] section") {
		t.Fatalf("want exit 1 with missing-header error, got %d:\n%s", code, errb)
	}
}

func TestRunFailsNonEmptyUnreleased(t *testing.T) {
	fsys := changelogFS("# Changelog\n\n## [Unreleased]\n- stranded entry\n\n## [" + project.Version + "] - 2026-07-08\n- x\n")
	code, _, errb := runOn(t, fsys)
	if code != 1 || !strings.Contains(errb, "[Unreleased] is not empty") {
		t.Fatalf("want exit 1 with non-empty error, got %d:\n%s", code, errb)
	}
}

// TestUnreleasedBodyAtEOF covers the header-is-last-section shape: [Unreleased]
// found but no later "## [" header — body runs to EOF and found stays true.
func TestUnreleasedBodyAtEOF(t *testing.T) {
	body, found := unreleasedBody("# Changelog\n\n## [Unreleased]\n- tail entry\n")
	if !found || !strings.Contains(body, "tail entry") {
		t.Fatalf("want found body through EOF, got found=%v body=%q", found, body)
	}
}

// TestReleaseWorkflowGatesOnTag backs inv: release-gate-on-tag (ADR-0079) — the
// Release workflow must run the ancestry check, ./x gate, and ./x check before
// the GoReleaser step, so an untested or off-main tag cannot publish.
// invariant: release-gate-on-tag
func TestReleaseWorkflowGatesOnTag(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	wf := string(b)
	build := strings.Index(wf, "goreleaser/goreleaser-action")
	if build < 0 {
		t.Fatal("release.yml does not run the GoReleaser action")
	}
	for _, step := range []string{
		"git merge-base --is-ancestor HEAD origin/main",
		"run: ./x gate",
		"run: ./x check",
	} {
		idx := strings.Index(wf, step)
		if idx < 0 {
			t.Errorf("release.yml is missing the %q step", step)
			continue
		}
		if idx > build {
			t.Errorf("%q must run before the GoReleaser step", step)
		}
	}
}

// TestReleaseWorkflowRunsReleasecheck backs the wiring half of
// inv: release-changelog-pin — the Release workflow must invoke releasecheck
// before the GoReleaser step, so unwiring the check fails the gate.
func TestReleaseWorkflowRunsReleasecheck(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	wf := string(b)
	check := strings.Index(wf, "go run ./cmd/releasecheck")
	build := strings.Index(wf, "goreleaser/goreleaser-action")
	if check < 0 {
		t.Fatal("release.yml does not invoke releasecheck")
	}
	if build < 0 {
		t.Fatal("release.yml does not run the GoReleaser action")
	}
	if check > build {
		t.Error("releasecheck must run before the GoReleaser step")
	}
}
