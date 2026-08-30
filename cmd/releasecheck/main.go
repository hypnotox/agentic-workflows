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
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	changelogfs "github.com/hypnotox/agentic-workflows/changelog"
	"github.com/hypnotox/agentic-workflows/internal/changelog"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/projectlicense"
	"golang.org/x/mod/semver"
)

func main() {
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
	case len(args) == 2 && args[0] == "--verify-archives":
		if err := verifyArchives(args[1]); err != nil {
			fmt.Fprintf(stderr, "releasecheck: release archives: %v\n", err)
			return 1
		}
		return 0
	case len(args) == 2 && args[0] == "--verify-release-notes":
		expected, err := os.ReadFile(args[1])
		if err != nil {
			fmt.Fprintf(stderr, "releasecheck: read curated release notes: %v\n", err)
			return 1
		}
		if err := verifyReleaseNotes(context.Background(), client, apiURL, getenv("GITHUB_REPOSITORY"), getenv("GITHUB_TOKEN"), getenv("GITHUB_REF_NAME"), expected); err != nil {
			fmt.Fprintf(stderr, "releasecheck: published release notes: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintln(stderr, "usage: releasecheck [--verify-archives <dist-root> | --verify-ci <sha> | --verify-release-notes <curated-notes-file>]")
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

func getGitHubJSON(ctx context.Context, client *http.Client, baseURL, token, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+path, nil)
	if err != nil {
		return fmt.Errorf("build GitHub API request for %s: %w", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request GitHub API %s: %w", path, err)
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

// verifyReleaseNotes checks the hosted body after GoReleaser publishes it.
func verifyReleaseNotes(ctx context.Context, client *http.Client, baseURL, repo, token, tag string, expected []byte) error {
	if baseURL == "" || repo == "" || token == "" || tag == "" {
		return fmt.Errorf("GitHub API URL, GITHUB_REPOSITORY, GITHUB_TOKEN, and GITHUB_REF_NAME are required")
	}
	if strings.TrimSpace(string(expected)) == "" {
		return fmt.Errorf("curated release notes are empty")
	}
	var release struct {
		Body string `json:"body"`
	}
	path := "/repos/" + repo + "/releases/tags/" + url.PathEscape(tag)
	if err := getGitHubJSON(ctx, client, baseURL, token, path, &release); err != nil {
		return err
	}
	if release.Body != string(expected) {
		return fmt.Errorf("published body does not match the curated release notes")
	}
	return nil
}

// verifyCI is the narrow GitHub Actions boundary for release acceptance. The
// base URL is a transport boundary, which permits deterministic API fixtures.
func verifyCI(ctx context.Context, client *http.Client, baseURL, repo, token, sha string) error {
	if baseURL == "" || repo == "" || token == "" || sha == "" {
		return fmt.Errorf("GitHub API URL, GITHUB_REPOSITORY, GITHUB_TOKEN, and requested SHA are required")
	}
	var runs workflowRuns
	if err := getGitHubJSON(ctx, client, baseURL, token, "/repos/"+repo+"/actions/workflows/ci.yml/runs?head_sha="+sha+"&status=completed&per_page=100", &runs); err != nil {
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
		if err := getGitHubJSON(ctx, client, baseURL, token, fmt.Sprintf("/repos/%s/actions/runs/%d/jobs?per_page=100", repo, candidate.ID), &jobs); err != nil {
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
	required := map[string]bool{"gate": false}
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

// verifyArchives validates the release artifacts produced in dist. It owns the
// release portability contract so CI can validate the exact snapshot it built.
func verifyArchives(dist string) error {
	return verifyArchivesWithExtractor(dist, restrictedRootlessExtract)
}

type archiveEntry struct {
	name         string
	mode         fs.FileMode
	uid, gid     int
	uname, gname string
	regular      bool
}

type releaseArchive struct {
	name, path, version, os, arch string
}

var releaseTargets = []struct{ os, arch string }{
	{"darwin", "amd64"}, {"darwin", "arm64"}, {"linux", "amd64"}, {"linux", "arm64"},
}

func verifyArchivesWithExtractor(dist string, extract func(string, string) error) error {
	entries, err := os.ReadDir(dist)
	if err != nil {
		return fmt.Errorf("read dist root: %w", err)
	}
	archives := make(map[string]releaseArchive, len(releaseTargets))
	version := ""
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "awf_") {
			continue
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unexpected release artifact %q", entry.Name())
		}
		archive, ok := parseReleaseArchive(entry.Name())
		if !ok {
			return fmt.Errorf("unexpected release artifact %q", entry.Name())
		}
		key := archive.os + "/" + archive.arch
		if _, exists := archives[key]; exists {
			return fmt.Errorf("duplicate release archive for %s", key)
		}
		if version == "" {
			version = archive.version
		} else if archive.version != version {
			return fmt.Errorf("release archive %q has version %q, want %q", archive.name, archive.version, version)
		}
		archive.path = filepath.Join(dist, entry.Name())
		archives[key] = archive
	}
	for _, target := range releaseTargets {
		key := target.os + "/" + target.arch
		if _, ok := archives[key]; !ok {
			return fmt.Errorf("missing release archive for %s", key)
		}
	}
	if err := verifyArchiveChecksums(filepath.Join(dist, "checksums.txt"), archives); err != nil {
		return err
	}
	for _, target := range releaseTargets {
		archive := archives[target.os+"/"+target.arch]
		contents, err := readArchive(archive.path)
		if err != nil {
			return fmt.Errorf("read archive %q: %w", archive.name, err)
		}
		if err := verifyArchiveContents(archive, contents); err != nil {
			return err
		}
	}
	linux := archives["linux/amd64"]
	extracted, err := os.MkdirTemp(dist, ".releasecheck-extract-")
	if err != nil {
		return fmt.Errorf("prepare restricted rootless extraction: %w", err)
	}
	defer os.RemoveAll(extracted)
	if err := extract(linux.path, extracted); err != nil {
		return fmt.Errorf("restricted rootless extraction failed for %q: %w", linux.name, err)
	}
	return nil
}

func parseReleaseArchive(name string) (releaseArchive, bool) {
	for _, target := range releaseTargets {
		suffix := "_" + target.os + "_" + target.arch + ".tar.gz"
		if strings.HasPrefix(name, "awf_") && strings.HasSuffix(name, suffix) && len(name) > len("awf_")+len(suffix) {
			version := strings.TrimSuffix(strings.TrimPrefix(name, "awf_"), suffix)
			return releaseArchive{name: name, version: version, os: target.os, arch: target.arch}, true
		}
	}
	return releaseArchive{}, false
}

func verifyArchiveChecksums(checksumPath string, archives map[string]releaseArchive) error {
	raw, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}
	checksums := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || len(fields[0]) != sha256.Size*2 || strings.Trim(fields[1], "*") != fields[1] {
			return fmt.Errorf("malformed checksum entry %q", line)
		}
		if _, err := hex.DecodeString(fields[0]); err != nil || strings.ToLower(fields[0]) != fields[0] {
			return fmt.Errorf("malformed checksum entry %q", line)
		}
		if _, exists := checksums[fields[1]]; exists {
			return fmt.Errorf("duplicate checksum entry for %q", fields[1])
		}
		checksums[fields[1]] = fields[0]
	}
	if len(checksums) != len(archives) {
		return fmt.Errorf("checksum entries do not match release archives")
	}
	for _, target := range releaseTargets {
		archive := archives[target.os+"/"+target.arch]
		want, ok := checksums[archive.name]
		if !ok {
			return fmt.Errorf("missing checksum for %q", archive.name)
		}
		file, err := os.Open(archive.path)
		if err != nil {
			return fmt.Errorf("read archive %q: %w", archive.name, err)
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil {
			return fmt.Errorf("read archive %q", archive.name)
		}
		if fmt.Sprintf("%x", hash.Sum(nil)) != want {
			return fmt.Errorf("checksum mismatch for %q", archive.name)
		}
	}
	return nil
}

func readArchive(filename string) ([]archiveEntry, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	var entries []archiveEntry
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return entries, nil
		}
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(io.Discard, reader); err != nil {
			return nil, err
		}
		entries = append(entries, archiveEntry{header.Name, header.FileInfo().Mode(), header.Uid, header.Gid, header.Uname, header.Gname, header.Typeflag == tar.TypeReg})
	}
}

func verifyArchiveContents(archive releaseArchive, entries []archiveEntry) error {
	want := map[string]fs.FileMode{"LICENSE": 0o644, "README.md": 0o644, "awf": 0o755}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !safeArchivePath(entry.name) {
			return fmt.Errorf("archive %q has unsafe path %q", archive.name, entry.name)
		}
		mode, wanted := want[entry.name]
		if !wanted || seen[entry.name] {
			return fmt.Errorf("archive %q has unexpected member %q", archive.name, entry.name)
		}
		seen[entry.name] = true
		if !entry.regular {
			return fmt.Errorf("archive %q member %q is not a regular file", archive.name, entry.name)
		}
		if entry.mode != mode {
			return fmt.Errorf("archive %q member %q mode %v, want %#o", archive.name, entry.name, entry.mode, mode)
		}
		if archive.os == "linux" && (entry.uid != 0 || entry.gid != 0 || entry.uname != "root" || entry.gname != "root") {
			return fmt.Errorf("archive %q member %q ownership is not root:root", archive.name, entry.name)
		}
		if archive.os != "linux" && (entry.uname == "root" || entry.gname == "root") {
			return fmt.Errorf("archive %q member %q has unexpected root ownership", archive.name, entry.name)
		}
	}
	for name := range want {
		if !seen[name] {
			return fmt.Errorf("archive %q is missing member %q", archive.name, name)
		}
	}
	return nil
}

func safeArchivePath(name string) bool {
	return name != "" && !strings.ContainsRune(name, '\x00') && !path.IsAbs(name) && path.Clean(name) == name && name != "." && !strings.HasPrefix(name, "../")
}

func restrictedRootlessExtract(archive, destination string) error {
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	cmd := exec.Command("unshare", "--user", "--map-root-user", "tar", "-xzf", archive, "-C", destination)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	entries, err := os.ReadDir(destination)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "LICENSE,README.md,awf" {
		return fmt.Errorf("extracted members are not canonical")
	}
	return nil
}
