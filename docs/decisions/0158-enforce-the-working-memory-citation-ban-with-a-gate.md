---
format: current-state-v2
status: Implemented
date: 2026-07-25
---
# ADR-0158: Enforce the working-memory citation ban with a gate

## Context

ADR-0069 established the working-memory convention: a per-effort file at `.awf/memory/<effort-slug>.md`, kept out of version control, that must never be committed or cited in an ADR, plan, or commit message. The convention has two halves, and only one is mechanically enforced. The committed half is guarded by the always-rendered self-ignoring `.awf/memory/.gitignore` (backed invariant `rendering/singletons-and-payloads:memory-gitignore-always-on`), so the file cannot be tracked. The cited half - never name a specific effort's memory file in a decision record or commit message - is convention only, enforced by nothing. A dangling citation of an ephemeral, gitignored, eventually-deleted file leaks that file as false authority into a durable record, which is exactly the harm the convention exists to prevent.

A fresh-context grounding check (2026-07-25) verified the corpus and the enforcement surfaces:

### The scan surface is the three named surfaces, not every tracked file

awf self-hosts the convention it defines, so roughly 159 tracked files legitimately name `.awf/memory/`: the rendered skill bodies and their `.awf/` sources, the domain and glossary docs, ADR-0069 itself, the lock manifest, and the Pi handoff extension together with its test fixtures (`tools/pi-extension-test/tests/handoff.test.ts` operates on the fixture `work.md` under `.awf/memory/` by design). A scan of every tracked file would drown in these. The convention names exactly three surfaces - an ADR, a plan, or a commit message - which scopes the scan to `docs/decisions/**`, `docs/plans/**`, and commit-message bodies and excludes the legitimate machinery entirely.

### The detector is syntactic: concrete segments flag, placeholders and the bare directory pass

Description text uses the placeholder form `.awf/memory/<effort-slug>.md` (158 occurrences) or the bare directory `.awf/memory/`; a citation names a concrete file such as the segment `domain-code-staleness-audit.md` written directly after the `.awf/memory/` prefix. The detector flags only a concrete path segment - one whose first character after `.awf/memory/` is a path-segment character `[A-Za-z0-9._-]`, that contains no angle bracket, and that is not `.gitignore`. It is purely syntactic and cannot read intent: the placeholder (never a real filename) and the bare directory always pass, but a concrete segment is flagged whether it is a citation or descriptive prose. So a legitimate description that must name a concrete memory file writes the filename separately from the `.awf/memory/` prefix (as this ADR does) or uses the placeholder; it must never write a contiguous `.awf/memory/<file>` literal. Within the three named surfaces `docs/decisions/**` carries zero such literals (this ADR is phrased to keep it so); only three plan lines carry a concrete reference.

### The closest existing mechanism is the prose gate, and its couplings are verified

ADR-0119's prose gate is the shape to mirror: an opt-in staged-tree scanner with a config knob, wired into `./x gate` and the pre-commit hook. Verified couplings that shape this decision:

- Prose-gate wiring is in two places, not one: a hardcoded `./awf prose-gate` step in the `x` runner, and a template line in `templates/hooks/pre-commit.sh.tmpl` driven by a `proseGateCmd` var. Mirroring it needs a matching `memoryGateCmd` var threaded through the catalog var descriptor (`internal/catalog/standard.go`), the availability map (`internal/configspec/spec.go`), and `.awf/config.yaml` vars, all pinned by the var-derivation parity test.
- Three existing claims enumerate their own contents, so the new var falsifies them rather than merely extending them, and each needs an `update` operation: the catalog's pinned functional var-key set (whose backing test asserts the exact set and fails outright until it is edited), the hook-payload fallback-safety claim's list of unset command vars, and the resolvable-hook-commands claim's list of vars a hooks-enabled project must set. ADR-0156 is the precedent: introducing `awfInvokeCmd` carried an `update` on the same pinned-key claim.
- An opt-in nil-pointer config field is backward-compatible and needs no schema-generation bump or migration entry (the `proseGate` field carries none); the ADR-0039 version gate still protects an old binary through the lock `awfVersion`. The new var is deliberately treated the same way. The comment on the pinned-key test reads ADR-0087's seed-on-introduction convention as obliging the release that adds a catalog var to ship a one-time migration seeding the key empty, but no catalog var has ever shipped one: neither `proseGateCmd` nor `awfInvokeCmd` did, and the only production caller of the seeding helper is the `awf new` scaffold. Here a seed would also change nothing, because the wiring guard treats a present-but-empty var as unset and refuses identically; its sole effect would be an unset-var advisory note, which the guard's actionable refusal already supersedes for the projects it binds.
- A new config field is covered by the existing generic `config/configspec-and-reference:configspec-key-parity` invariant, which requires a configspec `keys` entry per struct leaf and regenerates `docs/config-reference.md`; the enabling ADR number and any repo-identity literal are barred from the configspec description by `configspec-description-residue` and live only in the Go struct doc-comment.
- Unlike the prose gate, which scans the whole staged tree, the memory gate must path-filter its blob list to the two decision-record directories.
- Adding a commit-body scan to the commit gate is additive and does not touch the `tooling/audit-and-snapshots:commit-gate-shared-rule` claim, which is about the shared Conventional-Commit subject check.

### An authorized exception to the frozen-history norm

Three historical plans carry a concrete reference: `docs/plans/2026-07-07-working-memory-convention.md:243`, `docs/plans/2026-07-08-anchored-globs-domain-code-staleness.md:725`, and `docs/plans/2026-07-10-closed-config-tree.md:674`. The project normally leaves completed plans frozen. The effort owner explicitly authorized rewording these three to the placeholder form so the repository needs no gate exemptions; that authorization is recorded here so the deviation from the frozen-plan norm is on the record. A fourth line, `2026-07-07:411`, names the bare directory through a markdown-escaped backtick and passes the detector unchanged, so it is left as-is.

## Decision

1. Add an `internal/memorycite` package with a detector that, given a byte slice and a path, reports every reference to `.awf/memory/<segment>` whose `<segment>` is concrete: the first character after `.awf/memory/` is a path-segment character `[A-Za-z0-9._-]`, the segment contains neither `<` nor `>`, and the segment is not `.gitignore`. The segment runs to the first slash, ASCII whitespace character, backtick, or quote character (either kind), or to the end of the line. The quote terminators are what make the exclusion fire on a reference written inside a quoted string literal, and the corpus claim above depends on them. A reference whose next character is any other byte (whitespace, backtick, backslash, slash, either quote, end of input) is the bare directory and passes; a segment containing an angle bracket is a placeholder and passes. The detector accepts a synthetic path for callers that scan text with no file path.

2. Add an `awf memory-gate` subcommand mirroring `cmd/awf/prosegate.go`: read the staged tree, path-filter the blob list to `docs/decisions/**` and `docs/plans/**`, run the detector, and exit non-zero on any finding. It returns without scanning when the config knob is off. It is a blocking gate, not an advisory report.

3. Extend `cmd/awf/commitgate.go` to run the same detector over the commit-message body alongside the existing Conventional-Commit subject check, blocking the commit on any finding. It self-gates on the same `memoryCite.enabled` knob as the memory-gate command, so the policy is opt-in as a whole rather than half-on for every adopter who upgrades. It scans the git-cleaned message rather than the raw bytes: a comment line and the diff a verbose commit appends below the scissors line are both discarded by git, and neither should be able to fail a commit over text that never reaches the record. That second point is not hypothetical, because a legitimate concrete mention lives in this repository's Pi extension test fixtures, and a verbose commit touching them would otherwise carry it into the scanned text. The scan applies to every message the commit will record, merge and autosquash messages included: the existing exemption for a git-generated subject scopes the Conventional-Commit check alone, because git generates the subject while the body remains editable by hand, and a citation in a merge body persists in history exactly like any other. The scan lives in the commit-gate command, calls the shared detector, and leaves `internal/audit` unchanged.

4. Add an opt-in `memoryCite` config field mirroring `proseGate`, carrying `enabled` and an `exemptions` list of `{path, count}` entries, and a `memoryGateCmd` var mirroring `proseGateCmd`. Enable the knob in this repository with an empty exemptions list. Wire the gate into the `x` runner's gate step and the pre-commit hook template exactly as the prose gate is wired; the template line mirrors prose-gate's `{{ with .vars.memoryGateCmd }}...{{ else }}...{{ end }}` fallback so an unset `memoryGateCmd` degrades to a runnable `memory-gate` invocation with no unresolved-value token. `memoryGateCmd` also joins `checkCmd`, `commitGateCmd`, and `proseGateCmd` in the config-validation guard on hook-command resolvability, so a project with the hooks singleton enabled and the runner singleton disabled must set it explicitly instead of rendering a bare unshimmed `awf` call; a project on the seeded default of an enabled runner is unaffected, and no seeding migration accompanies the var. The enabling ADR number appears only in the Go struct doc-comment, never in the configspec description.

5. Reword the three authorized frozen-plan lines to the placeholder form, preserving each line's illustrative intent (the `2026-07-10` line's example must keep showing that a nested, non-`.md` path under `.awf/memory/` is covered). Keep `docs/decisions/**` free of concrete citations by phrasing rather than editing (this ADR names any concrete file separately from the `.awf/memory/` prefix, never as a contiguous literal), and leave `2026-07-07:411` untouched (the detector passes it).

6. This promotes the cited half of ADR-0069 from convention to a backed invariant `tooling/quality-gates:memory-citation-gate`: the detector flags a concrete `.awf/memory/<segment>` reference (the placeholder and bare-directory forms pass), and the memory-gate and commit-gate exit non-zero on any finding, so an ADR, a plan, or a commit-message body carrying one cannot be committed. It is `Backing: test`; proof markers sit on the command-level tests that exercise the blocking path (one on the memory-gate rejecting a flagged decision-record blob, one on the commit-gate rejecting a flagged body), with the detector's discrimination rule covered by the `internal/memorycite` unit table, all in `currentState.testGlobs` files. The claim text is authored in the `tooling/quality-gates` current-state part with `Origin: ADR-0158` in the Implemented commit, and the invariant joins the AGENTS.md Invariants list. It sits with the enforcement family in `tooling/quality-gates` rather than with the sibling gitignore invariant in `rendering/singletons-and-payloads`, because its subject is a gate and its backing is a tooling test.

7. Every status transition of this ADR regenerates `docs/decisions/INDEX.md` (and, when the config field lands, `docs/config-reference.md`) via `./x sync` in the same commit.

## State changes

- add `tooling/quality-gates:memory-citation-gate`
- update `rendering/catalog-and-targets:var-descriptor-set-pinned`
- update `rendering/companion-scripts:hook-payloads-fallback-safe`
- update `config/validation:hooks-commands-resolvable`

## Consequences

- The cited half of the working-memory convention becomes mechanically enforced. A decision record or commit message can no longer smuggle a specific effort's ephemeral memory file in as durable authority; the gate blocks it before the commit lands.
- The scan is deliberately narrow (the three named surfaces, concrete segments only), so it never fights awf's own self-hosted description of the convention. The cost is that a concrete memory reference outside a decision record or commit message is not caught; that is out of scope because it is not the harm, and widening the surface would only add the Pi extension's legitimate test fixtures behind an exemption.
- Because the detector is purely syntactic, it flags any contiguous `.awf/memory/<file>` literal even in legitimate descriptive prose. A new authoring constraint follows for the three named surfaces: a decision record or plan that must discuss a concrete memory file names it separately from the `.awf/memory/` prefix or uses the placeholder, exactly as this ADR is phrased (mirroring the prose gate's "name the banned glyph by codepoint, or exempt it"). The alternative (an exemption per descriptive mention) was declined to keep the exemptions list empty.
- New wiring surface: the `memoryGateCmd` var, the four `memoryCite` configspec key entries, the config-reference regeneration, and the two wiring points (runner and hook template). The staged handshake and the parity tests force these to land together, so a missing touchpoint fails the gate rather than shipping silent drift.
- Three claims that enumerate command vars are rewritten to name the new one, so the enumerations stay honest rather than silently incomplete. The pinned-key claim's backing test makes this mechanical: the var cannot land without it.
- The gate runs twice in this repository's pre-commit (once directly, once inside `./x gate`), exactly as the prose gate does today; this is redundant but harmless and keeps the two gates uniform.
- Three frozen plans are edited, a deliberate and authorized departure from the frozen-history norm, recorded in Context. The alternative (permanent exemption entries for historical example text) was declined in favor of a clean, exemption-free repository state.
- Adopters who enable the knob gain the same enforcement over their own decision records; those who leave it off keep a gate that no-ops when disabled. One adopter shape does feel the change regardless of the knob: a project with the hooks singleton enabled and the runner singleton explicitly disabled must set `memoryGateCmd` before `awf sync` or `awf check` will pass, and gets an error naming the exact var. That is the same population and the same refusal ADR-0156 already accepted for the other three hook-referenced vars, and the runner's seeded default keeps it small; the alternative was to let those projects render a bare unshimmed `awf memory-gate` line, which is precisely the degradation ADR-0156 removed.
- The detector is a shared, path-agnostic function with a single discrimination rule, so its correctness is provable in isolation by a table-driven test, and the two callers become thin wiring verified by their own command tests.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Scan every tracked file, not just the three named surfaces | Adds only the Pi extension's legitimate concrete test fixtures (which need real paths), forcing a blanket exemption for zero added protection against the actual harm |
| Fold the check into the prose gate | Conflates two unrelated content policies under one name and one knob, so a repository cannot enable one without the other and the prose gate loses its single responsibility |
| Enforce commit messages only, leave ADRs and plans to convention | Leaves the highest-value surface (durable decision records) unenforced; the invariant names all three surfaces |
| Report file citations as an advisory `awf audit` finding | Advisory is the wrong severity for a hard-invariant violation; the check must block, not warn |
| Grandfather the three existing plan references with exemption entries | The owner authorized rewording them instead, which keeps the exemptions list empty and the repository state clean |
| Leave `memoryGateCmd` out of the hook-command resolvability guard | Spares a narrow adopter shape one config line at the price of letting it render a bare unshimmed `awf memory-gate` call, the exact degradation ADR-0156 removed, and of making the new var the only hook-referenced command var outside the guard |
| Ship a one-time migration seeding `memoryGateCmd` empty | No catalog var has ever shipped such a seed, and a present-but-empty value is unset to the guard, so the seed would change no behaviour and buy only an advisory note the guard's refusal already supersedes |
| Leave the commit-gate body scan always on, independent of the knob | Makes the policy half-on for every adopter the moment they upgrade, which contradicts the field being opt-in and would block commits in a repository that never chose the rule |
| Scan the raw commit message rather than the git-cleaned one | A verbose commit appends the staged diff below a scissors line, so a commit touching a file that legitimately names a concrete memory file would be rejected over text git discards before recording the message |
| Let the git-generated-subject exemption skip the citation scan too | Cheaper to implement, but it leaves a hand-edited merge body free to carry a citation into permanent history, which is the harm this decision exists to prevent, and it makes the invariant weaker than the convention it enforces |

## Status history

- 2026-07-25: Proposed
- 2026-07-25: Implementing; content-sha256: 91f3d1061c0caf48c5e660b0a54660332a30b1ea6589b6634ab6cca9673fdee5
- 2026-07-25: Applied; state-sequence: 44; operations: update `rendering/catalog-and-targets:var-descriptor-set-pinned`, update `rendering/companion-scripts:hook-payloads-fallback-safe`, update `config/validation:hooks-commands-resolvable`
- 2026-07-25: Applied; state-sequence: 45; operations: add `tooling/quality-gates:memory-citation-gate`
- 2026-07-25: Implemented; content-sha256: 91f3d1061c0caf48c5e660b0a54660332a30b1ea6589b6634ab6cca9673fdee5
