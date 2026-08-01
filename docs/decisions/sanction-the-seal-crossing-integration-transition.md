---
format: current-state-v3
slug: sanction-the-seal-crossing-integration-transition
status: Implementing
date: 2026-08-01
---
# ADR-sanction-the-seal-crossing-integration-transition: Make ADR format intrinsic and authorize stale-format merges at commit-msg

## Context

awf currently decides a numbered ADR's parser from three permanent number cutoffs in
`.awf/awf.lock`. The V1 cutoff and its explicit lower gaps entered with the current-state
migration bridge; later schema generations added V2 and V3 cutoffs. Staged validation
preserves or re-derives those values, then reparses the whole corpus under them. The authored
record's own `format:` marker is checked against the parser selected by its number instead of
selecting that parser itself.

That model made an ADR's meaning depend on where integration eventually numbers it. A stale
branch exposed the contradiction. It carried an ADR authored and implemented as V2 while V2
was current. Main had since sealed the V3 cutoff and consumed the number the branch had
chosen. Renumbering the stale ADR above the cutoff forced a V2-to-V3 retrofit even though
neither its decision nor its implementation had been authored under V3. Three inline
relaxations then admitted the merge: inheriting a cutoff, widening digest pairing to a newly
slugged after-side record, and allowing the governed format change. They entered in the evil
merge `728f6695`, in neither parent as a reviewable commit. Terminal review subsequently
found that the first pairing predicate admitted deletions and unrelated mutations and had to
narrow it in `1cda10f8`.

The incident is not a missing cutoff edge. A format is an authored property. Legacy records
have no marker; governed records declare `current-state-v1`, `current-state-v2`, or
`current-state-v3`. All of those formats already have parsers and lifecycle rules. A number
is identity, not a schema router. Making number placement choose the format adds integration
strictness without adding semantic evidence.

The existing lock authority spreads that strictness across more than parsing. Manifest
validation requires the ordered cutoffs and legacy gap set, initialization computes them,
schema migrations seal later values, staged checking validates their transitions, and the
bridge cutover promotes its attested V1 cutoff and gaps into permanent authority. Removing
only parser routing would leave this parallel state machine active. The cutoff and gap
fields therefore leave permanent authority together. A resident bridge attestation still
has to parse and verify in its historical shape so an in-progress adopter is recoverable,
but final cutover no longer promotes its routing payload.

Cutoff-free parsing creates one narrower admission question. Existing corpora may contain any
mixture of intrinsically valid formats, and existing older-format records keep following the
lifecycle of the format they declare. New authoring, however, must not choose an obsolete
format merely because every historical parser remains available. The current binary owns one
current authoring format, and `awf new adr` emits it. An ordinary staged transition may
introduce only that format.

A real merge needs one auditable exception. An incoming parent can already contain an ADR
that was validly authored under an older binary and implemented against that format. The
merge must be able to carry that record without rewriting it into a newer semantic contract.
A true fast-forward introduces no integration commit and needs no exception: it only moves a
reference to history already at its tip. A merge commit, by contrast, can distinguish an
incoming-parent artifact from code or prose invented in the merge and can carry durable
authorization in its message.

Git fixes where that authorization can be final. For a conflict-free merge,
`pre-merge-commit` sees neither `MERGE_HEAD` nor `MERGE_MSG`. By `commit-msg`, both the
assembled index and merge-parent state exist and the proposed message is available. If
`commit-msg` refuses, Git preserves `MERGE_HEAD` and the staged merge result, so the agent
can add the required trailers and complete the same merge rather than repeat it. A manual
merge may be checked earlier once its merge state and message exist, but `commit-msg` is the
final authorization boundary in every case.

Plans have no authored format marker, cutoff routing, or version admission rule. This
decision does not add one.

## Decision

1. **ADR parsing is intrinsic.** A numbered ADR declaring `format: current-state-v1`,
   `format: current-state-v2`, or `format: current-state-v3` is parsed and validated by that
   declared format, independent of its number and of every other record. Absence of a format
   marker selects the frozen legacy parser. An absent marker is the only legacy signal; an
   unknown, duplicate, or malformed marker is an error rather than a fallback. Pending ADRs
   retain the current-format identity rules, including the mandatory V3 slug while V3 is the
   current format.

2. **The binary owns the current authoring format.** One code-level format-activation
   registry identifies the running binary's current ADR format and the schema generation at
   which each format became current. The rendered ADR template and `awf new adr` derive from
   it rather than from lock boundaries. Their derivation remains publication-safe under
   missingkey=zero and empty render data, with no unresolved or no-value token, and stays on
   the existing golden-test surface. A staged transition may introduce a new ADR only in the
   current format. Existing records in any older intrinsic format may continue along that
   format's legal lifecycle and transition matrix; they are not upgraded merely because a
   newer binary checks them.

3. **Cutoff and gap authority is retired completely.** Permanent lock fields for the V1,
   V2, and V3 cutoffs and for legacy ADR gaps are removed. Manifest parsing and writing,
   initialization, project render and sync, corpus loading, schema migrations, staged lock
   validation, final upgrade, config-reference derivation, and their fixtures stop consuming
   or producing permanent routing values. The forward schema migration removes those fields
   without rewriting ADR bytes and stamps the intrinsic-format generation. A pre-migration
   binary sees that newer schema generation and refuses through the existing binary-version
   gate rather than misparsing the cutoff-free lock. The first-adoption binary version
   remains immutable and continues to serve its independent compatibility purpose.

4. **Resident bridge attestations remain consumable.** The compatibility parser continues
   to accept the attestation version that carries `adrFormatV1From` and `legacyADRGaps`, and
   final upgrade verifies the complete resident attestation and its approval artifact before
   committing cutover. The final permanent lock discards those two routing values rather
   than promoting them. Attestation consumption, approval-file deletion, and final lock
   replacement retain the existing journaled atomicity, and no step rewrites an ADR. Thus
   permanent manifest authority is cutoff-free while the one historical bridge input stays
   readable until it is consumed.

5. **A real merge may carry an incoming older-format ADR.** Relative to the first parent, a
   result record whose format is older than the running binary's current format is admitted
   only when at least one incoming parent already carries the paired record in that same
   format. Pairing uses the same retained-slug, unique slugless digest, and number resolution
   as the transition checker. After pairing, the incoming artifact and result must be
   byte-identical except for the exact deterministic number, filename, heading, `Origin:`,
   and `Revised-by:` substitutions already sanctioned by numbering or renumbering. Legacy
   records receive the same exactness check with only their applicable number, filename, and
   heading substitutions; they have no governed provenance exception. No status, history,
   format, decision content, or other byte may change under this admission. A lifecycle
   transition or amendment to the incoming record therefore lands in an ordinary later
   commit, not in the merge that imports it. This rule applies across every incoming parent,
   including an octopus merge; one qualifying parent is sufficient for a given result
   record, but no parent record can cover an unrelated evil-merge edit.

6. **Authorization is a paired commit-message trailer.** Each older format a qualifying
   merge needs is authorized by this adjacent trailer pair in the final Git trailer block:

   ```text
   AWF-Allow-Version: current-state-v2
   AWF-Allow-Reason: integrate an ADR authored and implemented on the incoming branch
   ```

   The shared parser first applies the repository's Git-cleaning contract: normalize CRLF,
   remove lines whose first nonblank character is `#`, stop at the scissors line, and trim
   trailing blank lines. The final trailer block is the maximal nonempty suffix after a
   blank separator in which every line is an unindented `Key: value` trailer. Other valid
   trailers may surround an AWF pair but may not split it. Continuation lines are not legal
   for either AWF key. Any cleaned line beginning `AWF-Allow-` outside that block, any
   reserved key other than the two exact case-sensitive spellings, or any malformed pair is
   a refusal rather than ignored prose.

   `AWF-Allow-Version` uses the exact intrinsic format marker, or the literal `legacy` for a
   markerless record. Its value must be either `legacy` or a governed format known to the
   activation registry. The immediately following `AWF-Allow-Reason` belongs to that version
   and must remain nonempty
   after ASCII whitespace is trimmed. A complete pair authorizes every qualifying
   incoming-parent ADR of that version in this merge. Pairs may repeat; duplicate pairs and
   complete pairs for a version that needs no exception are harmless and auditable. An
   orphan key, reversed pair, unknown version, empty reason, interleaved trailer, or body
   lookalike authorizes nothing. Commit gate and audit consume the same cleaned-message
   parser and the same parsed pair model.

7. **`commit-msg` is the definitive merge authorization boundary.** The earlier staged gate
   performs every check derivable from HEAD and the index. Because `pre-merge-commit` cannot
   see merge parents, it treats every otherwise-valid older-format introduction as
   provisional, whether the eventual commit is a merge or an ordinary commit; it does not
   pretend to qualify a parent or authorization it cannot observe. `commit-msg` receives the
   cleaned message, checks whether `MERGE_HEAD` exists, loads every incoming parent, and
   makes the definitive decision. A non-merge commit with a provisional older-format
   introduction is refused. A merge must satisfy item 5 and carry every pair item 6 requires.

   Missing or invalid authorization returns an actionable `operation` outcome whose
   condition names the observed merge and deficient message, reports no validator-caused
   changed axis, and gives the ordered remedy: correct the message trailers, then run
   `git commit` to finish the existing merge. The validator does not clear `MERGE_HEAD` or
   alter the staged merge. Manual merge flows may call the same validation once their
   message exists, but no earlier success bypasses `commit-msg`. No allowance file, ledger,
   receipt, preparation command, or automatic upgrade is introduced.

8. **Audit replays committed authorization from an explicit epoch.** The intrinsic-format
   schema generation is the activation epoch. `awf audit` applies this rule only to a merge
   whose result tree carries that generation or later, and derives the merge-time current
   authoring format from the same format-activation registry and the result tree's schema
   generation. A future format therefore registers its own schema activation without adding
   a number cutoff. The ordinary version gate ensures staged checking has upgraded the
   project to the running binary's schema before it can author a commit.

   For an applicable merge, audit uses the shared trailer parser and qualification validator
   against the committed merge tree, its first parent, and all incoming parents. It reports
   an error when the merge admitted an older-format incoming ADR without a matching complete
   pair or when an `AWF-Allow-` trailer is malformed, and stays silent for valid or redundant
   complete pairs. Pre-epoch merges and historical non-merge commits are not retroactively
   judged by a later binary's current format. A true fast-forward has no merge commit or
   authorization event to replay.

9. **The incident relaxations are removed, not generalized.** Intrinsic parsing means a
   renumbered stale ADR keeps its authored format, so no governed-format retrofit is
   sanctioned. Digest pairing returns to the slugless-only rule already declared by
   `adr-system/adr-lifecycle:renumber-digest-paired`; the after-side newly-slugged widening
   and `isRenumberRetrofit` disappear. The V3 inherited-cutoff edge and its earlier sealing
   and re-sealing relatives disappear with cutoff authority. This record does not add the
   previously proposed `governed-format-change-bounded` or port-forward-retirement claims.

10. **Shared mechanisms have cohesive owners.** `internal/adr` owns the format-activation
    registry alongside intrinsic parsing and scaffolding. A new leaf package,
    `internal/commitmsg`, owns Git-cleaned commit-message normalization, the final trailer
    block parser, and the parsed authorization-pair model; the existing cleaning logic moves
    out of `cmd/awf` rather than being copied. `internal/currentstate` owns exact
    incoming-parent ADR qualification alongside transition pairing. `cmd/awf`,
    `internal/project`, and `internal/audit` orchestrate their respective live, staged, and
    historical flows by depending inward on these owners. The owners do not import those
    callers, and neither commit-message syntax nor parent qualification is reimplemented at
    a command or audit boundary.

11. **New authority is test-backed.** All four added claims use `Backing: test`. The
    implementation supplies proof markers on `TestParseRecordRoutesByIntrinsicFormat` for
    `intrinsic-format-routing`, `TestFinalUpgradeDiscardsBridgeADRRoutingPayload` for
    `bridge-attestation-cutoff-payload-discarded`,
    `TestCheckCommitAuthorizesOlderFormatIncomingParent` for
    `older-format-incoming-parent-sanction`, and `TestAuditReplaysStaleMergeTrailers` for
    `stale-merge-trailer-replay`. Each test covers its refusal half as well as its admission
    or compatibility half; narrower helper tests may supplement but not replace these named
    units.

12. **Every claim mutation uses the checked lifecycle handshake.** The implementation commit
    that records an operation's Applied event also performs exactly its matching claim add,
    update, or removal. Adds name this record as Origin; updates preserve Origin and the
    prior Revised-by prefix before appending this record and make the substantive change
    declared here; removals disappear without reusing their IDs. If implementation is
    batched, the Applied and Remaining partitions preserve the declaration order below.

## State changes

- remove `adr-system/adr-lifecycle:fresh-adoption-v1-cutoff`
- add `adr-system/adr-lifecycle:intrinsic-format-routing`
- update `adr-system/adr-lifecycle:adr-amendable-until-terminal`
- update `adr-system/adr-lifecycle:adr-status-enum-and-matrix`
- remove `config/migrations-and-locks:adr-v2-cutoff-atomic-immutable`
- update `tooling/upgrade-runtime:initial-adoption-version-immutable`
- remove `tooling/upgrade-runtime:legacy-format-set-is-closed`
- add `tooling/upgrade-runtime:bridge-attestation-cutoff-payload-discarded`
- update `invariants/current-state-authority:merge-transition-ordered-aggregate`
- add `adr-system/adr-lifecycle:older-format-incoming-parent-sanction`
- add `tooling/audit-and-snapshots:stale-merge-trailer-replay`

## Consequences

An ADR's parser and lifecycle no longer change when integration changes its number. Existing
mixed-format corpora remain valid, and a stale branch can integrate an ADR under the schema
in which its decision and implementation were actually authored. The dangerous retrofit and
pairing relaxations that motivated this record can be deleted instead of expanded to every
future format.

The lock and migration model becomes smaller. Four permanent routing fields, their ordered
and generation-sealed validation, three migration paths, staged transition exceptions, and
legacy-gap enforcement leave normal authority. Compatibility code for the resident bridge
shape remains until that historical input no longer needs to be consumed; it verifies old
bytes but produces no replacement routing state.

Keeping all historical parsers is now a permanent compatibility obligation. Adding a future
ADR format requires a new intrinsic parser and transition matrix, a new current-authoring
constant and template, and tests that older formats still parse without becoming available
for ordinary new authoring. It does not require allocating a number cutoff or changing old
records.

The stale-format exception is narrow but not invisible. Authorization lives in the merge
commit that used it, names each admitted format, and explains why. Qualification against an
actual incoming parent prevents the trailer from laundering an ADR invented or semantically
rewritten in the merge. Finalizing at `commit-msg` makes the ordinary conflict-free failure
recoverable while avoiding a second state machine. The cost is that the commit gate and
audit need richer Git snapshots: first parent, every incoming parent, result tree, and the
full message must reach one shared validator.

A redundant valid stamp does not make a merge fail, so proactive agents may stamp before
knowing whether pairing will require it. Malformed reserved trailers do fail, which keeps a
typo from looking like durable authorization. Reasons are intentionally human prose rather
than a controlled vocabulary; audit proves presence and scope, not the truth of the reason.

True fast-forwards remain ordinary Git reference movement. They neither fabricate a merge
result nor create a place for an authorization message, so forcing a no-fast-forward commit
solely to carry a stamp would add ceremony without adding transition evidence.

Plans remain unchanged. Their current frontmatter, ADR-link, and commit-fence checks continue
without a version field or stale-merge exception.

Documentation travels with implementation. The owning `.awf/` inputs and all rendered
outputs that state ADR parsing and authoring, lock and bridge fields, migration behavior,
hook timing, commit-message checking, merge integration, audit replay, config reference, and
future-format procedure change in the same applicable transaction. The implementation sweep
includes the agent guide, ADR lifecycle and workflow guidance, working-with-awf, architecture,
domain docs, release and pitfall guidance, repository README, CLI help/spec text, templates,
and generated config-reference surfaces where they actually state affected behavior; it
updates sources and re-renders rather than hand-editing generated files.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Generalize inherited cutoffs and format retrofits across every seal | Preserves the mechanism that caused the defect: number placement would still rewrite authored semantics, and each future format would add another permanent boundary and transition edge. |
| Automatically upgrade an incoming ADR to the current format | A newer marker asserts lifecycle and history semantics under which neither the decision nor its implementation was reviewed. Integration must not fabricate that conformance. |
| Admit every incoming-parent older format without a stamp | Parent provenance prevents an evil-merge invention but leaves no durable record that the agent knowingly exercised the exception or why. |
| Store allowances in the lock, an approval file, or a receipt ledger | Adds persistent authority and a preparation workflow for an event already represented durably by the merge commit. |
| Validate definitively in `pre-merge-commit` | Conflict-free Git merges expose neither `MERGE_HEAD` nor `MERGE_MSG` there, so it cannot validate the same evidence as `commit-msg` without auxiliary state. |
| Require `git merge --no-commit --no-ff` for every stale integration | Gives proactive agents an optional early workflow but makes ordinary conflict-free merges needlessly restartable. A recoverable `commit-msg` refusal preserves the assembled merge. |
| Add plan format versions and matching merge stamps | Plans have no version-dependent parser or transition rule today, so this would create a new problem rather than solve the ADR one. |

## Status history

- 2026-08-01: Proposed
- 2026-08-01: Implementing; content-sha256: 723cd9410d4d09ff285031655c441f9b4187a5209fc31ecc25fe653b1545d3a4
- 2026-08-01: Applied; operations: remove `adr-system/adr-lifecycle:fresh-adoption-v1-cutoff`, add `adr-system/adr-lifecycle:intrinsic-format-routing`, update `adr-system/adr-lifecycle:adr-amendable-until-terminal`, update `adr-system/adr-lifecycle:adr-status-enum-and-matrix`
- 2026-08-01: Applied; operations: remove `config/migrations-and-locks:adr-v2-cutoff-atomic-immutable`, update `tooling/upgrade-runtime:initial-adoption-version-immutable`, remove `tooling/upgrade-runtime:legacy-format-set-is-closed`, add `tooling/upgrade-runtime:bridge-attestation-cutoff-payload-discarded`
- 2026-08-01: Applied; operations: update `invariants/current-state-authority:merge-transition-ordered-aggregate`
- 2026-08-01: Applied; operations: add `adr-system/adr-lifecycle:older-format-incoming-parent-sanction`
