---
status: Implemented
date: 2026-07-04
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [rendering, parts]
related: [0012, 0034, 0045, 0057]
domains: [rendering]
---
# ADR-0058: Backslash escape and value-token-free guard for placeholders

## Context

ADR-0057 added the `{{=awf:identifier}}` sandbox placeholder: a literal substitution over *raw
convention-part bodies* (`substitutePlaceholders`, wired in `planSections`), with a strict pass
(`\{\{=awf:([A-Za-z][A-Za-z0-9]*)\}\}`) and a residual guard (`\{\{=\s*awf`) that hard-errors any
malformed near-miss. Two gaps showed up the moment awf consumed it:

- **You cannot write the syntax in a raw part.** awf's own architecture and rendering *narrative
  parts* (`.awf/docs/parts/architecture/data-flow.md`, `.awf/domains/parts/rendering/current-state.md`)
  describe the placeholder mechanism, and every literal `{{=awf` in them tripped the substitution
  or the residual guard — they had to be reworded to avoid the token. The same wall hits any adopter
  who documents the feature in one of their own overrides. There is no escape.
- **A registry value containing the token trips the guard confusingly.** A registry value is
  config-derived (a scope `Name`/`Meaning`, `prefix`, the gate-command vars). An adopter whose scope
  `meaning` literally contained `{{=awf` would get the post-substitution residual guard firing on a
  *value* it produced — an error pointing at the wrong thing.

This ADR refines ADR-0057's raw-part surface only. Template *defaults* never pass through
`substitutePlaceholders` — they use `text/template`'s own escaping (`{{ "{{=awf:key}}" }}`) to show
the literal syntax — so nothing here touches them.

## Decision

1. **Backslash escape for the raw-part surface.** In a convention part, a backslash immediately
   before the token opener — matched as `\` + `{{=` + optional whitespace + `awf` — is an escape:
   the backslash is consumed and the remainder renders **verbatim**, invisible to *both* the
   strict-substitution and residual-guard passes. So `\{{=awf:commitScopeTable}}` renders the literal
   `{{=awf:commitScopeTable}}`. The target deliberately mirrors the residual guard's `\s*awf` scope
   (not a bare `\{{=`), so it neutralises both passes and does not consume a backslash before an
   unrelated `{{=` in prose.

2. **NUL-sentinel implementation.** `substitutePlaceholders` consumes only the leading backslash and
   stands the matched `{{=` behind a NUL-delimited sentinel *before* the two existing passes,
   restoring that `{{=` verbatim *after* — the same inert-sentinel technique ADR-0034 uses for part
   bodies. Only the `{{=` is hidden; the matched `\s*awf` tail and everything after it stay in the
   body untouched throughout, so on restore the entire remainder (the `{{=`, any interior
   whitespace, `awf`, and the rest of the token) renders verbatim minus the backslash — including the
   whitespace near-miss `\{{= awf:x}}`. The sentinel is created and fully restored within the
   function, never reaches `Assemble`, and a NUL byte cannot occur in markdown, so it cannot collide
   with real content — nor with `render.Assemble`'s own `\x00awf:part:` sentinel, which sees no NUL in
   this function's fully-restored output. The escape composes with the existing
   `strings.Contains(body, "{{=")` fast path: an escaped token still contains `{{=`, so the fast
   path never wrongly skips an escaped body.

3. **Single backslash only.** `\\{{=awf:key}}` (double backslash) is a documented edge: it yields a
   literal backslash followed by the literal token, not a backslash followed by the substituted
   value. There is no `\\`→`\` unescaping layer — that is deliberately out of scope for a rare case.
   Covered by a test and one sentence of doc.

4. **Registry values are token-free.** `placeholderRegistry` gains an `error` return (its sole
   caller is `planSections`). At build it hard-errors, naming the offending key, if any registry
   *value* matches the residual pattern `\{\{=\s*awf` — a clear, correctly-located build-time error
   instead of the confusing post-substitution residual error on awf-produced text.

**Unifying rationale.** Items 1–4 are one decision: they give the `{{=awf` token a way to appear as
inert literal *data* on both surfaces of the ADR-0057 mechanism — the authored raw-part surface
(escape, items 1–3) and the config-derived registry-value surface (token-free guard, item 4) — so
neither the substitution pass nor the residual guard ever fires on text that merely names the token.

**Relationship to ADR-0057 and ADR-0034.** This refines ADR-0057's raw-part surface only; ADR-0057
stays `Implemented`, recorded via `related` with no status flip. It further narrows the
byte-for-byte promise a second time — after ADR-0057 narrowed ADR-0034 item 1's "appears verbatim"
clause for the sentinel, the escape adds one more narrow exception (a lone `\{{=…awf` drops its
backslash). Neither `parts-raw` nor `part-placeholder-sandboxed` is retired (`retires_invariants:
[]`): the load-bearing "never run through `text/template`" clause stays true, and the escape is a
literal pre-pass, not a template action. ADR-0034 is not re-touched here — ADR-0057 already recorded
that relationship.

## Invariants

- `inv: escaped-placeholder-literal` — a `\{{=…awf` sequence in a convention part renders as the
  literal token (backslash consumed) and triggers neither substitution nor the residual-guard error.
  Backed by a substitution test.
- `inv: placeholder-value-token-free` — `placeholderRegistry` returns a hard error, naming the key,
  when any registry value contains a `{{=…awf` token. Backed by a registry test.

## Consequences

- Raw parts — adopter overrides and awf's own mechanism-describing narrative parts — can now
  document the `{{=awf:…}}` syntax by escaping it; the parts reworded during the ADR-0057 rollout can
  state the literal token directly.
- The value-token-free guard converts a confusing, mislocated failure into a precise one, and makes
  "a registry value never re-introduces the token" a checked contract rather than an accident.
- `placeholderRegistry`'s new `error` return is a one-line ripple at its single caller.
- New non-test branches the 100% gate (ADR-0012) must cover: escaped-token present and absent; the
  value-token-free check clean and offending.
- This narrows the raw-input contract a second time (after ADR-0057): a lone `\{{=…awf` in a part is
  no longer byte-for-byte — the backslash is consumed. This is the intended, documented cost of an
  escape; a part that genuinely wants a literal backslash before the token uses the double-backslash
  edge.

Doc-currency obligations the implementing commit(s) must satisfy (this ADR is the first of a
two-ADR effort with ADR-0059; concrete sequencing lands through the shared plan authored after both
settle):
- The two invariants' backing tests (`// invariant: escaped-placeholder-literal`,
  `// invariant: placeholder-value-token-free`) land in the same change that flips this ADR to
  `Implemented`; no existing Implemented invariant is retired (`retires_invariants: []`).
- The mechanism-describing narrative parts reworded during the ADR-0057 rollout
  (`.awf/docs/parts/architecture/data-flow.md`, `.awf/domains/parts/rendering/current-state.md`) may
  state the `{{=awf:…}}` token directly via the new escape; any such edit re-renders through
  `./x sync` in the same commit.
- `docs/architecture.md`'s render-flow note gains the pre-pass escape / NUL-sentinel step so the
  raw-part narrative stays accurate.
- The status flip to `Accepted`/`Implemented` regenerates `docs/decisions/ACTIVE.md` and the
  `rendering` domain index (this ADR carries `domains: [rendering]`, so ADR-0033's
  ADR→domain-index co-change and ADR-0019 staleness apply) via `./x sync`, staged in the same commit.
- No `docs/decisions/README.md` row is owed — the index is the generated `ACTIVE.md` (ADR-0005).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| No escape; reword any prose that mentions the token | The wall awf hit itself, and imposes it on every adopter documenting the feature; a namespaced, unambiguous token *can* be escaped robustly (unlike ADR-0034's generic `{{`). |
| A doubled-token escape (`{{=awf:awf}}` → literal `awf:`) or an HTML-comment escape | Less discoverable than a backslash and still cannot render an arbitrary literal token; the backslash is the conventional, obvious escape. |
| Let a value carry `{{=awf` and rely on the residual guard | The guard fires post-substitution on awf-produced output, pointing at the rendered body rather than the offending registry key — a worse error for the same failure. |
| Full `\\`→`\` unescaping | Adds a second escaping layer and its own edge cases for a case no real part needs; single-backslash with a documented double-backslash edge is sufficient. |
