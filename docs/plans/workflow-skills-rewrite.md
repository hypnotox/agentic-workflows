---
type: Implementation Plan
title: Rewrite the workflow skills
description: Route for shorter, clearly triggered workflow skills that rely on agentic-skills for method, with a one-off observation trial.
---

# Plan: Rewrite the workflow skills

## Basis

Implements items 1–8, 10, and 11 of the [workflow streamlining intent](../changes/workflow-streamlining/intent.md). Items 9 and 12 (adoption surface, CLI changes, this repository's own fixes) get a second plan once this one has landed, because the trial may change them.

Settled here, beyond the intent:

- `awf docs <page>` and the skill bodies stay one canonical text per workflow. The existing identity test keeps guarding that.
- Method references use bare `agentic-skills` names. Before relying on a name, confirm in the `agentic-skills` package which skill owns that method: brainstorming and premise check, code design, planning, independent review and its delegation and fallback rules, implementation, retrospective, and subagent settings.
- AWF owns when a checkpoint occurs in a repository's lifecycle; `agentic-skills` owns how it is done. Where an `agentic-skills` method carries its own threshold, AWF's cues are the repository's specialization of it, which that package permits. Changing `agentic-skills` is out of scope: report a missing or mismatched method to the owner instead of working around it silently.
- Detail removed from a skill body must not be lost when an agent needs it to use a command. Move it to that command's help text in `cmd/awf` with its tests in the same change; drop it only when it changes no behavior. Making error messages name a next step consistently belongs to the second plan.
- Size is a guide, not a contract: roughly half the current words per skill, and no test pins a count.

## Changes

### 1. Trial harness and baseline

Add a small harness under `tools/` that prepares a temporary fixture repository with AWF skills rendered from a chosen build, runs a headless agent on a few fixed task prompts, and saves the transcripts. It runs only when invoked explicitly: not from `./x gate`, `./x test`, or CI, and it adds nothing the gate compiles. Document how to run it and what it does not establish.

The harness drives one headless agent CLI, which needs authentication and network access. Baseline and rerun use the same CLI version, model, and installed `agentic-skills`, so the comparison isolates the rewrite. Transcripts are written outside the repository and are not committed. The agent runs without blanket permission bypass.

Choose three or four tasks that exercise the intended behavior: a small routine fix; a change with a material design choice; multi-stage work that should use an effort; work that ends with integration. Record the baseline against the current skills before changing them.

### 2. Drop the Pi output

Remove `.pi/skills` from the fixed output set and its tests. Delete this repository's retired `.pi/skills` copies. Swap test fixtures that use a Pi path as their sample generated file. Update the projection and efforts topics, the overview and integration guides, the README's output list, and the changelog, including the note that Pi's own project-skill discovery was not observed.

Adopters' existing `.pi/skills` files keep their ownership marker and will fail `check` as unmanaged until removed. AWF does not delete them: the changelog and the integration guide give the manual removal step.

### 3. Rewrite descriptions and bodies

Rewrite the five skill descriptions in their templates to state what the workflow does and when to use it, with the trigger conditions first.

Rewrite the five guides in `internal/docs/`:

- Each opens with when it applies, then states required steps, defaults, and options distinguishably.
- Checkpoints name the `agentic-skills` method instead of restating delegation, briefing, reuse, and fallback rules.
- Review checkpoints trigger on the positive cues in intent item 10. The retrospective follows item 11.
- The three steering points of item 2 appear where they occur: direction in the changes workflow, deviation and integration or publication in the completion workflow.
- The effort workflow lets an effort choose and record whether it uses a worktree (item 4).
- Commit cadence stays in the completion workflow as an overridable default that excludes pushing and merging (item 5).
- Cross-references use one form throughout.

Update text elsewhere that describes what the guides contain, such as the efforts topic.

Depends on change 1's baseline being recorded.

### 4. Requirement and routing wording

State the `agentic-skills` requirement in the README and reword the "usable without external skills" constraint there and in `AGENTS.md`; the CLI stays usable offline. Reduce the routing cues in the `docs agents` example and in this repository's `AGENTS.md` to one stable sentence, since descriptions now carry routing. Update every statement of that constraint in the overview and integration guides, including the integration guide's paragraph on external skills.

Depends on change 3, which defines what the descriptions carry.

### 5. Run the trial and reconcile

Run the same tasks against the rewritten skills and compare transcripts with the baseline: which skills loaded and when, whether topics were resolved and read, whether the direction came back to the user before planning, whether review and retrospective matched the new triggers, and whether integration waited for confirmation. Treat results as directional. Fix guidance at its source where the trial shows a misreading, as its own commit, and report what the trial did and did not show. The owner decides whether a trial result blocks integration.

## Trial result

Run on 2026-11-15 with one headless agent, one model, and one run per task; results are directional and live outside the repository.

- With the previous guidance, the agent chose a design itself and reported afterwards, and merged into the default branch unasked.
- With the rewritten guidance, after one wording fix found by the trial, the agent brought unstated material choices to the user before building and waited for confirmation before merging. The multi-stage task loaded native skills and obtained an independent review of a new output contract.
- Limits of that evidence: the wording fix names examples that match two of the trial tasks, and the rerun used the same tasks, so its success is not independent of the fix. The first integration prompt authorized the merge and had to be reworded and its baseline rerun.
- Not improved: native skills loaded only in the multi-stage task; the other tasks read `awf docs <page>` or no workflow, and the routine fix was not committed under either guidance. Whether descriptions alone route reliably, the condition for revisiting intent item 6, is therefore not established and is the owner's decision.

## Verification

- `./x gate` and `./x render && ./x check` pass after each change.
- An independent instruction review of the rewritten guides against the intent, the doctrine, and the `agentic-skills` methods they reference.
- The trial comparison from change 5.

## Integration

Changes 2, 3, and 4 each land as one commit with their tests and documentation; the harness is its own commit. This is a breaking change for adopters (Pi output removed, external skills required): the changelog says so. Merging into `main`, pushing, and releasing wait for the owner's confirmation.
