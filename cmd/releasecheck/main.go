// Command releasecheck is the release-time project-license and changelog pin. The
// every-commit gate only guarantees changelog ordering (entries strictly descending,
// newest at or below project.Version); this check closes the exact match at the one moment it matters:
// the Release workflow runs it before GoReleaser, and the release runbook runs it
// locally as the pre-tag rehearsal. It fails unless the newest embedded changelog
// entry equals project.Version and a standing [Unreleased] section is present and
// empty modulo whitespace - so a tag can neither ship without its own release notes
// nor strand late entries under [Unreleased].
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"

	changelogfs "github.com/hypnotox/agentic-workflows/changelog"
	"github.com/hypnotox/agentic-workflows/internal/changelog"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/projectlicense"
	"golang.org/x/mod/semver"
)

func main() { // coverage-ignore: os.Exit wrapper; dispatch is unit-tested
	os.Exit(dispatch(os.Args[1:], os.DirFS("."), changelogfs.FS, os.Stdout, os.Stderr, http.DefaultClient, "https://api.github.com", os.Getenv))
}

// dispatch keeps CLI argument and credential policy at the command boundary.
func dispatch(args []string, root, changelogFS fs.FS, stdout, stderr io.Writer, client *http.Client, apiURL string, getenv func(string) string) int {
	switch {
	case len(args) == 0:
		return run(root, changelogFS, stdout, stderr)
	case len(args) == 2 && args[0] == "--verify-ci":
		if err := verifyCI(context.Background(), client, apiURL, getenv("GITHUB_REPOSITORY"), getenv("GITHUB_TOKEN"), args[1]); err != nil {
			fmt.Fprintf(stderr, "releasecheck: exact CI: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintln(stderr, "usage: releasecheck [--verify-ci <sha>]")
		return 2
	}
}

func run(root, changelogFS fs.FS, stdout, stderr io.Writer) int {
	if err := projectlicense.Verify(root); err != nil {
		fmt.Fprintf(stderr, "releasecheck: project license: %v\n", err)
		return 1
	}
	raw, err := fs.ReadFile(changelogFS, "CHANGELOG.md")
	if err != nil {
		fmt.Fprintf(stderr, "releasecheck: read CHANGELOG.md: %v\n", err)
		return 1
	}
	entries, err := changelog.Parse(raw)
	if err != nil {
		fmt.Fprintf(stderr, "releasecheck: %v\n", err)
		return 1
	}
	fails := 0
	if entries[0].Version != project.Version {
		fmt.Fprintf(stderr, "releasecheck: newest changelog entry %s != project.Version %s; promote [Unreleased] before tagging\n",
			entries[0].Version, project.Version)
		fails++
	}
	// Ordering is ordinarily the gate test's job (inv: changelog-monotonic), but
	// releasecheck remains a self-contained local pre-tag rehearsal. Re-check it
	// here so a mis-sorted file cannot make a stray newer entry pass as pinned
	// merely because entries[0] matched.
	for i := 0; i+1 < len(entries); i++ {
		if semver.Compare("v"+entries[i].Version, "v"+entries[i+1].Version) <= 0 {
			fmt.Fprintf(stderr, "releasecheck: changelog entries out of order: %s is not strictly newer than %s\n",
				entries[i].Version, entries[i+1].Version)
			fails++
		}
	}
	switch body, found := unreleasedBody(string(raw)); {
	case !found:
		fmt.Fprintln(stderr, "releasecheck: no ## [Unreleased] section; restore the standing header (the changelog-unreleased audit rule keys on it)")
		fails++
	case strings.TrimSpace(body) != "":
		fmt.Fprintln(stderr, "releasecheck: [Unreleased] is not empty; fold its entries into the release section before tagging")
		fails++
	}
	if fails > 0 {
		return 1
	}
	fmt.Fprintf(stdout, "releasecheck: changelog pins %s and [Unreleased] is empty\n", project.Version)
	return 0
}

// unreleasedBody returns the body between the "## [Unreleased]" header and the next
// top-level "## [" header (or EOF), and whether the header was found at all. The
// section walk deliberately duplicates repoaudit's git-bound unreleasedSection: the
// two read from different sources (embedded bytes vs `git show`), per ADR-0078.
func unreleasedBody(raw string) (string, bool) {
	var body []string
	in := false
	for _, ln := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(ln, "## [Unreleased]"):
			in = true
		case in && strings.HasPrefix(ln, "## ["):
			return strings.Join(body, "\n"), true
		case in:
			body = append(body, ln)
		}
	}
	if !in {
		return "", false
	}
	return strings.Join(body, "\n"), true
}

type workflowRuns struct {
	Total int           `json:"total_count"`
	Runs  []workflowRun `json:"workflow_runs"`
}
type workflowRun struct {
	ID         int64  `json:"id"`
	HeadSHA    string `json:"head_sha"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Path       string `json:"path"`
	Name       string `json:"name"`
}
type jobsResponse struct {
	Total int           `json:"total_count"`
	Jobs  []workflowJob `json:"jobs"`
}
type workflowJob struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// verifyCI is the narrow GitHub Actions boundary for release acceptance. The
// base URL is a transport boundary, which permits deterministic API fixtures.
func verifyCI(ctx context.Context, client *http.Client, baseURL, repo, token, sha string) error {
	if baseURL == "" || repo == "" || token == "" || sha == "" {
		return fmt.Errorf("GitHub API URL, GITHUB_REPOSITORY, GITHUB_TOKEN, and requested SHA are required")
	}
	get := func(path string, dst any) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+path, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("GitHub API %s returned %s", path, resp.Status)
		}
		if resp.Header.Get("Link") != "" {
			return fmt.Errorf("GitHub API pagination is incomplete")
		}
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(dst); err != nil {
			return fmt.Errorf("decode GitHub API %s: %w", path, err)
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			if err == nil {
				return fmt.Errorf("decode GitHub API %s: trailing JSON document", path)
			}
			return fmt.Errorf("decode GitHub API %s: trailing data: %w", path, err)
		}
		return nil
	}
	var runs workflowRuns
	if err := get("/repos/"+repo+"/actions/workflows/ci.yml/runs?head_sha="+sha+"&status=completed&per_page=100", &runs); err != nil {
		return err
	}
	if runs.Total != len(runs.Runs) {
		return fmt.Errorf("CI run pagination is incomplete")
	}
	var candidates []workflowRun
	for _, run := range runs.Runs {
		if run.HeadSHA == sha && run.Status == "completed" && run.Conclusion == "success" && run.Path == ".github/workflows/ci.yml" && run.Name == "CI" {
			candidates = append(candidates, run)
		}
	}
	if len(candidates) == 0 {
		return fmt.Errorf("no completed successful CI run for exact SHA %s", sha)
	}
	var candidateErrors []error
	for _, candidate := range candidates {
		var jobs jobsResponse
		if err := get(fmt.Sprintf("/repos/%s/actions/runs/%d/jobs?per_page=100", repo, candidate.ID), &jobs); err != nil {
			candidateErrors = append(candidateErrors, fmt.Errorf("run %d: %w", candidate.ID, err))
			continue
		}
		if err := verifyRequiredJobs(jobs); err != nil {
			candidateErrors = append(candidateErrors, fmt.Errorf("run %d: %w", candidate.ID, err))
			continue
		}
		return nil
	}
	return fmt.Errorf("no completed successful CI run for exact SHA %s has complete required evidence: %w", sha, errors.Join(candidateErrors...))
}

func verifyRequiredJobs(jobs jobsResponse) error {
	if jobs.Total != len(jobs.Jobs) {
		return fmt.Errorf("CI jobs pagination is incomplete")
	}
	required := map[string]bool{"gate": false, "release-config": false}
	for _, job := range jobs.Jobs {
		if _, ok := required[job.Name]; !ok {
			continue
		}
		if job.Status != "completed" || job.Conclusion != "success" {
			return fmt.Errorf("required CI job %q did not complete successfully", job.Name)
		}
		if required[job.Name] {
			return fmt.Errorf("duplicate required CI job %q", job.Name)
		}
		required[job.Name] = true
	}
	for name, ok := range required {
		if !ok {
			return fmt.Errorf("required CI job %q did not complete successfully", name)
		}
	}
	return nil
}
