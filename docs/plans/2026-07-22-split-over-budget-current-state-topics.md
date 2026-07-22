---
date: 2026-07-22
adrs: [148]
status: Proposed
---
# Plan: Split over-budget current-state topics

## Goal

Apply ADR-0148: move 157 claims out of the five over-budget topics into the 15
already-landed destination topics via five V2 application batches, one staged transaction
each, until `awf check` prints no topic-claim-budget advisory. Non-goals: selector
narrowing, in-part claim grouping, and the advisory severity promotion (all deferred per
ADR-0148 Decision items 4 and 6).

## Architecture summary

One acceptance commit freezes the ADR body, then five batch commits apply the frozen
operations smallest-first (catalog-and-targets, configuration, cli, templates,
project-output-plan). Each batch commit is one staged transaction: the batch's claim
blocks move from the source part to their destination parts (Origin rewritten to
ADR-0148, `Revised-by:` dropped), every marker site and test fixture naming a moved id is
rewritten to the new id, the ADR appends its status/Applied events, and `./x sync`
regenerates rendered docs. `awf check --staged` is the transition oracle for every
commit. The Implementing status event travels with batch 1; the Implemented event, the
ADR status flip, and this plan's freeze land with batch 5.

## File structure

- **Created:** none (the 15 destination shells landed with the ADR proposal commit).
- **Modified:** `docs/decisions/0148-split-over-budget-current-state-topics-into-area-scoped-topics.md`
  (frontmatter status + Status history events); the five source parts under
  `.awf/topics/parts/{rendering/{project-output-plan,templates,catalog-and-targets},tooling/cli,config/configuration}/current-state.md`;
  the 15 destination parts under `.awf/topics/parts/`; marker-site and fixture files under
  `internal/**`, `cmd/**`, `templates/**`, and `.pi/**` that name moved claim ids;
  regenerated `docs/topics/**`, `docs/domains/**`, `docs/decisions/INDEX.md`,
  `.awf/awf.lock`; this plan (status flip in the final commit).
- **Deleted:** none.

## Shared batch mechanics

Every batch phase below uses the same two scripts, exact content here. Both read the
batch's operation pairs straight from the ADR's State changes section, so the ADR stays
the single source of truth. Save both under the session scratchpad (any path works; the
plan refers to them as `move_claims.py` and `ops_line.py`); they are throwaway tooling and
are never committed.

`move_claims.py` (run as `python3 move_claims.py <source-topic-id>`, e.g.
`python3 move_claims.py rendering/catalog-and-targets`):

```python
import re, sys

ADR = "docs/decisions/0148-split-over-budget-current-state-topics-into-area-scoped-topics.md"
src = sys.argv[1]

text = open(ADR).read()
ops = text.split("## State changes\n", 1)[1].split("## Consequences", 1)[0]
pairs = []  # (old_topic, new_topic, slug)
lines = [l for l in ops.splitlines() if l.startswith("- ")]
for i, l in enumerate(lines):
    m = re.match(r"- remove `([a-z-]+/[a-z-]+):([a-z0-9-]+)`$", l)
    if not m or m.group(1) != src:
        continue
    m2 = re.match(r"- add `([a-z-]+/[a-z-]+):([a-z0-9-]+)`$", lines[i + 1])
    assert m2 and m2.group(2) == m.group(2), l
    pairs.append((m.group(1), m2.group(1), m.group(2)))
assert pairs, "no operations found for " + src

def part(topic):
    d, t = topic.split("/")
    return f".awf/topics/parts/{d}/{t}/current-state.md"

src_text = open(part(src)).read()
for old_topic, new_topic, slug in pairs:
    # Extract the claim block: heading line through the line before the next
    # heading (or end of file), trailing blank lines trimmed.
    pat = re.compile(
        r"### `(?:rule|invariant): " + re.escape(slug) + r"`\n.*?(?=\n### |\Z)",
        re.S,
    )
    m = pat.search(src_text)
    assert m, slug
    block = m.group(0).rstrip("\n")
    src_text = src_text[: m.start()].rstrip("\n") + "\n\n" + src_text[m.end() :].lstrip("\n")
    block = re.sub(r"(?m)^Origin: ADR-\d+$", "Origin: ADR-0148", block, count=1)
    block = re.sub(r"(?m)^Revised-by: .*\n", "", block)
    dst = part(new_topic)
    open(dst, "a").write("\n" + block + "\n")
open(part(src), "w").write(src_text.rstrip("\n") + "\n")
print(f"moved {len(pairs)} claims out of {src}")
```

`ops_line.py` (run as `python3 ops_line.py <source-topic-id>`; prints the exact
`operations:` payload for the batch's Applied event, in declaration order):

```python
import re, sys

ADR = "docs/decisions/0148-split-over-budget-current-state-topics-into-area-scoped-topics.md"
src = sys.argv[1]
text = open(ADR).read()
ops = text.split("## State changes\n", 1)[1].split("## Consequences", 1)[0]
lines = [l for l in ops.splitlines() if l.startswith("- ")]
out = []
for i, l in enumerate(lines):
    if re.match(r"- remove `" + re.escape(src) + r":", l):
        out.append(l[2:])
        out.append(lines[i + 1][2:])
print(", ".join(out))
```

Marker and fixture rewrite, per batch (bash; substitute the source topic id): for every
moved pair the old id is replaced by the new id across tracked source files, excluding the
frozen decision history, the plans directory, and the `.awf` tree (the parts were already
rewritten by `move_claims.py`, and rendered `docs/` output regenerates via sync):

```bash
python3 ops_line.py <source-topic-id> | tr ',' '\n' | sed -n 's/^ *remove `\(.*\)`$/\1/p' > /tmp/old_ids.txt
python3 ops_line.py <source-topic-id> | tr ',' '\n' | sed -n 's/^ *add `\(.*\)`$/\1/p' > /tmp/new_ids.txt
paste -d'|' /tmp/old_ids.txt /tmp/new_ids.txt | while IFS='|' read -r old new; do
  git grep -l -F "$old" -- ':!docs/decisions' ':!docs/plans' ':!docs/topics' ':!docs/domains' ':!.awf' \
    | xargs -r sed -i "s|$old|$new|g"
done
```

Post-check for the rewrite (zero output required):

```bash
cat /tmp/old_ids.txt | while read -r old; do
  git grep -F "$old" -- ':!docs/decisions' ':!docs/plans' ':!docs/topics' ':!docs/domains' ':!.awf/memory' || true
done
```

Each batch's closing steps are identical: `./x sync`, stage the complete transaction
(`git add -- .awf docs internal cmd templates .pi` plus any other rewritten paths the
post-check surfaced), `./x check --staged` clean, `./x gate` exit 0, commit. The Applied
event's `state-sequence` is the next contiguous global sequence; `awf check` reports the
expected value on mismatch and is the authority (at authoring time the next five are
21-25, indicative only).

## Phase 1: Accept ADR-0148

- [ ] **Task 1.1: Append the Accepted event.** In
  `docs/decisions/0148-split-over-budget-current-state-topics-into-area-scoped-topics.md`,
  set frontmatter `status: Accepted` and append to `## Status history`:

  ```
  - 2026-07-22: Accepted; content-sha256: 0000000000000000000000000000000000000000000000000000000000000000
  ```

  Run `./x check`; it fails naming the computed digest ("does not match the computed
  digest"); replace the 64-zero placeholder with the reported digest; re-run `./x check`
  until the digest error is gone. Use the real date if it is no longer 2026-07-22.
- [ ] **Task 1.2: Capture the uncovered baseline.** Run
  `./x context --uncovered > <scratchpad>/uncovered-before.txt` (any session-scratch
  path; the file is throwaway tooling, never committed). The whole-effort Verification
  section diffs against it.
- [ ] **Task 1.3: Verify and commit.** `./x sync` (INDEX regenerates), stage
  `docs/decisions/0148-*.md docs/decisions/INDEX.md .awf/awf.lock`, `./x check --staged`
  clean, `./x gate` exit 0, commit:

  ```commit
  docs(adr): accept 0148 topic split
  ```

## Phase 2: Batch 1, rendering/catalog-and-targets

- [ ] **Task 2.1: Move the batch's claims.** Run
  `python3 move_claims.py rendering/catalog-and-targets`. Representative transformation
  (every site identical modulo slug/destination; the affected-site set is exactly the
  ADR's State changes pairs whose remove id starts with `rendering/catalog-and-targets:`):
  the block

  ```
  ### `invariant: pi-child-process-safety`

  In the generated Pi subagent extension, every child exit path removes the temporary role prompt and its listeners, ...
  Origin: ADR-0123
  Backing: test
  ```

  leaves `.awf/topics/parts/rendering/catalog-and-targets/current-state.md` and is
  appended verbatim to `.awf/topics/parts/rendering/pi-runtime/current-state.md` with
  `Origin: ADR-0148`. Edge (a `Revised-by:` carrier, e.g. `pi-extension-target-render`):
  identical, plus its `Revised-by: ADR-0145, ADR-0146` line is deleted. Post-check: the
  staged check in the closing task; interim sanity:
  `grep -c '^### ' .awf/topics/parts/rendering/catalog-and-targets/current-state.md`
  drops by the batch's pair count (indicative: 23 to 16).
- [ ] **Task 2.2: Rewrite marker sites and fixtures.** Run the shared rewrite loop with
  `rendering/catalog-and-targets`, then the shared post-check (zero output).
- [ ] **Task 2.3: Append Implementing plus the first Applied event.** In the ADR, set
  frontmatter `status: Implementing` and append two lines to `## Status history`: an
  `- <date>: Implementing; content-sha256: <frozen digest from Phase 1>` event, then
  `- <date>: Applied; state-sequence: <next>; operations: <output of ops_line.py rendering/catalog-and-targets>`.
- [ ] **Task 2.4: Verify and commit.** Shared closing steps. Commit:

  ```commit
  docs(invariants): apply 0148 batch 1 catalog-and-targets
  ```

## Phase 3: Batch 2, config/configuration

- [ ] **Task 3.1: Move the batch's claims.** `python3 move_claims.py config/configuration`.
  Same representative shape as Task 2.1; this batch has no `Revised-by:` carrier, and the
  shape is identical at every site. Interim sanity: the source part's `^### ` count drops
  by the pair count (indicative: 30 to 15).
- [ ] **Task 3.2: Rewrite marker sites and fixtures.** Shared rewrite loop with
  `config/configuration`; shared post-check (zero output).
- [ ] **Task 3.3: Append the Applied event.** One line:
  `- <date>: Applied; state-sequence: <next>; operations: <output of ops_line.py config/configuration>`.
  No status change (middle batch).
- [ ] **Task 3.4: Verify and commit.** Shared closing steps. Commit:

  ```commit
  docs(invariants): apply 0148 batch 2 configuration
  ```

## Phase 4: Batch 3, tooling/cli

- [ ] **Task 4.1: Move the batch's claims.** `python3 move_claims.py tooling/cli`. Same
  representative shape; edges are the seven `Revised-by:` carriers (dropped as in Task
  2.1). `tooling/cli:topic-claim-budget-advisory` has no operation and must remain in the
  source part untouched. Interim sanity: source count drops by the pair count
  (indicative: 49 to 16).
- [ ] **Task 4.2: Rewrite marker sites and fixtures.** Shared rewrite loop with
  `tooling/cli`; shared post-check (zero output).
- [ ] **Task 4.3: Append the Applied event.** One line, as in Task 3.3, with
  `ops_line.py tooling/cli`.
- [ ] **Task 4.4: Verify and commit.** Shared closing steps. Commit:

  ```commit
  docs(invariants): apply 0148 batch 3 cli
  ```

## Phase 5: Batch 4, rendering/templates

- [ ] **Task 5.1: Move the batch's claims.** `python3 move_claims.py rendering/templates`.
  Same representative shape; edges are the five `Revised-by:` carriers. Interim sanity:
  source count drops by the pair count (indicative: 50 to 9).
- [ ] **Task 5.2: Rewrite marker sites and fixtures.** Shared rewrite loop with
  `rendering/templates`; shared post-check (zero output).
- [ ] **Task 5.3: Append the Applied event.** One line, as in Task 3.3, with
  `ops_line.py rendering/templates`.
- [ ] **Task 5.4: Verify and commit.** Shared closing steps. Commit:

  ```commit
  docs(invariants): apply 0148 batch 4 templates
  ```

## Phase 6: Batch 5, rendering/project-output-plan, and the terminal flips

- [ ] **Task 6.1: Move the batch's claims.**
  `python3 move_claims.py rendering/project-output-plan`. Same representative shape; no
  `Revised-by:` carrier in this batch. Interim sanity: source count drops by the pair
  count (indicative: 76 to 15).
- [ ] **Task 6.2: Rewrite marker sites and fixtures.** Shared rewrite loop with
  `rendering/project-output-plan`; shared post-check (zero output).
- [ ] **Task 6.3: Append the final Applied event and the Implemented flip.** In the ADR,
  append `- <date>: Applied; state-sequence: <next>; operations: <output of ops_line.py rendering/project-output-plan>`,
  then `- <date>: Implemented; content-sha256: <frozen digest>`, and set frontmatter
  `status: Implemented`.
- [ ] **Task 6.4: Freeze this plan.** Set this plan's frontmatter `status: Implemented`
  and record any implementation findings in Notes.
- [ ] **Task 6.5: Verify and commit.** Shared closing steps (stage the plan too). Commit:

  ```commit
  docs(invariants): apply 0148 batch 5 project-output-plan
  ```

## Verification

- `./x check` on the clean working tree prints no `above maxClaimsPerTopic` note.
- `./x context --uncovered` output diffs empty against the Task 1.2 baseline (no selector
  changes occur in this plan).
- `git grep -F "<old-id>"` for every removed id, excluding `docs/decisions` and
  `docs/plans`, returns no matches.
- `awf topic rendering/pi-runtime` (and any other destination) lists its claims;
  `awf topic 'rendering/catalog-and-targets:pi-child-process-safety' --history` on a
  removed id resolves to its history rather than an error.
- `./x gate` exit 0.

## Notes

- Follow-ups deliberately out of scope (tracked in ADR-0148): advisory severity promotion
  via its own ADR with an adopter-facing `error`/`warn`/`off` config; per-area selector
  narrowing.
- Known benign advisory: the repoaudit changelog-unreleased Warning fires for the
  comment-only marker rewrites in adopter-facing files; ADR-0148 Consequences records
  that no changelog entry is owed.
- Pre-existing awf bug found during review, separate fix owed: `awf check` advises
  "carries no tags: add a narrow topic tag" for `current-state-v2` ADRs, but
  `governedFrontmatter` (internal/adr/format.go) rejects a `tags:` key, so the advisory
  is unsatisfiable for governed ADRs.
