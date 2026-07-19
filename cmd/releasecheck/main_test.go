package main

import (
	"bytes"
	"os"
	"os/exec"
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
	code := run(fsys, &out, &errb, true)
	return code, out.String(), errb.String()
}

func TestRunRefusesIncompleteBridgeTranche(t *testing.T) {
	var out, errb bytes.Buffer
	// Exercise run's incomplete-tranche refusal branch directly with a literal
	// false: project.BridgeTrancheComplete is now true (the tranche landed), so
	// only a forced false still reaches the refusal the 100% gate must cover.
	code := run(changelogFS("not parsed while incomplete"), &out, &errb, false)
	if code != 1 || out.Len() != 0 || !strings.Contains(errb.String(), "Plans 1 and 2 must both land before release") {
		t.Fatalf("incomplete bridge result: code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}

// TestMainClearsBridgeTranche proves the flipped sentinel is wired into
// production main: the binary passes project.BridgeTrancheComplete (now true),
// so it no longer emits the incomplete-tranche refusal. Pre-Phase-5 the
// embedded changelog is not yet promoted, so the binary still exits 1 on the
// changelog pin; that is a transient state Phase 5 fixes, so this asserts only
// that the tranche message is gone, not the exit code or downstream messages.
func TestMainClearsBridgeTranche(t *testing.T) {
	cmd := exec.Command("go", "run", ".")
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	_ = cmd.Run()
	if strings.Contains(errb.String(), "Plans 1 and 2 must both land before release") {
		t.Fatalf("production releasecheck still refuses the completed tranche: stdout=%q stderr=%q", out.String(), errb.String())
	}
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
	// invariant: release-changelog-pin
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
// found but no later "## [" header - body runs to EOF and found stays true.
func TestUnreleasedBodyAtEOF(t *testing.T) {
	body, found := unreleasedBody("# Changelog\n\n## [Unreleased]\n- tail entry\n")
	if !found || !strings.Contains(body, "tail entry") {
		t.Fatalf("want found body through EOF, got found=%v body=%q", found, body)
	}
}

// TestReleaseWorkflowGatesOnTag backs inv: release-gate-on-tag (ADR-0079) - the
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
// inv: release-changelog-pin - the Release workflow must invoke releasecheck
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

// TestReleaseNotesFromCuratedChangelog backs inv: release-notes-from-changelog
// (ADR-0096) - the Release workflow must extract the tagged version's section from
// the curated changelog via `awf changelog --version` before the GoReleaser step and
// pass it through `--release-notes`, and `.goreleaser.yaml` must disable GoReleaser's
// commit-derived changelog, so a commit subject can no longer reach the release notes.
// invariant: release-notes-from-changelog
func TestReleaseNotesFromCuratedChangelog(t *testing.T) {
	wfb, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	wf := string(wfb)
	extract := strings.Index(wf, "awf changelog --version")
	build := strings.Index(wf, "goreleaser/goreleaser-action")
	if extract < 0 {
		t.Error("release.yml does not extract release notes via `awf changelog --version`")
	}
	if build < 0 {
		t.Fatal("release.yml does not run the GoReleaser action")
	}
	if extract > build {
		t.Error("the `awf changelog --version` extraction must run before the GoReleaser step")
	}
	// The extraction redirect and the --release-notes arg must name the same file, or
	// the release body silently diverges from what was written. Assert the extraction line
	// redirects to a RUNNER_TEMP-scoped release-notes.md (outside the worktree, so
	// GoReleaser's dirty-tree check passes) and that the --release-notes arg names the same
	// basename - checking the components rather than one pinned interpolation form.
	const notesFile = "release-notes.md"
	extractLine := wf[extract:]
	if nl := strings.IndexByte(extractLine, '\n'); nl >= 0 {
		extractLine = extractLine[:nl]
	}
	if !strings.Contains(extractLine, ">") || !strings.Contains(extractLine, "RUNNER_TEMP") || !strings.Contains(extractLine, notesFile) {
		t.Errorf("the extraction step must redirect (>) into a RUNNER_TEMP-scoped %s, got %q", notesFile, extractLine)
	}
	relIdx := strings.Index(wf, "--release-notes")
	if relIdx < 0 {
		t.Error("release.yml does not pass --release-notes to the GoReleaser step")
	} else {
		argLine := wf[relIdx:]
		if nl := strings.IndexByte(argLine, '\n'); nl >= 0 {
			argLine = argLine[:nl]
		}
		if !strings.Contains(argLine, notesFile) {
			t.Errorf("--release-notes must point at %s (the file the extraction step writes), got %q", notesFile, argLine)
		}
	}

	glb, err := os.ReadFile("../../.goreleaser.yaml")
	if err != nil {
		t.Fatalf("read goreleaser config: %v", err)
	}
	gl := string(glb)
	// Scope the assertion to the changelog block's stable two-line token, so an unrelated
	// `disable: true` elsewhere cannot mask a revert of the changelog disable.
	if !strings.Contains(gl, "changelog:\n  disable: true") {
		t.Error(".goreleaser.yaml does not disable the commit-derived changelog (changelog:\\n  disable: true)")
	}
	if strings.Contains(gl, "use: github") {
		t.Error(".goreleaser.yaml still derives release notes from commits (use: github)")
	}
}
