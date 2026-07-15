# 2026-07-02: Single version authority (ADR-0049)

**Goal:** implement [ADR-0049](../decisions/0049-single-version-authority.md): `project.Version`
becomes the sole binary-version source (gate, note, lock stamp, bootstrap pin, CLI output), build
provenance demotes to display-only, schema-generation bumps mechanically require version bumps,
and the rendered bootstrap gains local-first resolution, path-only stdout, and a `shasum` fallback.

**Architecture summary:** no package moves. `cmd/awf/version.go` collapses to the const plus
pure `versionLine`/`formatProvenance` helpers; `internal/project` gains an unexported
`minVersionBySchema` table (ADR-0049 Decision 4's name: its only consumer is the in-package
backing test, so no export); `templates/bootstrap/awf-bootstrap.sh.tmpl` is rewritten under a stdout-is-API
contract; `.goreleaser.yaml` drops the ldflags injection and `release.yml` gains the tag==const
guard. Design rationale lives in ADR-0049; do not restate it in code comments beyond the ADR ref.

**Tech stack:** Go 1.26; packages touched: `cmd/awf`, `internal/project`, `templates` (bootstrap),
plus `.goreleaser.yaml`, `.github/workflows/release.yml`, `docs/releasing.md`, `.awf/` parts.

**File structure:**
- Created: `internal/project/version_test.go`
- Modified: `cmd/awf/version.go`, `cmd/awf/version_test.go`, `cmd/awf/gate.go` (comment only),
  `.goreleaser.yaml`, `.github/workflows/release.yml`, `internal/project/project.go`,
  `internal/project/bootstrap_test.go`, `templates/bootstrap/awf-bootstrap.sh.tmpl`,
  `docs/releasing.md`, `.awf/agents-doc.yaml`, `.awf/docs/parts/pitfalls/entries.md`,
  `.awf/domains/parts/tooling/current-state.md`, `docs/decisions/0049-single-version-authority.md`
  (status flip), plus re-rendered `AGENTS.md`, `docs/pitfalls.md`, `docs/domains/tooling.md`,
  `docs/decisions/ACTIVE.md`, `.awf/awf.lock`.
- Deleted: none.

---

## Phase 1: awfVersion() single-sourced (ADR-0049 Decisions 1-3)

- [ ] **Task 1.1: replace `cmd/awf/version.go` with the single-authority version.** Full new
  content:

  ```go
  package main

  import (
  	"fmt"
  	"io"
  	"runtime/debug"
  	"strings"

  	"github.com/hypnotox/agentic-workflows/internal/project"
  )

  // runVersion prints the awf version plus display-only build provenance.
  func runVersion(stdout io.Writer) {
  	info, ok := debug.ReadBuildInfo()
  	fmt.Fprintln(stdout, versionLine(info, ok))
  }

  // versionLine renders the "awf <version>" line, appending display-only build
  // provenance when present (ADR-0049 Decision 2). Split from runVersion so
  // every branch is reachable from tests regardless of what the test binary's
  // own build info carries.
  func versionLine(info *debug.BuildInfo, ok bool) string {
  	line := "awf " + awfVersion()
  	if !ok {
  		return line
  	}
  	if p := formatProvenance(info); p != "" {
  		line += " (" + p + ")"
  	}
  	return line
  }

  // awfVersion returns the awf version. project.Version is the single version
  // authority (ADR-0049): no ldflags var or module build info feeds version
  // gating, lock stamping, or bootstrap pinning.
  func awfVersion() string {
  	// invariant: single-version-authority
  	// invariant: version-ldflags-precedence, stale marker kept while ADR-0049
  	// is Proposed (ADR-0030 still requires the slug and a Proposed retirement
  	// does not drop it, ADR-0031); Task 5.2's flip commit deletes these lines.
  	return project.Version
  }

  // formatProvenance renders display-only build metadata: the module version
  // when it adds information beyond the const, and the short VCS revision
  // (ADR-0049 Decision 2).
  func formatProvenance(info *debug.BuildInfo) string {
  	var parts []string
  	if v := info.Main.Version; v != "" && v != "(devel)" && v != "v"+project.Version {
  		parts = append(parts, v)
  	}
  	for _, s := range info.Settings {
  		if s.Key == "vcs.revision" && s.Value != "" {
  			rev := s.Value
  			if len(rev) > 12 {
  				rev = rev[:12]
  			}
  			parts = append(parts, "rev "+rev)
  			break
  		}
  	}
  	return strings.Join(parts, ", ")
  }
  ```

  Notes: the old ldflags `var version string` and the `debug.ReadBuildInfo()` identity branch are
  gone with their stale `coverage-ignore` comment. The `version-ldflags-precedence` marker stays,
  deliberately stale, until the Task 5.2 flip commit: `awf check` enforces invariant backing, and
  while ADR-0049 is Proposed its `retires_invariants` does not drop ADR-0030's slug; removing
  the marker now would fail `./x check` (and the wired pre-commit hook) on every commit before
  the flip. `versionLine` is split from `runVersion` for the 100% coverage gate: under `go test`
  the test binary carries no VCS stamp and no useful `Main.Version`, so the provenance-append
  branch is unreachable via `runVersion` alone and must be covered by direct `versionLine` tests
  (Task 1.2); no `coverage-ignore` directive is needed anywhere in the file.

- [ ] **Task 1.2: replace `TestAwfVersionLdflagsPrecedence` in `cmd/awf/version_test.go`.**
  Keep `TestRunVersion` unchanged. Replace the ldflags test (lines 21-27) with:

  ```go
  func TestAwfVersionSingleAuthority(t *testing.T) {
  	if got := awfVersion(); got != project.Version {
  		t.Errorf("awfVersion() = %q, want project.Version %q", got, project.Version)
  	}
  }

  func TestVersionLine(t *testing.T) {
  	if got, want := versionLine(nil, false), "awf "+project.Version; got != want {
  		t.Errorf("versionLine(no build info) = %q, want %q", got, want)
  	}
  	if got, want := versionLine(&debug.BuildInfo{}, true), "awf "+project.Version; got != want {
  		t.Errorf("versionLine(empty provenance) = %q, want %q", got, want)
  	}
  	info := debug.BuildInfo{Main: debug.Module{Version: "v9.9.9-pre"}}
  	if got, want := versionLine(&info, true), "awf "+project.Version+" (v9.9.9-pre)"; got != want {
  		t.Errorf("versionLine(with provenance) = %q, want %q", got, want)
  	}
  }

  func TestFormatProvenance(t *testing.T) {
  	long := "0123456789abcdef0123456789abcdef01234567"
  	cases := []struct {
  		name string
  		info debug.BuildInfo
  		want string
  	}{
  		{"empty", debug.BuildInfo{}, ""},
  		{"devel skipped", debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, ""},
  		{"const echo skipped", debug.BuildInfo{Main: debug.Module{Version: "v" + project.Version}}, ""},
  		{"pseudo version kept", debug.BuildInfo{Main: debug.Module{Version: "v9.9.9-pre"}}, "v9.9.9-pre"},
  		{"revision truncated", debug.BuildInfo{
  			Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: long}},
  		}, "rev 0123456789ab"},
  		{"short revision kept", debug.BuildInfo{
  			Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}},
  		}, "rev abc123"},
  		{"both joined", debug.BuildInfo{
  			Main:     debug.Module{Version: "v9.9.9-pre"},
  			Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}},
  		}, "v9.9.9-pre, rev abc123"},
  		{"empty revision skipped", debug.BuildInfo{
  			Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: ""}},
  		}, ""},
  	}
  	for _, c := range cases {
  		if got := formatProvenance(&c.info); got != c.want {
  			t.Errorf("%s: formatProvenance() = %q, want %q", c.name, got, c.want)
  		}
  	}
  }
  ```

  Add `"runtime/debug"` to the test file's imports.

- [ ] **Task 1.3: refresh the stale comment on `normalizeSemver` in `cmd/awf/gate.go`.**
  Replace the comment block above `func normalizeSemver` (lines 13-16) with:

  ```go
  // normalizeSemver returns s in the single-leading-v form x/mod/semver requires.
  // project.Version and lock awfVersion values are the no-v form, but historical
  // locks may carry either; trimming any existing v first makes the
  // normalization idempotent (ADR-0039 Decision 3, single-sourced by ADR-0049).
  ```

  No code change in this file.

- [ ] **Task 1.4: drop the ldflags injection from `.goreleaser.yaml`.** Change the `ldflags`
  list (line 16-17) to:

  ```yaml
      ldflags:
        - -s -w
  ```

- [ ] **Task 1.5: add the tag==const guard to `.github/workflows/release.yml`.** Insert between
  the `setup-go` step and the `goreleaser-action` step:

  ```yaml
        - name: Verify tag matches project.Version
          run: |
            tag="${GITHUB_REF_NAME#v}"
            want="$(go run ./cmd/awf version | awk '{print $2}')"
            if [ "$tag" != "$want" ]; then
              echo "tag v${tag} does not match project.Version ${want}" >&2
              exit 1
            fi
  ```

- [ ] **Task 1.6: gate and commit.**

  ```
  ./x gate && ./x check
  git add cmd/awf/version.go cmd/awf/version_test.go cmd/awf/gate.go .goreleaser.yaml .github/workflows/release.yml
  git commit -m "feat(awf): single-source the binary version on project.Version"
  ```

  Expected: gate green at 100% coverage; check clean (the retained stale marker keeps ADR-0030's
  slug backed). Body notes ADR-0049 Decisions 1-3, that the tag guard replaces the retired
  ldflags stamping, and that the stale marker rides until the flip commit (ADR-0031).

## Phase 2: minVersionBySchema + version bump (Decisions 4-5)

- [ ] **Task 2.1: failing test first: create `internal/project/version_test.go`.**

  ```go
  package project

  import (
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/migrate"
  	"golang.org/x/mod/semver"
  )

  // invariant: schema-min-version
  func TestVersionCoversCurrentSchema(t *testing.T) {
  	min, ok := minVersionBySchema[migrate.Current()]
  	if !ok {
  		t.Fatalf("minVersionBySchema has no entry for schema generation %d; add one alongside the migration (ADR-0049 Decision 4)", migrate.Current())
  	}
  	if semver.Compare("v"+Version, "v"+min) < 0 {
  		t.Errorf("project.Version %s is below the minimum %s for schema generation %d; bump the const (ADR-0049 Decision 4)", Version, min, migrate.Current())
  	}
  }
  ```

  Run `go test ./internal/project/ -run TestVersionCoversCurrentSchema`; expected: compile error
  (`minVersionBySchema` undefined), the failing state. The table stays unexported per ADR-0049
  Decision 4's naming: this in-package test is its only consumer.

- [ ] **Task 2.2: add the table and bump the const in `internal/project/project.go`.** Replace
  the bare `const Version = "0.5.1"` (line 23; it carries no doc comment today) with:

  ```go
  // Version is the awf release version, the single version authority
  // (ADR-0049): gate comparisons, the lock stamp, the bootstrap pin, and the
  // CLI output all read this const.
  const Version = "0.6.0"

  // minVersionBySchema maps each config-schema generation to the minimum
  // project.Version allowed to render it; adding a migration without an entry
  // here (and a matching const bump) fails the gate (ADR-0049 Decision 4).
  var minVersionBySchema = map[int]string{
  	6: "0.6.0",
  }
  ```

- [ ] **Task 2.3: restamp the lock and gate.**

  ```
  ./x sync && ./x check
  ```

  Expected: `.awf/awf.lock` changes `"awfVersion": "0.5.1"` → `"0.6.0"`; check clean (a
  transient ahead-note may print on the pre-sync run; it must be absent after sync, which is
  exactly the behaviour ADR-0049 restores).

  ```
  ./x gate
  git add internal/project/project.go internal/project/version_test.go .awf/awf.lock
  git commit -m "feat(awf): tie schema generations to version bumps"
  ```

## Phase 3: bootstrap hardening (Decisions 6-7)

- [ ] **Task 3.1: failing tests first: extend `internal/project/bootstrap_test.go`.**
  Replace `TestBootstrapVerifiesBeforeInstall` and append two tests:

  ```go
  // invariant: bootstrap-checksum
  func TestBootstrapVerifiesBeforeInstall(t *testing.T) {
  	rf := bootstrapFile(t, "prefix: example\nbootstrap:\n  enabled: true\n")
  	if rf == nil {
  		t.Fatal("expected .awf/bootstrap.sh to render when enabled")
  	}
  	install := strings.Index(rf.Content, "install -m 0755")
  	if install < 0 {
  		t.Fatalf("bootstrap missing install step:\n%s", rf.Content)
  	}
  	for _, verify := range []string{"sha256sum -c - >&2", "shasum -a 256 -c - >&2"} {
  		idx := strings.Index(rf.Content, verify)
  		if idx < 0 {
  			t.Fatalf("bootstrap missing checksum branch %q:\n%s", verify, rf.Content)
  		}
  		if idx >= install {
  			t.Errorf("checksum verify %q (index %d) must precede install (index %d)", verify, idx, install)
  		}
  	}
  }

  // invariant: bootstrap-stdout-path-only
  func TestBootstrapStdoutPathOnly(t *testing.T) {
  	rf := bootstrapFile(t, "prefix: example\nbootstrap:\n  enabled: true\n")
  	if rf == nil {
  		t.Fatal("expected .awf/bootstrap.sh to render when enabled")
  	}
  	for _, line := range strings.Split(rf.Content, "\n") {
  		if !strings.Contains(line, "echo ") || strings.Contains(line, ">&2") {
  			continue
  		}
  		if strings.TrimSpace(line) == `echo "$binary"` {
  			continue
  		}
  		t.Errorf("stdout-polluting line in bootstrap: %q (only the binary path may print to stdout)", line)
  	}
  }

  // invariant: bootstrap-local-first
  func TestBootstrapLocalFirstResolution(t *testing.T) {
  	rf := bootstrapFile(t, "prefix: example\nbootstrap:\n  enabled: true\n")
  	if rf == nil {
  		t.Fatal("expected .awf/bootstrap.sh to render when enabled")
  	}
  	probe := strings.Index(rf.Content, "command -v awf")
  	download := strings.Index(rf.Content, "curl ")
  	if probe < 0 {
  		t.Fatalf("bootstrap missing the local PATH probe:\n%s", rf.Content)
  	}
  	if download >= 0 && probe >= download {
  		t.Errorf("local probe (index %d) must precede the download (index %d)", probe, download)
  	}
  	if !strings.Contains(rf.Content, `[ "${local_version}" = "${AWF_VERSION}" ]`) {
  		t.Errorf("local probe must require an exact pinned-version match:\n%s", rf.Content)
  	}
  }
  ```

  Run `go test ./internal/project/ -run 'TestBootstrap'`; expected: the three assertions fail
  against the current template (missing `>&2`, missing shasum branch, missing probe). This is the
  cache-miss stdout regression coverage: the assertions pin the checksum path, which only executes
  on a cache miss. The stdout audit scans every line *containing* `echo ` (not just line-leading
  echoes) so diagnostics embedded in case arms or compound commands cannot slip through without
  a `>&2`.

- [ ] **Task 3.2: rewrite `templates/bootstrap/awf-bootstrap.sh.tmpl`.** Full new content:

  ```bash
  #!/usr/bin/env bash
  # Fetches and verifies a pinned awf binary, caches it, and prints its path.
  # Stdout carries exactly one line: the resolved binary path; every
  # diagnostic goes to stderr (ADR-0049).
  set -euo pipefail

  AWF_VERSION="{{ .version }}"
  REPO="hypnotox/agentic-workflows"

  # Local-first: an awf on PATH reporting exactly the pinned version wins over
  # any download (ADR-0049).
  if command -v awf >/dev/null 2>&1; then
    local_version="$(awf version 2>/dev/null | awk '{print $2}' || true)"
    if [ "${local_version}" = "${AWF_VERSION}" ]; then
      command -v awf
      exit 0
    fi
  fi

  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) echo "awf bootstrap: unsupported arch: $arch" >&2; exit 1 ;;
  esac
  case "$os" in
    linux|darwin) ;;
    *) echo "awf bootstrap: unsupported os: $os" >&2; exit 1 ;;
  esac

  cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/awf/${AWF_VERSION}"
  binary="${cache_dir}/awf"
  if [ -x "$binary" ]; then
    echo "$binary"
    exit 0
  fi

  asset="awf_${AWF_VERSION}_${os}_${arch}.tar.gz"
  base="https://github.com/${REPO}/releases/download/v${AWF_VERSION}"
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT

  curl -fsSL "${base}/${asset}" -o "${tmp}/${asset}"
  curl -fsSL "${base}/checksums.txt" -o "${tmp}/checksums.txt"

  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$tmp" && grep " ${asset}\$" checksums.txt | sha256sum -c - >&2)
  else
    (cd "$tmp" && grep " ${asset}\$" checksums.txt | shasum -a 256 -c - >&2)
  fi

  tar -xzf "${tmp}/${asset}" -C "$tmp"
  mkdir -p "$cache_dir"
  install -m 0755 "${tmp}/awf" "$binary"
  echo "$binary"
  ```

  Notes: the probe runs before the os/arch switch on purpose; a matching local binary serves
  even platforms the download path rejects. The `|| true` keeps `set -e` from killing the probe
  when no awf is on PATH mid-pipeline.

- [ ] **Task 3.3: smoke-check the script behaviour, then gate and commit.**

  ```
  go test ./internal/project/ -run 'TestBootstrap'
  ```

  Expected: all bootstrap tests pass. Then smoke-check stdout purity on the local-first path.
  This repo's own bootstrap singleton is disabled, so render a scratch copy straight from the
  template (the probe needs an `awf` on PATH whose `awf version` second field equals the pin:
  a source build at this commit prints `awf 0.6.0 (...)`, so it matches):

  ```
  mkdir -p /tmp/awf-smoke
  go build -o /tmp/awf-smoke/awf ./cmd/awf
  sed 's/{{ .version }}/0.6.0/' templates/bootstrap/awf-bootstrap.sh.tmpl > /tmp/awf-smoke/bootstrap.sh
  bash -c 'out="$(PATH=/tmp/awf-smoke:$PATH bash /tmp/awf-smoke/bootstrap.sh)" && test "$out" = /tmp/awf-smoke/awf && echo OK'
  ```

  Expected: `OK` (a single stdout line, the binary path).

  ```
  ./x gate && ./x check
  git add internal/project/bootstrap_test.go templates/bootstrap/awf-bootstrap.sh.tmpl
  git commit -m "fix(awf): make the bootstrap local-first with path-only stdout"
  ```

  Body: fixes the cold-cache stdout pollution that broke `"$(bash .awf/bootstrap.sh)" check` on
  every first run per machine, and adds the macOS `shasum -a 256` fallback (ADR-0049 Context
  item 3, Decisions 6-7).

## Phase 4: docs travel (Decision 8, minus release-prep changelog)

- [ ] **Task 4.1: rewrite the Versioning section and step 2 of `docs/releasing.md`.** Replace
  the `## Versioning` section body with:

  ```markdown
  awf is pre-1.0; versions are `vMAJOR.MINOR.PATCH` (SemVer). `project.Version`
  (`internal/project/project.go`) is the single version authority (ADR-0049): it drives `awf
  version`, the lock's `AWFVersion`, the bootstrap pin, and the binary-version gate. The git tag
  must equal it; the Release workflow hard-fails on a mismatch before building, so the tag can
  never mint a version the binary does not carry. Schema-generation bumps raise the floor
  mechanically: `minVersionBySchema` must contain an entry for the current generation, at or
  below `project.Version`, or the gate fails.
  ```

  Replace step 2's heading and first paragraph with:

  ```markdown
  2. **Verify `project.Version` equals the target version and add its changelog entry.** A
     schema-coupled change bumps the const mid-cycle (ADR-0049 Decision 4), so it often already
     matches; bump it only when it does not. Add a matching `## [0.2.0] - YYYY-MM-DD` entry to
     the top of `changelog/CHANGELOG.md` (newest first), grouped into Breaking
     changes/Features/Bug fixes/Others by adopter-facing effect (ADR-0041). The changelog entry
     is required for every tag.
  ```

  Keep step 2's command block and lock note as-is, and leave the intro paragraph (lines 6-10)
  unchanged: GoReleaser's changelog block stays, so its "generate a Conventional-Commits
  changelog" mention is still accurate.

- [ ] **Task 4.2: refresh the AGENTS.md invariants data.** In `.awf/agents-doc.yaml`, insert
  after the `ref: ADR-0048` bullet (the list is ADR-chronological; the ADR-0048 "Inert hook
  payloads" bullet is currently last):

  ```yaml
          - ref: ADR-0049
            text: '**Single version authority.** `awfVersion()` returns `project.Version`; no ldflags var or build info feeds gating, stamping, or pinning, and a schema-generation bump requires a matching `minVersionBySchema` entry and version bump.'
          - ref: ADR-0049
            text: '**Bootstrap output contract.** The rendered bootstrap prints exactly one stdout line (the resolved binary path), resolves an exactly-matching PATH `awf` before downloading, and verifies checksums via `sha256sum` or `shasum -a 256`.'
  ```

- [ ] **Task 4.3: add the pitfalls entry.** Append to `.awf/docs/parts/pitfalls/entries.md`:

  ```markdown

  ## Stdout is API in command-substitution scripts

  `.awf/bootstrap.sh` is consumed as `"$(bash .awf/bootstrap.sh)" <args>`, so its stdout is the
  binary path and nothing else: a checksum tool's `<asset>: OK` line on stdout execs as part of
  the command and fails only on cache-miss runs, which presents as flaky CI. Every diagnostic in
  rendered shell must carry `>&2`; `TestBootstrapStdoutPathOnly` pins this (`inv:
  bootstrap-stdout-path-only`, ADR-0049).
  ```

- [ ] **Task 4.4: re-render, gate, commit.**

  ```
  ./x sync && ./x check && ./x gate
  git add docs/releasing.md .awf/agents-doc.yaml .awf/docs/parts/pitfalls/entries.md AGENTS.md CLAUDE.md docs/pitfalls.md .awf/awf.lock
  git commit -m "docs(awf): single-version-authority doc updates"
  ```

  Stage exactly what sync rewrote (check `git status`: the rendered AGENTS.md, pitfalls doc, and
  lock hashes co-change; CLAUDE.md only if its hash moved).

## Phase 5: status flip

- [ ] **Task 5.1: refresh the tooling domain narrative.** In
  `.awf/domains/parts/tooling/current-state.md`, replace the sentence

  > awf ships as prebuilt cross-platform binaries built by GoReleaser on `v*` tag pushes
  > (ADR-0030): a dedicated ldflags-injected `version` var in `cmd/awf` drives the CLI version
  > while `project.Version` stays the lock's source of truth, and a PR-time `goreleaser
  > check`/snapshot job guards the release config.

  with:

  > awf ships as prebuilt cross-platform binaries built by GoReleaser on `v*` tag pushes
  > (ADR-0030); `project.Version` is the single version authority (ADR-0049): it drives the CLI
  > version, the lock, the bootstrap pin, and the version gate, the Release workflow refuses a
  > tag that does not match it, `minVersionBySchema` ties schema-generation bumps to version
  > bumps, and a PR-time `goreleaser check`/snapshot job guards the release config. The rendered
  > bootstrap resolves an exactly-matching PATH `awf` before downloading and keeps stdout to the
  > single resolved-path line.

- [ ] **Task 5.2: flip ADR-0049 to Implemented, retire the stale marker, and regenerate.** Edit
  `docs/decisions/0049-single-version-authority.md` frontmatter `status: Proposed` →
  `status: Implemented`, and delete the three-line stale
  `// invariant: version-ldflags-precedence` comment from `awfVersion` in `cmd/awf/version.go`
  (its retirement takes effect only now, ADR-0031; co-landing marker removal with the flip
  matches the a24d961 precedent), then:

  ```
  ./x sync && ./x check && ./x invariants && ./x gate
  ```

  Expected: all clean; the four new slugs (`single-version-authority`, `schema-min-version`,
  `bootstrap-stdout-path-only`, `bootstrap-local-first`) are backed by Phases 1-3, and
  `version-ldflags-precedence` is dropped from enforcement by ADR-0049's `retires_invariants`
  (ADR-0031 successor mechanics), so ADR-0030 stays green without its marker.

  ```
  git add docs/decisions/0049-single-version-authority.md cmd/awf/version.go docs/decisions/ACTIVE.md docs/domains/tooling.md .awf/domains/parts/tooling/current-state.md .awf/awf.lock
  git commit -m "docs(adr): flip 0049 Implemented"
  ```

  (`docs/domains/tooling.md` and the lock co-change from sync; the `domain-doc-staleness` audit
  rule wants the current-state part in this same commit.)

- [ ] **Task 5.3: post-flip audit.**

  ```
  ./x audit
  ```

  Expected: exit 0; no Error findings (Warnings acceptable and reported to the terminal review).

---

**Not in this plan (deliberate):** the `## [0.6.0]` changelog entry lands at release-prep per
ADR-0049 Decision 8, after Plans 2 and 3 of the fix batch add their adopter-facing changes; the
release tag itself follows the whole batch (ADR-0049 Decision 5).
