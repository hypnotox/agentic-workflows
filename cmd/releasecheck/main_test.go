package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
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
	"sort"
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

// TestReleaseWorkflowGatesOnTag backs inv: release-gate-on-tag (ADR-0079).
// Release publication retains exact revision identity and depends on both native
// verifiers of the single credential-free candidate.
// invariant: tooling/changelog-and-release:release-gate-on-tag (TestReleaseWorkflowGatesOnTag)
func TestReleaseWorkflowGatesOnTag(t *testing.T) {
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
		t.Fatalf("release gate problems: %q", problems)
	}
	for _, mutation := range []struct {
		name  string
		apply func(map[string]any, map[string]any)
	}{
		{"missing aggregate gate", func(c, _ map[string]any) { delete(workflowJobs(c), "gate") }},
		{"missing Linux dependency", func(c, _ map[string]any) { workflowMap(workflowJobs(c)["gate"])["needs"] = []any{"macos", "pi"} }},
		{"selection loses two-dot range", func(c, _ map[string]any) {
			step := workflowStep(workflowMap(workflowJobs(c)["selection"]), "Select typed affected lanes")
			step["run"] = strings.Replace(stringValue(step["run"]), "${BASE_SHA}..${HEAD_SHA}", "${BASE_SHA}...${HEAD_SHA}", 1)
		}},
		{"selection consumes package policy", func(c, _ map[string]any) {
			step := workflowStep(workflowMap(workflowJobs(c)["selection"]), "Select typed affected lanes")
			step["run"] = stringValue(step["run"]) + "\necho '.packages'"
		}},
		{"widened selection can suppress lanes", func(c, _ map[string]any) {
			step := workflowStep(workflowMap(workflowJobs(c)["selection"]), "Select typed affected lanes")
			step["run"] = strings.Replace(stringValue(step["run"]), `if .outcome == "widened" then`, `if .outcome == "widened-without-lanes" then`, 1)
		}},
		{"missing complete Go behavior", func(c, _ map[string]any) {
			workflowStep(workflowMap(workflowJobs(c)["linux"]), "Complete Go behavior with calibration timing")["run"] = "true"
		}},
		{"missing calibration timing artifact", func(c, _ map[string]any) {
			workflowMap(workflowStep(workflowMap(workflowJobs(c)["linux"]), "Upload full Linux calibration timing")["with"])["name"] = "foreign-timing"
		}},
		{"missing Linux gate", func(c, _ map[string]any) {
			workflowStep(workflowMap(workflowJobs(c)["linux"]), "Build, blocking lint, version, and pins")["run"] = "true"
		}},
		{"missing selected render check", func(c, _ map[string]any) {
			workflowStep(workflowMap(workflowJobs(c)["linux"]), "Selected render and repository checks")["run"] = "true"
		}},
		{"missing release archive behavior", func(c, _ map[string]any) {
			workflowStep(workflowMap(workflowJobs(c)["linux"]), "Selected release-archive behavior")["run"] = "true"
		}},
		{"missing strict Pi behavior", func(c, _ map[string]any) {
			workflowExactRunStep(workflowJobs(c)["pi"], "./x pi-test run")["run"] = "true"
		}},
		{"publish loses native verifier dependency", func(_, r map[string]any) {
			workflowMap(workflowJobs(r)["publish"])["needs"] = []any{"native-linux-amd64"}
		}},
		{"candidate upload name changes", func(_, r map[string]any) {
			workflowMap(workflowMap(workflowStep(workflowMap(workflowJobs(r)["verify"]), "Upload candidate"))["with"])["name"] = "foreign-candidate"
		}},
		{"native arm runner changes", func(c, _ map[string]any) { workflowMap(workflowJobs(c)["macos"])["runs-on"] = "macos-latest" }},
		{"PR macOS loses safety", func(c, _ map[string]any) {
			workflowStep(workflowMap(workflowJobs(c)["macos"]), "Filesystem, publication, Git, effort, and worktree safety")["run"] = "true"
		}},
		{"main macOS loses Go", func(c, _ map[string]any) {
			workflowStep(workflowMap(workflowJobs(c)["macos"]), "Complete macOS Go assurance")["run"] = "true"
		}},
		{"missing tag identity", func(_, r map[string]any) {
			workflowStep(workflowMap(workflowJobs(r)["verify"]), "Verify checkout and tag identity")["run"] = "true"
		}},
		{"missing version identity", func(_, r map[string]any) {
			workflowStep(workflowMap(workflowJobs(r)["verify"]), "Verify tag matches project.Version")["run"] = "true"
		}},
		{"missing main ancestry", func(_, r map[string]any) {
			workflowStep(workflowMap(workflowJobs(r)["verify"]), "Verify tagged commit is on main")["run"] = "true"
		}},
		{"publish rebuilds", func(_, r map[string]any) {
			workflowStep(workflowMap(workflowJobs(r)["publish"]), "Publish the exact verified candidate archives")["run"] = "goreleaser release --clean"
		}},
		{"publication loses tag identity", func(_, r map[string]any) {
			workflowStep(workflowMap(workflowJobs(r)["publish"]), "Repeat tag identity before publication")["run"] = "true"
		}},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			cloneCI, cloneRelease := cloneWorkflow(t, ci), cloneWorkflow(t, release)
			mutation.apply(cloneCI, cloneRelease)
			if problems := exactRevisionWorkflowProblems(cloneCI, cloneRelease); len(problems) == 0 {
				t.Fatal("release workflow mutation was accepted")
			}
		})
	}
}

func TestReleaseWorkflowRunsReleasecheck(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	wf := string(b)
	if !strings.Contains(wf, "go run ./cmd/releasecheck") {
		t.Fatal("release.yml does not invoke releasecheck")
	}
	if strings.Contains(wf, "--snapshot") {
		t.Error("release candidate must not be a snapshot")
	}
	if !strings.Contains(wf, "release --clean --skip=publish") {
		t.Error("release.yml does not build one non-publishing candidate")
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
	const notes = `go run ./cmd/awf changelog --version "${GITHUB_REF_NAME#v}" > "${RUNNER_TEMP}/release-notes.md"`
	if !strings.Contains(publish, notes) {
		return errors.New("publish does not prepare exact curated release notes")
	}
	create := `gh release create "$GITHUB_REF_NAME" dist/awf_*.tar.gz dist/checksums.txt --notes-file "${RUNNER_TEMP}/release-notes.md"`
	if !strings.Contains(publish, create) {
		return errors.New("publish does not upload the verified candidate archives with curated notes")
	}
	if strings.Contains(publish, "goreleaser/") {
		return errors.New("publish rebuilds instead of publishing the candidate")
	}
	verify := `go run ./cmd/releasecheck --verify-release-notes "${RUNNER_TEMP}/release-notes.md"`
	if !strings.Contains(publish, verify) {
		return errors.New("publish does not verify exact curated notes")
	}
	return nil
}

// TestReleaseNotesFromCuratedChangelog ensures publication sends the curated file
// unchanged while uploading the downloaded candidate, rather than rebuilding it.
// invariant: tooling/changelog-and-release:release-notes-from-changelog (TestReleaseNotesFromCuratedChangelog)
func TestReleaseNotesFromCuratedChangelog(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	if err := releaseNotesWorkflowError(string(b)); err != nil {
		t.Error(err)
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

func TestVerifyCIRequiresExactSuccessfulWorkflowAndGate(t *testing.T) {
	sha := strings.Repeat("a", 40)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/acme/repo/actions/workflows/ci.yml/runs":
			fmt.Fprintf(w, `{"total_count":1,"workflow_runs":[{"id":7,"head_sha":%q,"status":"completed","conclusion":"success","path":".github/workflows/ci.yml","name":"CI"}]}`, sha)
		case "/repos/acme/repo/actions/runs/7/jobs":
			_, _ = io.WriteString(w, `{"total_count":1,"jobs":[{"name":"gate","status":"completed","conclusion":"success"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	if err := verifyCI(context.Background(), server.Client(), server.URL, "acme/repo", "token", sha); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyCIAcceptsAnyCompleteExactSuccessfulRun(t *testing.T) {
	sha := strings.Repeat("a", 40)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/repo/actions/workflows/ci.yml/runs":
			fmt.Fprintf(w, `{"total_count":2,"workflow_runs":[{"id":7,"head_sha":%q,"status":"completed","conclusion":"success","path":".github/workflows/ci.yml","name":"CI"},{"id":8,"head_sha":%q,"status":"completed","conclusion":"success","path":".github/workflows/ci.yml","name":"CI"}]}`, sha, sha)
		case "/repos/acme/repo/actions/runs/7/jobs":
			_, _ = io.WriteString(w, `{"total_count":1,"jobs":[{"name":"gate","status":"completed","conclusion":"failure"}]}`)
		case "/repos/acme/repo/actions/runs/8/jobs":
			_, _ = io.WriteString(w, `{"total_count":1,"jobs":[{"name":"gate","status":"completed","conclusion":"success"}]}`)
		}
	}))
	defer server.Close()
	if err := verifyCI(context.Background(), server.Client(), server.URL, "acme/repo", "token", sha); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyCIRefusesIncompleteOrWrongEvidence(t *testing.T) {
	sha := strings.Repeat("a", 40)
	validRuns := fmt.Sprintf(`{"total_count":1,"workflow_runs":[{"id":7,"head_sha":%q,"status":"completed","conclusion":"success","path":".github/workflows/ci.yml","name":"CI"}]}`, sha)
	for _, jobs := range []string{
		`{"total_count":1,"jobs":[{"name":"gate","status":"completed","conclusion":"failure"}]}`,
		`{"total_count":0,"jobs":[]}`,
		`{"total_count":2,"jobs":[{"name":"gate","status":"completed","conclusion":"success"}]}`,
		`{"total_count":2,"jobs":[{"name":"gate","status":"completed","conclusion":"success"},{"name":"gate","status":"completed","conclusion":"success"}]}`,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/jobs") {
				_, _ = io.WriteString(w, jobs)
			} else {
				_, _ = io.WriteString(w, validRuns)
			}
		}))
		err := verifyCI(context.Background(), server.Client(), server.URL, "a/r", "t", sha)
		server.Close()
		if err == nil {
			t.Fatalf("invalid gate evidence accepted: %s", jobs)
		}
	}
}

func TestGetGitHubJSONAndVerifyCIPreserveTransportErrors(t *testing.T) {
	const endpoint = "/repos/acme/repo/releases/tags/v1.0.0"
	if err := getGitHubJSON(context.Background(), http.DefaultClient, "://invalid", "token", endpoint, &struct{}{}); err == nil || !strings.Contains(err.Error(), "build GitHub API request") {
		t.Fatalf("request error = %v", err)
	}
	transportErr := errors.New("jobs transport failed")
	sha := strings.Repeat("a", 40)
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/jobs") {
			return nil, transportErr
		}
		body := fmt.Sprintf(`{"total_count":1,"workflow_runs":[{"id":7,"head_sha":%q,"status":"completed","conclusion":"success","path":".github/workflows/ci.yml","name":"CI"}]}`, sha)
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	if err := verifyCI(context.Background(), client, "https://api.example.test", "acme/repo", "token", sha); !errors.Is(err, transportErr) {
		t.Fatalf("transport identity lost: %v", err)
	}
}

// invariant: tooling/quality-gates:exact-revision-repository-acceptance (TestExactRevisionWorkflowContract)
func TestExactRevisionWorkflowContract(t *testing.T) {
	read := func(name string) map[string]any {
		t.Helper()
		body, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", name))
		if err != nil {
			t.Fatal(err)
		}
		return parseWorkflow(t, body)
	}
	if problems := exactRevisionWorkflowProblems(read("ci.yml"), read("release.yml")); len(problems) != 0 {
		t.Fatalf("workflow contract problems: %q", problems)
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
	if ci["name"] != "CI" || release["name"] != "Release" {
		problems = append(problems, "stable workflow names")
	}
	ciJobs, releaseJobs := workflowJobs(ci), workflowJobs(release)
	selection := workflowMap(ciJobs["selection"])
	if !selectionConsumerExact(selection) {
		problems = append(problems, "typed selection consumer")
	}
	const candidateRef = "${{ github.event_name == 'pull_request' && github.event.pull_request.head.sha || github.sha }}"
	for _, name := range []string{"linux", "macos", "pi", "gate"} {
		job := workflowMap(ciJobs[name])
		if workflowFirstStepWith(job)["ref"] != candidateRef {
			problems = append(problems, "CI exact checkout "+name)
		}
	}

	linux, macos, pi := workflowMap(ciJobs["linux"]), workflowMap(ciJobs["macos"]), workflowMap(ciJobs["pi"])
	if linux["name"] != "native-linux-amd64" || macos["name"] != "native-darwin-arm64" || macos["runs-on"] != "macos-15" {
		problems = append(problems, "stable CI native names and arm runner")
	}
	const candidateSHA = `[ "$(git rev-parse HEAD)" = "$CANDIDATE_SHA" ]`
	if !nativeAssertionExact(workflowStep(linux, "Assert native target linux/amd64"), "Linux", "X64", "Linux", "x86_64", "linux", "amd64", candidateSHA) ||
		!nativeAssertionExact(workflowStep(macos, "Assert native target darwin/arm64"), "macOS", "ARM64", "Darwin", "arm64", "darwin", "arm64", candidateSHA) ||
		!nativeAssertionExact(workflowStep(pi, "Assert native target linux/amd64"), "Linux", "X64", "Linux", "x86_64", "linux", "amd64", candidateSHA) {
		problems = append(problems, "native target assertion")
	}
	if stringValue(linux["if"]) != "${{ always() && (github.event_name != 'pull_request' || needs.selection.result == 'success') }}" || !workflowNeeds(linux, "selection") {
		problems = append(problems, "Linux selection dependency")
	}
	for _, required := range []struct {
		step, condition, run string
	}{
		{"Complete Go behavior with calibration timing", "", `./x test-full-linux calibrate --artifact "$RUNNER_TEMP/awf-full-linux-timing-v1.json"`},
		{"Build, blocking lint, version, and pins", "", "./x gate"},
		{"Selected release-archive behavior", "${{ github.event_name == 'pull_request' && needs.selection.outputs.release_archive == 'true' }}", "go test -count=1 ./cmd/releasecheck"},
		{"Selected render and repository checks", "${{ github.event_name != 'pull_request' || needs.selection.outputs.render_template == 'true' }}", "./x check && ./x render && git diff --exit-code"},
	} {
		step := workflowStep(linux, required.step)
		if stringValue(step["if"]) != required.condition || strings.TrimSpace(stringValue(step["run"])) != required.run {
			problems = append(problems, required.step)
		}
	}
	calibrationUpload := workflowStep(linux, "Upload full Linux calibration timing")
	calibrationWith := workflowMap(calibrationUpload["with"])
	if stringValue(calibrationUpload["if"]) != "${{ always() }}" ||
		!strings.HasPrefix(stringValue(calibrationUpload["uses"]), "actions/upload-artifact@") ||
		calibrationWith["name"] != "full-linux-timing-linux-amd64-${{ env.CANDIDATE_SHA }}-attempt-${{ github.run_attempt }}" ||
		calibrationWith["path"] != "${{ runner.temp }}/awf-full-linux-timing-v1.json" ||
		calibrationWith["if-no-files-found"] != "error" || calibrationWith["retention-days"] != 30 {
		problems = append(problems, "full Linux calibration timing artifact")
	}
	prSafety := workflowStep(macos, "Filesystem, publication, Git, effort, and worktree safety")
	const prSafetyRun = `temp_root="$(cd "$RUNNER_TEMP" && pwd -P)"
env TMPDIR="$temp_root" GOTMPDIR="$temp_root" go test -count=1 ./internal/filesystem ./internal/filepublication ./internal/git ./internal/effort ./internal/worktree`
	if stringValue(macos["if"]) != "${{ always() && (github.event_name != 'pull_request' || (needs.selection.result == 'success' && needs.selection.outputs.platform_sensitive == 'true')) }}" || !workflowNeeds(macos, "selection") || stringValue(prSafety["if"]) != "${{ github.event_name == 'pull_request' }}" || strings.TrimSpace(stringValue(prSafety["run"])) != prSafetyRun {
		problems = append(problems, "PR macOS safety")
	}
	macGo, macPi, macRender := workflowStep(macos, "Complete macOS Go assurance"), workflowStep(macos, "Complete Pi/runtime behavior on main"), workflowStep(macos, "Complete render and check behavior on main")
	if stringValue(macGo["if"]) != "${{ github.event_name != 'pull_request' }}" || strings.TrimSpace(stringValue(macGo["run"])) != "./x test" ||
		stringValue(macPi["if"]) != "${{ github.event_name != 'pull_request' }}" || strings.TrimSpace(stringValue(macPi["run"])) != "./x pi-test run" ||
		stringValue(macRender["if"]) != "${{ github.event_name != 'pull_request' }}" || strings.TrimSpace(stringValue(macRender["run"])) != "./x check && ./x render && git diff --exit-code" {
		problems = append(problems, "main macOS complete assurance")
	}
	if stringValue(pi["if"]) != "${{ always() && (github.event_name != 'pull_request' || (needs.selection.result == 'success' && needs.selection.outputs.pi_runtime == 'true')) }}" || !workflowNeeds(pi, "selection") || countExactRun(pi, "./x pi-test run") != 1 {
		problems = append(problems, "selected Pi behavior")
	}
	gate := workflowMap(ciJobs["gate"])
	const acceptance = `[ "$(git rev-parse HEAD)" = "$CANDIDATE_SHA" ]
if [ "$GITHUB_EVENT_NAME" = pull_request ]; then
  [ '${{ needs.selection.result }}' = success ]
  [ '${{ needs.linux.result }}' = success ]
  if [ "$PLATFORM_SELECTED" = true ]; then [ '${{ needs.macos.result }}' = success ]; else [ '${{ needs.macos.result }}' = skipped ]; fi
  if [ "$PI_SELECTED" = true ]; then [ '${{ needs.pi.result }}' = success ]; else [ '${{ needs.pi.result }}' = skipped ]; fi
else
  [ '${{ needs.selection.result }}' = skipped ]
  [ '${{ needs.linux.result }}' = success ]
  [ '${{ needs.macos.result }}' = success ]
  [ '${{ needs.pi.result }}' = success ]
fi`
	if gate["name"] != "gate" || stringValue(gate["if"]) != "${{ always() }}" || !slices.Equal(workflowNeedNames(gate), []string{"linux", "macos", "pi", "selection"}) || strings.TrimSpace(stringValue(workflowStep(gate, "Require every selected assurance lane for the exact candidate")["run"])) != acceptance {
		problems = append(problems, "stable aggregate gate")
	}

	verify, publish := workflowMap(releaseJobs["verify"]), workflowMap(releaseJobs["publish"])
	verifySteps := workflowSteps(verify)
	verifyCheckout := workflowFirstStepWith(verify)
	if len(verifySteps) == 0 || verifyCheckout["ref"] != "${{ github.sha }}" || verifyCheckout["persist-credentials"] != false || !workflowPermissions(verify, "actions", "read") || !workflowPermissions(verify, "contents", "read") {
		problems = append(problems, "candidate exact read-only checkout")
	}
	if verify["name"] != "prepare-exact-candidate" || !nativeAssertionExact(workflowStep(verify, "Assert native linux/amd64"), "Linux", "X64", "Linux", "x86_64", "linux", "amd64", "") || countActionArgs(verify, "goreleaser/goreleaser-action@", "release --clean --skip=publish --release-notes ${{ runner.temp }}/release-notes.md") != 1 || countExactRun(verify, "go run ./cmd/releasecheck --verify-archives dist") != 1 {
		problems = append(problems, "single non-snapshot candidate")
	}
	const tagIdentity = `candidate='${{ github.sha }}'
[ "$(git rev-parse HEAD)" = "$candidate" ]
[ "$(git rev-parse "${GITHUB_REF_NAME}^{}")" = "$candidate" ]`
	const versionIdentity = `tag="${GITHUB_REF_NAME#v}"
want="$(go run ./cmd/awf version | awk '
  /^version: [^[:space:]()]+( \([^[:cntrl:]]+\))?$/ {
    if (found) { bad = 1; exit }
    found = 1
    value = substr($0, 10)
    sub(/ \(.*/, "", value)
    next
  }
  { bad = 1; exit }
  END { if (!bad && found == 1) print value; else exit 1 }
')"
[ "$tag" = "$want" ]`
	const mainAncestry = `git fetch origin main
git merge-base --is-ancestor HEAD origin/main`
	const curatedNotes = `go run ./cmd/awf changelog --version "${GITHUB_REF_NAME#v}" > "${RUNNER_TEMP}/release-notes.md"`
	for _, required := range []struct{ step, run string }{
		{"Verify checkout and tag identity", tagIdentity},
		{"Verify tag matches project.Version", versionIdentity},
		{"Verify tagged commit is on main", mainAncestry},
		{"Prepare release notes from the curated changelog", curatedNotes},
	} {
		if strings.TrimSpace(stringValue(workflowStep(verify, required.step)["run"])) != required.run {
			problems = append(problems, "candidate authority "+required.step)
		}
	}
	candidate := "release-candidate-${{ github.sha }}"
	if !workflowArtifactExact(verify, "actions/upload-artifact@", candidate, "dist/awf_*.tar.gz\ndist/checksums.txt") {
		problems = append(problems, "candidate upload identity")
	}
	for _, tc := range []struct {
		name, runner, assertStep, runnerOS, runnerArch, unameOS, unameArch, goos, goarch, smoke string
	}{
		{"native-linux-amd64", "ubuntu-latest", "Assert native target linux/amd64", "Linux", "X64", "Linux", "x86_64", "linux", "amd64", "linux/amd64"},
		{"native-darwin-arm64", "macos-15", "Assert native target darwin/arm64", "macOS", "ARM64", "Darwin", "arm64", "darwin", "arm64", "darwin/arm64"},
	} {
		job := workflowMap(releaseJobs[tc.name])
		steps := workflowSteps(job)
		checkout := workflowFirstStepWith(job)
		assurance := strings.TrimSpace(stringValue(workflowStep(job, "Complete source assurance")["run"]))
		smoke := strings.TrimSpace(stringValue(workflowStep(job, "Smoke exact candidate "+tc.smoke)["run"]))
		wantSmoke := `tools/native-release-test/run.sh dist ` + tc.goos + ` ` + tc.goarch + ` "${GITHUB_REF_NAME#v}"`
		if job["name"] != tc.name || job["runs-on"] != tc.runner || len(steps) == 0 || checkout["ref"] != "${{ github.sha }}" || checkout["persist-credentials"] != false || !workflowNeeds(job, "verify") || !nativeAssertionExact(workflowStep(job, tc.assertStep), tc.runnerOS, tc.runnerArch, tc.unameOS, tc.unameArch, tc.goos, tc.goarch, `[ "$(git rev-parse HEAD)" = '${{ github.sha }}' ]`) || !workflowArtifactExact(job, "actions/download-artifact@", candidate, "dist") || assurance != "./x test && ./x gate && ./x check && ./x render && git diff --exit-code && ./x pi-test run" || smoke != wantSmoke {
			problems = append(problems, "native verifier "+tc.name)
		}
	}
	publishSteps := workflowSteps(publish)
	publishRun := strings.TrimSpace(stringValue(workflowStep(publish, "Publish the exact verified candidate archives")["run"]))
	const wantPublish = `gh release create "$GITHUB_REF_NAME" dist/awf_*.tar.gz dist/checksums.txt --notes-file "${RUNNER_TEMP}/release-notes.md"`
	const publicationIdentity = `[ "$(git rev-parse HEAD)" = '${{ github.sha }}' ]
[ "$(git rev-parse "${GITHUB_REF_NAME}^{}")" = '${{ github.sha }}' ]`
	publishCheckout := workflowFirstStepWith(publish)
	if publish["name"] != "publish-verified-candidate" || !slices.Equal(workflowNeedNames(publish), []string{"native-darwin-arm64", "native-linux-amd64"}) || !workflowPermissions(publish, "contents", "write") || len(publishSteps) == 0 || publishCheckout["ref"] != "${{ github.sha }}" || publishCheckout["persist-credentials"] != false || !workflowArtifactExact(publish, "actions/download-artifact@", candidate, "dist") || publishRun != wantPublish || strings.TrimSpace(stringValue(workflowStep(publish, "Repeat tag identity before publication")["run"])) != publicationIdentity || countExactRun(publish, `go run ./cmd/releasecheck --verify-release-notes "${RUNNER_TEMP}/release-notes.md"`) != 1 {
		problems = append(problems, "verified candidate publication")
	}
	return problems
}

func selectionConsumerExact(job map[string]any) bool {
	if job["name"] != "selection-linux-amd64" || job["if"] != "${{ github.event_name == 'pull_request' }}" || workflowFirstStepWith(job)["ref"] != "${{ github.event.pull_request.head.sha }}" {
		return false
	}
	if !nativeAssertionExact(workflowStep(job, "Assert native target linux/amd64"), "Linux", "X64", "Linux", "x86_64", "linux", "amd64", `[ "$(git rev-parse HEAD)" = "$HEAD_SHA" ]`) {
		return false
	}
	outputs := workflowMap(job["outputs"])
	for _, key := range []string{"selection_path", "selection_status", "selection_outcome", "go", "pi_runtime", "render_template", "platform_sensitive", "release_archive"} {
		if stringValue(outputs[key]) == "" {
			return false
		}
	}
	run := stringValue(workflowStep(job, "Select typed affected lanes")["run"])
	for _, required := range []string{
		`selection_path="$RUNNER_TEMP/awf-test-selection-v2.json"`,
		`go build -o "$RUNNER_TEMP/awf-testselection" ./cmd/testselection`,
		`"$RUNNER_TEMP/awf-testselection" --range "${BASE_SHA}..${HEAD_SHA}" >"$selection_path"`,
		`.version == 2`,
		`.outcome == "selected"`,
		`.outcome == "empty"`,
		`.outcome == "widened"`,
		`if .outcome == "widened" then`,
		`[.lanes[].name] == ["go", "pi-runtime", "platform-sensitive", "release-archive", "render-template"]`,
		`.outcome == "refused"`,
		`.name == "go"`,
		`.name == "pi-runtime"`,
		`.name == "render-template"`,
		`.name == "platform-sensitive"`,
		`.name == "release-archive"`,
		`0:selected|0:empty|0:widened|2:refused`,
		`echo "selection_path=$selection_path"`,
		`echo "selection_status=$producer_status"`,
		`echo "selection_outcome=$outcome"`,
		`any(.lanes[]; .name == $lane)`,
		`exit 2`,
	} {
		if !strings.Contains(run, required) {
			return false
		}
	}
	for _, forbidden := range []string{"go run ./cmd/testselection", "--execute", ".packages", ".reasons", ".diagnostics"} {
		if strings.Contains(run, forbidden) {
			return false
		}
	}
	return true
}

func nativeAssertionExact(step map[string]any, runnerOS, runnerArch, unameOS, unameArch, goos, goarch, shaAssertion string) bool {
	want := fmt.Sprintf(`printf 'native target: %s/%s\n'
[ '${{ runner.os }}' = %s ]
[ '${{ runner.arch }}' = %s ]
[ "$(uname -s)" = %s ]
[ "$(uname -m)" = %s ]
[ "$(go env GOOS)" = %s ]
[ "$(go env GOARCH)" = %s ]`, goos, goarch, runnerOS, runnerArch, unameOS, unameArch, goos, goarch)
	if shaAssertion != "" {
		want += "\n" + shaAssertion
	}
	return strings.TrimSpace(stringValue(step["run"])) == want
}

func workflowFirstStepWith(job map[string]any) map[string]any {
	steps := workflowSteps(job)
	if len(steps) == 0 {
		return map[string]any{}
	}
	return workflowMap(workflowMap(steps[0])["with"])
}

func workflowArtifactExact(job map[string]any, actionPrefix, name, path string) bool {
	for _, raw := range workflowSteps(job) {
		step := workflowMap(raw)
		with := workflowMap(step["with"])
		if strings.HasPrefix(stringValue(step["uses"]), actionPrefix) && with["name"] == name && strings.TrimSpace(stringValue(with["path"])) == path {
			return true
		}
	}
	return false
}

func workflowNeedNames(job map[string]any) []string {
	var names []string
	switch needs := job["needs"].(type) {
	case string:
		names = append(names, needs)
	case []any:
		for _, need := range needs {
			if name, ok := need.(string); ok {
				names = append(names, name)
			}
		}
	}
	slices.Sort(names)
	return names
}

func countExactRun(job map[string]any, want string) int {
	count := 0
	for _, raw := range workflowSteps(job) {
		if strings.TrimSpace(stringValue(workflowMap(raw)["run"])) == want {
			count++
		}
	}
	return count
}

func countActionArgs(job map[string]any, prefix, want string) int {
	count := 0
	for _, raw := range workflowSteps(job) {
		step := workflowMap(raw)
		if strings.HasPrefix(stringValue(step["uses"]), prefix) && workflowMap(step["with"])["args"] == want {
			count++
		}
	}
	return count
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
func workflowExactRunStep(job any, run string) map[string]any {
	for _, raw := range workflowSteps(job) {
		step := workflowMap(raw)
		if strings.TrimSpace(stringValue(step["run"])) == run {
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
			_, _ = io.WriteString(w, `{"total_count":1,"jobs":[{"name":"gate","status":"completed","conclusion":"success"}]}`)
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

// invariant: tooling/changelog-and-release:release-platforms (TestVerifyArchivesSyntheticFixtures)
func TestVerifyArchivesSyntheticFixtures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, dist string, files map[string]archiveFixture)
		want   string
	}{
		{name: "canonical tar archives", want: ""},
		{name: "malformed archive", mutate: func(t *testing.T, dist string, files map[string]archiveFixture) {
			name := archiveName("darwin", "amd64", "tar.gz")
			if err := os.WriteFile(filepath.Join(dist, name), []byte("not a gzip"), 0o600); err != nil {
				t.Fatal(err)
			}
			writeFixtureChecksums(t, dist, files)
		}, want: `read archive "awf_1.2.3_darwin_amd64.tar.gz"`},
		{name: "missing archive", mutate: func(t *testing.T, dist string, _ map[string]archiveFixture) {
			if err := os.Remove(filepath.Join(dist, archiveName("darwin", "arm64", "tar.gz"))); err != nil {
				t.Fatal(err)
			}
		}, want: "missing release archive for darwin/arm64"},
		{name: "mismatched versions", mutate: func(t *testing.T, dist string, files map[string]archiveFixture) {
			oldName := archiveName("darwin", "arm64", "tar.gz")
			newName := "awf_9.9.9_darwin_arm64.tar.gz"
			if err := os.Rename(filepath.Join(dist, oldName), filepath.Join(dist, newName)); err != nil {
				t.Fatal(err)
			}
			files[newName] = files[oldName]
			delete(files, oldName)
			writeFixtureChecksums(t, dist, files)
		}, want: `has version "9.9.9", want "1.2.3"`},
		{name: "duplicate target", mutate: func(t *testing.T, dist string, files map[string]archiveFixture) {
			name := "awf_9.9.9_darwin_amd64.tar.gz"
			files[name] = files[archiveName("darwin", "amd64", "tar.gz")]
			writeFixtureArchive(t, dist, name, files[name])
			writeFixtureChecksums(t, dist, files)
		}, want: "duplicate release archive for darwin/amd64"},
		{name: "zip format", mutate: func(t *testing.T, dist string, _ map[string]archiveFixture) {
			if err := os.WriteFile(filepath.Join(dist, archiveName("darwin", "amd64", "zip")), []byte("synthetic zip"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, want: `unexpected release artifact "awf_1.2.3_darwin_amd64.zip"`},
		{name: "nonregular archive", mutate: func(t *testing.T, dist string, _ map[string]archiveFixture) {
			name := archiveName("darwin", "amd64", "tar.gz")
			if err := os.Remove(filepath.Join(dist, name)); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(archiveName("darwin", "arm64", "tar.gz"), filepath.Join(dist, name)); err != nil {
				t.Fatal(err)
			}
		}, want: `unexpected release artifact "awf_1.2.3_darwin_amd64.tar.gz"`},
		{name: "extra archive", mutate: func(t *testing.T, dist string, _ map[string]archiveFixture) {
			if err := os.WriteFile(filepath.Join(dist, "awf_1.2.3_windows_amd64.zip"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}, want: `unexpected release artifact "awf_1.2.3_windows_amd64.zip"`},
		{name: "unsafe member", mutate: func(t *testing.T, dist string, files map[string]archiveFixture) {
			name := archiveName("darwin", "amd64", "tar.gz")
			files[name] = archiveFixture{members: map[string]fixtureMember{"../escape": {mode: 0o644}, "README.md": {mode: 0o644}, "awf": {mode: 0o755}}}
			writeFixtureArchive(t, dist, name, files[name])
			writeFixtureChecksums(t, dist, files)
		}, want: `has unsafe path "../escape"`},
		{name: "wrong membership", mutate: func(t *testing.T, dist string, files map[string]archiveFixture) {
			name := archiveName("darwin", "amd64", "tar.gz")
			files[name] = archiveFixture{members: map[string]fixtureMember{"LICENSE": {mode: 0o644}, "README.md": {mode: 0o644}, "extra": {mode: 0o644}}}
			writeFixtureArchive(t, dist, name, files[name])
			writeFixtureChecksums(t, dist, files)
		}, want: `has unexpected member "extra"`},
		{name: "wrong mode", mutate: func(t *testing.T, dist string, files map[string]archiveFixture) {
			name := archiveName("linux", "arm64", "tar.gz")
			file := files[name]
			member := file.members["awf"]
			member.mode = 0o644
			file.members["awf"] = member
			files[name] = file
			writeFixtureArchive(t, dist, name, file)
			writeFixtureChecksums(t, dist, files)
		}, want: `member "awf" mode -rw-r--r--, want 0755`},
		{name: "wrong ownership", mutate: func(t *testing.T, dist string, files map[string]archiveFixture) {
			name := archiveName("linux", "arm64", "tar.gz")
			file := files[name]
			member := file.members["LICENSE"]
			member.uid = 99
			member.uname = "builder"
			file.members["LICENSE"] = member
			files[name] = file
			writeFixtureArchive(t, dist, name, file)
			writeFixtureChecksums(t, dist, files)
		}, want: `member "LICENSE" ownership is not root:root`},
		{name: "missing checksum", mutate: func(t *testing.T, dist string, files map[string]archiveFixture) {
			delete(files, archiveName("darwin", "arm64", "tar.gz"))
			writeFixtureChecksums(t, dist, files)
		}, want: "checksum entries do not match release archives"},
		{name: "missing checksum file", mutate: func(t *testing.T, dist string, _ map[string]archiveFixture) {
			if err := os.Remove(filepath.Join(dist, "checksums.txt")); err != nil {
				t.Fatal(err)
			}
		}, want: "read checksums"},
		{name: "malformed checksum", mutate: func(t *testing.T, dist string, _ map[string]archiveFixture) {
			if err := os.WriteFile(filepath.Join(dist, "checksums.txt"), []byte("not-a-checksum\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, want: "malformed checksum entry"},
		{name: "uppercase checksum", mutate: func(t *testing.T, dist string, _ map[string]archiveFixture) {
			path := filepath.Join(dist, "checksums.txt")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.ToUpper(string(raw))), 0o600); err != nil {
				t.Fatal(err)
			}
		}, want: "malformed checksum entry"},
		{name: "checksum names wrong target", mutate: func(t *testing.T, dist string, _ map[string]archiveFixture) {
			path := filepath.Join(dist, "checksums.txt")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			raw = bytes.Replace(raw, []byte(archiveName("darwin", "amd64", "tar.gz")), []byte("foreign.tar.gz"), 1)
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatal(err)
			}
		}, want: `missing checksum for "awf_1.2.3_darwin_amd64.tar.gz"`},
		{name: "duplicate checksum", mutate: func(t *testing.T, dist string, _ map[string]archiveFixture) {
			path := filepath.Join(dist, "checksums.txt")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			line := strings.SplitN(string(raw), "\n", 2)[0]
			if err := os.WriteFile(path, append(raw, []byte(line+"\n")...), 0o600); err != nil {
				t.Fatal(err)
			}
		}, want: "duplicate checksum entry"},
		{name: "checksum mismatch", mutate: func(t *testing.T, dist string, _ map[string]archiveFixture) {
			path := filepath.Join(dist, archiveName("darwin", "amd64", "tar.gz"))
			file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.Write([]byte("x")); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}, want: `checksum mismatch for "awf_1.2.3_darwin_amd64.tar.gz"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dist, files := canonicalArchiveFixture(t)
			if tc.mutate != nil {
				tc.mutate(t, dist, files)
			}
			err := verifyArchivesWithExtractor(dist, func(_, _ string) error { return nil })
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestVerifyArchivesReportsRestrictedRootlessExtractionFailure(t *testing.T) {
	dist, _ := canonicalArchiveFixture(t)
	cause := errors.New("fixture extraction failure")
	err := verifyArchivesWithExtractor(dist, func(_, _ string) error { return cause })
	if err == nil || !errors.Is(err, cause) || !strings.Contains(err.Error(), `restricted rootless extraction failed for "awf_1.2.3_linux_amd64.tar.gz"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestReadArchiveRefusals(t *testing.T) {
	if _, err := readArchive(filepath.Join(t.TempDir(), "missing.tar.gz")); err == nil {
		t.Fatal("missing archive accepted")
	}
	path := filepath.Join(t.TempDir(), "truncated.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	compressed := gzip.NewWriter(file)
	if _, err := compressed.Write([]byte("truncated tar body")); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readArchive(path); err == nil {
		t.Fatal("truncated tar accepted")
	}

	var complete bytes.Buffer
	writer := tar.NewWriter(&complete)
	if err := writer.WriteHeader(&tar.Header{Name: "LICENSE", Mode: 0o644, Size: 1024}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(make([]byte, 1024)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	memberBodyPath := filepath.Join(t.TempDir(), "truncated-member-body.tar.gz")
	memberBodyFile, err := os.Create(memberBodyPath)
	if err != nil {
		t.Fatal(err)
	}
	memberBodyGzip := gzip.NewWriter(memberBodyFile)
	if _, err := memberBodyGzip.Write(complete.Bytes()[:512+100]); err != nil {
		t.Fatal(err)
	}
	if err := memberBodyGzip.Close(); err != nil {
		t.Fatal(err)
	}
	if err := memberBodyFile.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readArchive(memberBodyPath); err == nil {
		t.Fatal("truncated tar member body accepted")
	}
}

func TestVerifyArchiveContentsRefusals(t *testing.T) {
	archive := releaseArchive{name: "fixture.tar.gz", os: "darwin"}
	canonical := []archiveEntry{
		{name: "LICENSE", mode: 0o644, regular: true},
		{name: "README.md", mode: 0o644, regular: true},
		{name: "awf", mode: 0o755, regular: true},
	}
	for _, tc := range []struct {
		name    string
		entries []archiveEntry
		want    string
	}{
		{name: "missing", entries: canonical[:2], want: `is missing member "awf"`},
		{name: "duplicate", entries: append(slices.Clone(canonical), canonical[0]), want: `has unexpected member "LICENSE"`},
		{name: "nonregular", entries: []archiveEntry{{name: "LICENSE", mode: 0o644}, canonical[1], canonical[2]}, want: `member "LICENSE" is not a regular file`},
		{name: "darwin root owner", entries: []archiveEntry{{name: "LICENSE", mode: 0o644, regular: true, uname: "root"}, canonical[1], canonical[2]}, want: `member "LICENSE" has unexpected root ownership`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyArchiveContents(archive, tc.entries)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestRestrictedRootlessExtractCommandAndMembership(t *testing.T) {
	bin := t.TempDir()
	argsPath := filepath.Join(t.TempDir(), "args")
	t.Setenv("ARGS", argsPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeUnshare := func(body string) {
		t.Helper()
		path := filepath.Join(bin, "unshare")
		if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeUnshare(`printf '%s\n' "$*" > "$ARGS"
while [ "$#" -gt 0 ]; do
  if [ "$1" = -C ]; then shift; destination="$1"; fi
  shift
done
touch "$destination/LICENSE" "$destination/README.md" "$destination/awf"
`)
	destination := filepath.Join(t.TempDir(), "nested", "extract")
	if err := restrictedRootlessExtract("fixture.tar.gz", destination); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(raw)); got != "--user --map-root-user tar -xzf fixture.tar.gz -C "+destination {
		t.Fatalf("unshare arguments = %q", got)
	}

	writeUnshare("echo denied >&2\nexit 23\n")
	if err := restrictedRootlessExtract("fixture.tar.gz", filepath.Join(t.TempDir(), "failure")); err == nil || !strings.Contains(err.Error(), "exit status 23: denied") {
		t.Fatalf("failure error = %v", err)
	}
	writeUnshare(`while [ "$#" -gt 0 ]; do
  if [ "$1" = -C ]; then shift; destination="$1"; fi
  shift
done
touch "$destination/extra"
`)
	if err := restrictedRootlessExtract("fixture.tar.gz", filepath.Join(t.TempDir(), "extra")); err == nil || !strings.Contains(err.Error(), "extracted members are not canonical") {
		t.Fatalf("membership error = %v", err)
	}
	writeUnshare(`while [ "$#" -gt 0 ]; do
  if [ "$1" = -C ]; then shift; destination="$1"; fi
  shift
done
rm -rf "$destination"
`)
	if err := restrictedRootlessExtract("fixture.tar.gz", filepath.Join(t.TempDir(), "removed")); err == nil {
		t.Fatal("removed extraction root accepted")
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := restrictedRootlessExtract("fixture.tar.gz", filepath.Join(blocked, "child")); err == nil {
		t.Fatal("non-directory extraction root accepted")
	}
}

func TestDispatchRoutesArchiveVerification(t *testing.T) {
	var out, errb bytes.Buffer
	if code := dispatch([]string{"--verify-archives", filepath.Join(t.TempDir(), "missing")}, fstest.MapFS{}, fstest.MapFS{}, &out, &errb, http.DefaultClient, "", func(string) string { return "" }); code != 1 || !strings.Contains(errb.String(), "release archives: read dist root") {
		t.Fatalf("archive dispatch = %d, %q", code, errb.String())
	}

	dist, _ := canonicalArchiveFixture(t)
	bin := t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	unshare := `#!/bin/sh
set -eu
while [ "$#" -gt 0 ]; do
  if [ "$1" = -C ]; then shift; destination="$1"; fi
  shift
done
touch "$destination/LICENSE" "$destination/README.md" "$destination/awf"
`
	if err := os.WriteFile(filepath.Join(bin, "unshare"), []byte(unshare), 0o755); err != nil {
		t.Fatal(err)
	}
	errb.Reset()
	if code := dispatch([]string{"--verify-archives", dist}, fstest.MapFS{}, fstest.MapFS{}, &out, &errb, http.DefaultClient, "", func(string) string { return "" }); code != 0 {
		t.Fatalf("successful archive dispatch = %d, %q", code, errb.String())
	}
}

type fixtureMember struct {
	mode         int64
	uid, gid     int
	uname, gname string
}
type archiveFixture struct {
	members map[string]fixtureMember
}

func archiveName(os, arch, format string) string {
	return "awf_1.2.3_" + os + "_" + arch + "." + format
}

func canonicalArchiveFixture(t *testing.T) (string, map[string]archiveFixture) {
	t.Helper()
	dist := t.TempDir()
	files := map[string]archiveFixture{}
	for _, target := range releaseTargets {
		format := "tar.gz"
		member := fixtureMember{mode: 0o644, uid: 1000, gid: 1000, uname: "builder", gname: "builder"}
		if target.os == "linux" {
			member = fixtureMember{mode: 0o644, uname: "root", gname: "root"}
		}
		binary := member
		binary.mode = 0o755
		name := archiveName(target.os, target.arch, format)
		files[name] = archiveFixture{members: map[string]fixtureMember{"LICENSE": member, "README.md": member, "awf": binary}}
		writeFixtureArchive(t, dist, name, files[name])
	}
	writeFixtureChecksums(t, dist, files)
	return dist, files
}

func writeFixtureArchive(t *testing.T, dist, name string, fixture archiveFixture) {
	t.Helper()
	file, err := os.Create(filepath.Join(dist, name))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	compressed := gzip.NewWriter(file)
	writer := tar.NewWriter(compressed)
	names := make([]string, 0, len(fixture.members))
	for name := range fixture.members {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		member := fixture.members[name]
		header := &tar.Header{Name: name, Mode: member.mode, Size: int64(len(name)), Uid: member.uid, Gid: member.gid, Uname: member.uname, Gname: member.gname}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeFixtureChecksums(t *testing.T, dist string, files map[string]archiveFixture) {
	t.Helper()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var contents strings.Builder
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(dist, name))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&contents, "%x  %s\n", sha256.Sum256(raw), name)
	}
	if err := os.WriteFile(filepath.Join(dist, "checksums.txt"), []byte(contents.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestNativeReleaseSmokeDriver(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("the local behavioral oracle builds and executes the native linux/amd64 candidate")
	}
	dist := t.TempDir()
	binary := filepath.Join(dist, "awf")
	build := exec.Command("go", "build", "-o", binary, "../awf")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build native candidate: %v\n%s", err, out)
	}
	archive := filepath.Join(dist, "awf_"+project.Version+"_linux_amd64.tar.gz")
	archiveFile, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	compressed := gzip.NewWriter(archiveFile)
	writer := tar.NewWriter(compressed)
	info, err := os.Stat(binary)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteHeader(&tar.Header{Name: "awf", Mode: 0o755, Size: info.Size(), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(binary)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(writer, input); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(input.Close(), writer.Close(), compressed.Close(), archiveFile.Close()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(binary); err != nil {
		t.Fatal(err)
	}

	smoke := exec.Command("../../tools/native-release-test/run.sh", dist, "linux", "amd64", project.Version)
	smoke.Env = append(os.Environ(), "TMPDIR="+t.TempDir())
	if out, err := smoke.CombinedOutput(); err != nil {
		t.Fatalf("native release smoke: %v\n%s", err, out)
	} else if !strings.Contains(string(out), "verified linux/amd64 candidate version "+project.Version) {
		t.Fatalf("native release smoke omitted completion identity:\n%s", out)
	}
}

func TestReleaseWorkflowDefersCredentialsUntilPublish(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(string(b), "\n  publish:\n", 2)
	if len(parts) != 2 {
		t.Fatal("release workflow has no publish boundary")
	}
	for _, credential := range []string{"GITHUB_TOKEN", "github.token", "secrets."} {
		if strings.Contains(parts[0], credential) {
			t.Errorf("pre-publication jobs access %q", credential)
		}
	}
	if !strings.Contains(parts[1], "GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}") {
		t.Error("publish job does not explicitly scope its credential")
	}
}
