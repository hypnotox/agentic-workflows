# Plan: Retrospective terminal step and finding-promotion ladder

**Date:** 2026-07-06
**ADR:** [ADR-0067](../decisions/0067-retrospective-terminal-step-and-finding-promotion-ladder.md) — Retrospective terminal step and finding-promotion ladder

## Goal

Add a Core `retrospective` skill as the new terminal node of the canonical workflow
chain (`… → reviewing-impl → retrospective`), run by the main thread, that promotes a
recurring, codifiable review finding or implementation pitfall up a four-rung ladder
(invariant → gate test/lint → code-reviewer project-focus item → pitfalls.md) toward a
deterministic check. Design rationale lives in ADR-0067; this plan is the execution record.

## Architecture summary

- A new catalog `SkillSpec` (`retrospective`, `Core: true`, no `RequiresAgent`/`RequiresDoc`)
  plus its `SKILL.md.tmpl`, enabled in awf's own `.awf/config.yaml`.
- `reviewing-impl` gains a `hand-off` section naming `<prefix>-retrospective` unconditionally
  (safe because both are Core, mirroring its existing `executing-plans` reference).
- The `internal/evals` chain graph gains the tenth node: add `retrospective` to `chainNodes`,
  flip the `chainTerminal` constant to `retrospective`, and pin the `reviewing-impl →
  retrospective` handoff edge. Flipping `chainTerminal` is mandatory — `TestChainConnectivity`
  exempts only that constant from the outgoing-edge requirement, so a `retrospective` added to
  `chainNodes` without moving it is flagged orphaned (ADR-0067 Decision 2).
- Docs travel: the canonical chain in `AGENTS.md` and `docs/workflow.md`, and a rung-4 framing
  in the `pitfalls` doc default.
- No new source-level `inv:` slug — chain-node integrity rides the existing ADR-0053
  (`inv: evals-full-catalog-coverage`, catalog-derived fixture) and ADR-0054 chain-graph
  assertions extended to the new node.

## Tech stack

- Go 1.26. Packages touched: `internal/catalog` (struct literal), `internal/evals`
  (`_test.go` only), `internal/project` (`_test.go` only).
- Templates: `templates/skills/`, `templates/agents-doc/`, `templates/docs/`.
- Config: `.awf/config.yaml`. Runner: `./x` (`sync`, `check`, `gate`).

## File structure

**Created**
- `templates/skills/retrospective/SKILL.md.tmpl`
- `.claude/skills/awf-retrospective/SKILL.md`, `.cursor/skills/awf-retrospective/SKILL.md` (rendered by `./x sync`)

**Modified**
- `internal/catalog/standard.go` (add `retrospective`; add `hand-off` to `reviewing-impl` sections)
- `.awf/config.yaml` (enable `retrospective`)
- `internal/project/spine_test.go` (add `TestRetrospectiveTemplate`; assert `example-retrospective` in reviewing-impl & agents-doc tests)
- `internal/evals/chain_test.go` (`chainNodes`, `chainTerminal`, `TestWorkflowChainHandoffs`)
- `templates/skills/reviewing-impl/SKILL.md.tmpl` (hand-off section + terminal wording)
- `templates/agents-doc/AGENTS.md.tmpl`, `templates/docs/workflow.md.tmpl`, `templates/docs/pitfalls.md.tmpl`
- Rendered outputs of the above + `docs/decisions/ACTIVE.md`, `docs/domains/rendering.md`, `docs/domains/tooling.md`, `.awf/awf.lock` (via `./x sync`)
- `docs/decisions/0067-...md` (status flip in the final commit)

---

## Phase 1 — Add the `retrospective` skill (self-contained, unwired)

This phase adds the skill so it renders and is enabled, but does not yet wire it into the
chain. It is green on its own: `retrospective` is not yet a `chainNode`, so no chain assertion
touches it, and nothing references it yet.

### Task 1.1 — Create the skill template

Create `templates/skills/retrospective/SKILL.md.tmpl` with exactly this content:

```
---
name: {{ .prefix }}-retrospective
description: Terminal step of an implementation, run by the main thread. Reflects on the work, records worthy pitfalls, and promotes recurring findings toward deterministic checks.
---

# {{ .prefix }}-retrospective

The closing step of the implementation phase, run by the **main thread** — not a dispatched subagent — because it depends on the full session: the review findings, the friction hit while implementing, and which issues recurred. It closes the feedback loop by turning a recurring, codifiable finding into a durable check instead of letting it resurface every session.

<!-- awf:section when-fires -->
## When this skill fires

Terminal step of the implementation phase, after {{ if index .skills "reviewing-impl" }}`{{ .prefix }}-reviewing-impl`{{ else }}the project's review step{{ end }} has concluded. It runs in the main thread and sees the whole session, so it catches what a fresh-context review cannot.

**Skip a trivial session** — a one-line or mechanical change with nothing worth recording and no recurring issue to promote. **Run even on a docs-only session**: a doc or process pitfall is still worth capturing, and the review step skips those.
<!-- awf:end -->

<!-- awf:section procedure -->
## Procedure

1. **Reflect on the session.** Gather its signals: the {{ if index .skills "reviewing-impl" }}`{{ .prefix }}-reviewing-impl`{{ else }}review{{ end }} findings, the pitfalls or friction hit while implementing, and any issue that came up more than once.

2. **Record the worthy observations.** A first-occurrence pitfall or tricky area is *recorded* (rung 3 or 4 below), not promoted — the record is the memory the next retrospective reads to detect recurrence.

3. **Promote recurring, codifiable observations** to the strongest rung each can support (see the ladder). Verify a candidate genuinely recurs and is worth the effort before promoting.

4. **Note where each landed** in the session summary, so the loop is visible.
<!-- awf:end -->

<!-- awf:section recurrence-signal -->
## Recurrence signal

An observation is a **promotion candidate** when the main thread saw it recur within this session, or when it matches something already recorded — {{ if .layout.docs.pitfalls }}`{{ .layout.docs.pitfalls }}`{{ else }}the project's pitfalls notes{{ end }} or the code-review agent's project-focus list — and *still happened*: prose memory recorded it and did not prevent it, which is the signal to climb to a deterministic rung. A genuine one-off is recorded, never promoted.
<!-- awf:end -->

<!-- awf:section promotion-ladder -->
## The promotion ladder

Route each recurring, codifiable observation to the **strongest** rung it can support:

1. **Invariant** — a load-bearing rule the project must remember. {{ if index .skills "proposing-adr" }}Record the decision via `{{ .prefix }}-proposing-adr`{{ else }}Record the decision through the project's decision process{{ end }}, give it an `inv:` slug, and back it with a `<marker> invariant: <slug>` comment or test{{ with .vars.invariantTestPath }} (backing tests live under `{{ . }}`){{ end }}. The marker and globs are config-driven, so this works in any language.
2. **Gate test or lint rule** — an ordinary mechanically-checkable rule that does not rise to an invariant. Add a test or linter rule so {{ with .vars.gateCmd }}`{{ . }}`{{ else }}the gate{{ end }} catches it; no decision record needed.
3. **Code-review focus item** — a rule that needs per-case judgment. Add a persistent project-focus item to the code-review agent's checklist: still probabilistic, but now applied on every review.
4. **Pitfalls note** — tricky knowledge that is not mechanically checkable.{{ if .layout.docs.pitfalls }} Add an entry to `{{ .layout.docs.pitfalls }}`.{{ else }} Record it in the project's pitfalls notes.{{ end }}
<!-- awf:end -->

<!-- awf:section control -->
## Control

The main thread controls the promotion effort — never an unverified auto-promotion, never delegated to a subagent:

- **Codify now** when the rung is cheap: a gate test, a focus item, a pitfalls note.
- **Defer as a follow-up** when the rung is expensive. A rung-1 (invariant) promotion is almost always deferred, because it spins the full decision-to-implementation chain; record it as an explicit follow-up rather than derailing the current session.
<!-- awf:end -->

<!-- awf:section notes -->
## Notes

- Reflection is cheap; promotion is deliberate. Most sessions record a thing or two and promote nothing — that is the expected steady state.
- The aim is to move a recurring issue *down* into a deterministic check, not to accumulate more prose the next agent must remember.{{ with .layout.workflowRef }} The canonical chain lives in `{{ . }}`.{{ end }}
<!-- awf:end -->
```

- [ ] Create the file with the exact content above.

### Task 1.2 — Register the skill in the catalog

In `internal/catalog/standard.go`, immediately after the `reviewing-impl` entry (the block
ending `}` on the line before `"refactor-coupling-audit": {`), add:

```go
		"retrospective": {Core: true, Sections: []string{
			"when-fires", "procedure", "recurrence-signal", "promotion-ladder", "control", "notes",
		}},
```

- [ ] Add the `retrospective` `SkillSpec`. The `Sections` set must exactly equal the template's
  `awf:section` markers (enforced by `inv: skill-section-parity`).

### Task 1.3 — Enable the skill in awf's own config

In `.awf/config.yaml`, insert `retrospective` into the `skills:` list between
`refactor-coupling-audit` and `reviewing-adr` (keeping the list alphabetical):

```yaml
  - refactor-coupling-audit
  - retrospective
  - reviewing-adr
```

- [ ] Add the `- retrospective` line.

### Task 1.4 — Add the per-skill template test

In `internal/project/spine_test.go`, immediately after `TestReviewingImplTemplate` (the `}` on
its last line, before `func TestRefactorCouplingAuditTemplate`), add:

```go
func TestRetrospectiveTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"skills": map[string]bool{"reviewing-impl": true, "proposing-adr": true},
		"vars": map[string]any{
			"gateCmd":           "./x gate",
			"invariantTestPath": "./internal/...",
		},
		"layout": map[string]any{
			"docs":        map[string]any{"pitfalls": "docs/pitfalls.md"},
			"workflowRef": "docs/workflow.md",
		},
		"data": map[string]any{},
	}

	out := renderSkillGolden(t, "retrospective", data)

	if !strings.Contains(out, "name: example-retrospective") {
		t.Errorf("expected 'name: example-retrospective' in output:\n%s", out)
	}

	// Load-bearing phrases unique to the retrospective ladder (ADR-0067).
	loadBearing := []string{
		"main thread",
		"promotion ladder",
		"Invariant",
		"example-proposing-adr",
		"docs/pitfalls.md",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}
```

- [ ] Add `TestRetrospectiveTemplate`.

### Task 1.5 — Render, verify, commit

- [ ] Run `./x sync` (renders the new skill into `.claude/` and `.cursor/`, updates the lock).
- [ ] Run `./x check` — expect `awf check: clean`.
- [ ] Run `./x gate` — expect the tail to end with `coverage: 100.0% (...)`, `0 issues.`,
  `deadcodecheck: no production dead code`. `TestRetrospectiveTemplate` and the `internal/evals`
  suite pass (retrospective is not yet a `chainNode`, so no chain assertion references it).
- [ ] Stage explicitly and commit:

```
git add templates/skills/retrospective/SKILL.md.tmpl internal/catalog/standard.go .awf/config.yaml internal/project/spine_test.go .claude/skills/awf-retrospective/SKILL.md .cursor/skills/awf-retrospective/SKILL.md .awf/awf.lock
git commit -m "feat(rendering): add retrospective skill (unwired)

Adds the Core retrospective SkillSpec, its SKILL.md template, and per-skill
test; enables it in awf's own config. Not yet wired into the chain — that is
the next commit. Implements part of ADR-0067.

Claude-Session: https://claude.ai/code/session_01CLtZiZoxUaLuo5cTGfxS4H"
```

Subject is 45 chars (< 72). Scope `rendering` (templates/catalog render surface).

---

## Phase 2 — Wire retrospective as the terminal chain node, update docs, flip ADR

This phase makes `retrospective` the terminal node and lands all doc-currency + the ADR status
flip in one cohesive commit (docs travel with the behaviour change; the ADR flips in the final
implementation commit).

### Task 2.1 — Give `reviewing-impl` a hand-off to the retrospective

In `internal/catalog/standard.go`, update the `reviewing-impl` `Sections` to add `"hand-off"`
before `"notes"`:

```go
		"reviewing-impl": {Core: true, RequiresAgent: "code-reviewer", Sections: []string{
			"when-fires", "sha-range-detection", "docs-only-check", "dispatch-subagent",
			"classify-route-findings", "apply-fixes-commit", "run-audit", "re-review-loop", "hand-off", "notes",
		}},
```

In `templates/skills/reviewing-impl/SKILL.md.tmpl`:

1. In the intro paragraph (line 8), change `Invoked as the terminal step of the implementation
   phase.` to `Invoked as the independent review step of the implementation phase.`
2. After the `re-review-loop` section's `<!-- awf:end -->` (line 63) and before `## Notes`,
   insert:

```
<!-- awf:section hand-off -->
8. **Invoke `{{ .prefix }}-retrospective` as the terminal step.** After the review settles, hand off to the main-thread retrospective, which reflects on the session and promotes any recurring, codifiable finding toward a deterministic check.
<!-- awf:end -->

```

3. In the `notes` section, change `This is the terminal step of the implementation phase: a
   single, independent` to `This is the independent review step of the implementation phase: a
   single, independent`.
4. In the frontmatter `description` (line 3), change `Terminal step after implementation
   commits.` to `Independent review step after implementation commits.` — `reviewing-impl` is no
   longer the chain's terminal step (retrospective is), so its own self-description must not
   claim otherwise.

- [ ] Apply the four template edits and the catalog `Sections` edit. The reference to
  `{{ .prefix }}-retrospective` is unconditional, mirroring the existing unconditional
  `{{ .prefix }}-executing-plans` reference (both Core).

### Task 2.2 — Extend the eval chain graph

In `internal/evals/chain_test.go`:

1. Add `"retrospective"` to `chainNodes`:

```go
var chainNodes = []string{
	"brainstorming", "proposing-adr", "reviewing-adr", "writing-plans",
	"reviewing-plan", "reviewing-plan-resync", "executing-plans",
	"subagent-driven-development", "reviewing-impl", "retrospective",
}
```

2. Flip the terminal constant:

```go
const (
	chainRoot     = "brainstorming"
	chainTerminal = "retrospective"
)
```

3. In `TestWorkflowChainHandoffs`, add the new marquee edge to the `tc` slice (after the
   `{"bugfix", "reviewing-impl"},` line):

```go
		{"reviewing-impl", "retrospective"},
```

- [ ] Apply the three edits. After this, `TestChainConnectivity` requires `reviewing-impl` to
  emit an outgoing edge (satisfied by Task 2.1's hand-off) and treats `retrospective` as the
  terminal node.

### Task 2.3 — Assert the new step renders in the reviewing-impl and agents-doc tests

In `internal/project/spine_test.go`:

1. In `TestReviewingImplTemplate`, add `"example-retrospective"` to the `loadBearing` slice:

```go
	loadBearing := []string{
		"code-reviewer",
		"user-decision",
		"SHA range",
		"docs/decisions/",
		"example-retrospective",
	}
```

2. In `TestAgentsDocTemplate`, add `"example-retrospective"` to the asserted-phrases slice
   (after the `"example-reviewing-impl",` line):

```go
		"example-retrospective",
```

- [ ] Apply both assertion additions.

### Task 2.4 — Update the canonical chain in the guide and workflow doc

In `templates/agents-doc/AGENTS.md.tmpl` (`workflow` section):

1. Change the fenced chain line to end with `→ retrospective`:

```
brainstorming → ADR (if warranted) → plan (if warranted) → resync (when both) → implementation → review → retrospective
```

2. Change `` `{{ .prefix }}-reviewing-impl` is the terminal review.`` to:

```
`{{ .prefix }}-reviewing-impl` is the terminal review, after which `{{ .prefix }}-retrospective` closes the feedback loop by promoting recurring findings toward deterministic checks.
```

3. In the **Chain skills** list, insert `` `{{ .prefix }}-retrospective` `` after
   `` `{{ .prefix }}-reviewing-impl` ``, so the list reads
   ``…, `{{ .prefix }}-reviewing-impl`, `{{ .prefix }}-retrospective`.{{ $tasks := "" }}…``

In `templates/docs/workflow.md.tmpl` (`chain` section):

4. Change the fenced chain line to end with `→ retrospective` (same as above).
5. Change `Implementation review is the terminal gate.` to:

```
Implementation review is the terminal gate; a main-thread **retrospective** then closes the feedback loop, promoting any recurring, codifiable finding toward a deterministic check.
```

- [ ] Apply the five doc-template edits. The two `strings.Contains` invariant checks in
  `TestAgentsDocTemplate` (`ADR (if warranted) → plan (if warranted)`, `resync (when both)`)
  remain satisfied — only the tail of the chain line changes.

### Task 2.5 — Frame the pitfalls doc as rung 4 (standard default)

In `templates/docs/pitfalls.md.tmpl`, append to the `entries` section intro (line 4), so it
reads:

```
Recurring bugs and tricky areas worth a warning before you touch them. _One subsection per pitfall: the symptom, the underlying cause, and how to avoid or fix it._ A pitfall that keeps recurring is a promotion candidate: prefer hardening it into a deterministic check (an invariant or a gate test) over letting it live here as a standing warning.
```

- [ ] Apply the edit. This changes the standard default only; awf's own `docs/pitfalls.md`
  overrides the `entries` section via `.awf/docs/parts/pitfalls/entries.md`, so awf's rendered
  doc is unaffected (no drift).

### Task 2.6 — Flip ADR-0067 to Implemented

In `docs/decisions/0067-retrospective-terminal-step-and-finding-promotion-ladder.md`, change
the frontmatter `status: Proposed` to `status: Implemented`. (No `inv:` slug exists, so the
invariant-backing check has nothing new to enforce.)

- [ ] Apply the status flip.

### Task 2.7 — Render, verify, commit

- [ ] Run `./x sync` (re-renders reviewing-impl, AGENTS.md, workflow.md, the pitfalls standard
  default, and regenerates `ACTIVE.md` + the domain docs for the ADR flip).
- [ ] Run `./x check` — expect `awf check: clean`. (awf's `docs/pitfalls.md` shows no change:
  it uses the override part.)
- [ ] Run `./x gate` — expect `coverage: 100.0% (...)`, `0 issues.`,
  `deadcodecheck: no production dead code`. The `internal/evals` chain tests now include
  `retrospective`: `TestChainConnectivity` sees `reviewing-impl → retrospective` and treats
  `retrospective` as terminal; `TestWorkflowChainHandoffs/reviewing-impl_to_retrospective`
  passes.
- [ ] Stage explicitly and commit:

```
git add internal/catalog/standard.go templates/skills/reviewing-impl/SKILL.md.tmpl internal/evals/chain_test.go internal/project/spine_test.go templates/agents-doc/AGENTS.md.tmpl templates/docs/workflow.md.tmpl templates/docs/pitfalls.md.tmpl docs/decisions/0067-retrospective-terminal-step-and-finding-promotion-ladder.md .claude/skills/awf-reviewing-impl/SKILL.md .cursor/skills/awf-reviewing-impl/SKILL.md AGENTS.md docs/workflow.md docs/decisions/ACTIVE.md docs/domains/rendering.md docs/domains/tooling.md .awf/awf.lock
git commit -m "feat(rendering): make retrospective the terminal chain node

Wires reviewing-impl -> retrospective, adds the tenth eval chain node and
flips chainTerminal, updates the canonical chain in AGENTS.md and workflow.md,
frames the pitfalls doc as rung 4, and flips ADR-0067 to Implemented. Closes
the feedback-promotion loop per ADR-0067.

Claude-Session: https://claude.ai/code/session_01CLtZiZoxUaLuo5cTGfxS4H"
```

Subject is 52 chars (< 72). Scope `rendering`.

---

## Verification (whole plan)

- [ ] `./x check` → `awf check: clean`.
- [ ] `./x gate` → 100% coverage, 0 lint issues, no dead code, all `internal/evals` chain tests green.
- [ ] `git log --oneline -2` shows the two feature commits above `docs(adr): note ADR-0067 …`.
- [ ] The rendered `.claude/skills/awf-retrospective/SKILL.md` exists and the rendered `AGENTS.md`
  chain ends with `→ retrospective`.

## Notes

- The `reviewing-impl → retrospective` reference is unconditional by design (ADR-0067 Decision 2);
  this matches the existing unconditional Core-sibling references in `reviewing-impl`.
- No new `inv:` slug is introduced; chain-node integrity is covered by the extended ADR-0053/0054
  eval machinery (ADR-0067 Decision 9).
- After this plan lands, the first real `awf-retrospective` run is available as the terminal step
  of any subsequent implementation.
