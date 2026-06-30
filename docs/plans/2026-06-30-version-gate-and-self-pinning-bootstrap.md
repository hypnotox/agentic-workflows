# Plan: Binary-version compatibility gate and self-pinning rendered bootstrap

Links: [ADR-0039](../decisions/0039-binary-version-compatibility-gate.md) (binary-version compatibility gate), [ADR-0040](../decisions/0040-self-pinning-rendered-bootstrap.md) (self-pinning rendered bootstrap). The design lives in those ADRs; this plan is the execution record only.

## Goal

Land the two coupled ADRs that make up the v0.5.0 release. **ADR-0039** adds a binary-version compatibility gate: a schema-`"ahead"` state plus a release-version sub-check, so a binary *behind* the project on either axis (schema generation or lock `awfVersion`) refuses to render, and `check` surfaces ahead-skew as a non-failing notice — wired into `sync`, `check`, `invariants`, `audit`, and `list`. **ADR-0040** renders a neutral, repo-root `awf-bootstrap.sh` singleton pinned to the rendering binary's `project.Version`, collapsing the two-sources-of-truth pin (hand-rolled `ensure_awf` vs lock `awfVersion`) into one; it is toggled by a bespoke `bootstrap` config token (the `target` precedent) and travels with a schema 4→5 migration. awf dogfoods both: it migrates its own tree to schema 5 and opts *out* of the bootstrap (it builds from source via `./x`).

## Architecture summary

- **Version gate** (ADR-0039): `migrate.gateStateFor` gains an `"ahead"` state (`gen > current`); `migrate.GateState` surfaces it. `cmd/awf`'s `gate()` maps `"ahead"` to a hard error and, after the schema check, loads `.awf/awf.lock` and compares the lock's `AWFVersion` against `awfVersion()` via normalized `golang.org/x/mod/semver`: lock-newer (binary behind) → error; binary at-or-ahead → permitted; absent/empty/unparseable → skip. `gate()` is wired into `runInvariants`, `runAudit`, and `runList` (today only `sync`/`check` gate). `check` prints a non-failing one-line notice when the binary is *ahead* of the lock. `golang.org/x/mod` is promoted from indirect to direct.
- **Bootstrap** (ADR-0040): `project.Version` enters the template data namespace (`p.data()`). A new `templates/bootstrap/awf-bootstrap.sh.tmpl` renders to repo-root `awf-bootstrap.sh` via an explicit `RenderAll` block (the `CLAUDE.md`-bridge pattern: neutral, no catalog spec, `nil` sections), so it is lock-tracked and drift-checked. `isManagedMarkdown` excludes the `.sh` template id from the dead-reference scan. Config gains `Bootstrap *BootstrapConfig{Enabled bool}` (the `Invariants` sub-struct pattern). A bespoke `bootstrap` CLI token (outside `kindDescriptors`) drives `awf add/remove/list bootstrap` via a new comment-preserving `config.SetMappingScalar`. A schema `{To:5}` migration enables the bootstrap for upgraded configs; `init` seeds it on. awf's own tree migrates to 5 and disables the bootstrap.

## Tech stack

- Go 1.26. Packages touched: `internal/migrate`, `internal/config`, `internal/catalog` (none — bootstrap has no catalog spec), `internal/project`, `cmd/awf`, `templates/`. New direct dependency: `golang.org/x/mod/semver` (already in the module graph, indirect).
- Gate: `./x gate` (~15s, fast tier; `./x gate full` pre-push). 100% statement coverage is enforced (ADR-0012). `./x check` enforces render-drift and the schema-version gate.
- The rendered `awf-bootstrap.sh` **runtime** (fetch/verify/cache) is **not** covered by the Go coverage gate — there is no shell-coverage harness in the repo. Its coverage comes entirely from golden-render Go tests asserting the rendered template *text* (ADR-0040 Consequences). Every *Go* branch this plan adds still needs a test or a justified `// coverage-ignore:`.

## File structure

**Created**
- `templates/bootstrap/awf-bootstrap.sh.tmpl` — the neutral repo-root bootstrap template (ADR-0040).
- `internal/migrate/enablebootstrap.go` — the To:5 `enable-bootstrap` migration (mirrors `drophooks.go`).
- `.awf/parts/agents-doc/awf-setup.md` — override part adding `bootstrap` to the AGENTS.md toggle grammar (no such part exists today).

**Modified**
- `go.mod` — promote `golang.org/x/mod` from indirect to direct (via `go mod tidy` after the import lands).
- `internal/migrate/migrate.go` (+ `migrate_test.go`) — `gateStateFor` `"ahead"` state; register `{To:5}`; update applied-list + `Current()` assertions.
- `cmd/awf/main.go` (+ `run_test.go`) — `gate()` gains the `"ahead"` error and the version sub-check + `// invariant: version-compat-gate`; wire `gate()` into `runInvariants`/`runAudit`/`runList` call sites; `add`/`remove`/`list` argSpecs help enumerations gain `bootstrap`.
- `cmd/awf/version.go` — no change (read by `gate`); a version-compare helper lands in `main.go`.
- `cmd/awf/check.go` (+ `check_test.go`) — ahead-skew non-failing notice.
- `cmd/awf/invariants.go`, `cmd/awf/audit.go` (+ tests) — prepend `gate(root)`.
- `cmd/awf/list_add.go` (+ `list_add_test.go`) — `unknownKind` hint gains `bootstrap`; `runAdd`/`runRemove`/`runList` special-case `kind=="bootstrap"`; new `addRemoveBootstrap` bespoke path.
- `internal/config/config.go` (+ `config_test.go`) — add `Bootstrap *BootstrapConfig`.
- `internal/config/edit.go` (+ `edit_test.go`) — add `SetMappingScalar` (nested `bootstrap.enabled` comment-preserving setter).
- `internal/project/render.go` (+ tests) — expose `project.Version` in `p.data()`; add the explicit `awf-bootstrap.sh` `RenderAll` block.
- `internal/project/banner.go` (+ `banner_test.go`) — `injectBanner` chooses a `#`-comment banner for shebang content so the rendered shell script stays executable.
- `internal/project/check.go` (+ tests) — `isManagedMarkdown` excludes the bootstrap `.sh` template id.
- `internal/project/scaffold.go` (+ tests) — `ScaffoldConfig`/`Skeleton` seed `bootstrap.enabled: true`.
- `internal/project/golden_test.go` (or a new render test) — `inv: bootstrap-pin` / `inv: bootstrap-checksum` golden assertions.
- `templates/embed.go` — add `bootstrap` to the `//go:embed` directive.
- `.awf/agents-doc.yaml` — add the three new invariant slugs to the rendered `data.invariants` list (confirmed `{ref, text}` shape).
- `.awf/domains/parts/{tooling,config,rendering}/current-state.md` — narrate the gate + bootstrap.
- ADR-0039, ADR-0040 frontmatter status flips; `docs/decisions/ACTIVE.md` + `docs/domains/*.md` regenerate.
- awf's own `.awf/config.yaml` + `.awf/awf.lock` (schema 4→5; bootstrap disabled).

**Deleted**
- (none)

---

## Phase 1 — ADR-0039 binary-version compatibility gate (no schema change)

This phase is self-contained: it adds the gate axes and wires the gate into the three new commands, with no config-surface or schema change. ADR-0040's schema bump lands in Phase 2.

- [ ] **Task 1.1 — Promote `golang.org/x/mod` to a direct dependency.**
  - Add the import `"golang.org/x/mod/semver"` in `cmd/awf/main.go` (used by Task 1.3's helper). Do not edit `go.mod` by hand.
  - Run `go mod tidy`. This moves `golang.org/x/mod v0.35.0` from the `// indirect` block to a direct require in `go.mod` (it is already in `go.sum`; no network fetch).
  - Verify: `grep -n "golang.org/x/mod" go.mod` shows the line **without** a `// indirect` suffix.

- [ ] **Task 1.2 — Add the `"ahead"` schema state in `internal/migrate`.** In `internal/migrate/migrate.go`, in `gateStateFor` (lines 90–100), insert an ahead branch ahead of the existing `gen >= current` "ok" return so `gen > current` is classified distinctly:
  ```go
  // gateStateFor is the pure classifier (extracted for testability): "ahead" when
  // gen is strictly above current (the binary is behind the project — ADR-0039);
  // "ok" when gen == current; "gate" when at least one To lands in the open interval
  // (gen, current]; "autobump" otherwise.
  func gateStateFor(gen, current int, tos []int) string {
  	if gen > current {
  		return "ahead"
  	}
  	if gen == current {
  		return "ok"
  	}
  	for _, to := range tos {
  		if to > gen && to <= current {
  			return "gate"
  		}
  	}
  	return "autobump"
  }
  ```
  `GateState` (line 103) is unchanged — it already returns `gateStateFor(...)` verbatim, so `"ahead"` surfaces automatically. No `inv:` lives in `internal/migrate` for this (the gate invariant is backed in `cmd/awf` — Task 1.5), so no marker comment here.

- [ ] **Task 1.3 — Add the version-compare helper and extend `gate()` in `cmd/awf/main.go`.**
  - Add `manifest` to the imports of `cmd/awf/main.go` (it currently imports `catalog`, `initspec`, `migrate`, `project`, `templates` — add `"github.com/hypnotox/agentic-workflows/internal/manifest"` and `"golang.org/x/mod/semver"`).
  - Add a normalize helper and a lock-version sub-check, then rewrite `gate()` (currently lines 407–417). Replace the whole `gate` function with:
    ```go
    // normalizeSemver returns s in the single-leading-v form x/mod/semver requires.
    // awfVersion() already returns the v-form for `go install` builds, so a naive
    // prefix would yield "vv0.4.0" and fail semver.IsValid; trimming any existing v
    // first makes the normalization idempotent (ADR-0039 Decision 3).
    func normalizeSemver(s string) (string, bool) {
    	v := "v" + strings.TrimPrefix(s, "v")
    	if !semver.IsValid(v) {
    		return "", false
    	}
    	return v, true
    }

    // gate refuses to operate against a config the running binary cannot correctly
    // interpret. It runs before project.Open. On the schema axis: "gate" (config
    // behind binary) → "run awf upgrade"; "ahead" (config ahead of binary) → "update
    // your pinned awf" (ADR-0039); "autobump" proceeds and the subsequent sync stamps
    // the current schema. On the release-version axis: after the schema check it loads
    // .awf/awf.lock and compares lock.AWFVersion vs awfVersion() — a lock semver-newer
    // than the binary (binary behind) errors; a binary at-or-ahead is the permitted
    // pre-upgrade state. The version sub-check is skipped (never errors) on an absent,
    // unparseable, empty, or non-normalizable version, mirroring Generation's no-lock
    // tolerance.
    // invariant: version-compat-gate
    func gate(root string) error {
    	switch migrate.GateState(root) {
    	case "gate":
    		return fmt.Errorf("config schema is behind (generation %d < %d); run awf upgrade",
    			migrate.Generation(root), migrate.Current())
    	case "ahead":
    		return fmt.Errorf("awf %s is behind this project's config (schema generation %d > %d); update your pinned awf",
    			awfVersion(), migrate.Generation(root), migrate.Current())
    	}
    	lockV, binV, ok := lockVsBinary(root)
    	if !ok {
    		return nil // version sub-check not computable; schema check already applied
    	}
    	if semver.Compare(lockV, binV) > 0 {
    		return fmt.Errorf("awf %s is behind this project (rendered by %s); update your pinned awf",
    			awfVersion(), strings.TrimPrefix(lockV, "v"))
    	}
    	return nil
    }

    // lockVsBinary returns the normalized lock awfVersion and binary version for the
    // release-version sub-check, with ok=false whenever the comparison cannot be
    // computed (no/unloadable lock, empty AWFVersion, or a version that fails semver
    // normalization) so the caller skips rather than errors (ADR-0039 Decision 5).
    func lockVsBinary(root string) (lockV, binV string, ok bool) {
    	l, err := manifest.Load(filepath.Join(root, ".awf", "awf.lock"))
    	if err != nil || l.AWFVersion == "" {
    		return "", "", false
    	}
    	lockV, lok := normalizeSemver(l.AWFVersion)
    	binV, bok := normalizeSemver(awfVersion())
    	if !lok || !bok {
    		return "", "", false
    	}
    	return lockV, binV, true
    }
    ```
  - Note: `lockVsBinary` is shared with `check`'s ahead-notice (Task 1.6), which is why it is factored out.

- [ ] **Task 1.4 — Wire `gate()` into `invariants`, `audit`, and `list`.**
  - `cmd/awf/invariants.go`, in `runInvariants` (line 10) — insert as the first statement, before `project.Open`:
    ```go
    	if err := gate(root); err != nil {
    		return err
    	}
    ```
  - `cmd/awf/audit.go`, in `runAudit` (line 11) — insert the identical guard as the first statement, before `project.Open`.
  - `cmd/awf/list_add.go`, in `runList` (line 169) — insert the identical guard as the first statement, before `project.Open`. (`add`/`remove` already inherit the gate transitively via the `runSync` they call.)
  - `runSync` (main.go:419) and `runCheck` (check.go:10) already call `gate(root)` first — leave them.

- [ ] **Task 1.5 — Test every new `gate()` branch in `cmd/awf` (100% coverage).** In `cmd/awf/run_test.go` (or a focused `gate_test.go` alongside it — match the existing test-file convention you find), add cases. Use a temp root with a `.awf/config.yaml` (`prefix: ex\n`) plus a hand-written `.awf/awf.lock` JSON carrying the `awfVersion`/`schemaVersion` each case needs (mirror how the existing run-level tests stand up a project; `manifest.Lock` marshals to JSON via `Save`). `awfVersion()` returns `project.Version` (`0.4.0`) in test builds, so drive the lock version relative to that:
  - **ahead-schema error**: lock `schemaVersion` = `migrate.Current()+1` → `gate` returns an error containing `"update your pinned awf"` and the `"schema generation"` clause. (Covers the `case "ahead"` arm.)
  - **behind-version error**: lock `awfVersion` = `"0.5.0"` (> binary `0.4.0`), `schemaVersion` = `migrate.Current()` → error containing `"is behind this project (rendered by 0.5.0)"`. (Covers the `semver.Compare(...) > 0` arm.)
  - **at-or-ahead permitted**: lock `awfVersion` = `"0.4.0"` (== binary) and a separate case `"0.3.0"` (< binary), `schemaVersion` = `Current()` → `gate` returns nil. (Covers the fall-through `return nil`.)
  - **skip: no lock**: `.awf/` with config but no `awf.lock` → `gate` returns nil (schema is `"ok"` via Generation's no-lock branch; `lockVsBinary` returns ok=false on the load error).
  - **skip: empty AWFVersion**: lock with `awfVersion: ""`, `schemaVersion` = `Current()` → nil. (Covers `l.AWFVersion == ""`.)
  - **skip: unparseable lock version**: lock `awfVersion: "garbage"`, `schemaVersion` = `Current()` → nil. (Covers `!lok`.)
  - **normalizeSemver idempotence**: a direct unit case asserting `normalizeSemver("v0.4.0")` and `normalizeSemver("0.4.0")` both yield `("v0.4.0", true)`, and `normalizeSemver("vv0.4.0")` yields `("", false)`. (Covers both `normalizeSemver` returns; the `vv` case also guards the regression the ADR calls out.)
  - The `!bok` sub-condition of `lockVsBinary`'s `if !lok || !bok { return "", "", false }` (binary version unparseable) is unreachable in a normal build because `awfVersion()` returns the fixed `project.Version` (`0.4.0`, always valid). **Do not add a `// coverage-ignore` for it**: `covercheck` measures Go *block* coverage (one basic block per `if` body), not per-sub-condition coverage, and the `!lok` skip case above already drives execution into that single `return "", "", false` block — so the whole block is covered and there is no separate uncovered statement to ignore. (Adding a `coverage-ignore` directive there would be a misleading no-op.) The `!bok` operand is simply never the cause that reaches the block; that is fine for the 100% statement gate.
  - Confirm the three new `runInvariants`/`runAudit`/`runList` gate guards are exercised: add (or extend) a run-level test that invokes each command against an **ahead-schema** project and asserts a non-zero exit / the gate error, so the inserted `if err := gate(root)` branch is covered in each. (The success path of each command is already covered by existing tests.)

- [ ] **Task 1.6 — `check` prints a non-failing ahead-skew notice.** In `cmd/awf/check.go`, in `runCheck`, after the existing `gate(root)` guard (lines 11–13) and before `project.Open`, add the notice:
  ```go
  	if lockV, binV, ok := lockVsBinary(root); ok && semver.Compare(binV, lockV) > 0 {
  		fmt.Fprintf(stdout, "note: awf %s is ahead of this project (rendered by %s); run awf sync to re-pin\n",
  			strings.TrimPrefix(binV, "v"), strings.TrimPrefix(lockV, "v"))
  	}
  ```
  Add `"strings"` and `"golang.org/x/mod/semver"` to `check.go`'s imports (`lockVsBinary` is in `package main`, so no new internal import). The gate already permits the ahead case (binary at-or-ahead), so this is a pure print.
  - In `cmd/awf/check_test.go`, add a case: a synced project whose lock `awfVersion` is `"0.3.0"` (< binary `0.4.0`) and `schemaVersion` = `Current()` → `runCheck` succeeds (exit 0, `awf check: clean`) **and** stdout contains `"is ahead of this project (rendered by 0.3.0)"`. Also assert the no-notice path (lock `awfVersion` = `"0.4.0"`, equal) prints no `"is ahead"` line, so the `semver.Compare(binV, lockV) > 0` false branch is covered. (Stand the project up with a real `./x`-style sync, or write the lock + rendered files the existing check tests use as a fixture — match the file's precedent.)

- [ ] **Task 1.7 — Update `migrate_test.go` for the `"ahead"` state.** In `internal/migrate/migrate_test.go`, in `TestNoopGapAutoBumps` (lines 164–177), the existing `gateStateFor(5,5,...) == "ok"` assertion (line 175) stays correct. Add ahead cases:
  ```go
  	// gen strictly above current → the binary is behind the project (ADR-0039).
  	if got := gateStateFor(6, 5, []int{1, 5}); got != "ahead" {
  		t.Errorf("gateStateFor(6,5,...) = %q, want ahead", got)
  	}
  	if got := gateStateFor(5, 4, []int{1, 4}); got != "ahead" {
  		t.Errorf("gateStateFor(5,4,...) = %q, want ahead", got)
  	}
  ```
  No `migrate.GateState`-level fixture is needed (the classifier cases cover the new branch); the `cmd/awf` tests in Task 1.5 cover `GateState`'s `"ahead"` surfacing end to end.

- [ ] **Task 1.8 — Flip ADR-0039 to Implemented; sync; verify; commit.**
  - In `docs/decisions/0039-binary-version-compatibility-gate.md` frontmatter, set `status: Implemented`.
  - Run `./x sync`. Expect `awf sync: done` (regenerates `ACTIVE.md` and the `tooling` domain index for ADR-0039).
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.` (ADR-0039 is now Implemented, so `inv: version-compat-gate` is enforced and backed by the Task 1.3 marker; the textual-contract bullet carries no `inv:` slug, so no second marker is needed.)
  - Run `./x check`. Expect `awf check: clean`. (The project lock's `awfVersion` is `0.4.0`, equal to the binary, so no ahead-notice prints and no version error.)
  - Stage: `go.mod go.sum internal/migrate/migrate.go internal/migrate/migrate_test.go cmd/awf/main.go cmd/awf/check.go cmd/awf/invariants.go cmd/awf/audit.go cmd/awf/list_add.go cmd/awf/run_test.go cmd/awf/check_test.go` and any regenerated `docs/` + lock, plus the ADR file. Commit: `feat(awf): add binary-version compatibility gate (ADR-0039)` then a separate `docs(adr): implement 0039 binary-version compatibility gate` — or combine into one `feat(awf)` commit that also flips the ADR, per the Phase-end precedent. One concern per commit; stage explicitly (no `git add -A`).

## Phase 2 — ADR-0040 self-pinning bootstrap (schema 4→5)

Adding the To:5 migration makes `migrate.Current() == 5`, so awf's own repo (lock schema 4) gates on `./x sync`/`./x check` until migrated — **Task 2.13** handles that self-migration atomically within this phase. Land the strict-decode config-surface pieces (config struct field, migration, scaffold seeding, CLI token) so `go test` and `./x gate` stay green throughout. Note on per-task verification: `./x gate` runs only `go test ./... -coverpkg`, `covercheck`, `go vet`, and lint — it does **not** invoke `awf check`/`awf sync` against this repo's own lock, so the schema-4 → 5 skew does not trip the per-task gate during Tasks 2.1–2.12; only an explicit `./x sync`/`./x check` would gate before Task 2.13 runs `awf upgrade`. The unit tests build their own temp projects and are unaffected. Do not run `./x check`/`./x sync` manually between Task 2.6 (which registers `{To:5}`) and Task 2.13 (which migrates this repo); the first `./x check` of the phase is Task 2.14's, after the self-migration.

- [ ] **Task 2.1 — Expose `project.Version` in the template data namespace.** In `internal/project/render.go`, in `func (p *Project) data` (lines 28–35), add a `version` key:
  ```go
  func (p *Project) data(sc config.Sidecar) map[string]any {
  	return map[string]any{
  		"prefix":  p.Cfg.Prefix,
  		"vars":    nonNil(p.Cfg.Vars),
  		"data":    nonNil(sc.Data),
  		"layout":  p.layout().templateMap(),
  		"version": Version,
  	}
  }
  ```
  This is the value `sync` stamps into the lock's `AWFVersion` (`SyncReport`: `AWFVersion: Version`), so the bootstrap pin and the lock share one source (ADR-0040 Decision 2). No other template references `.version`, so existing golden output is unchanged.

- [ ] **Task 2.2 — Add the bootstrap template.** Create `templates/bootstrap/awf-bootstrap.sh.tmpl`. It must resolve every variable (only `{{ .version }}`) so it can never render `<no value>` (ADR-0001). The URL path uses the v-tag (`v{{ .version }}`); the asset filename uses the no-v form (`{{ .version }}`). It runs `sha256sum -c` against `checksums.txt` **before** the install step (`inv: bootstrap-checksum`):
  ```bash
  #!/usr/bin/env bash
  # GENERATED by awf — do not edit; change .awf/ and run `awf sync`.
  # Fetches and verifies a pinned awf binary, caches it, and prints its path.
  set -euo pipefail

  AWF_VERSION="{{ .version }}"
  REPO="hypnotox/agentic-workflows"

  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) echo "awf-bootstrap: unsupported arch: $arch" >&2; exit 1 ;;
  esac
  case "$os" in
    linux|darwin) ;;
    *) echo "awf-bootstrap: unsupported os: $os" >&2; exit 1 ;;
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

  (cd "$tmp" && grep " ${asset}\$" checksums.txt | sha256sum -c -)

  tar -xzf "${tmp}/${asset}" -C "$tmp"
  mkdir -p "$cache_dir"
  install -m 0755 "${tmp}/awf" "$binary"
  echo "$binary"
  ```
  **Banner handling (load-bearing — confirmed against `internal/project/banner.go`).** `renderTarget` calls `injectBanner(content)` unconditionally, and `injectBanner` *always* emits an HTML comment (`<!-- GENERATED by awf … -->`) as the first line (banner.go:14,18). For a shell script that first line breaks the shebang/parse. The bootstrap therefore must get a `#`-style banner instead. Implement this by teaching `injectBanner` to choose its comment style by the rendered content's leading bytes: when `content` starts with `#!` (a shebang), emit the banner as a `# `-prefixed line **after** the shebang line; otherwise keep the existing HTML-comment behaviour. Concretely, in `internal/project/banner.go`:
  ```go
  func injectBanner(content string) string {
  	if strings.HasPrefix(content, "#!") {
  		// Shell/script target: banner as a # comment after the shebang line.
  		nl := strings.IndexByte(content, '\n')
  		if nl < 0 { // coverage-ignore: a rendered shebang script always has a trailing newline body
  			return content
  		}
  		return content[:nl+1] + "# " + bannerText + "\n" + content[nl+1:]
  	}
  	line := "<!-- " + bannerText + " -->\n"
  	if yamlBlock, body, found := frontmatter.Split([]byte(content)); found {
  		return "---\n" + string(yamlBlock) + "---\n" + line + string(body)
  	}
  	return line + content
  }
  ```
  Add `"strings"` to `banner.go`'s imports. **Drop the literal `# GENERATED by awf …` line from the template above** (the second template line) — `injectBanner` now supplies it, so keeping the literal would double the banner. The template's first line stays `#!/usr/bin/env bash`. Create `internal/project/banner_test.go` (no such file exists today — `injectBanner` is currently exercised only indirectly through the render-level tests in `project_test.go`) with a case asserting `injectBanner("#!/usr/bin/env bash\nset -e\n")` puts a `# GENERATED by awf …` line as the *second* line (after the shebang), and cases confirming the existing HTML-comment and frontmatter paths are unchanged — covering the new shebang branch and the unchanged branches directly. `inv: provenance-banner` stays backed on `injectBanner` (the marker is already there); the new branch is part of the same invariant (a banner is still injected). The golden-render test (Task 2.8) additionally asserts the final on-disk `awf-bootstrap.sh` first line is `#!/usr/bin/env bash` and its second line is the `#`-comment banner, so the script is executable.

- [ ] **Task 2.3 — Embed the bootstrap template.** In `templates/embed.go`, add `bootstrap` to the `//go:embed` directive and to the package doc comment:
  ```go
  // Package templates embeds the standard's template tree (catalog.yaml, skills, agents, docs, bootstrap).
  package templates

  import "embed"

  //go:embed catalog.yaml skills agents agents-doc docs domains claude adr-readme adr-template plans-readme bootstrap
  var FS embed.FS
  ```
  No `catalog.yaml` entry is added — the bootstrap is a neutral singleton with no overridable sections, exactly like the `claude/CLAUDE.md.tmpl` bridge, which also carries no catalog spec (render.go:186 renders it with `nil` sections).

- [ ] **Task 2.4 — Add the `Bootstrap` config sub-struct.** In `internal/config/config.go`, add the field to `Config` (after `Audit`, before `root`, line 47) and a sub-struct mirroring `InvariantConfig`:
  ```go
  	Audit      *AuditConfig     `yaml:"audit"`
  	Bootstrap  *BootstrapConfig `yaml:"bootstrap"`
  	root       string           // <project>/.awf, for sidecar/part resolution
  ```
  Add after the `InvariantConfig`/`InvariantSource` block:
  ```go
  // BootstrapConfig configures the rendered awf-bootstrap.sh singleton (ADR-0040). A
  // nil *BootstrapConfig (key absent) and Enabled false both mean "do not render";
  // only Enabled true renders the artifact — a nested enable entry rather than a
  // top-level scalar bool (the Alternatives table rejected the bare bool).
  type BootstrapConfig struct {
  	Enabled bool `yaml:"enabled"`
  }
  ```
  `Load` uses `KnownFields(true)`, so this new key is now accepted; an older binary (schema 4) rejects it — which is exactly why Task 2.6 ships the schema bump. No `Validate` change is needed (a bool has no invalid value). In `internal/config/config_test.go`, add a case decoding a config with `bootstrap:\n  enabled: true` and asserting `cfg.Bootstrap != nil && cfg.Bootstrap.Enabled`, plus an absent-key case asserting `cfg.Bootstrap == nil`.

- [ ] **Task 2.5 — Render the bootstrap when enabled, and exclude it from the dead-ref scan.**
  - In `internal/project/render.go` `RenderAll`, after the ADR-system singleton loop (after line 216, before `return out, nil`), add the explicit neutral-singleton block (the `CLAUDE.md`-bridge pattern — `nil` sections, zero sidecar, repo-root output):
    ```go
    	// awf-bootstrap.sh (neutral repo-root singleton; rendered only when enabled —
    	// ADR-0040). No catalog spec / no overridable sections, like the CLAUDE.md bridge.
    	// invariant: bootstrap-pin
    	if p.Cfg.Bootstrap != nil && p.Cfg.Bootstrap.Enabled {
    		brf, err := p.renderTarget("bootstrap", "", "bootstrap/awf-bootstrap.sh.tmpl",
    			nil, config.Sidecar{}, p.data(config.Sidecar{}), "awf-bootstrap.sh")
    		if err != nil { // coverage-ignore: the bootstrap template references only .version (always set) and no parts, so renderTarget cannot produce <no value> or a read error
    			return nil, err
    		}
    		out = append(out, brf)
    	}
    ```
    (Place the `// invariant: bootstrap-pin` marker here as the render site, *or* on the golden-render test in Task 2.8 — pick one; Task 2.8 places it on the test, matching the ADR-0040 wording "a golden-render test asserts…". If you back it on the test, drop this marker line to avoid a duplicate-but-harmless second backing.)
  - In `internal/project/check.go`, change `isManagedMarkdown` (lines 152–154) to also exclude the bootstrap template id:
    ```go
    func isManagedMarkdown(tid string) bool {
    	return tid != "claude/CLAUDE.md.tmpl" && tid != "bootstrap/awf-bootstrap.sh.tmpl"
    }
    ```
    Update its doc comment (lines 149–151) to read "…everything RenderAll produces except the CLAUDE.md bridge and the awf-bootstrap.sh shell script." In `internal/project/check_test.go` (or the file backing `inv: dead-reference-gated`), add a case proving a rendered `awf-bootstrap.sh` is **not** scanned for dead references — e.g. a bootstrap render whose body contains a `[x](missing)`-looking token does not produce a `dead-reference` drift (or, more simply, assert `isManagedMarkdown("bootstrap/awf-bootstrap.sh.tmpl") == false`). Cover both the new false-returning branch and the still-managed true branch.

- [ ] **Task 2.6 — Add the To:5 migration.** Create `internal/migrate/enablebootstrap.go` (mirroring `drophooks.go`), using the new `config.SetMappingScalar` from Task 2.7 to write `bootstrap.enabled: true` for ported configs (default-on for upgrades):
  ```go
  package migrate

  import (
  	"os"
  	"path/filepath"

  	"github.com/hypnotox/agentic-workflows/internal/config"
  )

  // applyEnableBootstrap ports schema 4 → 5: the self-pinning bootstrap artifact is
  // added (ADR-0040), enabled by default for ported configs so an upgraded project
  // gets the new default. It writes `bootstrap:\n  enabled: true` via
  // config.SetMappingScalar so config.yaml serialization stays owned by
  // internal/config (ADR-0026). A config absent on disk is a no-op (idempotent
  // re-run safe); a config that already carries bootstrap.enabled is overwritten to
  // true (the upgrade default).
  func applyEnableBootstrap(root string) error {
  	cfgPath := filepath.Join(root, ".awf", "config.yaml")
  	src, err := os.ReadFile(cfgPath)
  	if os.IsNotExist(err) {
  		return nil
  	}
  	if err != nil { // coverage-ignore: ReadFile faults only on a permission error that the test root bypasses
  		return err
  	}
  	out, err := config.SetMappingScalar(src, "bootstrap", "enabled", true)
  	if err != nil {
  		return err
  	}
  	return os.WriteFile(cfgPath, out, 0o644)
  }
  ```
  In `internal/migrate/migrate.go`, append to `registry` (after the `{To: 4}` line, line 27):
  ```go
  	{To: 5, Name: "enable-bootstrap", Apply: applyEnableBootstrap},
  ```

- [ ] **Task 2.7 — Add `config.SetMappingScalar` (nested `bootstrap.enabled` setter).** In `internal/config/edit.go`, add after `RemoveKey` (line 134). It sets a `bool` scalar under a nested mapping key (creating the parent mapping if absent), via a `yaml.Node` round-trip that preserves comments and untouched keys (ADR-0026):
  ```go
  // SetMappingScalar sets child to a bool value under a top-level mapping at key in
  // a config.yaml source, creating the key's mapping (and the child) if absent, via a
  // yaml.Node round-trip that preserves comments and every untouched key (ADR-0026).
  // It is the nested-scalar analog of SetArray (which writes a sequence): the
  // bootstrap enable entry is `bootstrap.enabled: <bool>`, not an enable array, so it
  // needs a mapping-scalar writer rather than SetArrayMember. An existing scalar under
  // key/child is overwritten.
  func SetMappingScalar(src []byte, key, child string, value bool) ([]byte, error) {
  	var doc yaml.Node
  	if err := yaml.Unmarshal(src, &doc); err != nil {
  		return nil, fmt.Errorf("config: parse: %w", err)
  	}
  	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
  		return nil, errors.New("config: not a YAML mapping")
  	}
  	root := doc.Content[0]
  	boolStr := "false"
  	if value {
  		boolStr = "true"
  	}
  	val, _ := mapValue(root, key)
  	if val == nil || val.Kind != yaml.MappingNode {
  		m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
  			strScalar(child), boolScalar(boolStr),
  		}}
  		if val == nil {
  			root.Content = append(root.Content, strScalar(key), m)
  		} else {
  			_, vi := mapValue(root, key)
  			root.Content[vi] = m
  		}
  		return encode(&doc)
  	}
  	if cv, _ := mapValue(val, child); cv != nil {
  		cv.Tag, cv.Value, cv.Style = "!!bool", boolStr, 0
  	} else {
  		val.Content = append(val.Content, strScalar(child), boolScalar(boolStr))
  	}
  	return encode(&doc)
  }

  func boolScalar(v string) *yaml.Node {
  	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: v}
  }
  ```
  In `internal/config/edit_test.go`, add cases covering every branch:
  - key absent → `bootstrap:\n  enabled: true` appended; other keys + comments preserved.
  - key present as a mapping without the child → child appended under it.
  - key present as a mapping **with** the child → child value overwritten (e.g. `false` → `true`).
  - key present but **not** a mapping (e.g. `bootstrap: 3`) → replaced with the fresh mapping. (Covers the `val.Kind != yaml.MappingNode` && `val != nil` arm.)
  - `value=false` input → asserts the `false` literal renders (covers the `boolStr` false branch).
  - non-mapping root input (`"- a\n"`) → error.
  - unparseable YAML (`"a: [b\n"`) → error.
  No `inv:` marker is required on `SetMappingScalar` itself; `inv: config-serialization-owned` is already backed on `encode`, which this routes through.

- [ ] **Task 2.8 — Golden-render tests backing `bootstrap-pin` and `bootstrap-checksum`.** In `internal/project/golden_test.go` (or a focused `bootstrap_test.go` matching the package convention), render a project with `bootstrap.enabled: true` and assert the rendered `awf-bootstrap.sh` content. Verify the exact slug spellings against ADR-0040's Invariants section (they are `bootstrap-pin` and `bootstrap-checksum`):
  - Stand up a minimal project (reuse the package's existing render-test helper that builds a `Project` over a temp `.awf/` — match the precedent in `golden_test.go`), set `Cfg.Bootstrap = &config.BootstrapConfig{Enabled: true}` (or write the config YAML), call `RenderAll`, and find the `RenderedFile` whose `Path == "awf-bootstrap.sh"`.
  - `// invariant: bootstrap-pin` — assert the content contains the literal `AWF_VERSION="0.4.0"` (i.e. `AWF_VERSION="` + `project.Version` + `"`). Build the expected string from `project.Version` so it cannot drift from the constant.
  - `// invariant: bootstrap-checksum` — assert the content contains `sha256sum -c` **and** that this `sha256` step appears at a byte index *before* the `install -m 0755` step (so the verify precedes the install, per the ADR). Use `strings.Index` on both and assert ordering.
  - Assert the first on-disk line is a `#`-comment (the executable-shebang/banner check from Task 2.2's decision point), not `<!--`.
  - Add a `bootstrap.enabled: false` (or nil `Bootstrap`) case asserting **no** `RenderedFile` has `Path == "awf-bootstrap.sh"` — covers the render-gate false branch in Task 2.5.
  - Place the two `// invariant:` markers on these test functions (markers in `*_test.go` are honoured — precedent: `internal/project/drift_test.go`). If you instead placed `bootstrap-pin` at the render site in Task 2.5, do not also mark it here.

- [ ] **Task 2.9 — Update `migrate_test.go` for To:5.** In `internal/migrate/migrate_test.go`:
  - Add a direct `applyEnableBootstrap` success case (mirroring `TestDropHooksStrips`): a `.awf/config.yaml` containing `prefix: ex\nskills:\n  - tdd\n` → after `applyEnableBootstrap`, the file contains `bootstrap:` with `enabled: true` and the other keys/comments are preserved.
  - Add an **overwrite** case: a config already carrying `bootstrap:\n  enabled: false` → after `applyEnableBootstrap`, it carries `enabled: true` (the upgrade default).
  - Add the **absent-config** case: `applyEnableBootstrap(t.TempDir())` returns nil (covers the `os.IsNotExist` arm).
  - Add a **malformed-YAML** case: a config that does not parse surfaces the `SetMappingScalar` error through `applyEnableBootstrap`.
  - Rename `TestCurrentIsFour` (line 587) to `TestCurrentIsFive` and assert `Current() == 5`.
  - Update the two applied-list assertions the new To:5 lengthens: `TestUpgradeAppliesInOrderIdempotent` (line 146/147) → `tree-layout,drop-replacewith,awf-dir-relocation,drop-hooks,enable-bootstrap`; `TestUpgradeStampsTreeLock` (line 801/802) → `drop-replacewith,awf-dir-relocation,drop-hooks,enable-bootstrap`. Update both the `strings.Join` comparison and the `want [...]` message text.
  - The `(ReadFile permission)` `// coverage-ignore` arm in `enablebootstrap.go` matches the precedent migrations and needs no test.

- [ ] **Task 2.10 — Seed `bootstrap.enabled: true` for fresh projects.** In `internal/config/edit.go`, add the field to `Skeleton` (after `Invariants`, line 23):
  ```go
  	Invariants *InvariantConfig  `yaml:"invariants,omitempty"`
  	Bootstrap  *BootstrapConfig  `yaml:"bootstrap,omitempty"`
  ```
  In `internal/project/scaffold.go`, in the `config.MarshalSkeleton(config.Skeleton{...})` literal (lines 83–90), add:
  ```go
  		Invariants: inv,
  		Bootstrap:  &config.BootstrapConfig{Enabled: true},
  ```
  Update the `ScaffoldConfig` doc comment (lines 15–21) to mention it now also seeds the bootstrap enabled by default (ADR-0040). In `internal/project/scaffold_test.go` (or wherever `ScaffoldConfig` output is asserted), extend a case to assert the scaffolded bytes contain `bootstrap:` / `enabled: true`. Confirm `config_test.go`/`edit_test.go` golden skeletons that pin exact `MarshalSkeleton` output are updated to include the new key (a strict-decode round-trip of the scaffold must still parse).

- [ ] **Task 2.11 — Add the bespoke `bootstrap` CLI token.** In `cmd/awf/list_add.go`:
  - `unknownKind` (lines 16–18): add `bootstrap` to the hint: `"unknown kind %q (want: skill, agent, doc, domain, target, bootstrap)"`.
  - Add an `addRemoveBootstrap` bespoke path (after `addRemoveTarget`, before `enabledNames`), the `target` precedent for a non-`kindDescriptor` token:
    ```go
    // addRemoveBootstrap enables or disables the self-pinning bootstrap singleton in
    // the config (ADR-0040). It is the bespoke path (bootstrap is not a kindDescriptor —
    // it has no catalog pool / sections / plural enable array, so it stays out of the
    // single dispatch table that inv: kind-dispatch-single-table guards): a nested
    // bootstrap.enabled scalar, written via config.SetMappingScalar.
    func addRemoveBootstrap(root string, add bool, stdout io.Writer) error {
    	p, err := project.Open(root)
    	if err != nil {
    		return err
    	}
    	enabled := p.Cfg.Bootstrap != nil && p.Cfg.Bootstrap.Enabled
    	if add && enabled {
    		return fmt.Errorf("bootstrap already enabled")
    	}
    	if !add && !enabled {
    		return fmt.Errorf("bootstrap is not enabled")
    	}
    	cfgPath := filepath.Join(root, ".awf", "config.yaml")
    	b, err := os.ReadFile(cfgPath)
    	if err != nil { // coverage-ignore: config.yaml was just read by project.Open; a re-read cannot fail without a race
    		return err
    	}
    	updated, err := config.SetMappingScalar(b, "bootstrap", "enabled", add)
    	if err != nil { // coverage-ignore: config.Load already parsed this config, so SetMappingScalar's parse/mapping checks cannot fail here
    		return err
    	}
    	if err := os.WriteFile(cfgPath, updated, 0o644); err != nil { // coverage-ignore: post-validation write; fails only on a permission fault root bypasses
    		return err
    	}
    	return runSync(root, stdout)
    }
    ```
  - `runAdd` (line 77): add `if kind == "bootstrap" { return addRemoveBootstrap(root, true, stdout) }` immediately after the existing `if kind == "target"` block.
  - `runRemove` (line 112): add `if kind == "bootstrap" { return addRemoveBootstrap(root, false, stdout) }` immediately after the existing `if kind == "target"` block.
  - `runList` (line 169): after the existing `if kindFilter == "target"` block (lines 174–184) and before the `kinds := project.Kinds()` line, add a bespoke branch:
    ```go
    	if kindFilter == "bootstrap" {
    		state := "available"
    		if p.Cfg.Bootstrap != nil && p.Cfg.Bootstrap.Enabled {
    			state = "enabled"
    		}
    		fmt.Fprintln(stdout, "bootstrap:")
    		fmt.Fprintf(stdout, "  %-28s %s\n", "awf-bootstrap.sh", state)
    		return nil
    	}
    ```
    (Note: the unfiltered `awf list` — `kindFilter == ""` — does **not** print bootstrap; it iterates `project.Kinds()`, which is the descriptor table only. This matches `target`, which the unfiltered list also omits. ADR-0040 Decision 3 says `runList` "gains a bespoke branch reporting its enabled/available state" — a `bootstrap`-filtered branch satisfies that; do not add it to the table-driven loop.)

- [ ] **Task 2.12 — Update the `add`/`remove`/`list` argSpecs help text.** In `cmd/awf/main.go` `argSpecs` (lines 167–267):
  - `list` summary/help (lines 226–230): the help body's kind enumeration `(skill|agent|doc|domain)` is fine to leave, but if you want it exact, it does not list `target` today either — leave `list` as-is unless ADR-0040 requires it; the prompt only mandates add/remove. (No change required.)
  - `add` (lines 232–237): change summary to `"Enable a target — kind ∈ {skill, agent, doc, domain, bootstrap}"` and the help body line to `Enable a target. <kind> is skill, agent, doc, domain, or bootstrap.` (Note: `target` is intentionally not in this user-facing list today — preserve the existing convention; only add `bootstrap`, matching the prompt. If `target` should also be listed, that is out of scope.)
  - `remove` (lines 239–244): change the help body to mention bootstrap: `Disable a target — a catalog skill/agent/doc, a freeform domain, or the bootstrap.`
  - In `cmd/awf/list_add_test.go`, add dispatch cases: `awf add bootstrap` on a project with it disabled enables it (config gains `enabled: true`, sync runs); `awf add bootstrap` when already enabled errors (`already enabled`); `awf remove bootstrap` when enabled disables it; `awf remove bootstrap` when disabled errors (`is not enabled`); `awf list bootstrap` prints `bootstrap:` + `awf-bootstrap.sh` with the right state for both enabled and disabled. These cover every branch of `addRemoveBootstrap` and the `runList` bootstrap branch. Also add an `unknownKind`-message assertion update if the test pins the exact hint string (it now ends `..., target, bootstrap`).

- [ ] **Task 2.13 — Migrate awf's own tree and opt OUT of the bootstrap (dogfood).** The To:5 migration makes `Current() == 5`, so awf's own repo (lock schema 4) now gates on `./x sync` with "run awf upgrade" until migrated. Migrate it, then disable the bootstrap (awf builds from source via `./x`; a self-fetching bootstrap is nonsensical):
  - Run `go run ./cmd/awf upgrade`. Expect `awf upgrade: applied enable-bootstrap` then `awf sync: done` (the chained sync stamps `.awf/awf.lock` to schema 5 and renders `awf-bootstrap.sh` into the repo root, because the migration enabled it).
  - Run `go run ./cmd/awf remove bootstrap`. Expect it to disable `bootstrap.enabled` in `.awf/config.yaml` and re-sync — the sync **prunes** the just-rendered `awf-bootstrap.sh` from the repo root (it is no longer produced, so the prune loop removes it).
  - Confirm `.awf/config.yaml` carries `bootstrap:\n  enabled: false`, `.awf/awf.lock` `schemaVersion` is `5`, and **no** `awf-bootstrap.sh` exists at the repo root, and the lock has no `awf-bootstrap.sh` entry.
  - (Alternative if `remove` after `upgrade` is awkward: hand-edit `.awf/config.yaml` to `bootstrap:\n  enabled: false` and run `./x sync` once — same end state. Prefer the `remove` path to also exercise the new CLI.)

- [ ] **Task 2.14 — Flip ADR-0040 to Implemented; sync; verify; commit.**
  - In `docs/decisions/0040-self-pinning-rendered-bootstrap.md` frontmatter, set `status: Implemented`.
  - Run `go test ./...`. Expect all green (confirms the strict-decoder accepts the new key everywhere, the migration list lengthened, and no golden drift).
  - Run `./x sync`. Expect `awf sync: done` (regenerates `ACTIVE.md` and the `tooling`/`config`/`rendering` domain indexes for ADR-0040; awf's own bootstrap stays disabled so nothing new renders at the root).
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.` (ADR-0040 now Implemented → `inv: bootstrap-pin` and `inv: bootstrap-checksum` enforced and backed by Task 2.8.)
  - Run `./x check`. Expect `awf check: clean` (no stray `awf-bootstrap.sh`, no drift; the lock `awfVersion` is still `0.4.0` == binary, so no version error or ahead-notice).
  - Stage: `templates/bootstrap/awf-bootstrap.sh.tmpl templates/embed.go internal/config/config.go internal/config/config_test.go internal/config/edit.go internal/config/edit_test.go internal/migrate/enablebootstrap.go internal/migrate/migrate.go internal/migrate/migrate_test.go internal/project/render.go internal/project/check.go internal/project/scaffold.go internal/project/golden_test.go` (+ any new `*_test.go`), `cmd/awf/main.go cmd/awf/list_add.go cmd/awf/list_add_test.go`, awf's migrated `.awf/config.yaml` + `.awf/awf.lock`, the regenerated `docs/`, and the ADR file. Commit: `feat(awf)!: render self-pinning bootstrap and migrate schema to 5 (ADR-0040)`. (Breaking-change `!`: the schema bump is adopter-visible.) Optionally split the ADR flip into a trailing `docs(adr): implement 0040 self-pinning rendered bootstrap` per the Phase-end precedent.

## Phase 3 — Docs currency + final

ADR status flips already landed at the end of Phases 1 and 2; `ACTIVE.md` and the domain indexes regenerate on each sync. This phase brings the authored AGENTS.md surface and domain narratives current with the two shipped changes.

- [ ] **Task 3.1 — Add the new invariant slugs to the rendered AGENTS.md Invariants list.** In `.awf/agents-doc.yaml`, the `data.invariants` list feeds the rendered Invariants section (AGENTS.md.tmpl lines 39–41). Add three bullets:
  - `version-compat-gate` (ADR-0039): "**Binary-version gate.** Every gated command (`sync`, `check`, `invariants`, `audit`, `list`) refuses to run when the binary is behind the project on schema generation or lock `awfVersion`. (ADR-0039)"
  - `bootstrap-pin` (ADR-0040): "**Self-pinning bootstrap.** The rendered `awf-bootstrap.sh` pins exactly the rendering binary's `project.Version`. (ADR-0040)"
  - `bootstrap-checksum` (ADR-0040): "**Bootstrap checksum.** The rendered bootstrap verifies the download SHA-256 before installing. (ADR-0040)"
  Match the existing entries' `{text, ref}` shape in `.awf/agents-doc.yaml`'s `data.invariants` (open the file to confirm the exact key names and prose style before editing).

- [ ] **Task 3.2 — Add `bootstrap` to the AGENTS.md toggle grammar.** The `awf-setup` section (AGENTS.md.tmpl line 11) enumerates `(kinds: skill, agent, doc, domain, target)` and currently has **no** override part (`.awf/parts/agents-doc/` holds only `identity.md` and `you-and-this-project.md` — confirmed). To change the rendered text without hand-editing `AGENTS.md`, create the override part `.awf/parts/agents-doc/awf-setup.md` carrying the full `awf-setup` section body (copy the template default from `templates/agents-doc/AGENTS.md.tmpl` lines 7–14) with the kind list changed to `(kinds: skill, agent, doc, domain, target, bootstrap)` and a trailing clause noting `bootstrap` toggles the self-pinning `awf-bootstrap.sh` installer singleton (which awf itself disables, building from source). The convention part is raw input (ADR-0034) — it replaces the section body verbatim, so include the `## Working with awf` heading and all the bullets, not just the changed line. `awf sync` re-renders AGENTS.md from this part.

- [ ] **Task 3.3 — Update the domain current-state narratives.** Edit the authored parts (re-rendered by sync into `docs/domains/<d>.md`):
  - `.awf/domains/parts/tooling/current-state.md`: in the `awf add`/`remove`/`list` sentence (it currently enumerates "the four catalog/domain config enable arrays" and the bespoke `target` token), add a clause for the bespoke `bootstrap` token (ADR-0040): like `target`, it is handled outside the kind-descriptor table and toggles the self-pinning `awf-bootstrap.sh` singleton via a nested `bootstrap.enabled` scalar. Add a sentence on the ADR-0039 gate: `awf sync`/`check`/`invariants`/`audit`/`list` now refuse to run against a project rendered by a *newer* awf (schema generation ahead, or lock `awfVersion` ahead), and `awf check` prints a non-failing notice when the binary is ahead of the project.
  - `.awf/domains/parts/config/current-state.md`: in the schema-migration sentence (it currently mentions `{To:4}` dropped `hooks:`), add that schema migration `{To:5}` added the `bootstrap` enable entry (a nested `bootstrap.enabled` scalar, not a top-level array) with the self-pinning bootstrap (ADR-0040). Note the new `config.SetMappingScalar` alongside `MarshalSkeleton`/`SetArrayMember`/`SetArray` in the `internal/config`-ownership sentence.
  - `.awf/domains/parts/rendering/current-state.md`: in the neutral-singletons sentence (it lists the agent guide, the two ADR-system files, and the plans guide), add that an optional `awf-bootstrap.sh` neutral repo-root singleton renders once when enabled (ADR-0040), pinned to the rendering binary's `project.Version`, and is excluded from the dead-reference scan.
  - Note on `docs/development.md`: awf's rendered `development.md` is the generic template placeholder (no `.awf/docs/parts/development/` overrides exist) — it carries **no** awf command reference to update. The authoritative command/grammar surface is the `tooling` domain narrative and the AGENTS.md Commands/awf-setup sections, edited above. (If a future `.awf/docs/parts/development/command-runner.md` override is added it would be the place, but none exists today — see the Anchor-reconciliation note.)

- [ ] **Task 3.4 — Sync, verify, commit.**
  - Run `./x sync`. Expect `awf sync: done` (re-renders AGENTS.md and the three domain docs from the edited parts).
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Run `./x check`. Expect `awf check: clean`.
  - Stage `.awf/agents-doc.yaml` (and any `.awf/parts/agents-doc/awf-setup.md`), the three `.awf/domains/parts/*/current-state.md`, and the regenerated `AGENTS.md` + `docs/` + lock. Commit: `docs(awf): document the version gate and self-pinning bootstrap (ADR-0039, ADR-0040)`.

## Verification (whole change)

- `./x gate full` green; `./x check` clean.
- `grep -n "golang.org/x/mod" go.mod` shows a direct (non-`// indirect`) require.
- `migrate.Current() == 5`; awf's `.awf/awf.lock` `schemaVersion` is `5`; no `awf-bootstrap.sh` at the repo root (awf opted out); awf's `.awf/config.yaml` has `bootstrap:\n  enabled: false`.
- `awf list bootstrap` reports `awf-bootstrap.sh   available`; `awf add bootstrap` then `awf check` would render and track a repo-root `awf-bootstrap.sh` in a fresh adopter (not in awf itself).
- ADR-0039 and ADR-0040 are Implemented; their `inv:` slugs (`version-compat-gate`, `bootstrap-pin`, `bootstrap-checksum`) are backed.

## Execution

Tasks are ordered and coupled (Phase 1's gate is the backstop ADR-0040 references; Phase 2's strict-decoder field addition + migration + self-migration must stay atomic to keep `go test`/gate green; Phase 3 is doc currency over the shipped behaviour). Execute inline with `awf-executing-plans` (one task at a time, `./x gate` per commit), not subagent dispatch.
