---
status: Proposed
date: 2026-06-30
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [tooling, distribution, rendering, schema]
related: [0024, 0027, 0030, 0039]
domains: [tooling, rendering, config]
---
# ADR-0040: Self-Pinning Rendered Bootstrap

## Context

awf is distributed as prebuilt, checksum-bearing release archives plus a `go install` fallback
([ADR-0030](0030-prebuilt-binary-distribution-and-release.md)). It does not, however, give an
adopter any *vendored* way to obtain a pinned binary. The first adopter (`fleet`) hand-rolled one:
a ~30-line `ensure_awf` in its `./x` dev wrapper that detects arch, fetches
`awf_<ver>_<os>_<arch>.tar.gz` plus `checksums.txt`, verifies the SHA-256, extracts, and caches the
binary — pinned by a hardcoded `AWF_VERSION="0.4.0"`. It took three follow-up commits to harden
(temp cleanup, fail-fast on fetch error, asset-name verification). Every future adopter would
re-derive and re-debug the same boilerplate.

Worse, that hardcoded pin is a *second* source of truth alongside `.awf/awf.lock`'s `awfVersion`
(which awf stamps from `project.Version` on every `sync`). The two drift independently — `awf
upgrade` touches neither the wrapper's pin nor (directly) the lock's `awfVersion`. [ADR-0039](0039-binary-version-compatibility-gate.md)
adds a gate that *detects* a behind binary, but detection is a backstop; the root cause is that the
pin is authored by hand in a place awf never rewrites.

awf already renders neutral (tool-agnostic) singletons to fixed repo paths — `AGENTS.md` is rendered
once to the repo root regardless of target. A bootstrap script is exactly such an artifact: it is a
shell tool, not a Claude/Cursor adapter file. Rendering it lets awf own the pin and rewrite it on
every `sync`, collapsing the two sources of truth into one.

Verified facts (grounding): neutral singletons are emitted by explicit blocks in `RenderAll`
(`internal/project/render.go:159-216`) — not via the `renderKind`/catalog loop — and a singleton
carries a `catalog` `TargetSpec` only when it has overridable sections (`AgentsDoc`, `AdrReadme`,
…). The bootstrap singleton therefore needs its own explicit `RenderAll` block; a `TargetSpec` is
required only if it exposes convention-part sections. Any file that flows through `RenderAll` is
lock-tracked and drift-checked for free. `project.Version` is *not*
currently in the template data namespace (`p.data()` exposes only prefix/vars/data/layout), so
exposing it is new work. `renderTarget` rejects any output containing `<no value>` (the ADR-0001
publication-safety mechanism). The dead-reference scan keys on `isManagedMarkdown(tid)`, which today
returns true for everything except `claude/CLAUDE.md.tmpl` — so a `.sh` template would be wrongly
scanned for markdown links. Config decoding uses `yaml` `KnownFields(true)` (strict): an older awf
binary reading a config that carries a new top-level key fails to parse, which is why a new config
surface must travel with a schema-generation bump so the [ADR-0039](0039-binary-version-compatibility-gate.md)
schema gate (and `awf upgrade`) engage rather than a cryptic decoder error. `awf add`/`remove`/`list`
([ADR-0024](0024-cli-config-management.md)) resolve a `<kind>` token through the unified kind
descriptor table ([ADR-0027](0027-unified-kind-descriptor.md)); `target` ([ADR-0037](0037-multi-target-rendering-and-cursor-adapter.md))
is precedent for a token handled bespoke, outside that table.

## Decision

1. **Render `awf-bootstrap.sh` as a neutral repo-root artifact.** A new template renders to
   `awf-bootstrap.sh` at the repo root (the `AGENTS.md` pattern: neutral, rendered once, not
   per-target). The script detects OS/arch, downloads
   `awf_<version>_<os>_<arch>.tar.gz` and `checksums.txt` from the GitHub release, verifies the
   SHA-256, extracts, installs into a cache directory, and prints the cached binary path. The
   download URL path uses the `v`-prefixed git tag (`v<version>`) while the asset filename uses the
   no-`v` form (`<version>`), per [ADR-0030](0030-prebuilt-binary-distribution-and-release.md) /
   `.goreleaser.yaml`.

2. **Pin to the rendering binary's version — one source of truth.** The script's pinned version is
   `project.Version` of the awf binary that ran `sync`, wired into the template data namespace. This
   is the same value awf stamps into the lock's `awfVersion`, so after `awf upgrade && awf sync` with
   a newer binary the bootstrap re-pins itself. The pin is no longer authored by hand anywhere.

3. **Toggle via a bespoke `bootstrap` token, following the `target` precedent.** `awf add bootstrap` /
   `awf remove bootstrap` enable and disable the artifact through a dedicated top-level config key
   (a `bootstrap` enable entry). Because the artifact is a once-rendered neutral singleton with no
   catalog pool and no plural enable-array, it does **not** fit the `kindDescriptor` table (`inv:
   kind-dispatch-single-table`, which every table kind backs with a `poolNames`/`sections`/plural
   enable facet). It is therefore handled bespoke *outside* that table — exactly as `target`
   ([ADR-0037](0037-multi-target-rendering-and-cursor-adapter.md)) is special-cased in `runAdd` /
   `runRemove` / `runList` (`cmd/awf/list_add.go`) rather than added to `kindDescriptors`. The
   `unknownKind` hint string (`cmd/awf/list_add.go:16-18`, today `"want: skill, agent, doc, domain,
   target"`) gains `bootstrap`, and `runList` gains a bespoke branch reporting its enabled/available
   state. The artifact renders only when enabled; `init` seeds it enabled by default.

4. **Bump the config schema to generation 5.** Because the `bootstrap` config surface is a new
   top-level key (`config.Config`, `internal/config/config.go:38-47`) that the strict YAML decoder
   (`KnownFields(true)`) of an older binary would reject, a new `migrate.Migration` (To: 5, appended
   to the `internal/migrate` registry after `{To: 4}`) ports existing configs and enables
   `bootstrap` for them (preserving the new default on upgrade). This makes the change visible to the
   [ADR-0039](0039-binary-version-compatibility-gate.md) schema-ahead gate rather than surfacing as a
   decoder error in binaries that contain the gate. The schema bump touches the config and render
   consumers; the **manifest/lock format is unchanged** (the bootstrap output is a new lock-tracked
   *entry*, not a new lock *field*), and `migrate` gains only the additive To:5 migration — no
   existing migration is rewritten.

5. **Drift-check the rendered script; exclude it from the dead-reference scan.** The artifact flows
   through `RenderAll`, so it is lock-tracked and `awf check` validates it like every other rendered
   file. `isManagedMarkdown` is taught to exclude the `.sh` template id so the markdown
   dead-reference scan ([ADR-0020](0020-dead-reference-check.md)) does not run against shell. The
   template resolves every variable so it can never render `<no value>` (ADR-0001).

6. **Bash, linux/darwin, for now.** The rendered script targets bash on linux/darwin. Windows
   adopters are out of scope for this artifact (a `.ps1` companion is a later, separate decision).

7. **Docs travel with the change.** The commit that lands this work also: regenerates
   `docs/decisions/ACTIVE.md` via `./x sync` (the ADR status flips to Accepted/Implemented); adds the
   `docs/decisions/README.md` index row for ADR-0040; and updates `AGENTS.md` plus any affected
   project docs (e.g. the command/conventions surface) — `AGENTS.md` is itself rendered, so this
   means a `.awf/` edit re-rendered through `awf sync`. The bootstrap toggle (`awf add/remove
   bootstrap`) and the schema-5 bump are workflow/convention-visible changes, so the agent guide and
   `docs/development.md` command reference are updated in the same commit.

## Invariants

- `inv: bootstrap-pin` — the rendered `awf-bootstrap.sh` pins exactly the rendering binary's
  `project.Version`: a golden-render test asserts the script contains the literal assignment
  `AWF_VERSION="<project.Version>"` (the same value `sync` stamps into the lock's `awfVersion`), so
  the pin has a single source of truth and cannot drift from the lock.
- `inv: bootstrap-checksum` — a golden-render test asserts the rendered `awf-bootstrap.sh` contains
  a `sha256` verification step (the `checksums.txt` comparison) ahead of the install step, so the
  download is always integrity-checked before use. (Checkable by string presence in the rendered
  output; unlike [ADR-0039](0039-binary-version-compatibility-gate.md)'s skip-on-unparseable
  textual contract, this is a golden-render assertion, not a runtime-only contract.)

## Consequences

- **One source of truth for the pin.** `awf upgrade && awf sync` rewrites the bootstrap pin
  automatically; adopters stop hand-maintaining (and re-debugging) an `ensure_awf`. This is the root
  fix that [ADR-0039](0039-binary-version-compatibility-gate.md) only backstops.
- **Schema 4→5 transition.** Once shipped, an adopter on an older awf binary that has not upgraded
  will hit the strict-decoder failure (binaries predating ADR-0039) or the schema-ahead gate
  (binaries containing it) if they run against a schema-5 config. The intended path is: bump the
  pinned binary, `awf upgrade`, `awf sync` — after which the bootstrap re-pins. This couples the two
  ADRs into a single v0.5.0 release.
- **Shell runtime is outside the coverage gate.** The 100% Go statement-coverage gate
  ([ADR-0012](0012-full-coverage-gate-and-conventions.md)) covers the Go code that *renders* the
  template (testable by golden render) but not the shell script's runtime behavior (fetch, verify,
  cache) — there is no shell-coverage harness in the repo. Real-world exercise comes from `fleet`
  migrating its `./x` onto the rendered script; that migration is downstream of this repo.
- **New default for fresh and upgraded projects.** `init` and the schema-5 migration both enable the
  bootstrap, so adopters get it unless they `awf remove bootstrap`.
- **`project.Version` enters the template data namespace**, a small widening of what templates can
  reference.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Always-on singleton, opt out via `local: true` (the strict `AGENTS.md` pattern) | Simplest and needs no schema bump, but offers no first-class `awf add/remove` toggle; an adopter with their own installer should be able to disable it through the normal CLI grammar. |
| Boolean `bootstrap: true` config field | No precedent for a scalar bool in the config; still needs a schema bump, with a less uniform shape than the kind/enable-array grammar already used for skills/agents/docs. |
| Ship a generic `curl … \| sh` installer instead of rendering | Not per-project pin-aware; adopters would still author and drift their own version pin — leaving the two-sources-of-truth root cause unsolved. |
| Document `go install …@vX` as the blessed path | Requires a Go toolchain, recompiles on every cache miss, and only serves Go adopters; contradicts ADR-0030's prebuilt-binary-as-canonical stance. |
