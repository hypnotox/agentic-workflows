---
status: Implemented
date: 2026-07-04
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [audit, config, rendering]
related: [0008, 0017, 0036, 0045, 0051]
domains: [config, rendering, tooling]
---
# ADR-0055: Granular domain-aligned commit scopes

## Context

awf's own `audit.allowedScopes` is `[adr, awf, plans]`. Across the last 200 commits that
resolves to `adr` (43, ADR documents) and `plans` (19, plan documents) as pure doc buckets,
with everything else — `feat`, `refactor`, `fix`, `test`, `chore`, `ci` — collapsing into a
single undifferentiated `awf` (138). The scope carries a *type* distinction but no *area*
distinction: a render-engine change and a config-schema change read identically as
`refactor(awf)`. The scope list is a real gate, not just advisory — `awf commit-gate`
(ADR-0036) is wired into `.githooks/commit-msg` and hard-rejects a commit whose scope is
outside the list — so the coarseness is a governance choice worth revisiting deliberately.

The project already maintains a vocabulary for its functional areas: the `domains:` array in
`.awf/config.yaml` (`adr-system`, `config`, `invariants`, `rendering`, `tooling`), each with a
`docs/domains/<name>.md`. That vocabulary is the natural anchor for area-scoped commits — but
nothing today connects domains to commit scopes.

ADR-0051 made `audit.allowedScopes` the single storage for commit scopes and had rendered
prose consume it through a `commitScopes` render-context key, so the reviewing-skill templates
and the commit-gate can never disagree. Its Consequences explicitly anticipated multi-scope
projects "including awf itself." One surface was left behind, though: the agent guide. The
guide's scope mention lives as a hand-written `text:` invariant entry in `.awf/agents-doc.yaml`
("scopes `adr`/`awf`/`plans`"), and the guide template (`templates/agents-doc/AGENTS.md.tmpl`)
renders each invariant literally via `{{ range .data.invariants }}` → `{{ .text }}`. This is
*not* a breach of ADR-0051's backed invariant `commit-scope-single-storage`: that invariant is
template-scoped (its check scans only the embedded `templates/` FS for `.vars.commitScope`),
and `agents-doc.yaml` is project *data*, not a template. So the hand-written mention slipped
between the contract's slats — a second place scopes are written by hand, which must be kept in
sync with `audit.allowedScopes` manually, exactly the drift trap ADR-0051 set out to remove
everywhere else.

## Decision

1. **awf adopts an eight-scope, domain-aligned taxonomy.** `audit.allowedScopes` in
   `.awf/config.yaml` becomes:

   | scope | covers |
   |---|---|
   | `adr` | ADR markdown documents (`docs(adr)`) |
   | `plans` | plan markdown documents (`docs(plans)`) |
   | `awf` | genuinely cross-cutting / repo-meta work (version bump, top-level README) — the umbrella of last resort |
   | `adr-system` | the ADR machinery code (ACTIVE.md generation, lifecycle) |
   | `config` | the `.awf` config tree, schema, migrations |
   | `invariants` | invariant backing and checks |
   | `rendering` | the render engine and templates |
   | `tooling` | CLI, audit/gate, coverage, CI, `./x`, changelog, evals |

   The five code scopes are exactly the `domains:` entries. `adr` (docs) and `adr-system`
   (code) are kept distinct because they name genuinely different concerns; `awf` is kept as an
   umbrella so a truly cross-cutting commit always has a valid home.

2. **The domains-to-scopes equality is a hand-maintained convention, not a mechanical link.**
   Adding or removing a `domains:` entry does not change `audit.allowedScopes`, and vice versa.
   No code enforces the correspondence; it is a documented convention (Decision item 4) that a
   maintainer upholds by hand. This is deliberate — scopes and domains have independent
   lifecycles, and coupling them would over-constrain both.

3. **The agent guide's scope mention derives from `audit.allowedScopes`.** The hand-written
   scope `text:` entry in `.awf/agents-doc.yaml` is replaced by a typed entry carrying no scope
   tokens:

   ```yaml
   - kind: scopes
   ```

   The guide template's invariants range gains a branch that renders the scope invariant from
   the *root* scope key `$.commitScopes` (not `.commitScopes` — inside `range` the dot is
   rebound to the loop element). The whole scope clause is guarded, so the accept-any case
   (`audit.allowedScopes` unset, `$.commitScopes` empty) degrades to coherent generic prose per
   ADR-0045:

   ```
   {{- if eq .kind "scopes" }}
   - **Conventional Commits{{ with $.commitScopes }}, scopes {{ . }}{{ end }}.** One concern per commit; stage explicitly, no `git add -A`{{ with $.commitScopes }}; the allowed-scope list lives in `audit.allowedScopes` (ADR-0051){{ end }}.
   {{- else }}
   - {{ .text }}{{ with .ref }} ({{ . }}){{ end }}
   {{- end }}
   ```

   - scopes defined → `**Conventional Commits, scopes `adr`, `adr-system`, ….** One concern per
     commit; stage explicitly, no `git add -A`; the allowed-scope list lives in
     `audit.allowedScopes` (ADR-0051).`
   - scopes unset → `**Conventional Commits.** One concern per commit; stage explicitly, no
     `git add -A`.`

   Because the template now contains a `$.commitScopes` action, `AGENTS.md` joins the
   config-hash reflag set (ADR-0051's `scopes-in-confighash` mechanism, `render.ReferencesScopes`
   matches both `.commitScopes` and `$.commitScopes`): a later `audit.allowedScopes` edit flags
   `AGENTS.md` stale in `awf check`.

4. **The taxonomy is documented in `docs/workflow.md`.** The commit-discipline section gains the
   scope table and each scope's meaning, plus a note that the code scopes mirror the `domains:`
   list by convention. The commit-gate only checks set membership — it cannot catch a
   wrong-but-valid pick (`adr` where `adr-system` was meant) — so the discriminating guidance
   must live in prose. A light cross-reference points at `docs/domains/`.

## Invariants

- `inv: guide-scopes-derived` — the agent-guide template renders its commit-scope mention from
  the `$.commitScopes` render key, and `.awf/agents-doc.yaml` carries no hand-written commit-scope
  token list; the mention degrades to generic Conventional-Commits prose when scopes are
  accept-any. Backed by a golden test under `./internal/...` (carrying the `// invariant:
  guide-scopes-derived` marker) that renders awf's *own* `agents-doc.yaml` invariants — not a
  synthetic fixture — under both a populated and an empty `commitScopes` and asserts the exact
  rendered scope line. This backs all three clauses at once: the derivation and the accept-any
  degradation are asserted directly, and a re-introduced hand-written scope `text:` entry would
  surface as a second scope mention in the asserted output, failing the test. (A fixture-only
  render would check derivation and degradation but leave the "no hand-written token list" clause
  aspirational, since it never inspects awf's real data.)
- awf's five code commit scopes (`adr-system`, `config`, `invariants`, `rendering`, `tooling`)
  equal the `domains:` entries — a hand-maintained textual convention, not machine-enforced.

## Consequences

- The tool's own commits gain an area dimension: `feat(rendering)`, `refactor(config)`,
  `fix(invariants)` instead of everything reading as `awf`. `git log` and `awf audit` become
  legible by area, and the umbrella `awf` scope now signals "genuinely cross-cutting" rather
  than "unclassified default."
- Scopes are written by hand in exactly one place — `audit.allowedScopes`. The guide, the
  reviewing skills, and the commit-gate all derive from it; the last hand-written mirror is
  removed.
- The commit-gate cannot distinguish `adr` from `adr-system` (both valid), so scope *correctness*
  remains a convention enforced by review and the workflow doc, not by the gate. Accepted: the
  gate guarantees membership, prose guarantees meaning.
- Adding a domain does not add a scope. A maintainer who introduces a new `domains:` entry must
  decide separately whether it earns a commit scope and edit `audit.allowedScopes` too. The
  workflow doc records this so the omission is a choice, not a surprise.
- The typed `kind: scopes` entry is a small new mini-convention in `agents-doc` data. It is
  free-form `map[string]any` with no struct schema, so it triggers no validation; the guarantee
  that the scopes branch renders (and never falls through to the empty-`text` generic line)
  rests on the golden test.
- Downstream: adopters are unaffected — the catalog `agents-doc` singleton ships no default
  invariants data, so the typed entry and its template branch are inert for a project that does
  not supply a `kind: scopes` invariant, and the accept-any fallback keeps the branch safe if
  one is supplied without scopes configured.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the coarse `[adr, awf, plans]` set | The single `awf` bucket is the problem being solved; it erases the area distinction the domains vocabulary already draws. |
| Drop the `awf` umbrella entirely | A genuinely cross-cutting commit (version bump, top-level README) would have no valid scope and would be rejected by the gate; the umbrella is a needed escape hatch. |
| Collapse `adr` and `adr-system` into one scope | Loses the docs-vs-machinery distinction; the two name real, separately-evolving concerns. |
| Mechanically link scopes to the `domains:` list | Over-constrains both — a domain need not be a commit scope, and `adr`/`plans`/`awf` are scopes with no domain. The equality is a convention, not a law. |
| Render the guide's `.text` field through the template engine (double-render) | Broadest blast radius, conflicts with the raw-data model, and over-engineered for one invariant line; a typed entry with a single template branch is the minimal change. |
| Leave the hand-written guide scope prose in place | Re-introduces the exact hand-sync drift ADR-0051 removed elsewhere; the guide would silently disagree with `audit.allowedScopes` after any edit. |
