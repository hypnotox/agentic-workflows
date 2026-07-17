# awf Deep Analysis 2026-07-16, Part 02: Issues and Defects

*The confirmed defect and risk ledger for the 2026-07-16 analysis set (companion
to `00-overview.md`). Draws on the cli-ux, rendering-templates, and
security-robustness review dimensions plus 18 adversarially-verified hunt bugs,
each carrying a repro command. A delta on
[deep-analysis-2026-07-15.md](deep-analysis-2026-07-15.md) and
[agentic-workflow-landscape-and-awf-standing-2026-07.md](agentic-workflow-landscape-and-awf-standing-2026-07.md).*

---

## In one paragraph

The engineering discipline the 07-15 report praised is intact: the tree at
commit 7a06af73 still passes the full `./x gate` clean at 100% statement
coverage, the render pipeline remains exemplary, and the corrupt-lock and
name-traversal choke points hold exactly as designed. But the newest command,
`awf prose-gate` (ADR-0119, shipped 2026-07-16 inside this delta window), landed
with a five-defect cluster that is the headline of this report: it validates
worktree bytes against index names, so a partially staged commit sends a banned
codepoint straight through the gate, while a git submodule, an unstaged
deletion, or a symlinked directory hard-blocks every commit for an adopter who
enables it. The two schema migrations (`pitfalls-data` and `retirement-tokens`)
can each silently corrupt an adopter's tree during `awf upgrade` and leave a
green gate over the wreckage, one of them after deleting the source part first.
Beneath those sit real but lower-likelihood holes in the invariant ledger, the
ADR-0121 comment stripper, the symlink-following write path, and several CLI
seams. Every finding below cites `file:line` against commit 7a06af73 (the
multi-runtime tree that carries the agent encoder these findings exercise); I
re-verified the top-cluster prose-gate, migration, invariant-ledger, lock, and
comment-strip claims plus the agent-encoder findings (3.3, 3.4) against the
source at that commit, and they hold as stated.

---

## What is genuinely good

This project's guardrail engineering is real, and a report that only lists holes
would misrepresent it.

- **The gate is green and the coverage is real.** `./x gate` passes clean on this
  tree at 100% statement coverage (221 `coverage-ignore` markers, honestly
  annotated). None of the defects below is a gate escape that awf's own
  discipline let rot; they are edge inputs and adopter-facing surfaces the
  in-repo hermetic check structurally does not exercise.
- **The prose-gate happy path is well built.** Clean per-codepoint blocking
  messages, a hook-safe disabled default that exits 0 with no output, exemption
  pins, and a deliberate "unreadable path is an error, never a silent pass"
  posture (`internal/prosegate/prosegate.go:70-71`). The defects are all at the
  boundary of that posture, not in its core.
- **The corrupt-lock choke point does exactly what ADR-0076 says.**
  `manifest.LoadOptional` (`internal/manifest/manifest.go:65`) is called first in
  `SyncReport` and refuses before any write on a genuinely unreadable lock. The
  symlink-read of the lock is read-through-then-refuse, verified.
- **Name-traversal is blocked in depth.** `ValidateDocName` /
  `ValidateArtifactName` / `ValidateDomainName` reject `..` and path separators,
  and local synthesis additionally requires a declaring sidecar derived from the
  name, so a traversing name cannot even find its sidecar.
- **The render pipeline stays the strongest code in the tree.** ADR-0121's
  comment strip is executed with care (exact-literal token boundary, hard error
  naming line and source, strip-before-substitution, hashes over unstripped
  bytes). Adapter parity is structural: `diff -r .claude/skills .cursor/skills`
  is empty after enabling the cursor target. `examples/sundial` drift-checks
  clean and is now guarded hermetically in the staged-slice pre-commit.
- **The CLI surface is single-sourced and disciplined** where it counts: one
  `internal/clispec` table drives help, usage, and the gated-command list; exit
  codes are centralized 0/1/2; degraded schema-behind and schema-ahead modes name
  both generations and point the right way.

---

## Cluster 1: the prose-gate defect cluster (ADR-0119, new this window)

`awf prose-gate` is the freshly shipped command from ADR-0119. Five distinct real
defects live in it, sharing two root causes: it takes the file **set** from the
git index but the file **bytes** from the worktree, and it treats every read
outcome that is not a clean regular-file read as either a hard error or a silent
skip. awf's own repo is shielded by its hermetic staged-slice hook (which never
runs prose-gate) plus a fresh-checkout CI `./x gate`; an adopter wiring the
**shipped** `.awf/hooks/pre-commit.sh` payload has neither backstop.

### 1.1 CONFIRMED (defect) - index/worktree byte mismatch lets a banned codepoint through, and false-blocks unrelated commits

`cmd/awf/prosegate.go:32` reads the path set from `git.IndexPaths(root)`, but
`prosegate.Scan` reads content from the worktree via `os.ReadFile`
(`internal/prosegate/prosegate.go:79`). The bytes validated are not the bytes
being committed.

- **Mechanism (false pass):** stage a file containing U+2014, then overwrite the
  worktree copy with a clean version without re-adding. prose-gate reports clean,
  exit 0, and the commit lands carrying the banned codepoint.
- **Mechanism (false block):** `rm` a tracked file without `git rm`; the index
  still lists it, the worktree read fails, and prose-gate exits 1 blocking every
  commit of unrelated work. The `git.IndexPaths` comment
  (`internal/git/git.go:118-122`) claims a deletion is out of scope "so reading it
  would fail", but an *unstaged* deletion is exactly the case where the index
  still holds the path and the read does fail.
- **Blast radius:** the gate's whole purpose (keep banned glyphs out of commits)
  is defeated in the false-pass direction; in the false-block direction the
  adopter cannot commit at all.
- **Repro:** `cd /tmp/pg1 && git init -q && mkdir .awf && printf 'proseGate:\n  enabled: true\n' > .awf/config.yaml && printf 'clean line\n' > a.md && git add -A && printf 'has \xe2\x80\x94 dash\n' > a.md && git add a.md && printf 'clean line\n' > a.md && awf prose-gate` -> "prose-gate: clean", exit 0.
- **Fix:** read the staged blob for each index path (go-git can hand back the
  index entry's object) rather than the worktree file, so set and bytes come from
  the same source; that also removes the unstaged-deletion false block.

### 1.2 CONFIRMED (defect) - a git submodule hard-fails the entire gate on a clean tree

`git.IndexPaths` returns every index entry name with no mode filtering
(`internal/git/git.go:135-140`), so a submodule's gitlink entry (mode 160000) is
handed to `os.ReadFile`, which fails with "is a directory", and Scan aborts the
whole run (`internal/prosegate/prosegate.go:79-81`).

- **Blast radius:** any adopter with any submodule who sets `proseGate.enabled`
  has **every** commit blocked with no exemption escape (exemptions filter
  findings, not read errors). Tracked symlinks-to-directory and dangling symlink
  targets hit the same seam.
- **Repro:** create a submodule, then `awf prose-gate` -> exit 1,
  "prose-gate: read sub: ... is a directory".
- **Fix:** filter `IndexPaths` to regular blobs (skip mode 160000 gitlinks and,
  with 1.3/1.5, non-regular entries), or `Lstat`+`IsRegular` each path in Scan
  and skip non-regular entries rather than erroring.

### 1.3 CONFIRMED (defect) - index/worktree mismatch and non-regular files hard-block unrelated commits

The same root as 1.1 and 1.2 stated as its own charter item: Scan's
"read failure is a hard error" rule (`internal/prosegate/prosegate.go:79-81`)
fires on (a) an unstaged deletion and (b) a tracked symlink to a directory, both
with zero banned glyphs present. The rendered adopter hook runs `awf prose-gate`
unconditionally against the worktree, so either state blocks commits until the
offending path is staged or removed.

- **Fix:** distinguish "not a scannable regular text file" (skip) from "genuine
  I/O fault" (error); only the latter should abort.

### 1.4 CONFIRMED (defect) - a pinned exemption whose count drops to zero passes silently

Scan consults the exemption map only from inside `for r, n := range counts`
(`internal/prosegate/prosegate.go:92-99`), i.e. only for runes actually found. If
the real count is 0 (file swept clean, deleted, or renamed), a pin of 7 is never
compared, so `pin=7` versus `count=0` yields "clean".

- **Mechanism:** contradicts ADR-0119's own backed invariant
  `prose-gate-tracked-file-scan` ("an exemption with a pinned count fails when the
  file's count for that codepoint differs",
  `docs/decisions/0119-...-prose-gate.md:352-353`) and Decision item 5's pitch
  ("ADR-0113 has exactly seven em-dashes and always will"). All four of awf's own
  exemptions are pinned; if ADR-0113's seven were ever swept, nothing fails and
  the exemption rots. The invariant test
  (`internal/prosegate/prosegate_test.go:63-90`) covers `pin=0/count=2` but never
  `pin>0/count=0`, so the ledger claim is broader than its backing.
- **Blast radius:** silent exemption rot; the "fails-when-stale" property the rest
  of the repo is built on is absent here, on the surface that advertises it.
- **Repro:** pin `count: 7` for a file containing no dashes -> "prose-gate: clean".
- **Fix:** iterate the exemption entries as well as the found counts, and emit a
  finding whenever a pinned count does not equal the actual count (including 0).

### 1.5 CONFIRMED (risk) - one invalid byte reclassifies a whole text file as binary, hiding a real banned codepoint

Scan skips any file failing `utf8.Valid` wholesale and silently
(`internal/prosegate/prosegate.go:83-85`). A markdown file with one pasted
CP1252 byte (Word smart quotes are single bytes 0x91-0x97, exactly this
punctuation in another encoding) plus a genuine UTF-8 em-dash reports clean.

- **Mechanism:** the skip is a documented decision ("a default-deny gate must not
  guess at binary input"), but skipping is itself a guess in the permissive
  direction; nothing surfaces the skipped file.
- **Blast radius:** a smart-quote paste from a CP1252 source lands unflagged
  forever. Calibrated risk (not defect) because the behavior is documented.
- **Repro:** a `doc.md` containing `\xe2\x80\x94` (U+2014) plus a stray `\xff` ->
  "prose-gate: clean", exit 0.
- **Fix:** scan valid-UTF-8 runs per-line and skip only the undecodable spans, or
  at minimum print a "skipped N non-text files" line so the permissive skip is
  visible.

### 1.6 ANALYST-OPINION (smell) - findings carry no line or column

`Format` emits only `<path>: <name> (U+xxxx) appears N time(s)`
(`internal/prosegate/prosegate.go:112-119`); no line, no column. A blocked commit
on a long doc containing one pasted curly quote sends the user hunting with an
external grep for the literal codepoint. Every comparable linter emits
`file:line:col`. Fix: record and print the first occurrence's position.

---

## Cluster 2: the migration defect cluster (`internal/migrate`)

Both schema migrations run inside `awf upgrade` and can corrupt an adopter's tree
while leaving a green gate. This is the most dangerous class in the ledger:
corruption during the one command whose job is a safe forward migration.

### 2.1 CONFIRMED (defect) - `pitfalls-data` (schema 9) emits invalid or lossy YAML and deletes the source part first

`renderPitfallsSidecar` writes each body as a literal block scalar `body: |` with
a fixed 8-space indent and **no** indentation indicator
(`internal/migrate/pitfalls.go:111-118`). YAML infers a literal scalar's
indentation from its first non-empty line, so:

- **(a) unparseable output:** a body whose first line is indented (a Markdown
  indented code block, the normal pitfalls shape) but with a later column-0 line
  yields a sidecar that fails to parse. With the pitfalls doc already enabled,
  `awf upgrade` itself exits 1 mid-flight **after** `applyPitfallsData` has already
  `os.Remove`d the `entries.md` part (`pitfalls.go:37`) and the lock has been
  restamped to schema 10, so a re-run reports the schema current and never
  revisits. The adopter's content is trapped in broken YAML with the source gone.
- **(b) silent demotion:** a body that is entirely an indented code block
  round-trips "successfully" with the indentation absorbed into the detected
  scalar indent, so the rendered `docs/pitfalls.md` shows the code at column 0,
  silently demoting a code block to prose, with upgrade, enable, sync, and check
  all green.
- **Blast radius:** data loss (source part deleted) plus a green gate over the
  damage. Only triggered on a schema-8 tree carrying an authored pitfalls part.
- **Repro:** `go build -o /tmp/awf ./cmd/awf; mkdir -p /tmp/p1 && cd /tmp/p1 && git init -q . && /tmp/awf init </dev/null >/dev/null 2>&1; sed -i 's/"schemaVersion": 10/"schemaVersion": 8/' .awf/awf.lock; mkdir -p .awf/docs/parts/pitfalls; printf '## T\n\n    code := 1\n\nProse at column 0.\n' > .awf/docs/parts/pitfalls/entries.md; /tmp/awf upgrade >/dev/null 2>&1; /tmp/awf enable doc pitfalls` -> exit 1, "parse sidecar docs/pitfalls.yaml: yaml: line 2: did not find expected key".
- **Fix:** add an explicit block-scalar indentation indicator (`body: |8`) or, more
  robustly, emit the body via a YAML marshaller instead of hand-built string
  concatenation; and reorder `applyPitfallsData` so the source part is removed only
  after the sidecar is written and re-parsed successfully.

### 2.2 CONFIRMED (defect) - `retirement-tokens` (schema 10) splices its bookkeeping item inside a fenced code block and mints a duplicate item number

The insertion scan `strings.Index(raw[bodyStart:], "\n## ")`
(`internal/migrate/retirementtokens.go:131`) is fence-blind, and the item number
comes from `a.DecisionItems()`, which reads the same fence-blind `sections()`.
On a schema-9 corpus where a retiring ADR's Decision quotes a markdown template
in a column-0 fence containing a `## ` line, `awf upgrade` exits 0 but inserts
`2. **Retirement bookkeeping...**` **inside** the fence (before the fenced
`## Overview`) and numbers it 2 while a real item 2 already exists after the
fence.

- **Blast radius:** corrupts a frozen decision record (append-only ADR invariant)
  and makes ADR-0120 item anchors (`supersedes: ADR-NNNN#2`) ambiguous. The
  corruption is invisible to `awf check`: the adr-decision-format check in
  `internal/project/supersession.go:92` parses with the same fence-blind
  `sections()`, sees a clean `[1,2]` sequence, and reports clean.
- **Repro:** see the repro in the source ledger; upgrade exits 0 and check reports
  clean over the mangled file.
- **Fix:** run the insertion-point scan and `DecisionItems` through
  `refs.WithoutFences` (see Cluster 4.3; the project already owns this tool).

---

## Cluster 3: rendering and ingestion correctness

### 3.1 CONFIRMED (defect) - the ADR-0121 comment-strip fence model diverges from CommonMark

This appears both as a rendering-dimension issue (verdict confirmed) and as a
hunt bug (3 of 3 confirmed); it is one bug. The fence-close test
`strings.HasPrefix(trimmed, fence)` (`internal/render/comment.go:31`) is
length-agnostic and accepts info strings, but CommonMark closes a fence only with
at least as many markers of the same character and no info string.

- **Mechanism:** (a) inside a 4-backtick fence demonstrating a 3-backtick fence,
  the inner ``` line "closes" the tracker, so a following `<!-- awf:comment ... -->`
  directive is treated as outside any fence and silently stripped; (b) a `` ```go ``
  content line inside an open ``` fence is treated as the closer. A malformed-opener
  demo in the same fenced position hard-errors the whole render
  (`comment.go:45-46`), bricking sync until the doc is edited.
- **Blast radius:** contradicts ADR-0121 Decision 3 ("inside a fenced code block,
  directive lines are preserved verbatim") and the invariants
  `authoring-comment-whole-line-only` / `authoring-comment-malformed-fails`.
  Demonstrated end-to-end against a sundial part: the fenced demo line vanished
  from `docs/development.md`.
- **Repro:** append a 4-backtick-wrapped `<!-- awf:comment ... -->` demo to
  `.awf/docs/parts/development/setup.md`, `awf sync`, then
  `grep -c 'awf:comment' docs/development.md` -> 0.
- **Fix:** track the opener's fence character and length and require a closer of
  the same character, at least that length, with no info string. `refs.WithoutFences`
  (`internal/refs/refs.go:35`) shares the same length-agnostic bug (advisory-only
  there), so unify both onto one CommonMark-correct classifier.

### 3.2 CONFIRMED (defect) - a YAML-escaped NUL in a config var forges a part sentinel and splices part content through a clean sync

The sentinel design's premise ("NUL bytes cannot occur in template or markdown
text", `internal/render/render.go:134-135`) is false for config vars: YAML
double-quoted scalars carry `\0` escapes and nothing validates.

- **Mechanism:** with a part body `IDENTITY-PART-BODY here.` and
  `checkCmd: "x\0awf:part:identity\0x"`, `awf sync` completes, `awf check` reports
  clean, and the rendered AGENTS.md Commands section reads
  `` - `xIDENTITY-PART-BODY here.\nx`: check rendered files for drift `` - the
  forged sentinel in the var was replaced by the part body in Execute's restore
  loop (`render.go:272-273`). A NUL var of non-sentinel shape writes raw control
  bytes into rendered output instead.
- **Blast radius:** silent, drift-clean corruption of rendered artifacts. Trigger
  is exotic (an author must write a control-character escape), hence lower
  likelihood.
- **Fix:** reject control characters (at minimum NUL) in vars, sidecar data
  values, and part bytes at load/read; that turns the sentinel premise from an
  assumption into an enforced invariant. (This also re-validates the
  `internal/project/project.go:117` coverage-ignore, whose "cannot be invalid at
  sync time" justification is falsified today by a NUL-bearing var reaching
  frontmatter - an ANALYST-OPINION smell in the source ledger.)

### 3.3 CONFIRMED (defect) - `yamlPlainSafe` admits YAML-invalid plain scalars, hard-failing sync on a legal description

`yamlPlainSafe` (`internal/project/agent.go:72-77`) rejects a leading `-`, `?`,
`:`, the character set `"'[]{}#&*!|>@\`` anywhere, and `": "`. It misses three
shapes that are invalid as YAML plain scalars: a value ending in `:`, a value
starting with `%`, and a value starting with `,`. Such a value takes the
unquoted branch (`agent.go:60-61`) instead of the existing `strconv.Quote` branch,
producing invalid frontmatter.

- **Mechanism:** descriptions reach this path verbatim from adopter sidecar
  `data.description` via the synthesized local-agent template
  (`internal/project/local.go:67`). The downstream read-back catches it, so
  nothing broken ships, but sync hard-fails on a legal config value with a
  confusing error naming a rendered file that was never written.
- **Blast radius:** an adopter with a legal local-agent description like
  `"Reviews the release checklist:"` cannot sync. Bounded (loud error, no bad
  output), but a real usability defect.
- **Repro:** add `.awf/agents/release-helper.yaml` with
  `data.description: "Reviews the release checklist:"`, enable the agent,
  `awf sync` -> exit 1, "mapping values are not allowed in this context".
- **Fix:** extend `yamlPlainSafe` to reject a trailing `:` and a leading `%` or
  `,` (or, simpler, always emit descriptions as a `strconv.Quote`d or block scalar).

### 3.4 CONFIRMED (risk) - the twice-widened ADR-0082 residue guard has a third gap: `AgentSpec.Description` is never scanned

`TestCatalogDataResidue`'s agents loop
(`internal/project/residue_scan_test.go:148-152`) collects only `spec.Data`;
`AgentSpec.Description` (`internal/catalog/catalog.go:28`) is written into the
adopter-facing agent frontmatter by `encodeMarkdownAgent`
(`internal/project/agent.go:51-66`) but is never scanned for ADR-citation residue.

- **Mechanism:** commit bb04ea16 widened the scan to Data/Title/Desc and e0572f65
  to var descriptors, both after ADR-0120 citations shipped into adopter trees
  through unscanned catalog strings; the agent Description surface was left out
  both times. Planting `Per ADR-9999, ` into the code-reviewer Description passes
  both residue tests, and a subsequent sync ships it into
  `.claude/agents/code-reviewer.md`.
- **Blast radius:** a fresh-context-review-agent frontmatter that leaks awf's
  internal ADR provenance into every adopter. Not leaking today (the three shipped
  descriptions are clean), hence risk.
- **Repro:** plant the citation into `internal/catalog/standard.go`, run the two
  residue tests (both pass), then sync and `grep ADR-9999 .claude/agents/code-reviewer.md`.
- **Fix:** add `AgentSpec.Description` (and any sibling normally-rendered catalog
  string) to the residue scan's collected surfaces, guarded by the same
  fails-when-stale discipline.

---

## Cluster 4: invariant-ledger integrity

The invariant ledger is the tool's own false-confidence surface: a hole here
means a green gate over an unenforced or corrupted contract.

### 4.1 CONFIRMED (defect; 2 of 3 verifiers, one dissent) - `supersedes-invariant` resolves by slug name, ignoring the token's ADR target

`DeclaringADRs` applies a retirement as `delete(required, r.Slug)`
(`internal/invariants/invariants.go:185`) without consulting `r.Target`; the only
guard is `declaredAnywhere[r.Slug]` (line 182), a corpus-global, status-independent
name check. ADR-0120 item 2 specifies the opposite: the anchor must be "resolved
by a status-independent raw scan of the target ADR's Invariants section".

- **Mechanism:** slug reuse across a full supersession is legal (the duplicate-slug
  refusal at `invariants.go:144` is scoped to Implemented ADRs) and attractive
  (reuse preserves proof markers). Once a successor re-declares a predecessor's
  slug, an Implemented ADR carrying `supersedes-invariant: <dead-predecessor>#<slug>`
  retires the **live** successor's declaration: the token-ref check passes (the
  dead target does declare the slug), the back-pointer check is skipped (target not
  live), and the dead-target condition is only an advisory note.
- **Blast radius:** a live, backed invariant is silently un-owed with a green gate;
  its production code and proof marker can then be deleted with `awf invariants`
  and `awf check` both clean. Second facet: a token whose live target does not
  declare the slug still retires it inside the invariants module, so standalone
  `awf invariants` reports clean while `awf check` reports the token-ref drift - the
  command named for the ledger gives false confidence.
- **Repro:** see the source ledger; `awf invariants: clean` and `awf check: clean`
  despite a live backed declaration with zero proof markers.
- **Fix:** delete `required[r.Slug]` only when `required[r.Slug].ADR` equals the
  token's target ADR, else flag the divergence.

### 4.2 CONFIRMED (risk) - a malformed slug (e.g. uppercase) silently vanishes from the ledger

`declRe` (`internal/invariants/invariants.go:110`) admits only `[a-z0-9-]` slugs,
so a bullet `` - `invariant: Almanac-Clamped-Latitude`: ... `` in an Implemented
ADR matches nothing: never required, no Unbacked finding, no drift, no note. The
proof-marker regex `slugRe` (line 119) is lowercase-only too, so the matching
marker is equally invisible and does not even surface as a dangling-marker
advisory.

- **Mechanism:** the realistic path is an agent typing the same near-miss slug in
  both places (copy-paste), which yields total silence; the code and marker can
  then be deleted with all checks green. Same-family edge: both regexes are
  unanchored, so `invariant: foo_bar` silently declares and backs the truncated
  slug `foo`.
- **Blast radius:** an attempted invariant declaration drops out of the ledger with
  no signal, on the system whose stated job is catching probabilistic-agent output
  errors deterministically.
- **Repro:** uppercase both the declaration and the marker slug; `awf invariants`
  and `awf check` both clean, zero notes.
- **Fix:** flag any Invariants list item whose lead matches
  `(unbacked-)?invariant:` but whose slug fails the `[a-z0-9-]+`-to-end grammar
  (near-miss guard); anchor the slug capture to the bullet end.

### 4.3 CONFIRMED (defect; 2 of 3 verifiers, one dissent) - decision-item enumeration is fence-blind, raising false drift on a frozen ADR

`decisionItemRe` (`internal/adr/adr.go:69`) and `DecisionItems` enumerate every
column-0 `N. ` line in `Sections["Decision"]` with no fenced-code exclusion, and
`checkDecisionFormat` (`internal/project/supersession.go:90-94`) hard-fails on any
non-sequential result.

- **Mechanism:** a legal ADR whose Decision quotes a numbered transcript inside a
  ``` fence fails `awf check` with "Decision item 1 found where 2 expected". Because
  ADR bodies are frozen once they leave Proposed, an adopter whose historical
  Implemented ADR has this shape goes permanently red after upgrading to the
  ADR-0120 checks, with no legal remediation (re-indenting a frozen fence is
  itself an edit).
- **Blast radius:** a permanently-red gate on an append-only artifact. The same
  fence-blindness makes `parseRefs` treat a backticked token quoted inside a fenced
  example as a live supersession claim (compounding with Cluster 4.1).
- **Repro:** add an Implemented ADR whose Decision item 1 is followed by a fenced
  numbered list, then item 2; `awf check` -> exit 1, adr-decision-format drift.
- **Fix:** run `DecisionItems`, `parseRefs`, and the retirement-token scans through
  `refs.WithoutFences` (`internal/refs/refs.go:28`), already used by the sibling
  scanners at `render.go:246` and `check.go:710` for exactly this reason. This one
  fix also closes Cluster 2.2.

---

## Cluster 5: CLI and lock defects

### 5.1 CONFIRMED (defect) - `init` derives an invalid skill prefix from the directory basename with no validation

`init.go:79` passes `filepath.Base(root)` straight to `ScaffoldConfig`.
`config.Validate` bans only an empty or path-separator prefix
(`internal/config/config.go:267-273`), and `validateFrontmatter` checks only a
non-empty name/description (`internal/project/validate.go:195-211`).

- **Mechanism:** a repo dir like `My Repo` renders skills whose frontmatter is
  `name: My Repo-brainstorming`, which the Claude Code slug constraint (lowercase,
  digits, hyphens) rejects, while `awf check` stays green.
- **Corroboration:** the same weak `config.Validate` is a separate hunt bug
  (3 of 3 confirmed): a prefix `"a: b"` passes Load+Validate and dies only at sync
  read-back with a message that names neither the prefix nor the offending key and
  mislabels a skill an "agent artifact". That read-back guard
  (`internal/project/project.go:117`) is coverage-ignored as "cannot be invalid at
  sync time", a claim both this prefix path and Cluster 3.2's NUL var refute.
- **Blast radius:** the fresh-adopter happy path in any repo whose dir name is not
  already a slug silently produces artifacts the target adapter cannot load, and
  the drift oracle stays green.
- **Repro:** `mkdir '/tmp/My Repo' && cd '/tmp/My Repo' && git init -q . && awf init </dev/null` -> `prefix: My Repo`, skills named `My Repo-brainstorming`, `awf check` exit 0.
- **Fix:** slugify the basename at scaffold time (print a note when adjusted),
  tighten prefix validation to the adapter-safe charset, and extend
  `validateFrontmatter` to the adapter name constraint.

### 5.2 CONFIRMED (defect) - bare `awf list` omits the runner block, and `enable`/`disable`/`list` help omit the runner kind

Two corroborating findings on the same ADR-0101 runner-singleton wiring miss,
reported across the cli-ux dimension and two hunt bugs (all confirmed):

- `runList`'s all-kinds tail appends `listTargets`, `listBootstrap`, `listHooks`
  but never `listRunner` (`cmd/awf/list_add.go:456-460`), so `awf list` never shows
  the runner even though `awf list runner` prints it and the kindFilter switch
  handles it.
- The `enable`/`disable`/`list` help bodies and summaries
  (`internal/clispec/clispec.go:230-231`, `:125`, `:243`) name bootstrap and hooks
  but not runner, even though the unknown-kind error itself advertises runner
  (`cmd/awf/list_add.go:17`). Commit 2243152b (2026-07-15, completing ADR-0101)
  wired the dispatcher, the error message, and AGENTS.md but missed the clispec
  strings: a docs-travel miss inside the tool's own help, landing in this delta
  window.
- **Blast radius:** low; the runner is discoverable via the error message and
  `awf list runner`, just not the default listing or help. But it shows the kind
  vocabulary, unlike the command table, is not single-sourced.
- **Repro:** `awf list | grep -c '^runner:'` -> 0; `awf list runner` prints the block.
- **Fix:** append `listRunner` to the all-kinds tail, and single-source one exported
  kinds list feeding the clispec bodies and the unknown-kind error, guarded by a
  fails-when-stale test.

### 5.3 CONFIRMED (risk) - no upward project-root discovery, so a subdirectory `awf init` scaffolds a nested adoption

`run()` uses `os.Getwd()` directly as the project root
(`cmd/awf/main.go:68`) and nothing walks up to find `.awf/`
(`internal/config/config.go:175`), so inside a subdirectory of an adopted project
every gated command fails with "not an awf project (run `awf init`)".

- **Mechanism:** the hint is actively wrong there. `awf init` from that subdirectory
  succeeds with exit 0 and scaffolds a second complete tree (AGENTS.md, CLAUDE.md,
  docs/, .awf/, .claude/) nested inside the real project, which the parent's
  `awf check` cannot flag (the nested tree is not lock-tracked).
- **Blast radius:** agents are the primary operators and the most likely to `cd`
  into a package dir, hit the error, and comply with the printed remedy.
- **Repro:** `cd <adopted>/subdir && awf init </dev/null; echo exit=$?` -> exit 0,
  nested tree scaffolded.
- **Fix:** walk up from `os.Getwd()` to the nearest ancestor containing
  `.awf/config.yaml` and use it as the root; if none is found, keep the current
  error but do not let `init` scaffold inside an ancestor adoption.

### 5.4 CONFIRMED (defect; 2 of 3 verifiers, one dissent) - a lock mangled to valid-but-empty JSON bypasses the ADR-0076 choke point

`LoadOptional` (`internal/manifest/manifest.go:65-74`) validates JSON syntax only;
a lock decoding to zero values (`{}`, `null`, or one whose files map was lost in a
bad merge resolution) passes as `found=true` with `SchemaVersion 0` / empty
`Files`, and no semantic floor (schemaVersion >= 1, non-empty files) exists.

- **Mechanism:** (a) files map emptied but version fields intact -> `awf sync`
  treats all 32 managed files as foreign, creating 32 `.awf-bak` backups, skipping
  every prune, then `awf check` goes red with 32 orphaned-backup drifts - the exact
  hazards `SyncReport`'s own contract forbids
  (`internal/project/project.go:104-106`); (b) a full `{}` lock ->
  `migrate.Generation` trusts `SchemaVersion 0` blindly and every gated command
  refuses with "config schema is behind (generation 0 < 10); run awf upgrade", and
  following that guidance re-applies all 10 migrations to a current-schema tree,
  ending in the same 32-backup spray. The ADR-0076 "restore it from version control"
  hint never appears on this path.
- **Blast radius:** the corrupt-lock choke point that Cluster (strengths) credits as
  working has a gap: it catches unreadable locks but not valid-JSON-but-empty locks,
  the more likely real-world corruption (bad merge). No test pins the
  zero-value-lock behavior.
- **Repro:** empty the `files` map in `.awf/awf.lock`, then `awf sync | grep -c 'backed up'` -> 32.
- **Fix:** add a semantic floor in `LoadOptional` or `Load`: reject
  `schemaVersion < 1` or an empty `files` map as corrupt, routing to the same
  recovery hint.

---

## Cluster 6: security and robustness risks

Both are CONFIRMED and both rest on the same pattern the 07-15 report did not
cover: awf interpolates a string into a foreign grammar (the filesystem, YAML
frontmatter) without resolving or escaping it.

### 6.1 CONFIRMED (risk) - sync write and prune paths follow symlinks out of the repo root, and `awf check` is blind to it

No write site resolves the real path or checks containment; the `filepath.IsLocal`
guards validate only the lock's path **string**, not symlinked components.

- **Mechanism (write escape):** replacing a managed output (e.g. `AGENTS.md`) with a
  symlink to an out-of-tree file, then `awf sync`, overwrites the victim with
  rendered content (`os.WriteFile` at `internal/project/project.go:189` follows the
  link). A `docs -> /tmp/victim` dir symlink makes `MkdirAll`+`WriteFile` populate
  the out-of-tree victim.
- **Mechanism (delete escape):** a managed path that is a symlink to an out-of-tree
  dir, disabled so its lock entry goes stale, is deleted by the prune `os.Remove`
  (`project.go:232`, past the `IsLocal` string guard at `:226`); same class in
  `Uninstall` (`internal/install/install.go:121`).
- **Mechanism (blind oracle):** with `AGENTS.md` a symlink pointing outside the
  repo, `awf check` reports clean, so every subsequent sync silently rewrites the
  out-of-tree target.
- **Blast radius:** out-of-tree file clobber and deletion, demonstrated in /tmp
  scratch trees. Requires a hostile or careless symlink at a managed path, so this
  is defense-in-depth if "you own and trust the repo you run awf in" is a stated
  axiom (it is not written down anywhere - see the fix).
- **Fix:** before writing/removing a managed path, `Lstat` the final component (and
  `EvalSymlinks` the parent) and enforce `filepath.Rel` containment on the resolved
  path, or open final writes `O_NOFOLLOW`; teach `awf check` to flag a managed path
  that is a symlink. Write a short SECURITY.md stating the trust boundary.

### 6.2 CONFIRMED (risk) - unescaped interpolation into YAML frontmatter lets a `---` in a description truncate the frontmatter and leak into the body

The skill base template splices `data.description` straight into a folded scalar
(`templates/skills/_base/SKILL.md.tmpl:3-4`) with only the first line indented,
so a description whose *subsequent* line is `---` renders frontmatter that closes
early. This is a skills-only path on this tree: since the ADR-0122 refactor agent
descriptions are Go-encoded through `encodeMarkdownAgent`
(`internal/project/agent.go:43-70`), which indents *every* line of a multi-line
description by two spaces (`agent.go:51-57`), so an embedded `---` becomes
`  ---` and cannot terminate the frontmatter, and the agent `_base.md.tmpl`
carries no `---` block of its own (Cluster 3.3 covers the agent-encoder path).

- **Mechanism:** a description `Innocent looking desc.\n---\nname: injected-name\n...`
  produced a skill whose active frontmatter silently truncated to the pre-`---`
  content, with the remainder dumped into the skill body (agent-instruction text),
  and `awf sync` exited 0. `validateFrontmatter` (`internal/project/validate.go:195`)
  only checks that the first parseable block has non-empty name+description, so it
  does not notice the smuggled `---`. (A plain multi-line description without `---`
  IS caught: sync exits 1.)
- **Blast radius:** bounded (injected keys land in the body, not active
  frontmatter), but the result is a silently corrupted skill/agent whose
  description is truncated and whose instruction body carries unintended content.
- **Fix:** emit interpolated descriptions as a forced-indent block scalar or
  JSON-encoded value so embedded newlines/`---` cannot break out, and extend
  `validateFrontmatter` to reject a body that begins with orphaned `key: value`
  lines.

### Lower-severity security notes (ANALYST-OPINION)

- **Non-atomic rendered writes (smell):** only the lock and two migrate config edits
  use `WriteFileAtomic`; every rendered file, ADR, and plan uses plain
  `os.WriteFile` (`project.go:189`, `adr.go:436`, `plan.go:181`). This both permits a
  truncated file on mid-sync crash and is what enables 6.1's write escape. This
  also **corrects the 07-15 report's claim** ("Trust-bearing writes are atomic
  temp-file-plus-rename", `deep-analysis-2026-07-15.md:84`), which overstated the
  scope. Fix: route managed writes through a temp-file+rename helper.
- **Shell hook var interpolation (nit):** `.vars.checkCmd`/`gateCmd`/`proseGateCmd`
  splice verbatim into the rendered hook scripts (`templates/hooks/pre-commit.sh.tmpl:10`).
  By design (those vars are shell commands), and inert until an adopter wires the
  hook, but unvalidated. Fix: a single-line/no-control-char sanity check.
- **Bootstrap tar extraction (nit):** `tar -xzf ... -C "$tmp"`
  (`templates/bootstrap/awf-bootstrap.sh.tmpl:55`) has no member-path restriction
  (release-tarball zip-slip). Out of the stated threat model (checksum-pinned to
  awf's own release) but cheap to close: extract only the `awf` member.

---

## Delta since 2026-07-15

What changed in this 2026-07-16 tree (commit 7a06af73, ADRs 0112 through 0124
added since the 07-15 report), and how prior recommendations fared:

- **The prose-gate cluster is net-new and the single largest source of defects.**
  `awf prose-gate` (ADR-0119) did not exist at 07-15; it shipped inside this window
  and brought five distinct real holes (Cluster 1). The 07-15 report could not have
  flagged them. This is the clearest signal that the delta window's velocity
  outran its own review: a freshly shipped, gate-wired command with a submodule
  that hard-blocks every adopter commit.
- **The 07-15 "tier the AGENTS.md invariants" recommendation was acted on within a
  day** via ADR-0112, executed at the config/data seam (not the template), so
  adopters' guides were untouched. Correct layer, done.
- **The hermetic staged-slice hook was extended** (ff7d9e2b) to drift-check
  `examples/sundial` inside the checkout-index slice, closing a three-recurrence
  pitfall - the retrospective-to-deterministic-check loop the 07-15 report praised,
  working. But note the gap it does not close: prose-gate is not in the staged-slice
  check, so awf's own repo is shielded while shipped adopter hooks are not (Cluster 1).
- **A 07-15 strength claim is corrected here:** "Trust-bearing writes are atomic
  temp-file-plus-rename" (`deep-analysis-2026-07-15.md:84`) is overstated; only the
  lock and two migrate edits are atomic (Cluster 6, note 1). The corrupt-lock choke
  point half of that claim is accurate, but has the valid-JSON-empty-lock gap
  (Cluster 5.4).
- **The migration surface grew** (schema 9 `pitfalls-data`, schema 10
  `retirement-tokens`) and brought two tree-corrupting defects (Cluster 2) that the
  07-15 report predates.
- **A large multi-runtime surface also shipped in this window and was mostly
  outside this slice's three dimensions.** ADR-0122 (format-neutral agents plus
  the Codex-TOML runtime), ADR-0123 (Pi workflow-subagent extension), and ADR-0124
  (deterministic-output plans and target capabilities) landed here and are largely
  test-covered. This ledger touched that surface only where it intersects the
  rendering dimension: the new Go agent encoder (`internal/project/agent.go`) is
  the subject of Cluster 3.3 and 3.4. The rest of the multi-runtime work (the
  broader target matrix and the Pi subagents) was not exercised by the cli-ux,
  rendering, or security lenses of this pass and is a candidate for a follow-up
  slice.
- **Security containment was never a 07-15 lens**, so the symlink-escape and
  frontmatter-interpolation risks (Cluster 6) are net-new observations, not
  regressions. Nothing the 07-15 report recommended touched these surfaces.
- **The 07-15 "positioning, not engineering" theme still stands** and is untouched
  by these findings: none of the defects here contradict the "wrap agent output"
  scope; they are output-guardrail bugs, which is exactly the layer awf claims to
  own, and therefore exactly where they matter most.

---

## Prioritized fix order for this topic

Ranked by severity times blast radius, with the cheapest structural fix first
where impact ties. "Green gate over broken state" and "blocks all adopter commits"
dominate the top.

| # | Finding | Sev | file:line | Why it ranks here | Fix cost |
|---|---------|-----|-----------|-------------------|----------|
| 1 | prose-gate submodule/non-regular gitlink hard-fails whole gate | defect | `internal/git/git.go:136` | Blocks EVERY commit for any adopter with a submodule; no escape | Trivial (mode filter) |
| 2 | prose-gate index/worktree byte mismatch (false pass + false block) | defect | `cmd/awf/prosegate.go:32` | Defeats the gate's purpose; banned codepoint lands in a commit | Small (read staged blob) |
| 3 | `pitfalls-data` migration invalid/lossy YAML, deletes source first | defect | `internal/migrate/pitfalls.go:111` | Data loss during upgrade, green gate over it | Small (indent indicator + reorder) |
| 4 | `retirement-tokens` splices inside fence, duplicate item number | defect | `internal/migrate/retirementtokens.go:131` | Corrupts a frozen ADR, invisible to check | Small (WithoutFences) |
| 5 | prose-gate zero-count exemption passes silently | defect | `internal/prosegate/prosegate.go:92` | Contradicts a declared invariant; exemption rot | Small (iterate exemptions) |
| 6 | supersedes-invariant resolves by slug, ignores ADR target | defect | `internal/invariants/invariants.go:185` | Un-owes a live backed invariant, green gate | Small (compare `.ADR` to target) |
| 7 | lock mangled to valid-empty JSON bypasses ADR-0076 choke point | defect | `internal/manifest/manifest.go:65` | 32-backup spray, misrouted recovery, on a likely merge corruption | Small (semantic floor) |
| 8 | ADR-0121 comment strip fence model diverges from CommonMark | defect | `internal/render/comment.go:31` | Strips fenced directives / bricks render; unify with WithoutFences | Medium |
| 9 | decision-item enumeration fence-blind false drift | defect | `internal/adr/adr.go:69` | Permanently-red gate on a frozen ADR; same fix as #4 | Small (WithoutFences) |
| 10 | symlink write/delete escape; check blind | risk | `internal/project/project.go:189` | Out-of-tree clobber/delete, but needs a planted symlink | Medium (path resolution helper) |
| 11 | `yamlPlainSafe` admits YAML-invalid scalars | defect | `internal/project/agent.go:72` | Sync hard-fails on a legal local-agent description | Trivial (widen check) |
| 12 | init derives invalid skill prefix from dir basename | defect | `cmd/awf/init.go:79` | Fresh-adopter path renders unloadable skills, green check | Small (slugify + validate) |
| 13 | frontmatter `---` truncation via unescaped description | risk | `internal/project/validate.go:195` | Silently corrupted skill/agent body | Small (block scalar) |
| 14 | invalid-UTF-8 byte hides a real banned codepoint | risk | `internal/prosegate/prosegate.go:83` | Smart-quote paste unflagged forever | Small (per-run scan / surface skips) |
| 15 | YAML-escaped NUL forges a part sentinel | defect | `internal/render/render.go:135` | Silent drift-clean corruption; exotic trigger | Trivial (reject control chars) |
| 16 | residue guard third gap: `AgentSpec.Description` unscanned | risk | `internal/project/residue_scan_test.go:148` | ADR provenance could leak to adopters; not leaking today | Trivial (add surface) |
| 17 | malformed uppercase/truncated slug vanishes from ledger | risk | `internal/invariants/invariants.go:110` | Attempted invariant silently unenforced | Small (near-miss guard) |
| 18 | no upward project-root discovery; subdir init nests an adoption | risk | `internal/config/config.go:175` | Agents follow a wrong hint into a nested tree | Small (walk up) |
| 19 | bare `awf list` + help omit the runner kind | defect | `cmd/awf/list_add.go:456` | Discoverability only; single-source the kind list | Trivial |

The top seven are the ones to fix before the next release: each either blocks
adopter commits outright, corrupts an adopter's tree, or produces a green gate
over an unenforced or damaged contract. Items 3, 4, and 9 share a single
underlying fix (route fence-sensitive scans through `refs.WithoutFences`), and
items 5, 1, 2, and 14 are one coherent prose-gate hardening pass. Items 15, 12,
and the config-Validate corroboration converge on one control-character/charset
validation at config load.
