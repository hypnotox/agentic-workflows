# Plan: Documentation Authoring Standard (ADR-0018)

Implements [ADR-0018](../decisions/0018-documentation-authoring-standard.md). Design and rationale
live there; this plan is the execution record only.

## Goal

Ship two new awf-managed docs (`doc-standard.md`, `agents-md-standard.md`), add in-context
authoring hints to the agents-doc template, and dogfood the standard by compressing this repo's
own AGENTS.md Invariants. The final commit flips ADR-0018 `Accepted → Implemented`.

## Architecture summary

Pure ADR-0011 catalog-docs mechanism: no engine, catalog-schema, or lock-format change. Each new
doc is a `templates/catalog.yaml` entry + a `templates/docs/<name>.md.tmpl` (auto-embedded) + an
entry in `.awf/config.yaml` `docs:`. The docs auto-appear in the agent guide's document map via
`resolvedDocs()`; `docs_sections_test.go` auto-covers their section-parity and `<no value>` guard.
The two new doc templates are deliberately **var-free plain markdown** (no `{{ }}` tokens) so they
carry zero unresolved-token risk.

## Tech stack

- Go 1.26 (tool only; not touched). `awf sync`/`check` via `./x sync` / `./x check`.
- All changed artifacts are markdown + YAML config (catalog, `.awf/`). No `.go` changes. The
  pre-commit hook runs `./x check` then `./x gate` (full Go suite) on every commit; both must pass.

## File structure

**Created:**
- `templates/docs/doc-standard.md.tmpl`
- `templates/docs/agents-md-standard.md.tmpl`
- `docs/doc-standard.md` (rendered by `./x sync`)
- `docs/agents-md-standard.md` (rendered by `./x sync`)

**Modified:**
- `templates/catalog.yaml` (two doc entries)
- `.awf/config.yaml` (enable two docs)
- `templates/agents-doc/AGENTS.md.tmpl` (identity hint + two HTML-comment hints)
- `.awf/agents-doc.yaml` (compress `data.invariants`)
- `docs/decisions/0018-documentation-authoring-standard.md` (status flip, final phase)
- `AGENTS.md` (regenerated: document map in Phase 1; invariants-section hint in Phase 2; terser
  invariants in Phase 3: the `identity` and `you-and-this-project` hints are template-default
  only and are NOT rendered in this repo, since both sections are overridden by convention parts)
- `docs/decisions/ACTIVE.md`, `docs/domains/rendering.md`, `.awf/awf.lock` (regenerated)

**Deleted:** none.

> After each `./x sync`, run `git status` and stage exactly the source file(s) for that phase plus
> the regenerated artifacts. Use explicit `git add <paths>` (never `git add -A`).

---

## Phase 1: Ship the two standard docs

The two docs are coupled (`agents-md-standard` links `doc-standard`) and share one rationale, so
they land in one commit.

### Task 1.1: Add catalog entries

In `templates/catalog.yaml`, immediately after the `roadmap:` doc entry (the last entry under
`docs:`, before the `agentsDoc:` line), insert:

```yaml
  doc-standard:
    title: Documentation Standard
    desc: how-to-write rules for all awf-managed prose
    sections: [principles, rules, structure]
  agents-md-standard:
    title: Authoring AGENTS.md
    desc: layout, content, and rules for the agent guide
    sections: [layout, content, rules]
```

### Task 1.2: Create `templates/docs/doc-standard.md.tmpl`

Create the file with exactly this content:

```
# Documentation Standard

<!-- awf:section principles -->
## Principles

awf-managed docs orient and link; they do not restate. Each fact lives in the single most specific doc that owns it, and the agent guide's document map is the index that reaches it. A reader should find any fact one hop from the map.
<!-- awf:end -->

<!-- awf:section rules -->
## Rules

- **Terse.** Managed docs are read repeatedly; every word is a recurring cost. Use the shortest phrasing that stays precise.
- **Linter-rules out of prose.** State a tooling-enforced rule once and cite the ADR that owns it; do not narrate the mechanism. The agent meets the check regardless.
- **Reference, don't restate.** Link an ADR by id for rationale instead of reproducing it: one source of truth per fact.
- **No editorializing or dating.** Write the rule, not its history or a judgement of it.
- **Present-tense, authoritative voice.** Describe what is; use the imperative for instructions.
<!-- awf:end -->

<!-- awf:section structure -->
## Structure

A managed doc is a sequence of `awf:section` marker blocks whose names match its catalog `sections` list. Each section default holds a content prompt naming what belongs there. Override one section with a convention part at `.awf/docs/parts/<doc>/<section>.md`; the rest inherits the default. Documentation travels with the change that makes it true; update the doc in the same commit.
<!-- awf:end -->
```

### Task 1.3: Create `templates/docs/agents-md-standard.md.tmpl`

Create the file with exactly this content:

```
# Authoring AGENTS.md

The agent guide (`AGENTS.md`) is the one doc loaded every session. Follow the [Documentation Standard](doc-standard.md) for how to write; this doc adds what is specific to the guide.

<!-- awf:section layout -->
## Layout

The guide renders a fixed sequence of sections:

1. **Working with awf**: awf-given; how the config tree renders the project.
2. **You and this project**: adopter-authored; the agent's standing responsibility.
3. **Identity**: adopter-authored; what the project is.
4. **Invariants**: adopter-authored; the hard rules every change respects.
5. **Workflow**: awf-given; the canonical chain.
6. **Commands**: adopter data; the handful of commands an agent runs.
7. **Document map**: awf-given; the index into the docs, generated from the enabled set.

Override an adopter-authored section with a convention part at `.awf/parts/agents-doc/<section>.md`, or supply `data` (invariants, commands) in `.awf/agents-doc.yaml`.
<!-- awf:end -->

<!-- awf:section content -->
## Content

- **Identity**: one dense paragraph: what the project is, its stack and module path, its maturity, and who it serves. No history.
- **You and this project**: two or three sentences on the agent's ownership stance: the project's long-term health, not just the task.
- **Invariants**: the hard rules, one terse imperative line each, with the owning ADR in parentheses. The mechanism lives in that ADR.
<!-- awf:end -->

<!-- awf:section rules -->
## Rules

The guide is loaded every session, so it is held to an extra-terse bar; it pays its word cost on every task. State each invariant as a one-line imperative and let its ADR carry the detail. Push anything not every-session-critical into a doc and reach it through the document map.
<!-- awf:end -->
```

### Task 1.4: Enable both docs in this repo

In `.awf/config.yaml`, replace the `docs:` array:

```yaml
docs:
    - architecture
    - development
    - glossary
    - pitfalls
    - testing
    - workflow
```

with (insert the two new docs in alphabetical position):

```yaml
docs:
    - agents-md-standard
    - architecture
    - development
    - doc-standard
    - glossary
    - pitfalls
    - testing
    - workflow
```

### Task 1.5: Render, verify, commit

Run:

```
./x sync
./x check
go test ./internal/project/ ./internal/catalog/
```

Expected: `./x sync` completes without error; `./x check` exits 0, no drift; tests pass
(`docs_sections_test.go` now exercises both new docs for section-parity + `<no value>`).

Confirm the docs rendered and auto-listed in the document map:

```
ls docs/doc-standard.md docs/agents-md-standard.md
grep -n "Documentation Standard\|Authoring AGENTS.md" AGENTS.md
```

Expected: both files exist; both titles appear in AGENTS.md's Document map section.

Stage `templates/catalog.yaml`, both new `templates/docs/*.tmpl`, `.awf/config.yaml`, both rendered
`docs/*.md`, the regenerated `AGENTS.md`, and `.awf/awf.lock` (confirm the exact set with
`git status`), then:

```
git commit -m "docs(awf): add doc-standard and agents-md-standard docs"
```

---

## Phase 2: In-context authoring hints in the agents-doc template

### Task 2.1: Refine the `identity` default hint

In `templates/agents-doc/AGENTS.md.tmpl`, replace:

```
`{{ .prefix }}` is a software project. Override this paragraph with a convention part at `.awf/parts/agents-doc/identity.md`: one dense paragraph stating what `{{ .prefix }}` is, its stack and module path, its maturity, and who it serves.
```

with:

```
`{{ .prefix }}` is a software project. Replace this with a convention part at `.awf/parts/agents-doc/identity.md`: one dense paragraph: what `{{ .prefix }}` is, its stack and module path, its maturity, and who it serves. See `agents-md-standard.md`.
```

### Task 2.2: Add an HTML-comment hint to `you-and-this-project`

In `templates/agents-doc/AGENTS.md.tmpl`, replace:

```
<!-- awf:section you-and-this-project -->
## You and this project

You are a developer on `{{ .prefix }}`, responsible for its long-term health as well as the task in front of you. Bugs you notice in passing are yours; coverage gaps are yours; documentation drift is yours to fix in the same commit that caused it.
<!-- awf:end -->
```

with:

```
<!-- awf:section you-and-this-project -->
## You and this project

<!-- Authoring: see agents-md-standard.md: the agent's ownership stance in 2-3 sentences. -->
You are a developer on `{{ .prefix }}`, responsible for its long-term health as well as the task in front of you. Bugs you notice in passing are yours; coverage gaps are yours; documentation drift is yours to fix in the same commit that caused it.
<!-- awf:end -->
```

### Task 2.3: Add an HTML-comment hint to `invariants`

In `templates/agents-doc/AGENTS.md.tmpl`, replace:

```
<!-- awf:section invariants -->
## Invariants

Hard rules every change must respect:
```

with:

```
<!-- awf:section invariants -->
## Invariants

<!-- Authoring: see agents-md-standard.md: hard rules, one terse imperative line each, owning ADR in parens; mechanism lives in the ADR. -->
Hard rules every change must respect:
```

### Task 2.4: Render, verify, commit

Run:

```
./x sync
./x check
go test ./internal/project/
```

Expected: `./x sync` clean; `./x check` exits 0, no drift; tests pass (the golden agents-doc
assertions use `strings.Contains` + a no-leaks check that rejects only `awf:section`/`awf:end`/
`<no value>`/`{{}}`: the HTML comments are none of these).

Confirm the `invariants` hint renders (invisible in markdown, present as a comment):

```
grep -n "Authoring: see agents-md-standard.md" AGENTS.md
```

Expected: **one** matching line (the `invariants` hint). The `identity` and `you-and-this-project`
hints live in the template defaults but are not rendered in this repo: both sections are
overridden by convention parts under `.awf/parts/agents-doc/`, which replace the default body. The
hints still ship to adopters who have not overridden those sections.

Stage `templates/agents-doc/AGENTS.md.tmpl`, the regenerated `AGENTS.md`, and `.awf/awf.lock`, then:

```
git commit -m "docs(awf): add in-context AGENTS.md authoring hints"
```

---

## Phase 3: Dogfood the leanness pass and mark ADR-0018 Implemented

### Task 3.1: Compress this repo's `data.invariants`

In `.awf/agents-doc.yaml`, replace the entire `invariants:` block (the six entries under
`data.invariants`) with:

```yaml
    invariants:
        - ref: ADR-0001
          text: '**Publication-safe templates.** Wrap optional output in a conditional; no var leaks an unresolved-value token.'
        - text: '**`awf check` is the drift oracle.** After any `.awf/` edit run `./x sync && ./x check`; commit rendered files with their config and never hand-edit one.'
        - text: '**Conventional Commits, `awf` scope.** One concern per commit; stage explicitly, no `git add -A`.'
        - ref: ADR-0006
          text: '**Valid skill/agent frontmatter.** Rendered skills and agents carry parseable frontmatter with non-empty `name`/`description`.'
        - ref: ADR-0008
          text: '**Backed invariants.** Every `inv: <slug>` tag in an Implemented ADR is backed by a matching `<marker> invariant: <slug>` comment in source.'
        - ref: ADR-0012
          text: '**100% coverage gate.** `./x gate` fails below 100% statement coverage; exclude a genuinely-unreachable branch only with `// coverage-ignore: <reason>`.'
```

### Task 3.2: Flip ADR-0018 to Implemented

In `docs/decisions/0018-documentation-authoring-standard.md`, change the frontmatter:

```
status: Accepted
```

to:

```
status: Implemented
```

(ADR-0018 carries no tagged `inv:` slug, so the `Implemented` status adds no backing requirement;
`awf check` stays clean.)

### Task 3.3: Render, verify, commit

Run:

```
./x sync
./x check
./x gate
```

Expected: `./x sync` regenerates `AGENTS.md` (terser invariants), `docs/decisions/ACTIVE.md`
(0018 now under Implemented), and `docs/domains/rendering.md`; `./x check` exits 0; `./x gate`
passes.

Confirm the leanness landed and the ADR flipped:

```
grep -n "Implemented" docs/decisions/ACTIVE.md | grep 0018
awk '/^## Invariants/{f=1;next} f&&/^## /{f=0} f' AGENTS.md | wc -w
```

Expected: ADR-0018 listed under Implemented; the Invariants section **body** word count is ~160
(down from ~270).

Stage `.awf/agents-doc.yaml`, `docs/decisions/0018-documentation-authoring-standard.md`,
`docs/decisions/ACTIVE.md`, `docs/domains/rendering.md`, the regenerated `AGENTS.md`, and
`.awf/awf.lock`, then:

```
git commit -m "docs(awf): compress AGENTS.md invariants and mark 0018 Implemented"
```

---

## Verification (whole plan)

After all three phases:

```
./x check          # expect: exit 0, no drift
./x gate           # expect: pass (full Go suite + 100% coverage)
grep -c "Authoring: see agents-md-standard.md" AGENTS.md   # expect: 1 (only invariants; identity + you-and-this-project are part-overridden here)
```

Both new docs appear in the AGENTS.md document map; the Invariants section is terser; ADR-0018 is
Implemented.

## Terminal step

The ADR flip lands in Phase 3 (the final implementation commit), so no separate lifecycle commit is
needed. Invoke `awf-reviewing-impl` against the Phase 1-3 commit range.

## Notes

- ADR-driven plan: the ADR-0018 `Accepted → Implemented` flip is the final-commit action (Task 3.2).
- The two new doc templates are var-free, so they cannot trip the `<no value>` publication-safety
  guard; `docs_sections_test.go` enforces their section-parity automatically.
- Scope boundary (ADR-0018 Decision 5): this does **not** re-audit awf's six pre-existing docs to
  the new standard; that conformance sweep is separate, later work.
