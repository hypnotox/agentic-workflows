package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
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
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
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

func workflowLineFrom(s string, at int) string {
	line := s[at:]
	if nl := strings.IndexByte(line, '\n'); nl >= 0 {
		line = line[:nl]
	}
	return line
}

func normalizedNotesPath(raw string) string {
	path := strings.Trim(strings.TrimSpace(raw), `"'`)
	path = strings.ReplaceAll(path, "${{ runner.temp }}", "${RUNNER_TEMP}")
	return path
}

func releaseNotesWorkflowError(wf string) error {
	publishAt := strings.Index(wf, "\n  publish:")
	if publishAt < 0 {
		return errors.New("release.yml does not define the publish job")
	}
	publish := wf[publishAt:]
	var errs []error

	extract := strings.Index(publish, "awf changelog --version")
	build := strings.Index(publish, "goreleaser/goreleaser-action")
	if extract < 0 {
		errs = append(errs, errors.New("release.yml does not extract release notes via `awf changelog --version`"))
	}
	if build < 0 {
		errs = append(errs, errors.New("release.yml does not run the GoReleaser action"))
	}
	if extract >= 0 && build >= 0 && extract > build {
		errs = append(errs, errors.New("the release-note extraction must run before GoReleaser"))
	}

	var extractPath string
	if extract >= 0 {
		extractLine := workflowLineFrom(publish, extract)
		if !strings.Contains(extractLine, `awf changelog --version "${GITHUB_REF_NAME#v}"`) {
			errs = append(errs, fmt.Errorf("release-note extraction must derive the version from GITHUB_REF_NAME, got %q", extractLine))
		}
		if redirect := strings.Index(extractLine, ">"); redirect < 0 {
			errs = append(errs, fmt.Errorf("release-note extraction does not redirect to a file: %q", extractLine))
		} else {
			extractPath = normalizedNotesPath(extractLine[redirect+1:])
		}
	}

	var goreleaserPath string
	if relIdx := strings.Index(publish, "--release-notes"); relIdx < 0 {
		errs = append(errs, errors.New("release.yml does not pass --release-notes to GoReleaser"))
	} else {
		argLine := workflowLineFrom(publish, relIdx)
		goreleaserPath = normalizedNotesPath(strings.TrimPrefix(argLine, "--release-notes"))
	}

	verify := strings.Index(publish, "Verify published release notes")
	var verifyPath string
	if verify < 0 {
		errs = append(errs, errors.New("release.yml does not verify the published release body"))
	} else {
		if build >= 0 && verify < build {
			errs = append(errs, errors.New("published release-note verification must run after GoReleaser"))
		}
		verifyBlock := publish[verify:]
		if verifyCmd := strings.Index(verifyBlock, "releasecheck --verify-release-notes"); verifyCmd < 0 {
			errs = append(errs, errors.New("published release-note verification does not invoke releasecheck --verify-release-notes"))
		} else {
			verifyLine := workflowLineFrom(verifyBlock, verifyCmd)
			verifyPath = normalizedNotesPath(strings.TrimPrefix(verifyLine, "releasecheck --verify-release-notes"))
		}
	}

	const wantPath = "${RUNNER_TEMP}/release-notes.md"
	if extractPath != wantPath {
		errs = append(errs, fmt.Errorf("release-note extraction path = %q, want %q", extractPath, wantPath))
	}
	if goreleaserPath != extractPath {
		errs = append(errs, fmt.Errorf("GoReleaser notes path = %q, extraction path = %q", goreleaserPath, extractPath))
	}
	if verifyPath != extractPath {
		errs = append(errs, fmt.Errorf("verification notes path = %q, extraction path = %q", verifyPath, extractPath))
	}
	return errors.Join(errs...)
}

// TestReleaseNotesFromCuratedChangelog backs inv: release-notes-from-changelog
// (ADR-0096, ADR-load-curated-release-notes-through-goreleaser). The Release workflow
// must feed the tagged curated section through GoReleaser's enabled changelog pipe, then
// compare the published body with that exact file. Commit-derived configuration stays absent.
// invariant: tooling/changelog-and-release:release-notes-from-changelog (TestReleaseNotesFromCuratedChangelog)
func TestReleaseNotesFromCuratedChangelog(t *testing.T) {
	wfb, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	wf := string(wfb)
	if err := releaseNotesWorkflowError(wf); err != nil {
		t.Error(err)
	}
	for _, mutation := range []struct {
		name string
		old  string
		new  string
	}{
		{name: "fixed version", old: `${GITHUB_REF_NAME#v}`, new: `0.41.0`},
		{name: "different GoReleaser path", old: `${{ runner.temp }}/release-notes.md`, new: `${{ runner.temp }}/other.md`},
		{name: "different verification path", old: `--verify-release-notes "${RUNNER_TEMP}/release-notes.md"`, new: `--verify-release-notes "${RUNNER_TEMP}/other.md"`},
	} {
		t.Run("rejects "+mutation.name, func(t *testing.T) {
			publishAt := strings.Index(wf, "\n  publish:")
			mutated := wf[:publishAt] + strings.Replace(wf[publishAt:], mutation.old, mutation.new, 1)
			if mutated == wf {
				t.Fatalf("publish-job mutation target %q not found", mutation.old)
			}
			if err := releaseNotesWorkflowError(mutated); err == nil {
				t.Fatal("workflow mutation passed the release-note invariant")
			}
		})
	}

	glb, err := os.ReadFile("../../.goreleaser.yaml")
	if err != nil {
		t.Fatalf("read goreleaser config: %v", err)
	}
	var config struct {
		Changelog struct {
			Disable bool             `yaml:"disable"`
			Use     string           `yaml:"use"`
			Groups  []map[string]any `yaml:"groups"`
			Filters map[string]any   `yaml:"filters"`
		} `yaml:"changelog"`
	}
	if err := yaml.Unmarshal(glb, &config); err != nil {
		t.Fatalf("parse .goreleaser.yaml: %v", err)
	}
	if config.Changelog.Disable {
		t.Error(".goreleaser.yaml disables the changelog pipe that must load --release-notes")
	}
	if config.Changelog.Use != "" || len(config.Changelog.Groups) != 0 || len(config.Changelog.Filters) != 0 {
		t.Error(".goreleaser.yaml configures commit-derived changelog use, groups, or filters")
	}
}

func TestVerifyReleaseNotesRequiresExactCuratedBody(t *testing.T) {
	const expected = "## [0.41.0] - 2026-08-27\n\n### Bug fixes\n\n- Curated.\n"
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{name: "exact", body: expected},
		{name: "blank", body: "", want: "does not match"},
		{name: "commit-derived suffix", body: expected + "\n- internal commit\n", want: "does not match"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/repos/acme/repo/releases/tags/v0.41.0" {
					t.Errorf("path = %q", r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer token" {
					t.Errorf("authorization = %q", r.Header.Get("Authorization"))
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{"body": tc.body})
			}))
			defer server.Close()

			err := verifyReleaseNotes(context.Background(), server.Client(), server.URL, "acme/repo", "token", "v0.41.0", []byte(expected))
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestVerifyReleaseNotesRejectsMissingInputsAndBlankExpectation(t *testing.T) {
	if err := verifyReleaseNotes(context.Background(), http.DefaultClient, "", "", "", "", []byte("notes")); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("missing inputs error = %v", err)
	}
	if err := verifyReleaseNotes(context.Background(), http.DefaultClient, "https://api.github.com", "acme/repo", "token", "v1.0.0", []byte(" \n")); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("blank expectation error = %v", err)
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
	if !strings.Contains(err.Error(), "request GitHub API /repos/acme/repo/actions/runs/7/jobs") {
		t.Fatalf("candidate transport error lacks request context: %v", err)
	}
}

func TestGetGitHubJSONWrapsRequestConstructionError(t *testing.T) {
	const path = "/repos/acme/repo/releases/tags/v1.0.0"
	err := getGitHubJSON(context.Background(), http.DefaultClient, "://invalid", "token", path, &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "build GitHub API request for "+path) {
		t.Fatalf("request-construction error lacks endpoint context: %v", err)
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
	load := func(path string) map[string]any {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var workflow map[string]any
		if err := yaml.Unmarshal(b, &workflow); err != nil {
			t.Fatal(err)
		}
		return workflow
	}
	ci := load("../../.github/workflows/ci.yml")
	release := load("../../.github/workflows/release.yml")
	if problems := exactRevisionWorkflowProblems(ci, release); len(problems) != 0 {
		t.Fatalf("landed workflow violates exact-revision contract: %s", strings.Join(problems, "; "))
	}
	clone := func(in map[string]any) map[string]any {
		t.Helper()
		b, err := yaml.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]any
		if err := yaml.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any, map[string]any)
	}{
		{"CI identity", func(ci, _ map[string]any) { ci["name"] = "other" }},
		{"gate identity", func(ci, _ map[string]any) { delete(workflowJobs(ci), "gate") }},
		{"release-config identity", func(ci, _ map[string]any) { delete(workflowJobs(ci), "release-config") }},
		{"exact CI call", func(_, release map[string]any) {
			workflowStep(workflowJobs(release)["verify"], "Verify bridge readiness and exact CI conclusions")["run"] = "go run ./cmd/releasecheck"
		}},
		{"verification read-only", func(_, release map[string]any) {
			workflowMap(workflowJobs(release)["verify"])["permissions"] = map[string]any{"actions": "write", "contents": "read"}
		}},
		{"release Node runtime environment", func(_, release map[string]any) {
			delete(workflowMap(workflowMap(workflowJobs(release)["verify"])["env"]), "AWF_PI_TEST_SKIP_NVM")
		}},
		{"release Node action pin", func(_, release map[string]any) {
			workflowUsesStep(workflowJobs(release)["verify"], "actions/setup-node@")["uses"] = "actions/setup-node@fixture"
		}},
		{"release Node version", func(_, release map[string]any) {
			workflowMap(workflowUsesStep(workflowJobs(release)["verify"], "actions/setup-node@")["with"])["node-version-file"] = "other"
		}},
		{"release Node cache", func(_, release map[string]any) {
			workflowMap(workflowUsesStep(workflowJobs(release)["verify"], "actions/setup-node@")["with"])["cache"] = "other"
		}},
		{"release Node cache dependency", func(_, release map[string]any) {
			workflowMap(workflowUsesStep(workflowJobs(release)["verify"], "actions/setup-node@")["with"])["cache-dependency-path"] = "other"
		}},
		{"publication dependency", func(_, release map[string]any) { delete(workflowMap(workflowJobs(release)["publish"]), "needs") }},
		{"publication write permission", func(_, release map[string]any) {
			workflowMap(workflowJobs(release)["publish"])["permissions"] = map[string]any{"contents": "read"}
		}},
		{"GoReleaser bypass", func(_, release map[string]any) {
			workflowSteps(workflowJobs(release)["verify"])[0].(map[string]any)["uses"] = "goreleaser/goreleaser-action@fixture"
		}},
		{"checkout exact SHA", func(_, release map[string]any) {
			workflowSteps(workflowJobs(release)["publish"])[0].(map[string]any)["with"].(map[string]any)["ref"] = "main"
		}},
		{"publication tag identity", func(_, release map[string]any) {
			workflowStep(workflowJobs(release)["publish"], "Repeat tag identity before publication")["run"] = "git rev-parse HEAD"
		}},
		{"previous release selector", func(_, release map[string]any) {
			workflowStep(workflowJobs(release)["verify"], "Covercheck mutation regression from previous release")["run"] = "./x covercheck-mutants --select-range old new"
		}},
		{"first release fallback", func(_, release map[string]any) {
			workflowStep(workflowJobs(release)["verify"], "Covercheck mutation regression from previous release")["run"] = "previous=$(false)\n./x covercheck-mutants --select-range $previous candidate"
		}},
		{"dispatch fallback", func(ci, _ map[string]any) {
			workflowStep(workflowJobs(ci)["gate"], "Covercheck mutation regression")["run"] = "case $EVENT in workflow_dispatch) exit 0;; esac"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutatedCI, mutatedRelease := clone(ci), clone(release)
			tc.mutate(mutatedCI, mutatedRelease)
			if problems := exactRevisionWorkflowProblems(mutatedCI, mutatedRelease); len(problems) == 0 {
				t.Fatal("controlled workflow mutation was accepted")
			}
		})
	}
}

func TestReleaseMutationSelectorUsesPreviousStableReleaseOrRunsAlways(t *testing.T) {
	workflowBytes, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow map[string]any
	if err := yaml.Unmarshal(workflowBytes, &workflow); err != nil {
		t.Fatal(err)
	}
	selector := stringValue(workflowStep(workflowJobs(workflow)["verify"], "Covercheck mutation regression from previous release")["run"])

	for _, tc := range []struct {
		name         string
		withPrior    bool
		wantPrefix   string
		candidateTag string
	}{
		{name: "annotated previous release excludes unrelated and future tags", withPrior: true, wantPrefix: "covercheck-mutants --select-range v1.0.0 ", candidateTag: "v2.0.0"},
		{name: "first release runs blocker unconditionally", wantPrefix: "covercheck-mutants", candidateTag: "v1.0.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := gitfixture.InitNativeAt(t, t.TempDir())
			root := fixture.Root()
			if err := os.WriteFile(filepath.Join(root, "tracked"), []byte("base\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitfixture.NativeAdd(t, fixture, "tracked")
			gitfixture.NativeCommit(t, fixture, "base")
			if tc.withPrior {
				base := gitfixture.NativeRevParse(t, fixture, "HEAD")
				gitfixture.NativeAnnotatedTag(t, fixture, "v1.0.0", base)
				gitfixture.NativeLightweightTag(t, fixture, "v9.0.0", base)
				gitfixture.NativeLightweightTag(t, fixture, "not-a-release", base)
				if err := os.WriteFile(filepath.Join(root, "tracked"), []byte("candidate\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				gitfixture.NativeAdd(t, fixture, "tracked")
				gitfixture.NativeCommit(t, fixture, "candidate")
			}
			candidate := gitfixture.NativeRevParse(t, fixture, "HEAD")
			gitfixture.NativeLightweightTag(t, fixture, tc.candidateTag, candidate)
			logPath := filepath.Join(t.TempDir(), "x.log")
			fakeX := "#!/bin/sh\nprintf '%s' \"$*\" >\"$AWF_X_LOG\"\n"
			if err := os.WriteFile(filepath.Join(root, "x"), []byte(fakeX), 0o755); err != nil {
				t.Fatal(err)
			}
			script := strings.Replace(selector, "candidate='${{ github.sha }}'", "candidate='"+candidate+"'", 1)
			cmd := exec.Command("bash", "-eu", "-c", script)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GITHUB_REF_NAME="+tc.candidateTag, "AWF_X_LOG="+logPath)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("selector failed: %v: %s", err, out)
			}
			got, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(got), tc.wantPrefix) {
				t.Fatalf("selector invocation = %q, want prefix %q", got, tc.wantPrefix)
			}
			if tc.withPrior && !strings.HasSuffix(string(got), candidate) {
				t.Fatalf("selector candidate = %q, want suffix %s", got, candidate)
			}
		})
	}
}

func exactRevisionWorkflowProblems(ci, release map[string]any) []string {
	var problems []string
	if ci["name"] != "CI" {
		problems = append(problems, "CI name")
	}
	ciJobs, releaseJobs := workflowJobs(ci), workflowJobs(release)
	for _, name := range []string{"gate", "release-config"} {
		if _, ok := ciJobs[name]; !ok {
			problems = append(problems, "CI job "+name)
		}
	}
	verify, publish := workflowMap(releaseJobs["verify"]), workflowMap(releaseJobs["publish"])
	verifyRun := workflowStep(verify, "Verify bridge readiness and exact CI conclusions")["run"]
	if !strings.Contains(stringValue(verifyRun), `go run ./cmd/releasecheck --verify-ci "${{ github.sha }}"`) {
		problems = append(problems, "exact CI verification")
	}
	if !workflowPermissions(verify, "actions", "read") || !workflowPermissions(verify, "contents", "read") {
		problems = append(problems, "read-only verification")
	}
	nodeProvisioned := false
	for _, rawStep := range workflowSteps(verify) {
		step := workflowMap(rawStep)
		if stringValue(step["uses"]) != "actions/setup-node@49933ea5288caeca8642d1e84afbd3f7d6820020" {
			continue
		}
		with := workflowMap(step["with"])
		nodeProvisioned = with["node-version-file"] == ".nvmrc" && with["cache"] == "npm" && with["cache-dependency-path"] == "tools/pi-extension-test/package-lock.json"
	}
	if !nodeProvisioned || workflowMap(verify["env"])["AWF_PI_TEST_SKIP_NVM"] != "1" {
		problems = append(problems, "release Node runtime")
	}
	if !workflowNeeds(publish, "verify") || !workflowPermissions(publish, "contents", "write") {
		problems = append(problems, "needs-bound publication")
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
	for _, job := range []map[string]any{verify, publish} {
		checkout := workflowSteps(job)[0]
		if workflowMap(checkout)["with"].(map[string]any)["ref"] != "${{ github.sha }}" {
			problems = append(problems, "checkout exact SHA")
		}
	}
	identity := stringValue(workflowStep(publish, "Repeat tag identity before publication")["run"])
	if !strings.Contains(identity, "git rev-parse HEAD") || !strings.Contains(identity, "${GITHUB_REF_NAME}^{}") || !strings.Contains(identity, "${{ github.sha }}") {
		problems = append(problems, "publication tag identity")
	}
	selector := stringValue(workflowStep(verify, "Covercheck mutation regression from previous release")["run"])
	if !strings.Contains(selector, "git tag --merged") || !strings.Contains(selector, "^v[0-9]+\\.[0-9]+\\.[0-9]+$") || !strings.Contains(selector, "sort -Vu") || !strings.Contains(selector, `$0 == current`) || !strings.Contains(selector, "--select-range") {
		problems = append(problems, "previous release selector")
	}
	if !strings.Contains(selector, "if [ -n \"$previous\" ]") || !strings.Contains(selector, "else") || !strings.Contains(selector, "./x covercheck-mutants") {
		problems = append(problems, "first release fallback")
	}
	dispatch := stringValue(workflowStep(workflowMap(ciJobs["gate"]), "Covercheck mutation regression")["run"])
	if !strings.Contains(dispatch, "workflow_dispatch) ./x covercheck-mutants ; exit 0") || !strings.Contains(dispatch, "*) ./x covercheck-mutants ; exit 0") {
		problems = append(problems, "CI dispatch unconditional mutation blocker")
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
func workflowUsesStep(job any, prefix string) map[string]any {
	for _, raw := range workflowSteps(job) {
		step := workflowMap(raw)
		if strings.HasPrefix(stringValue(step["uses"]), prefix) {
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
		case "/repos/acme/repo/releases/tags/v0.41.0":
			_, _ = io.WriteString(w, `{"body":"curated notes\n"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	getenv := func(key string) string {
		return map[string]string{"GITHUB_REPOSITORY": "acme/repo", "GITHUB_TOKEN": "token", "GITHUB_REF_NAME": "v0.41.0"}[key]
	}
	errb.Reset()
	if code := dispatch([]string{"--verify-ci", sha}, fstest.MapFS{}, fstest.MapFS{}, &out, &errb, server.Client(), server.URL, getenv); code != 0 {
		t.Fatalf("exact-CI dispatch = %d, %q", code, errb.String())
	}

	notesPath := filepath.Join(t.TempDir(), "release-notes.md")
	if err := os.WriteFile(notesPath, []byte("curated notes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	errb.Reset()
	if code := dispatch([]string{"--verify-release-notes", notesPath}, fstest.MapFS{}, fstest.MapFS{}, &out, &errb, server.Client(), server.URL, getenv); code != 0 {
		t.Fatalf("release-notes dispatch = %d, %q", code, errb.String())
	}
	errb.Reset()
	if code := dispatch([]string{"--verify-release-notes", notesPath + ".missing"}, fstest.MapFS{}, fstest.MapFS{}, &out, &errb, server.Client(), server.URL, getenv); code != 1 || !strings.Contains(errb.String(), "read curated release notes") {
		t.Fatalf("missing-notes dispatch = %d, %q", code, errb.String())
	}
	errb.Reset()
	missingReleaseEnv := func(key string) string {
		if key == "GITHUB_REF_NAME" {
			return "v0.42.0"
		}
		return getenv(key)
	}
	if code := dispatch([]string{"--verify-release-notes", notesPath}, fstest.MapFS{}, fstest.MapFS{}, &out, &errb, server.Client(), server.URL, missingReleaseEnv); code != 1 || !strings.Contains(errb.String(), "GitHub API") {
		t.Fatalf("missing-release dispatch = %d, %q", code, errb.String())
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
func TestReleaseArchivesPortableSnapshot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("restricted rootless extraction fixture requires Linux user namespaces")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dist := filepath.Join(t.TempDir(), "dist")
	relativeDist, err := filepath.Rel(root, dist)
	if err != nil {
		t.Fatal(err)
	}
	if relativeDist != ".." && !strings.HasPrefix(relativeDist, ".."+string(os.PathSeparator)) {
		t.Fatalf("snapshot dist %q is inside checkout %q", dist, root)
	}
	config, err := os.ReadFile(filepath.Join(root, ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), ".goreleaser.yaml")
	isolatedConfig := fmt.Appendf(nil, "dist: %q\n", dist)
	isolatedConfig = append(isolatedConfig, config...)
	if err := os.WriteFile(configPath, isolatedConfig, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "github.com/goreleaser/goreleaser/v2@v2.17.0", "release", "--snapshot", "--clean", "--config", configPath)
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
	if len(names) != 6 {
		t.Fatalf("snapshot archive count = %d, want 6: %q", len(names), names)
	}
	assertSnapshotChecksums(t, filepath.Join(dist, "checksums.txt"), names)
	for _, suffix := range []string{
		"_darwin_amd64.tar.gz", "_darwin_arm64.tar.gz", "_linux_amd64.tar.gz",
		"_linux_arm64.tar.gz", "_windows_amd64.zip", "_windows_arm64.zip",
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
