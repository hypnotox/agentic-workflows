---
status: Implemented
date: 2026-07-08
supersedes: []
retires_invariants: [invariants-glob-basename]
superseded_by: ""
tags: [audit, config, staleness, migration]
related: [17, 19, 26, 33, 39, 49, 76, 86]
domains: [tooling, config, invariants]
---
# ADR-0077: Anchored Path Globs and the Domain Code-Staleness Audit Rule

## Context

ADR-0019 closed the ADR-driven half of domain-doc currency: an ADR reaching `Implemented`
without a co-change to `.awf/domains/parts/<X>/current-state.md` warns, and an ADR tagged with an
unconfigured domain warns. The other half is still open: **ADR-less code churn**. Commits routinely
change a domain's territory — its packages, commands, templates — without any ADR being involved,
and nothing connects those commits to the domain narrative that describes the changed behaviour.
The narrative silently drifts until a reader trusts prose that no longer matches reality.

Detecting that requires knowing *which files belong to which domain* — a path→domain mapping —
and matching changed-file paths against it. awf's existing glob support cannot express such a
mapping: both glob-consuming config surfaces (`invariants.sources[].globs` and
`audit.dependencyManifests`) are validated by `validateBasenameGlob`, which **rejects any pattern
containing `/`** and matches via `filepath.Match` against a file's basename only
(`internal/config/config.go`, `internal/invariants/invariants.go`, `internal/audit/audit.go`
`matchesAny`). `*.go` works; `cmd/**` and `internal/audit/*.go` are inexpressible — in the new
mapping and in the existing fields alike.

Two glob dialects side by side (basename-only in the old fields, path-aware in the new one) would
be a standing confusion, so the glob semantics are unified in the same decision. A gitignore-style
hybrid — no-slash patterns match basenames at any depth, slashed patterns are anchored — was
considered and deliberately rejected in favour of **one pure rule with a one-time migration**:
every pattern is an anchored full-path match, `*.go` means top-level `.go` files only, and any-depth
must be written `**/*.go`. This diverges from gitignore intuition, which the docs must state
explicitly; in exchange there is exactly one semantic to learn, document, and test.

Grounding facts that shaped the decision (verified against source):

- Changed-file paths in the audit are repo-relative and slash-separated (go-git tree entry names),
  directly usable for anchored matching. Merge commits contribute no `Changes` at the engine level,
  so no per-rule merge exemption is needed. Renames surface as delete+insert pairs (go-git performs
  no rename detection), so a pure rename inside a domain's paths registers as churn.
- `Inputs.GeneratedPaths` is an exact-path set built from lock keys; rendered `docs/domains/<X>.md`
  is in it, hand-authored `.awf/domains/parts/**` is not.
- Per-domain sidecars `.awf/domains/<name>.yaml` are an existing, strictly-parsed
  (`KnownFields(true)`) surface read via `Cfg.Sidecar("domains", name)` and covered by orphan
  detection — a natural home for per-domain settings that keeps `domains:` in `config.yaml` a plain
  string list (no union entry type, no `SetArrayMember`/kind-descriptor changes).
- No scaffold or init default emits a `*.go` glob (`Skeleton.Invariants` is set nowhere in
  production), so the config migration has no scaffold counterpart — but
  `defaultDependencyManifests()` hard-codes 19 basename patterns in code, which the semantics
  change must rewrite lest nested manifests (`packages/*/package.json`) silently stop matching.
- The migration machinery fits: migrations are ordered `{To, Name, Apply}` entries (current highest
  generation 6), config rewrites route through `internal/config` editors ending in
  `manifest.WriteFileAtomic` (ADR-0076), and a schema bump requires a `minVersionBySchema` entry
  plus a `project.Version` bump (ADR-0049), enforced by a gate test. No existing config editor
  rewrites nested sequence members (`SetMappingScalar` handles only a nested scalar), so the
  migration needs a new nested-edit helper, owned by `internal/config` per ADR-0026.

## Decision

1. **One glob semantic, one package.** Add a leaf package `internal/pathglob` wrapping the new
   direct dependency `github.com/bmatcuk/doublestar/v4`, exposing `Validate(pattern) error` and
   `Match(pattern, relPath) bool`. Semantics are **pure anchored full-path doublestar**: a pattern
   matches against the slash-separated repo-relative path, with no basename mode — `*.go` matches
   only top-level `.go` files; any-depth is written `**/*.go`; `cmd/**` and `internal/audit/*.go`
   work as read. Matching is case-sensitive, as today.

2. **Every glob consumer moves onto it.** `validateBasenameGlob` is replaced by `pathglob.Validate`
   (the reject-slashes rule is dropped); `internal/invariants` converts each walked file to a
   slash-separated repo-relative path (`filepath.Rel` + `filepath.ToSlash`) and matches via
   `pathglob.Match`; the audit's `matchesAny` matches `Inputs.DependencyManifests` against the full
   `FileChange.Path` instead of its basename. `defaultDependencyManifests()` rewrites its built-in
   basename patterns to `**/<pattern>` so default behaviour is preserved. Doc-strings and error
   messages that describe "filename pattern matched against a basename" are updated to the anchored
   semantics. No production matcher retains a basename mode. This retires ADR-0008's Implemented
   invariant `invariants-glob-basename` (which asserts basename matching via `filepath.Match`) via
   the ADR-0031 mechanism — the `retires_invariants` frontmatter takes effect when this ADR reaches
   `Implemented`, and the retired slug's backing test is removed in the same change.

3. **One-time config migration, schema generation 7.** A new migration (`To: 7`) rewrites every
   no-slash pattern in `invariants.sources[].globs` and `audit.dependencyManifests` in
   `.awf/config.yaml` to `**/<pattern>`, preserving behaviour for every pattern valid under the old
   validator — with one edge: doublestar is a syntax superset of `filepath.Match`, so a legacy
   pattern containing `{`/`}` (literal before, alternation after) changes meaning; accepted as a
   theoretical corner no known config exercises. Patterns already containing
   `/` are untouched, making the migration idempotent. The rewrite goes through a new nested-edit
   helper in `internal/config` (ADR-0026 ownership) and lands atomically (ADR-0076). The bump
   carries a `minVersionBySchema[7]` entry and the matching `project.Version` bump (ADR-0049);
   the ADR-0039 version gate then keeps old binaries away from migrated trees — which also protects
   the sidecar extension in Decision 4, since an old binary's strict sidecar parse would reject the
   new key.

4. **Per-domain `paths` in the domain sidecar.** The shared `Sidecar` struct gains a
   `Paths []string` field (`yaml:"paths"`). It is meaningful only on domain sidecars
   (`.awf/domains/<name>.yaml`), where it declares the domain's file territory as pathglob
   patterns; `config.yaml`'s `domains:` array stays a plain string list. `Project.Audit()` reads
   each configured domain's sidecar, validates every pattern via `pathglob.Validate` (a malformed
   pattern is a hard error), and supplies a new `Inputs.DomainPaths map[string][]string` alongside
   the existing `ConfiguredDomains` — keeping the audit package decoupled from config, mirroring
   ADR-0019 Decision 4. Validation is deliberately audit-time only: `paths` is consumed nowhere
   else, and validating it in sync/check would pull audit-rule semantics into passes that otherwise
   ignore the field — so a malformed pattern surfaces on the first `awf audit` run rather than at
   sync time; an accepted deferral for an advisory-only input.

5. **New advisory audit rule `domain-code-staleness`.** Range-scoped, in the shipped `awf audit`,
   mirroring ADR-0019's shape. For each configured domain `X` with non-empty `paths`: if any
   in-range commit changed a file whose repo-relative path matches one of `X`'s patterns and is not
   in `Inputs.GeneratedPaths`, and no in-range commit changed
   `.awf/domains/parts/<X>/current-state.md`, emit **one branch-level `Warning`** for `X`: files
   belonging to the domain changed but its current-state narrative was not refreshed — if anything
   meaningful changed, document it. The rule keys on the source part, never the rendered
   `docs/domains/<X>.md` (in the generated-paths set and regenerated on unrelated ADR changes). It
   is inert for a domain without `paths` and gets its own `*bool` toggle in `AuditConfig`
   (nil = enabled), per the established per-rule config semantics.

## Invariants

- `invariant: pathglob-anchored` — `pathglob.Match` is an anchored full-path doublestar match against a
  slash-separated repo-relative path: `*.go` does not match `cmd/a.go`; `**/*.go` matches both
  `a.go` and `cmd/a.go`; `cmd/**` matches every file under `cmd/`. No production matcher matches
  against a basename.
- `invariant: audit-domain-code-staleness` — the rule emits a `Warning` for domain `X` exactly when `X`
  is configured with non-empty sidecar `paths`, an in-range commit changed a non-generated file
  matching those patterns, and no in-range commit changed
  `.awf/domains/parts/<X>/current-state.md`; it is silent when the part is co-changed, when only
  generated paths matched, when `X` declares no `paths`, and when disabled via its toggle.
- `invariant: glob-migration-anchored` — the generation-7 migration rewrites every no-slash pattern in
  `invariants.sources[].globs` and `audit.dependencyManifests` to `**/<pattern>`, leaves slashed
  patterns untouched, and is idempotent.
- The rule is advisory: it produces only `Warning` findings and never makes `awf audit` exit
  non-zero (textual; inherited from ADR-0017's `warn-exit-zero`).
- A malformed `paths` pattern in a configured domain's sidecar is a hard error from
  `Project.Audit()`, not a silently skipped pattern (textual).

## Consequences

- Closes the ADR-less half of the domain-doc currency gap: code churn in a domain's declared
  territory now nudges the narrative refresh the same way an ADR flip does (ADR-0019). Adoption is
  opt-in per domain — no `paths`, no rule.
- One glob semantic everywhere: path patterns become legal in `invariants.sources[].globs` and
  `audit.dependencyManifests` too (e.g. scoping invariant scanning to `internal/**`). The
  divergence from gitignore intuition (`*.go` is top-level only) is the accepted cost, stated in
  the docs; the migration keeps every existing config behaving identically.
- Adopters must run `awf upgrade` once (schema 7); the version gate refuses mismatched
  binary/tree pairs in both directions, as designed. The changelog entry is a breaking change.
- New direct dependency `bmatcuk/doublestar/v4` (zero transitive requirements) — this ADR is its
  ADR-0017 `dependency-adr` record.
- The rule inherits go-git's diff view: a pure rename inside a domain's territory counts as churn
  and can warn with no content change; a domain glob covering another domain's part files produces
  cross-domain noise. Both are absorbed by the advisory `Warning` severity, the per-rule toggle,
  and the human materiality judgment ADR-0019 already relies on.
- `Paths` on a non-domain sidecar (skill, agent, doc) is legal and inert — the shared strictly-
  parsed struct gains the field for all kinds. Accepted as pre-1.0 slack rather than adding
  per-kind sidecar types; revisit if it misleads in practice.
- Downstream doc work owed by the implementing range: the `tooling`, `config`, and `invariants`
  current-state narratives (the `config` one also corrects two pre-existing stale claims found
  during grounding: `ScaffoldConfig` takes no invariants config, and additive config fields are
  not unconditionally bump-free — this ADR's sidecar extension is the counterexample);
  `docs/architecture.md` (new package); `docs/working-with-awf.md` (glob semantics, domain
  `paths`); `docs/development.md` (dependency reference: doublestar entry); `docs/glossary.md`
  ("anchored glob", domain `paths`); the agent guide's invariants section (add this ADR's
  invariant bullets, drop the retired basename bullet if present) re-rendered via `./x sync`;
  and the status-flip commit regenerates `docs/decisions/ACTIVE.md` via `./x sync`. Dogfood
  `paths` for awf's own five domains.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Gitignore-style hybrid semantics (no-slash = basename at any depth), no migration | Two matching modes forever and a per-pattern dispatch rule; the user chose one explicit semantic plus a one-time mechanical migration. |
| New field path-aware, old fields basename-only (no unification) | Two glob dialects side by side in one config file — a standing confusion about which rule applies where. |
| Hand-rolled `**` matcher instead of doublestar | Glob edge cases are a bug farm; a small battle-tested dependency is exactly what a dependency is for. |
| `paths` as enriched `config.yaml` domain entries (string \| `{name, paths}` union) | Verified blast radius: custom `UnmarshalYAML`, a names projection for the kind-descriptor `enable` facet, `SetArrayMember` taught to match mappings by `name`, and the union itself forces the backward-parse coupling. The existing domain sidecar surface carries the same information with none of that. |
| `paths` in a separate `audit.domainPaths` map | Splits a domain's definition across two config sections; the sidecar keeps it colocated with the domain's parts. |
| `paths` in the current-state part's frontmatter | Machine config inside a prose part — a new pattern nothing else uses, and the audit would parse part files. |
| Fold the trigger into ADR-0019's `domain-doc-staleness` rule | Muddies that rule's precisely-stated invariant and shared toggle; an adopter could not keep the low-noise ADR trigger while silencing the chattier churn trigger. |
| Repo-wide staleness scan (age thresholds, commit counts since last part touch) | Not range-scoped; ADR-0019 already rejected repo-state scanning for the audit, and this rule stays within the in-range co-change model. |
