---
status: Implemented
date: 2026-06-28
tags: [config-serialization]
related: [1, 9, 10, 22, 24]
domains: [config, tooling]
---
# ADR-0026: Config Serialization Owned by internal/config

## Context

`.awf/config.yaml` is *read* through one strict `yaml.v3` decoder (`config.Load`), but it is
*written* by hand in two other packages: `internal/project/scaffold.go` builds a fresh config with a
`strings.Builder` (`writeArray`), and `cmd/awf/list_add.go` mutates the enable arrays by splicing
text lines (`editArray`). `scaffold.go` even documents the rationale: "emit YAML manually ... to avoid
round-trip issues with the strict config.Load decoder"; and ADR-0024 chose the string editor
deliberately, rejecting "round-trip the YAML through a parser" on the grounds that it "loses the
scaffold's hand-authored formatting/comments."

Two facts overturn that reasoning:

1. **The rejection's premise is false for a node round-trip.** A `yaml.v3` *Node* round-trip (decode
   to `*yaml.Node`, edit one node, re-encode) preserves head/line comments and unrelated keys,
   verified empirically. The "lossy round-trip" ADR-0024 feared is the *struct* round-trip
   (`Unmarshal` into `Config`, `Marshal` back), not the node form.
2. **The string editor is actively corrupting this repo.** `editArray` hard-codes two-space
   indentation, but `internal/migrate` last wrote this project's own `.awf/config.yaml` at four-space
   indentation (sorted keys). Running `awf add skill <x>` here today emits a mixed-indent block that
   the strict decoder silently mangles into a single corrupt scalar. The line editor is safe only for
   configs it itself produced: a latent bug, not just inelegance.

So config serialization is split across three hand-rolled sites with no single owner of the format,
and the mutation path is fragile to any indentation it did not emit. The fix is to give `internal/config`
(which already owns the schema and the read path) ownership of construction and mutation too.

## Decision

1. **`internal/config` owns live config.yaml serialization.** Construction moves to
   `config.MarshalSkeleton(Skeleton)` (replacing `project`'s `writeArray`/`strings.Builder`); mutation
   moves to `config.SetArrayMember(src []byte, key, name string, add bool) ([]byte, error)` (replacing
   `cmd/awf`'s `editArray`). Both funnel through one private `encode` helper using a `yaml.v3` encoder
   with `SetIndent(2)`, so the on-disk format has exactly one definition. This **reverses ADR-0024
   Decision item 3** (`refines: ADR-0024#3`; the generic string-surgery array editor) and overturns ADR-0024's rejected
   "round-trip the YAML through a parser" alternative; ADR-0024 otherwise stands (its kind grammar,
   validation, warnings, and `list` surface are unchanged).

2. **Mutation is a structure-preserving node round-trip.** `SetArrayMember` decodes the source to a
   `*yaml.Node`, edits *only* the target key's sequence (appending or removing the scalar and
   normalizing that one sequence to block style) and re-encodes. Comments and every untouched key
   survive; a flow-style array (`skills: [a, b]`) is accepted and edited rather than refused (a
   deliberate change from `editArray`, which errored on flow style). Add of a present member and
   remove of an absent member are defined and tested; the caller's existing "already enabled" /
   "not enabled" guards are unchanged.

3. **Construction emits the canonical skeleton.** `Skeleton` carries `prefix`, `vars`, and the four
   catalog arrays (`skills`, `agents`, `hooks`, `docs`), and **no `domains` field**, so the scaffold
   omits a `domains:` key exactly as today. `Skeleton.Vars` is typed `map[string]string` so a `null`
   var value is *unrepresentable*: the scaffold seeds each var with an empty string, and an empty
   string marshals as `x: ""` while a nil interface would marshal as `x: null` and decode back to a
   nil value that renders as `<no value>`, tripping the publication-safe check (ADR-0001/ADR-0022).
   `MarshalSkeleton` output is byte-compatible with the prior `writeArray` format.

4. **`internal/migrate` is carved out.** The two `yaml.Marshal` sites in `migrate`
   (`treelayout.go`, `dropreplacewith.go`) keep their own untyped, forward-compatible marshalling;
   routing them through `config` would couple the ADR-0010-quarantined package (it imports only
   `manifest`) to `config`. "Single owner" governs the *live* read/write/mutate path, not the
   one-time schema-migration emitter, whose concern (marshalling an unknown forward map) is distinct.

5. **Re-home ADR-0024's `remove-block-scoped` backing.** Deleting `editArray` would orphan the
   `// invariant: remove-block-scoped` marker it carries (ADR-0024 is Implemented). The implementing
   commit moves that marker onto `SetArrayMember`'s block-scoped removal, keeping ADR-0024's invariant
   backed.

## Invariants

- `invariant: config-serialization-owned`: the live `.awf/config.yaml` is constructed and mutated only
  through `internal/config` (`MarshalSkeleton`, `SetArrayMember`), which share one `encode` funnel; no
  other package hand-rolls config.yaml serialization (`internal/migrate`'s forward-compat marshalling
  excepted per Decision 4).
- `invariant: config-mutation-roundtrip`: `SetArrayMember` edits config.yaml via a `yaml.Node` round-trip
  (not line/string surgery), preserving comments and unrelated formatting and accepting both block-
  and flow-style input arrays.

## Consequences

- One package owns the config format; hand-authored comments and any indentation survive an edit, and
  the live four-space-config corruption (Context 2) is fixed rather than worked around.
- `SetIndent(2)` reindents the *whole* document on the first mutation, so a config last written by
  `awf upgrade` (four-space) is rewritten to two-space block style on its first `awf add`/`remove`.
  This is a one-time, semantically-null change to an **input** file (`config.yaml` is not a rendered
  output, so `awf check` does not gate it), but it produces a visible diff that an adopter commits
  deliberately. `migrate` continues to emit four-space configs on upgrade (Decision 4), so the two
  styles coexist until the next mutation.
- Behaviour changes that the implementing commits absorb: flow-style arrays now edit instead of
  erroring, and added members append rather than prepend (both harmless: the arrays are consumed as
  sets). `editArray`'s tests (including its flow-style-refusal cases and the `// coverage-ignore` at
  its tail) are replaced by direct `SetArrayMember`/`MarshalSkeleton` unit tests carrying no
  coverage-ignore dodge.
- Invariant backing lands with the implementation: the ADR stays `Proposed` (its tagged slugs are
  unenforced until `Implemented`); the markers are added when `internal/config/edit.go` is written and
  the status flips to `Implemented` in the same series.

Doc-currency obligations the implementing commit(s) must satisfy:

- The `config` and `tooling` domain narratives gain the single-owner, structure-preserving serializer.
- `docs/architecture.md`'s `internal/config` package entry gains its construction/mutation role; today it records only the load path and schema ownership (via `.awf/docs/parts/architecture/components.md`).
- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` via `./x sync`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Struct round-trip (`Load` → mutate slice → `Marshal`) | Lossy: strips user comments and reorders fields/indent. The node round-trip preserves them; the struct form is exactly what ADR-0024 rightly feared. |
| Keep ADR-0024's string editor | Corrupts any config it did not itself emit (this repo's four-space config today) and refuses flow style; brittle hand-parsing with no single format owner. |
| Patch `editArray` to detect the file's existing indentation | Fixes only the four-space *symptom* while keeping hand-rolled YAML surgery: still three serialization sites with no single format owner, still cannot preserve comments through a structural edit, and still refuses (or risks corrupting) flow style. The node round-trip removes the fragility class, not one instance of it. |
| Widen ownership to `migrate`'s emitters too | Couples the ADR-0010-quarantined `migrate` package to `config`; its untyped forward-compat marshalling is a separate concern from the live path. |
