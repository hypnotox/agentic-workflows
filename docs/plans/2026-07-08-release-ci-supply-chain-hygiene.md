# 2026-07-08 — Release and CI supply-chain hygiene

**Goal:** implement [ADR-0079](../decisions/0079-release-and-ci-supply-chain-hygiene.md)
(SHA-pinned workflow actions machine-enforced by a new `cmd/pincheck`, dependabot,
gate-and-ancestry checks on tag push, token-optional Codecov uploads, documented-accept
tamper posture) plus one ADR-external fix: the bootstrap template's unsupported-platform
failure points at the manual-install path.

**Architecture summary:** the workflow files and `dependabot.yml` are hand-maintained,
outside awf's render/lock set — direct edits. `cmd/pincheck` follows the repo's
checker-cmd idiom (coverage-ignored `main` wrapping a unit-tested `run` seam) and is
wired into `./x gate`; its wiring test plus a real-tree test give the
`workflow-actions-sha-pinned` invariant teeth. The release-workflow step ordering is
backed by a new wiring test beside `TestReleaseWorkflowRunsReleasecheck`
(`release-gate-on-tag`). Gate-composition prose lives in awf-rendered docs, so those
edits go through `.awf/` parts + `./x sync`. Design rationale lives in the ADR — not
duplicated here.

**Tech stack:** Go 1.26; stdlib only (`io/fs`, `regexp`, `strings`, `testing/fstest`).
Packages touched: new `cmd/pincheck`; `cmd/releasecheck` (test only);
`internal/project` (test only); `templates/bootstrap`.

**File structure:**

- Created: `.github/dependabot.yml`, `cmd/pincheck/main.go`, `cmd/pincheck/main_test.go`
- Modified: `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `x`,
  `cmd/releasecheck/main_test.go`, `templates/bootstrap/awf-bootstrap.sh.tmpl`,
  `internal/project/bootstrap_test.go`, `changelog/CHANGELOG.md`,
  `.awf/parts/workflow/composing-the-gate.md`, `.awf/docs/parts/testing/gate.md`,
  `.awf/docs/parts/development/command-runner.md`, `.awf/agents-doc.yaml`,
  `.awf/domains/parts/rendering/current-state.md`,
  `.awf/domains/parts/tooling/current-state.md`, `docs/releasing.md`,
  `docs/decisions/0079-release-and-ci-supply-chain-hygiene.md` (status flip),
  plus rendered files refreshed by `./x sync` (`AGENTS.md`, `CLAUDE.md` bridge targets,
  `docs/workflow.md`, `docs/testing.md`, `docs/development.md`,
  `docs/domains/rendering.md`, `docs/domains/tooling.md`, `docs/decisions/ACTIVE.md`,
  `.awf/awf.lock`)
- Deleted: none

**Resolved pins** (verified against upstream tags 2026-07-08; the commit SHA is the
peeled `^{}` object for annotated tags):

| Action | Pin |
|---|---|
| `actions/checkout` | `df4cb1c069e1874edd31b4311f1884172cec0e10` # v6.0.3 |
| `actions/setup-go` | `924ae3a1cded613372ab5595356fb5720e22ba16` # v6.5.0 |
| `goreleaser/goreleaser-action` | `f06c13b6b1a9625abc9e6e439d9c05a8f2190e94` # v7.2.3 |
| `codecov/codecov-action` | `0fb7174895f61a3b6b78fc075e0cd60383518dac` # v5.5.5 |
| GoReleaser tool (`version:` input + runbook) | `v2.17.0` |

---

## Phase 1 — SHA-pin the workflow actions and the GoReleaser tool

- [ ] In `.github/workflows/ci.yml`, replace the action references (four `uses:` lines
      keep their surrounding context; the two `version:` lines sit in the
      `release-config` job):

      - `      - uses: actions/checkout@v6` (gate job, no `with:`) →
        `      - uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3`
      - `      - uses: actions/setup-go@v6` (gate job) →
        `      - uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6.5.0`
      - `      - uses: actions/checkout@v6` (release-config job, has `fetch-depth: 0`) →
        `      - uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3`
      - `      - uses: actions/setup-go@v6` (release-config job) →
        `      - uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6.5.0`
      - both `      - uses: goreleaser/goreleaser-action@v7` →
        `      - uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3`
      - both `          version: '~> v2'` → `          version: v2.17.0`
      - both `      - uses: codecov/codecov-action@v5` →
        `      - uses: codecov/codecov-action@0fb7174895f61a3b6b78fc075e0cd60383518dac # v5.5.5`

- [ ] In `.github/workflows/release.yml`:

      - `      - uses: actions/checkout@v6` →
        `      - uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3`
      - `      - uses: actions/setup-go@v6` →
        `      - uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6.5.0`
      - `      - uses: goreleaser/goreleaser-action@v7` →
        `      - uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3`
      - `          version: '~> v2'` → `          version: v2.17.0`

- [ ] Run `./x gate` — green (workflow files are outside the Go build; expect
      `coverage: 100.0%`, `0 issues.`, `deadcodecheck: no production dead code`).
- [ ] Commit:

      ```
      git add .github/workflows/ci.yml .github/workflows/release.yml
      git commit -m "ci: SHA-pin workflow actions and pin GoReleaser to v2.17.0

      ADR-0079 Decision 1: remote uses: refs pin full commit SHAs with
      dependabot-maintained version comments, and the GoReleaser tool
      version is exact everywhere so neither a moved tag nor a floated
      range can inject unreviewed code into CI."
      ```

## Phase 2 — Token-optional Codecov uploads

- [ ] In `.github/workflows/ci.yml`, add a job-level `env:` to the `gate` job
      (between `runs-on: ubuntu-latest` and `steps:`):

      ```yaml
          env:
          CODECOV_TOKEN: ${{ secrets.CODECOV_TOKEN }}
      ```

      (indented to match: `env:` at the same level as `runs-on:`, the mapping key one
      level deeper.)

- [ ] Add `        if: env.CODECOV_TOKEN != ''` as the first line under each of the two
      upload step names (`- name: Upload raw coverage to Codecov` and
      `- name: Upload covered coverage to Codecov`, above their `uses:` lines), and
      extend the existing `# disable_search …` comment block with:

      ```yaml
      # Fork and dependabot PRs run without repo secrets: both uploads skip there
      # (ADR-0079) — coverage enforcement is ./x gate, Codecov is reporting only.
      ```

- [ ] Run `./x gate` — green.
- [ ] Commit:

      ```
      git add .github/workflows/ci.yml
      git commit -m "ci: make Codecov uploads token-optional

      ADR-0079 Decision 4 (partially amends ADR-0065 Decision 3): fork and
      dependabot PRs have no repo secrets, so the upload steps skip when
      CODECOV_TOKEN is absent instead of failing their CI; enforcement
      stays in ./x gate, where the 100% floor lives."
      ```

## Phase 3 — Dependabot

- [ ] Create `.github/dependabot.yml`:

      ```yaml
      # Staleness counterpart to the SHA pins (ADR-0079 Decision 2): dependabot
      # bumps the pinned action SHAs (keeping the # vX.Y.Z comments) and Go deps.
      # A red gate on a gomod tool bump is signal about the new tool version.
      version: 2
      updates:
        - package-ecosystem: github-actions
          directory: /
          schedule:
            interval: weekly
        - package-ecosystem: gomod
          directory: /
          schedule:
            interval: weekly
      ```

- [ ] Run `./x gate` — green.
- [ ] Commit:

      ```
      git add .github/dependabot.yml
      git commit -m "ci: add weekly dependabot for actions and gomod

      ADR-0079 Decision 2: SHA pins without an updater rot silently."
      ```

## Phase 4 — Gate the release workflow on tag push

- [ ] In `.github/workflows/release.yml`, insert three steps between
      `- name: Verify changelog pins the release` / `run: go run ./cmd/releasecheck`
      and the `goreleaser/goreleaser-action` step:

      ```yaml
            - name: Verify tagged commit is on main
              run: |
                git fetch origin main
                if ! git merge-base --is-ancestor HEAD origin/main; then
                  echo "tagged commit $(git rev-parse HEAD) is not on origin/main — push main first" >&2
                  exit 1
                fi
            - name: Gate (test + 100% coverage + vet + lint)
              run: ./x gate
            - name: Drift check (rendered output matches config)
              run: ./x check
      ```

- [ ] Append to `cmd/releasecheck/main_test.go`:

      ```go
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
      ```

- [ ] Run `go test ./cmd/releasecheck` — `ok`.
- [ ] Run `./x gate` — green.
- [ ] Commit:

      ```
      git add .github/workflows/release.yml cmd/releasecheck/main_test.go
      git commit -m "ci(tooling): gate the release workflow on tag push (ADR-0079)

      ADR-0079 Decision 3: release.yml refuses a tag whose commit is not
      an ancestor of origin/main (HEAD, not GITHUB_SHA, so annotated tags
      resolve to the commit) and runs ./x gate && ./x check before
      GoReleaser; a wiring test beside TestReleaseWorkflowRunsReleasecheck
      backs inv: release-gate-on-tag."
      ```

## Phase 5 — `cmd/pincheck`, gate wiring, and doc parts

- [ ] Create `cmd/pincheck/main.go`:

      ```go
      // Command pincheck is the workflow supply-chain pin gate (ADR-0079). Every
      // remote `uses:` reference under .github/workflows must pin a full 40-hex
      // commit SHA (repo-local `./` references are exempt — they are repo code;
      // `docker://` references must pin an image digest), and every
      // goreleaser-action `version:` input must be an exact semver version, so
      // neither a moved tag nor a re-floated tool range can inject unreviewed code
      // into CI. ./x gate runs it on every commit.
      package main

      import (
      	"fmt"
      	"io"
      	"io/fs"
      	"os"
      	"regexp"
      	"strings"
      )

      func main() { os.Exit(run(os.DirFS(".github/workflows"), os.Stdout, os.Stderr)) } // coverage-ignore: os.Exit wrapper; run is unit-tested

      var (
      	commitSHA   = regexp.MustCompile(`^[0-9a-f]{40}$`)
      	imageDigest = regexp.MustCompile(`@sha256:[0-9a-f]{64}$`)
      	exactSemver = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
      )

      // invariant: workflow-actions-sha-pinned
      func run(fsys fs.FS, stdout, stderr io.Writer) int {
      	entries, err := fs.ReadDir(fsys, ".")
      	if err != nil {
      		fmt.Fprintf(stderr, "pincheck: read .github/workflows: %v\n", err)
      		return 1
      	}
      	var files []string
      	for _, e := range entries {
      		if n := e.Name(); !e.IsDir() && (strings.HasSuffix(n, ".yml") || strings.HasSuffix(n, ".yaml")) {
      			files = append(files, n)
      		}
      	}
      	if len(files) == 0 {
      		fmt.Fprintln(stderr, "pincheck: no workflow files found (run from the repo root)")
      		return 1
      	}
      	fails := 0
      	for _, name := range files {
      		b, err := fs.ReadFile(fsys, name)
      		if err != nil {
      			fmt.Fprintf(stderr, "pincheck: %s: %v\n", name, err)
      			fails++
      			continue
      		}
      		fails += checkFile(name, string(b), stderr)
      	}
      	if fails > 0 {
      		return 1
      	}
      	fmt.Fprintln(stdout, "pincheck: all workflow references pinned")
      	return 0
      }

      // checkFile scans one workflow's lines and reports every violation. Line-based
      // on purpose: the workflow YAML here is flat enough that `uses:`/`version:`
      // key scans are exact, and a parser dependency would outweigh the rule.
      func checkFile(name, content string, stderr io.Writer) int {
      	fails := 0
      	lastUses := ""
      	for i, raw := range strings.Split(content, "\n") {
      		ln := strings.TrimSpace(raw)
      		if c := strings.Index(ln, " #"); c >= 0 {
      			ln = strings.TrimSpace(ln[:c])
      		}
      		ln = strings.TrimPrefix(ln, "- ")
      		switch {
      		case strings.HasPrefix(ln, "uses:"):
      			ref := unquote(strings.TrimSpace(strings.TrimPrefix(ln, "uses:")))
      			lastUses = ref
      			if bad := usesViolation(ref); bad != "" {
      				fmt.Fprintf(stderr, "pincheck: %s:%d: %s: %s\n", name, i+1, bad, ref)
      				fails++
      			}
      		case strings.HasPrefix(ln, "version:") && strings.HasPrefix(lastUses, "goreleaser/goreleaser-action@"):
      			v := unquote(strings.TrimSpace(strings.TrimPrefix(ln, "version:")))
      			if !exactSemver.MatchString(v) {
      				fmt.Fprintf(stderr, "pincheck: %s:%d: goreleaser version must be an exact vX.Y.Z, got: %s\n", name, i+1, v)
      				fails++
      			}
      		}
      	}
      	return fails
      }

      // usesViolation classifies a uses: reference; empty means acceptably pinned.
      func usesViolation(ref string) string {
      	switch {
      	case strings.HasPrefix(ref, "./"):
      		return "" // repo-local action: repo code, nothing to pin
      	case strings.HasPrefix(ref, "docker://"):
      		if imageDigest.MatchString(ref) {
      			return ""
      		}
      		return "docker reference must pin an image digest"
      	default:
      		at := strings.LastIndex(ref, "@")
      		if at >= 0 && commitSHA.MatchString(ref[at+1:]) {
      			return ""
      		}
      		return "action must pin a full 40-hex commit SHA"
      	}
      }

      func unquote(s string) string {
      	return strings.Trim(s, `'"`)
      }
      ```

- [ ] Create `cmd/pincheck/main_test.go`:

      ```go
      package main

      import (
      	"bytes"
      	"errors"
      	"os"
      	"strings"
      	"testing"
      	"testing/fstest"
      )

      func wfFS(files map[string]string) fstest.MapFS {
      	m := fstest.MapFS{}
      	for name, content := range files {
      		m[name] = &fstest.MapFile{Data: []byte(content)}
      	}
      	return m
      }

      func runOn(t *testing.T, fsys fs.FS) (int, string, string) {
      	t.Helper()
      	var out, errb bytes.Buffer
      	code := run(fsys, &out, &errb)
      	return code, out.String(), errb.String()
      }

      const pinnedWorkflow = `jobs:
        gate:
          steps:
            - uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3
            - uses: ./.github/actions/local-helper
            - uses: docker://alpine@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
            - uses: codecov/codecov-action@0fb7174895f61a3b6b78fc075e0cd60383518dac # v5.5.5
              with:
                token: x
            - uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3
              with:
                version: 'v2.17.0'
      `

      func TestRunPassesPinned(t *testing.T) {
      	code, out, errb := runOn(t, wfFS(map[string]string{"ci.yml": pinnedWorkflow}))
      	if code != 0 {
      		t.Fatalf("want exit 0, got %d, stderr:\n%s", code, errb)
      	}
      	if !strings.Contains(out, "all workflow references pinned") {
      		t.Errorf("expected confirmation on stdout, got:\n%s", out)
      	}
      }

      func TestRunFailsTagPinnedAction(t *testing.T) {
      	code, _, errb := runOn(t, wfFS(map[string]string{"ci.yml": "      - uses: actions/checkout@v6\n"}))
      	if code != 1 || !strings.Contains(errb, "full 40-hex commit SHA") {
      		t.Fatalf("want exit 1 with SHA error, got %d:\n%s", code, errb)
      	}
      }

      func TestRunFailsUndigestedDocker(t *testing.T) {
      	code, _, errb := runOn(t, wfFS(map[string]string{"ci.yml": "      - uses: docker://alpine:3.20\n"}))
      	if code != 1 || !strings.Contains(errb, "image digest") {
      		t.Fatalf("want exit 1 with digest error, got %d:\n%s", code, errb)
      	}
      }

      func TestRunFailsFloatedGoreleaserVersion(t *testing.T) {
      	wf := "      - uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3\n        with:\n          version: '~> v2'\n"
      	code, _, errb := runOn(t, wfFS(map[string]string{"release.yaml": wf}))
      	if code != 1 || !strings.Contains(errb, "exact vX.Y.Z") {
      		t.Fatalf("want exit 1 with version error (and .yaml scanned), got %d:\n%s", code, errb)
      	}
      }

      func TestRunIgnoresVersionKeysOfOtherActions(t *testing.T) {
      	wf := "      - uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6.5.0\n        with:\n          version: not-semver-but-not-goreleaser\n"
      	if code, _, errb := runOn(t, wfFS(map[string]string{"ci.yml": wf})); code != 0 {
      		t.Fatalf("version under a non-goreleaser action must be ignored, got %d:\n%s", code, errb)
      	}
      }

      func TestRunSkipsDirectoriesAndNonYAML(t *testing.T) {
      	fsys := wfFS(map[string]string{
      		"ci.yml":     pinnedWorkflow,
      		"sub/x.yml":  "      - uses: actions/checkout@v6\n",
      		"README.md":  "      - uses: actions/checkout@v6\n",
      	})
      	if code, _, errb := runOn(t, fsys); code != 0 {
      		t.Fatalf("subdirectories and non-YAML files must be skipped, got %d:\n%s", code, errb)
      	}
      }

      func TestRunFailsNoWorkflowFiles(t *testing.T) {
      	code, _, errb := runOn(t, wfFS(nil))
      	if code != 1 || !strings.Contains(errb, "no workflow files") {
      		t.Fatalf("want exit 1 with no-files error, got %d:\n%s", code, errb)
      	}
      }

      type readDirErrFS struct{ fstest.MapFS }

      func (readDirErrFS) ReadDir(string) ([]fs.DirEntry, error) { return nil, errors.New("boom") }

      func TestRunFailsUnreadableDir(t *testing.T) {
      	code, _, errb := runOn(t, readDirErrFS{})
      	if code != 1 || !strings.Contains(errb, "read .github/workflows") {
      		t.Fatalf("want exit 1 with readdir error, got %d:\n%s", code, errb)
      	}
      }

      type readFileErrFS struct{ fstest.MapFS }

      func (readFileErrFS) ReadFile(string) ([]byte, error) { return nil, errors.New("boom") }

      func TestRunFailsUnreadableFile(t *testing.T) {
      	code, _, errb := runOn(t, readFileErrFS{wfFS(map[string]string{"ci.yml": pinnedWorkflow})})
      	if code != 1 || !strings.Contains(errb, "boom") {
      		t.Fatalf("want exit 1 with read error, got %d:\n%s", code, errb)
      	}
      }

      // TestRepoWorkflowsPinned runs the check against the repo's real workflows, so
      // an unpinned reference fails the test suite even without the ./x gate wiring.
      func TestRepoWorkflowsPinned(t *testing.T) {
      	code, _, errb := runOn(t, os.DirFS("../../.github/workflows"))
      	if code != 0 {
      		t.Fatalf("repo workflows are not pin-clean:\n%s", errb)
      	}
      }
      ```

      Add `"io/fs"` to the test imports (used by `runOn`, `readDirErrFS`).

- [ ] In `x`, add the pin check to the gate (after the deadcode line, inside `gate)`):

      ```
          go tool deadcode -json ./... | go run ./cmd/deadcodecheck
          go run ./cmd/pincheck
      ```

- [ ] In `.awf/parts/workflow/composing-the-gate.md`, replace:
      "`golangci-lint`, and the dead-code gate\n(`cmd/deadcodecheck`, ADR-0063)." with
      "`golangci-lint`, the dead-code gate (`cmd/deadcodecheck`, ADR-0063), and the
      workflow-pin check (`cmd/pincheck`, ADR-0079)."

- [ ] In `.awf/docs/parts/testing/gate.md`, replace "and a whole-program dead-code
      check (ADR-0063) —" with "a whole-program dead-code check (ADR-0063), and the
      workflow supply-chain pin check (`cmd/pincheck`, ADR-0079) —".

- [ ] In `.awf/docs/parts/development/command-runner.md`, in the `./x gate` row,
      replace "and the whole-program dead-code check (`cmd/deadcodecheck`)." with
      "the whole-program dead-code check (`cmd/deadcodecheck`), and the workflow-pin
      check (`cmd/pincheck`, ADR-0079)."

- [ ] In `.awf/agents-doc.yaml`, add after the ADR-0078 invariants entry:

      ```yaml
              - ref: ADR-0079
                text: '**Release and CI supply-chain hygiene.** Every remote `uses:` reference under `.github/workflows/` pins a full 40-hex commit SHA (repo-local `./` refs exempt, `docker://` refs digest-pinned) and every goreleaser-action `version:` input is an exact semver version — `cmd/pincheck`, run by `./x gate`, fails otherwise, and weekly dependabot (actions + gomod) keeps the pins current. `release.yml` refuses a tag whose commit is not on `origin/main` and runs `./x gate` and `./x check` before GoReleaser (a gate test asserts the wiring); Codecov uploads skip without `CODECOV_TOKEN` (enforcement is the local gate); checksum verification is integrity-only — publisher compromise is a documented, deliberately-accepted risk.'
      ```

      (match the file's existing indentation for `- ref:` entries.)

- [ ] Run `./x sync` — refreshes `AGENTS.md`, bridge outputs, `docs/workflow.md`,
      `docs/testing.md`, `docs/development.md`, `.awf/awf.lock`.
- [ ] Run `go run ./cmd/pincheck` — expect `pincheck: all workflow references pinned`.
- [ ] Run `./x gate && ./x check` — green / `awf check: clean`.
- [ ] Commit:

      ```
      git add cmd/pincheck x .awf/parts/workflow/composing-the-gate.md \
        .awf/docs/parts/testing/gate.md .awf/docs/parts/development/command-runner.md \
        .awf/agents-doc.yaml .awf/awf.lock AGENTS.md CLAUDE.md docs/workflow.md \
        docs/testing.md docs/development.md
      git commit -m "feat(tooling): machine-enforce workflow pins via cmd/pincheck (ADR-0079)

      ADR-0079 Decision 1: a gate-run checker fails any unpinned remote
      uses: ref, undigested docker:// ref, or floated goreleaser-action
      version input; a real-tree test gives the invariant teeth inside
      go test as well. Gate-composition docs and the AGENTS.md invariants
      bullet update through their .awf parts."
      ```

      (If `./x check` lists other rendered targets (e.g. cursor outputs), stage those
      too — stage exactly what sync reports, never `git add -A`.)

## Phase 6 — Bootstrap unsupported-platform pointer (ADR-external)

- [ ] In `templates/bootstrap/awf-bootstrap.sh.tmpl`, replace:

      - `  *) echo "awf bootstrap: unsupported arch: $arch" >&2; exit 1 ;;` →
        `  *) echo "awf bootstrap: unsupported arch: $arch — install manually: https://github.com/${REPO}#install" >&2; exit 1 ;;`
      - `  *) echo "awf bootstrap: unsupported os: $os" >&2; exit 1 ;;` →
        `  *) echo "awf bootstrap: unsupported os: $os — install manually: https://github.com/${REPO}#install" >&2; exit 1 ;;`

- [ ] Append to `internal/project/bootstrap_test.go`:

      ```go
      // TestBootstrapUnsupportedPlatformPointsAtManualInstall pins the pointer added
      // for Windows/git-bash users: both unsupported-platform failures must name the
      // manual-install path (README → Install) on their stderr line.
      func TestBootstrapUnsupportedPlatformPointsAtManualInstall(t *testing.T) {
      	rf := bootstrapFile(t, "prefix: example\nbootstrap:\n  enabled: true\n")
      	if rf == nil {
      		t.Fatal("expected .awf/bootstrap.sh to render when enabled")
      	}
      	for _, branch := range []string{"unsupported arch", "unsupported os"} {
      		line := ""
      		for _, ln := range strings.Split(rf.Content, "\n") {
      			if strings.Contains(ln, branch) {
      				line = ln
      				break
      			}
      		}
      		if line == "" {
      			t.Errorf("bootstrap missing the %q branch", branch)
      			continue
      		}
      		if !strings.Contains(line, "#install") {
      			t.Errorf("%q failure must point at the manual-install path: %q", branch, line)
      		}
      	}
      }
      ```

- [ ] In `changelog/CHANGELOG.md`, under `## [Unreleased]`, add:

      ```markdown
      ### Others
      - The bootstrap script's unsupported-OS/arch failure now points at the
        manual-install path (`https://github.com/hypnotox/agentic-workflows#install`),
        so Windows/git-bash users see the way forward instead of a bare error.
      ```

- [ ] In `.awf/domains/parts/rendering/current-state.md`, replace "pinned to the
      rendering binary's `project.Version`, and is excluded from the dead-reference
      scan." with "pinned to the rendering binary's `project.Version`, excluded from
      the dead-reference scan, and pointing its unsupported-OS/arch failures at the
      manual-install README section."
- [ ] Run `./x sync` (refreshes `docs/domains/rendering.md` and the lock), then
      `go test ./internal/project` — `ok`.
- [ ] Run `./x gate && ./x check` — green / clean.
- [ ] Commit:

      ```
      git add templates/bootstrap/awf-bootstrap.sh.tmpl internal/project/bootstrap_test.go \
        changelog/CHANGELOG.md .awf/domains/parts/rendering/current-state.md \
        docs/domains/rendering.md .awf/awf.lock
      git commit -m "fix(rendering): point unsupported-platform bootstrap failure at manual install

      Windows/git-bash users hit the unsupported-os branch with no way
      forward; both failure branches now name the README install section.
      Stderr-only, so the ADR-0049 one-stdout-line contract holds.
      Adopters pick it up at the next release."
      ```

## Phase 7 — Runbook, domain narrative, ADR flip

- [ ] In `docs/releasing.md`:

      - Intro paragraph: replace "which\nverifies the tag, `project.Version`, and
        changelog all pin the same release (see Versioning),\nthen runs GoReleaser"
        with "which verifies the tag, `project.Version`, and changelog all pin the
        same release (see Versioning), verifies the tagged commit is on `main`, runs
        the full gate (`./x gate && ./x check`), then runs GoReleaser".
      - Step 4 ("Tag and push the tag."): after "The tag push starts the `Release`
        workflow." add "It refuses a tag whose commit is not on `origin/main` and
        re-runs the gate before building (ADR-0079), so pushing `main` first (step 3)
        is load-bearing, not just tidy."
      - "Preview locally" section: replace both
        `go run github.com/goreleaser/goreleaser/v2@latest check` /
        `go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean`
        with the `@v2.17.0` forms, and add after the commands: "The version matches
        the `version:` input pinned in the workflows; `cmd/pincheck` enforces the
        workflow side — keep these two commands in step by hand (ADR-0079)."
      - Notes: add a bullet: "**Tamper posture (ADR-0079).** `checksums.txt` and the
        bootstrap's SHA-256 check verify download *integrity*, not publisher
        authenticity — a compromise of the release workflow or its token can rewrite
        binary and checksums together. The accepted mitigations are the SHA-pinned
        actions, dependabot currency, and the gate-and-ancestry checks on tag push;
        artifact attestation and cosign signing are deliberately deferred (revisit at
        1.0 or on adopter demand)."

- [ ] In `.awf/domains/parts/tooling/current-state.md`, replace "and a PR-time
      `goreleaser check`/snapshot job guards the release config." with "and a PR-time
      `goreleaser check`/snapshot job guards the release config. ADR-0079 hardens the
      release path: every remote workflow action reference is SHA-pinned and the
      GoReleaser tool version exact — `cmd/pincheck`, run by `./x gate`, fails an
      unpinned ref, an undigested `docker://` ref, or a floated `version:` input,
      with weekly dependabot keeping the pins current — while `release.yml` refuses a
      tag whose commit is not on `origin/main` and runs `./x gate` and `./x check`
      before building; Codecov uploads skip without their token (coverage enforcement
      is the local gate), and the release checksum is documented as integrity-only,
      publisher compromise being an accepted, recorded risk."
- [ ] Flip `docs/decisions/0079-release-and-ci-supply-chain-hygiene.md` frontmatter
      `status: Proposed` → `status: Implemented`.
- [ ] Run `./x sync` (refreshes `docs/decisions/ACTIVE.md`, `docs/domains/tooling.md`,
      the lock), then `./x invariants` — both new slugs backed
      (`workflow-actions-sha-pinned` in `cmd/pincheck/main.go`, `release-gate-on-tag`
      in `cmd/releasecheck/main_test.go`).
- [ ] Run `./x gate && ./x check` — green / clean. Run `./x audit` — advisory; expect
      no Errors.
- [ ] Commit:

      ```
      git add docs/releasing.md .awf/domains/parts/tooling/current-state.md \
        docs/domains/tooling.md docs/decisions/0079-release-and-ci-supply-chain-hygiene.md \
        docs/decisions/ACTIVE.md .awf/awf.lock
      git commit -m "docs(adr): implement 0079 release and CI supply-chain hygiene

      Runbook documents the new tag-push preconditions, the pinned
      GoReleaser preview commands, and the integrity-only tamper posture;
      the tooling domain narrative absorbs the hardened release path; the
      ADR flips to Implemented with both invariants backed."
      ```

## Post-merge verification (not plan tasks — live GitHub behavior)

After pushing `main`: confirm the CI run is green end-to-end (pinned actions resolve,
Codecov uploads still fire with the token present), confirm in the repo's GitHub
settings that no Codecov status is a required branch-protection check, and expect the
first dependabot PRs within a week. The release-path steps (ancestry check, gate on
tag) prove out fully at the next `v*` tag.
