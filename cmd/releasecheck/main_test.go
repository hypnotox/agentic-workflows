package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"gopkg.in/yaml.v3"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func changelogFS(content string) fstest.MapFS {
	return fstest.MapFS{"CHANGELOG.md": &fstest.MapFile{Data: []byte(content)}}
}

func runOn(t *testing.T, fsys fstest.MapFS) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(os.DirFS("../.."), fsys, &out, &errb)
	return code, out.String(), errb.String()
}

func TestReleaseVersionProbeContract(t *testing.T) {
	workflow, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	const start = "want=\"$(go run ./cmd/awf version | awk '\n"
	startAt := strings.Index(string(workflow), start)
	if startAt < 0 {
		t.Fatal("release workflow is missing the labeled version parser")
	}
	rest := string(workflow[startAt+len(start):])
	program, _, ok := strings.Cut(rest, "\n          ')\"")
	if !ok {
		t.Fatal("release workflow version parser has no closing command substitution")
	}

	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{"core version", "version: " + project.Version + "\n", project.Version + "\n"},
		{"display provenance", "version: " + project.Version + " (v9.9.9-pre, rev abc123)\n", project.Version + "\n"},
		{"missing", "", ""},
		{"duplicate", "version: " + project.Version + "\nversion: " + project.Version + "\n", ""},
		{"malformed label", "version : " + project.Version + "\n", ""},
		{"legacy unlabeled", "awf " + project.Version + "\n", ""},
		{"malformed provenance", "version: " + project.Version + " provenance\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("awk", program)
			cmd.Stdin = strings.NewReader(tc.input)
			got, runErr := cmd.Output()
			if tc.want == "" {
				if runErr == nil {
					t.Fatal("invalid version output was accepted")
				}
				if len(got) != 0 {
					t.Errorf("invalid version output produced %q", got)
				}
				return
			}
			if runErr != nil {
				t.Fatalf("valid version output failed: %v", runErr)
			}
			if string(got) != tc.want {
				t.Errorf("parsed version = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRunFailsProjectLicense(t *testing.T) {
	var out, errb bytes.Buffer
	code := run(fstest.MapFS{}, changelogFS("not read after project-license failure"), &out, &errb)
	if code != 1 || out.Len() != 0 || !strings.Contains(errb.String(), "project license: read LICENSE") {
		t.Fatalf("project-license result: code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
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
	// invariant: tooling/changelog-and-release:release-changelog-pin (TestRunFailsStaleNewestEntry)
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
// invariant: tooling/changelog-and-release:release-gate-on-tag (TestReleaseWorkflowGatesOnTag)
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
		"./x gate full",
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
// invariant: tooling/changelog-and-release:release-notes-from-changelog (TestReleaseNotesFromCuratedChangelog)
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

func TestVerifyCIRequiresExactSuccessfulWorkflowAndJobs(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/acme/repo/actions/workflows/ci.yml/runs":
			if r.URL.Query().Get("head_sha") != sha {
				t.Errorf("head_sha = %q", r.URL.Query().Get("head_sha"))
			}
			fmt.Fprintf(w, `{"total_count":1,"workflow_runs":[{"id":7,"head_sha":%q,"status":"completed","conclusion":"success","path":".github/workflows/ci.yml","name":"CI"}]}`, sha)
		case "/repos/acme/repo/actions/runs/7/jobs":
			_, _ = io.WriteString(w, `{"total_count":2,"jobs":[{"name":"gate","status":"completed","conclusion":"success"},{"name":"release-config","status":"completed","conclusion":"success"}]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	if err := verifyCI(context.Background(), server.Client(), server.URL, "acme/repo", "token", sha); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyCIAcceptsAnyCompleteExactSuccessfulRun(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/repo/actions/workflows/ci.yml/runs":
			fmt.Fprintf(w, `{"total_count":2,"workflow_runs":[{"id":7,"head_sha":%q,"status":"completed","conclusion":"success","path":".github/workflows/ci.yml","name":"CI"},{"id":8,"head_sha":%q,"status":"completed","conclusion":"success","path":".github/workflows/ci.yml","name":"CI"}]}`, sha, sha)
		case "/repos/acme/repo/actions/runs/7/jobs":
			_, _ = io.WriteString(w, `{"total_count":2,"jobs":[{"name":"gate","status":"completed","conclusion":"failure"},{"name":"release-config","status":"completed","conclusion":"success"}]}`)
		case "/repos/acme/repo/actions/runs/8/jobs":
			_, _ = io.WriteString(w, `{"total_count":2,"jobs":[{"name":"gate","status":"completed","conclusion":"success"},{"name":"release-config","status":"completed","conclusion":"success"}]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	if err := verifyCI(context.Background(), server.Client(), server.URL, "acme/repo", "token", sha); err != nil {
		t.Fatalf("equivalent exact-SHA rerun evidence rejected: %v", err)
	}
}

func TestVerifyCIPreservesCandidateErrorIdentity(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	transportErr := errors.New("jobs transport failed")
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/actions/runs/7/jobs") {
			return nil, transportErr
		}
		body := fmt.Sprintf(`{"total_count":1,"workflow_runs":[{"id":7,"head_sha":%q,"status":"completed","conclusion":"success","path":".github/workflows/ci.yml","name":"CI"}]}`, sha)
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}

	err := verifyCI(context.Background(), client, "https://api.example.test", "acme/repo", "token", sha)
	if !errors.Is(err, transportErr) {
		t.Fatalf("candidate error identity lost: %v", err)
	}
}

func TestVerifyCIRefusesIncompleteOrWrongEvidence(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for name, response := range map[string]string{
		"nearby SHA":        `{"total_count":1,"workflow_runs":[{"id":7,"head_sha":"bbbb","status":"completed","conclusion":"success","path":".github/workflows/ci.yml","name":"CI"}]}`,
		"wrong workflow":    `{"total_count":1,"workflow_runs":[{"id":7,"head_sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","status":"completed","conclusion":"success","path":"other.yml","name":"CI"}]}`,
		"failed conclusion": `{"total_count":1,"workflow_runs":[{"id":7,"head_sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","status":"completed","conclusion":"failure","path":".github/workflows/ci.yml","name":"CI"}]}`,
		"pagination count":  `{"total_count":2,"workflow_runs":[{"id":7,"head_sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","status":"completed","conclusion":"success","path":".github/workflows/ci.yml","name":"CI"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, response) }))
			defer s.Close()
			if err := verifyCI(context.Background(), s.Client(), s.URL, "a/r", "t", sha); err == nil {
				t.Fatal("invalid CI evidence accepted")
			}
		})
	}
}

// invariant: tooling/quality-gates:exact-revision-repository-acceptance (TestExactRevisionWorkflowContract)
func TestVerifyCIRefusesTransportAndEvidenceFaults(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	validRuns := fmt.Sprintf(`{"total_count":1,"workflow_runs":[{"id":7,"head_sha":%q,"status":"completed","conclusion":"success","path":".github/workflows/ci.yml","name":"CI"}]}`, sha)
	validJobs := `{"total_count":2,"jobs":[{"name":"gate","status":"completed","conclusion":"success"},{"name":"release-config","status":"completed","conclusion":"success"}]}`

	if err := verifyCI(context.Background(), http.DefaultClient, "://bad", "a/r", "t", sha); err == nil {
		t.Fatal("invalid API URL accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := verifyCI(ctx, http.DefaultClient, "https://example.invalid", "a/r", "t", sha); err == nil {
		t.Fatal("canceled API request accepted")
	}

	for _, tc := range []struct {
		name       string
		runsStatus int
		runsLink   bool
		runsBody   string
		jobsStatus int
		jobsBody   string
		wantErr    bool
	}{
		{name: "runs status", runsStatus: http.StatusBadGateway, wantErr: true},
		{name: "runs link pagination", runsLink: true, runsBody: validRuns, wantErr: true},
		{name: "runs malformed JSON", runsBody: `{`, wantErr: true},
		{name: "jobs status", runsBody: validRuns, jobsStatus: http.StatusBadGateway, wantErr: true},
		{name: "jobs count", runsBody: validRuns, jobsBody: `{"total_count":3,"jobs":[]}`, wantErr: true},
		{name: "failed required job", runsBody: validRuns, jobsBody: `{"total_count":2,"jobs":[{"name":"gate","status":"completed","conclusion":"failure"},{"name":"release-config","status":"completed","conclusion":"success"}]}`, wantErr: true},
		{name: "duplicate required job", runsBody: validRuns, jobsBody: `{"total_count":3,"jobs":[{"name":"gate","status":"completed","conclusion":"success"},{"name":"gate","status":"completed","conclusion":"success"},{"name":"release-config","status":"completed","conclusion":"success"}]}`, wantErr: true},
		{name: "missing required job", runsBody: validRuns, jobsBody: `{"total_count":1,"jobs":[{"name":"gate","status":"completed","conclusion":"success"}]}`, wantErr: true},
		{name: "irrelevant job ignored", runsBody: validRuns, jobsBody: `{"total_count":3,"jobs":[{"name":"other","status":"completed","conclusion":"failure"},{"name":"gate","status":"completed","conclusion":"success"},{"name":"release-config","status":"completed","conclusion":"success"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				isJobs := strings.Contains(r.URL.Path, "/runs/7/jobs")
				status, body := tc.runsStatus, tc.runsBody
				if isJobs {
					status, body = tc.jobsStatus, tc.jobsBody
				} else if tc.runsLink {
					w.Header().Set("Link", "<next>; rel=next")
				}
				if status == 0 {
					status = http.StatusOK
				}
				if body == "" {
					if isJobs {
						body = validJobs
					} else {
						body = validRuns
					}
				}
				w.WriteHeader(status)
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			err := verifyCI(context.Background(), server.Client(), server.URL, "a/r", "t", sha)
			if (err != nil) != tc.wantErr {
				t.Fatalf("verifyCI error = %v, want error %t", err, tc.wantErr)
			}
		})
	}
}

func TestExactRevisionWorkflowContract(t *testing.T) {
	read := func(name string) map[string]any {
		t.Helper()
		body, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", name))
		if err != nil {
			t.Fatal(err)
		}
		return parseWorkflow(t, body)
	}
	ci, release := read("ci.yml"), read("release.yml")
	if problems := exactRevisionWorkflowProblems(ci, release); len(problems) != 0 {
		t.Fatalf("workflow contract problems: %q", problems)
	}
	for _, mutation := range []struct {
		name  string
		apply func(map[string]any, map[string]any)
	}{
		{"missing stable gate job", func(ci, _ map[string]any) { delete(workflowJobs(ci), "gate") }},
		{"missing stable release-config job", func(ci, _ map[string]any) { delete(workflowJobs(ci), "release-config") }},
		{"missing macOS dependency", func(ci, _ map[string]any) { workflowMap(workflowJobs(ci)["gate"])["needs"] = []any{"linux-full"} }},
		{"wrong Linux architecture", func(ci, _ map[string]any) {
			workflowStep(workflowMap(workflowJobs(ci)["linux-full"]), "Verify native Linux amd64")["run"] = "false"
		}},
		{"wrong macOS architecture", func(ci, _ map[string]any) {
			workflowStep(workflowMap(workflowJobs(ci)["macos-go"]), "Verify native macOS arm64")["run"] = "false"
		}},
		{"missing exact SHA verification", func(_, release map[string]any) {
			workflowStep(workflowMap(workflowJobs(release)["verify"]), "Verify bridge readiness and exact CI conclusions")["run"] = "false"
		}},
		{"release uses fast gate", func(_, release map[string]any) {
			workflowStep(workflowMap(workflowJobs(release)["verify"]), "Gate (full release assurance)")["run"] = "./x gate"
		}},
		{"release full gate lacks range", func(_, release map[string]any) {
			workflowStep(workflowMap(workflowJobs(release)["verify"]), "Gate (full release assurance)")["run"] = "./x gate full"
		}},
		{"release checkout is not exact", func(_, release map[string]any) {
			workflowMap(workflowMap(workflowSteps(workflowMap(workflowJobs(release)["verify"]))[0])["with"])["ref"] = "HEAD"
		}},
		{"duplicate macOS full lane", func(ci, _ map[string]any) {
			workflowMap(workflowSteps(workflowMap(workflowJobs(ci)["macos-go"]))[len(workflowSteps(workflowMap(workflowJobs(ci)["macos-go"])))-1])["run"] = "./x gate full"
		}},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			cloneCI, cloneRelease := cloneWorkflow(t, ci), cloneWorkflow(t, release)
			mutation.apply(cloneCI, cloneRelease)
			if problems := exactRevisionWorkflowProblems(cloneCI, cloneRelease); len(problems) == 0 {
				t.Fatal("structural mutation was accepted")
			}
		})
	}
}

func parseWorkflow(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var workflow map[string]any
	if err := yaml.Unmarshal(body, &workflow); err != nil {
		t.Fatal(err)
	}
	return workflow
}

func cloneWorkflow(t *testing.T, workflow map[string]any) map[string]any {
	t.Helper()
	body, err := yaml.Marshal(workflow)
	if err != nil {
		t.Fatal(err)
	}
	return parseWorkflow(t, body)
}

func exactRevisionWorkflowProblems(ci, release map[string]any) []string {
	var problems []string
	if ci["name"] != "CI" {
		problems = append(problems, "CI name")
	}
	ciJobs, releaseJobs := workflowJobs(ci), workflowJobs(release)
	for _, name := range []string{"linux-full", "macos-go", "gate", "release-config"} {
		if _, ok := ciJobs[name]; !ok {
			problems = append(problems, "CI job "+name)
		}
	}
	linux, macOS, gate := workflowMap(ciJobs["linux-full"]), workflowMap(ciJobs["macos-go"]), workflowMap(ciJobs["gate"])
	if !workflowNeeds(gate, "linux-full") || !workflowNeeds(gate, "macos-go") {
		problems = append(problems, "gate native dependencies")
	}
	if !strings.Contains(stringValue(workflowStep(linux, "Verify native Linux amd64")["run"]), `go env GOOS)" = linux`) || !strings.Contains(stringValue(workflowStep(linux, "Verify native Linux amd64")["run"]), `go env GOARCH)" = amd64`) {
		problems = append(problems, "Linux amd64 assertion")
	}
	if !strings.Contains(stringValue(workflowStep(linux, "Gate (full Linux assurance)")["run"]), "./x gate full --range") {
		problems = append(problems, "Linux full gate")
	}
	if !strings.Contains(stringValue(workflowStep(macOS, "Verify native macOS arm64")["run"]), `go env GOOS)" = darwin`) || !strings.Contains(stringValue(workflowStep(macOS, "Verify native macOS arm64")["run"]), `go env GOARCH)" = arm64`) {
		problems = append(problems, "macOS arm64 assertion")
	}
	macOSNativeGo := false
	for _, step := range workflowSteps(macOS) {
		macOSNativeGo = macOSNativeGo || strings.Contains(stringValue(workflowMap(step)["run"]), "go test ./...")
	}
	if !macOSNativeGo {
		problems = append(problems, "macOS native Go lane")
	}
	for _, step := range workflowSteps(macOS) {
		run := stringValue(workflowMap(step)["run"])
		if strings.Contains(run, "./x gate") || strings.Contains(run, "coverage") || strings.Contains(run, "pi-test") {
			problems = append(problems, "duplicated macOS platform-independent lane")
			break
		}
	}
	verify, publish := workflowMap(releaseJobs["verify"]), workflowMap(releaseJobs["publish"])
	verifyRun := stringValue(workflowStep(verify, "Verify bridge readiness and exact CI conclusions")["run"])
	if !strings.Contains(verifyRun, `go run ./cmd/releasecheck --verify-ci "${{ github.sha }}"`) {
		problems = append(problems, "exact CI verification")
	}
	if !workflowPermissions(verify, "actions", "read") || !workflowPermissions(verify, "contents", "read") {
		problems = append(problems, "read-only verification")
	}
	if !workflowNeeds(publish, "verify") || !workflowPermissions(publish, "contents", "write") {
		problems = append(problems, "needs-bound publication")
	}
	for _, job := range []map[string]any{verify, publish} {
		checkout := workflowMap(workflowSteps(job)[0])
		if workflowMap(checkout["with"])["ref"] != "${{ github.sha }}" {
			problems = append(problems, "checkout exact SHA")
		}
	}
	identity := stringValue(workflowStep(publish, "Repeat tag identity before publication")["run"])
	if !strings.Contains(identity, "git rev-parse HEAD") || !strings.Contains(identity, "${GITHUB_REF_NAME}^{}") || !strings.Contains(identity, "${{ github.sha }}") {
		problems = append(problems, "publication tag identity")
	}
	releaseGate := stringValue(workflowStep(verify, "Gate (full release assurance)")["run"])
	if !strings.Contains(releaseGate, "./x gate full --range") || !strings.Contains(releaseGate, "git tag --merged") || !strings.Contains(releaseGate, "sort -Vu") || !strings.Contains(releaseGate, "${previous:-invalid-base}") {
		problems = append(problems, "release full-gate range")
	}
	if strings.Contains(releaseGate, "covercheck-mutants") {
		problems = append(problems, "obsolete standalone mutation blocker")
	}
	for name, raw := range releaseJobs {
		job := workflowMap(raw)
		for _, rawStep := range workflowSteps(job) {
			step := workflowMap(rawStep)
			if strings.HasPrefix(stringValue(step["uses"]), "goreleaser/goreleaser-action@") && (name != "publish" || !workflowNeeds(job, "verify") || !workflowPermissions(job, "contents", "write")) {
				problems = append(problems, "GoReleaser publication bypass")
			}
		}
	}
	return problems
}

func workflowMap(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value.(map[string]any)
}
func workflowJobs(workflow map[string]any) map[string]any { return workflowMap(workflow["jobs"]) }
func workflowSteps(job any) []any                         { steps, _ := workflowMap(job)["steps"].([]any); return steps }
func workflowStep(job any, name string) map[string]any {
	for _, raw := range workflowSteps(job) {
		step := workflowMap(raw)
		if step["name"] == name {
			return step
		}
	}
	return map[string]any{}
}
func stringValue(value any) string { text, _ := value.(string); return text }
func workflowPermissions(job map[string]any, key, want string) bool {
	return workflowMap(job["permissions"])[key] == want
}
func workflowNeeds(job map[string]any, want string) bool {
	switch needs := job["needs"].(type) {
	case string:
		return needs == want
	case []any:
		for _, need := range needs {
			if need == want {
				return true
			}
		}
	}
	return false
}

func TestDispatchRoutesLocalAndExactCIModes(t *testing.T) {
	var out, errb bytes.Buffer
	if code := dispatch(nil, fstest.MapFS{}, fstest.MapFS{}, &out, &errb, http.DefaultClient, "https://api.github.com", func(string) string { return "" }); code != 1 || !strings.Contains(errb.String(), "project license") {
		t.Fatalf("local dispatch = %d, %q", code, errb.String())
	}
	errb.Reset()
	if code := dispatch([]string{"--verify-ci"}, fstest.MapFS{}, fstest.MapFS{}, &out, &errb, http.DefaultClient, "https://api.github.com", func(string) string { return "" }); code != 2 || !strings.Contains(errb.String(), "usage:") {
		t.Fatalf("malformed dispatch = %d, %q", code, errb.String())
	}
	errb.Reset()
	if code := dispatch([]string{"--verify-ci", "sha"}, fstest.MapFS{}, fstest.MapFS{}, &out, &errb, http.DefaultClient, "https://api.github.com", func(string) string { return "" }); code != 1 || !strings.Contains(errb.String(), "required") {
		t.Fatalf("offline verify = %d, %q", code, errb.String())
	}

	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/repo/actions/workflows/ci.yml/runs":
			fmt.Fprintf(w, `{"total_count":1,"workflow_runs":[{"id":7,"head_sha":%q,"status":"completed","conclusion":"success","path":".github/workflows/ci.yml","name":"CI"}]}`, sha)
		case "/repos/acme/repo/actions/runs/7/jobs":
			_, _ = io.WriteString(w, `{"total_count":2,"jobs":[{"name":"gate","status":"completed","conclusion":"success"},{"name":"release-config","status":"completed","conclusion":"success"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	getenv := func(key string) string {
		return map[string]string{"GITHUB_REPOSITORY": "acme/repo", "GITHUB_TOKEN": "token"}[key]
	}
	errb.Reset()
	if code := dispatch([]string{"--verify-ci", sha}, fstest.MapFS{}, fstest.MapFS{}, &out, &errb, server.Client(), server.URL, getenv); code != 0 {
		t.Fatalf("exact-CI dispatch = %d, %q", code, errb.String())
	}
}

func TestVerifyCIRefusesTrailingDocumentsAndGarbage(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for _, suffix := range []string{" trailing", ` {"again":true}`} {
		t.Run(suffix, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, `{"total_count":0,"workflow_runs":[]}`+suffix)
			}))
			defer s.Close()
			if err := verifyCI(context.Background(), s.Client(), s.URL, "a/r", "t", sha); err == nil {
				t.Fatal("trailing API data accepted")
			}
		})
	}
}

// TestReleaseArchivesPortableSnapshot runs the pinned snapshot production path.
// A root-mapped user namespace can represent root:root archive entries as its own
// identity but cannot represent this checkout's uid/gid. It therefore catches
// ownership metadata rather than masking it with tar's --no-same-owner fallback.
// invariant: tooling/changelog-and-release:release-platforms (TestReleaseArchivesPortableSnapshot)
func TestReleaseArchivesPortableSnapshot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("restricted rootless extraction fixture requires Linux user namespaces")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dist := filepath.Join(root, "dist")
	if err := os.RemoveAll(dist); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dist); err != nil {
			t.Errorf("remove snapshot dist: %v", err)
		}
	})
	cmd := exec.Command("go", "run", "github.com/goreleaser/goreleaser/v2@v2.17.0", "release", "--snapshot", "--clean")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build snapshot release: %v\n%s", err, out)
	}

	archives, err := filepath.Glob(filepath.Join(dist, "awf_*"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, archive := range archives {
		name := filepath.Base(archive)
		if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".zip") {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	if len(names) != 4 {
		t.Fatalf("snapshot archive count = %d, want 4: %q", len(names), names)
	}
	assertSnapshotChecksums(t, filepath.Join(dist, "checksums.txt"), names)
	for _, suffix := range []string{
		"_darwin_amd64.tar.gz", "_darwin_arm64.tar.gz", "_linux_amd64.tar.gz",
		"_linux_arm64.tar.gz",
	} {
		found := false
		for _, name := range names {
			found = found || strings.HasSuffix(name, suffix)
		}
		if !found {
			t.Errorf("snapshot archives = %q; missing target suffix %q", names, suffix)
		}
	}

	for _, name := range names {
		archive := filepath.Join(dist, name)
		if strings.HasSuffix(name, ".zip") {
			assertZipArchivePaths(t, archive, []string{"LICENSE", "README.md", "awf.exe"})
			continue
		}
		entries := assertTarArchivePaths(t, archive, []string{"LICENSE", "README.md", "awf"})
		for _, entry := range entries {
			if strings.Contains(name, "_linux_") {
				if entry.Uid != 0 || entry.Gid != 0 || entry.Uname != "root" || entry.Gname != "root" {
					t.Errorf("%s entry %s ownership = %d:%d %q:%q, want 0:0 root:root", name, entry.Name, entry.Uid, entry.Gid, entry.Uname, entry.Gname)
				}
			} else if entry.Uname == "root" || entry.Gname == "root" {
				t.Errorf("%s entry %s ownership = %d:%d %q:%q, want unchanged builder metadata rather than Linux normalization", name, entry.Name, entry.Uid, entry.Gid, entry.Uname, entry.Gname)
			}
			wantMode := int64(0o644)
			if entry.Name == "awf" {
				wantMode = 0o755
			}
			if entry.Mode != wantMode {
				t.Errorf("%s entry %s mode = %#o, want %#o", name, entry.Name, entry.Mode, wantMode)
			}
		}
	}

	linux, err := filepath.Glob(filepath.Join(dist, "awf_*_linux_amd64.tar.gz"))
	if err != nil || len(linux) != 1 {
		t.Fatalf("linux amd64 snapshot archive = %q, err = %v", linux, err)
	}
	extracted := t.TempDir()
	// Do not add --no-same-owner: successful extraction must exercise the archive
	// ownership metadata in the restricted rootless namespace.
	extract := exec.Command("unshare", "--user", "--map-root-user", "tar", "-xzf", linux[0], "-C", extracted)
	if out, err := extract.CombinedOutput(); err != nil {
		t.Fatalf("restricted rootless extraction failed: %v\n%s", err, out)
	}
	entries, err := os.ReadDir(extracted)
	if err != nil {
		t.Fatal(err)
	}
	var extractedNames []string
	for _, entry := range entries {
		extractedNames = append(extractedNames, entry.Name())
	}
	slices.Sort(extractedNames)
	if !slices.Equal(extractedNames, []string{"LICENSE", "README.md", "awf"}) {
		t.Fatalf("restricted rootless extracted paths = %q, want exactly binary, LICENSE, and README", extractedNames)
	}
}

func assertSnapshotChecksums(t *testing.T, path string, archives []string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot checksums: %v", err)
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || len(fields[0]) != 64 {
			t.Fatalf("snapshot checksum line = %q, want SHA-256 and archive name", line)
		}
		names = append(names, fields[1])
	}
	slices.Sort(names)
	if !slices.Equal(names, archives) {
		t.Fatalf("snapshot checksum entries = %q, want exactly six archives %q", names, archives)
	}
}

func assertTarArchivePaths(t *testing.T, archive string, want []string) []*tar.Header {
	t.Helper()
	file, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	var entries []*tar.Header
	var names []string
	for {
		entry, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
		names = append(names, entry.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Fatalf("archive %s paths = %q, want %q", filepath.Base(archive), names, want)
	}
	return entries
}

func assertZipArchivePaths(t *testing.T, archive string, want []string) {
	t.Helper()
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	names := make([]string, 0, len(reader.File))
	for _, entry := range reader.File {
		names = append(names, entry.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Fatalf("archive %s paths = %q, want %q", filepath.Base(archive), names, want)
	}
}
