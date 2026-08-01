A minimal simple fix uses no effort. For a concrete non-minimal outcome, run `awf effort new "<outcome>"`; the immutable slug identifies `.awf/efforts/<slug>/state.json`, its always-owned `.awf/efforts/<slug>/memory.md`, `.awf/worktrees/<slug>/`, and the `awf/<slug>` branch. Creation makes the managed worktree by default (`--no-worktree` opts out; `--base <ref>` selects the base); `awf effort worktree add <slug>` remains the standalone operation for efforts created without one. Git topology, not effort state, owns integration and removal facts; finish is restartable deletion and refuses until every managed path, registration, and branch is absent.

Pi's `handoff_session` accepts only the exact repository-relative `.awf/efforts/<slug>/memory.md` path or an absolute spelling that normalizes to it. It validates the slug, confinement, ownership, bounded UTF-8 regular-file identity, and repository identity without selecting an effort or mutating lifecycle state.

`awf new adr "<title>"` scaffolds by branch. On the configured `integrationBranch` it writes the next numbered record, `NNNN-<slug>.md`; on any other branch, and so in every managed worktree, it writes a pending record as `<slug>.md` headed `# ADR-<slug>: <title>`. A pending record is an ordinary corpus member that can be reviewed and implemented to completion, and the `slug:` frontmatter key it carries is retained forever, so a reference written while it was pending keeps resolving after it is numbered. `awf check` refuses a pending record on the integration branch, which is what forces numbering to happen at integration.

`awf adr number [<slug>...]` performs that numbering. Run it inside the effort's worktree, between merging the integration branch in and integrating, so the numbers are allocated against the corpus the record is about to join:

```
git merge <integrationBranch>   # bring in whatever numbers were taken meanwhile
awf adr number                  # assign, substitute, re-render
<gate>                          # the project's gate
awf effort integrate <slug>     # from the receiving checkout, not the worktree
```

Numbering covers ADR identity and nothing else, and it is not the only namespace two branches allocate optimistically. Schema migration generations can also collide and have no reconciliation command: resolve them by hand inside that merge, before numbering. Move any migration generation the integration branch has taken to sit above it, carrying the minimum-version map and every generation-pinning test and comment with it. Keep the branch's higher schema stamp rather than the integration branch's, and hand-apply the effect of any migration that stamp now skips as applied; a retired key is stripped from a historical config unconditionally, so only the live config tree needs the hand edit. ADR format does not need similar reconciliation: every record keeps its authored format marker, and its number never selects a parser.

Bare invocation numbers a single pending record. Several pending records require an explicit list naming every one of them, in an order that numbers a record before any record that revises what it adds; a partial list is refused, because a corpus half-numbered has no legal state to be in. The command prints one `<slug> -> NNNN` line per assignment for the integration commit message, and it does not precondition on a green check, so an unrelated finding cannot deadlock the one command that resolves the corpus.

A number, once assigned, never changes. If another integration lands between your numbering and your own integration, the numbering is unmade rather than corrected: reset it, take the new state, and number again. The command detects that shape (duplicate numbers with no pending record left) and refuses with the recipe rather than guessing:

```
git reset --hard HEAD~1 && git merge <integrationBranch> && awf adr number
```

A record predating the slug format has no slug to be numbered by, and merging the integration branch in can bring a record that has taken its number. Renaming it is the one sanctioned exception to the paragraph above, and it is done by hand: no command performs it. The transition pairs the two ends by their canonical content digest, which covers the five body sections and excludes the frontmatter and the heading, so the rename must touch the filename and the heading line and nothing else. A file-wide substitution of the old number moves the digest, dissolves the pairing, and is reported as an unrelated deletion and addition rather than as a rename. For the same reason a rename and a content amendment cannot share a commit: rename first, amend second. Rename one record per commit as well: the pairing reads two records exchanging numbers as two ordinary renames and accepts it, so a crossed rename is the one mistake in this area no check reports. The record keeps its authored format across the rename because parser selection is intrinsic rather than number-based.

For `awf context`, bare directories provide tier-0 census, compact grouping, provenance, topic counts, and bounded pending orientation; bare exact, staged, and range-selected files additionally provide tier-1 `State`, `Touches`, and `Proofs` relationships from actual markers. The eight named facets expand directory relationships, non-direct authority, evidence, selectors, references, pending operations, or artifacts; only `artifacts` refines groups, and `--full` is their union. Output above 8,192 bytes retains secure caller-owned spill delivery.

### Context spill notices

When `awf context` output would exceed 8,192 bytes, the report securely spills outside the
repository and the command returns exactly a two-line `AWF_CONTEXT_SPILL_V1` notice. On that
exact notice, read the file named on its second line and verify that its byte length equals
the `bytes=<decimal>` descriptor before treating its contents as the context packet.
Best-effort delete the named file after packet use, whether packet use succeeds or fails.
Treat any other output as the context packet itself; do not interpret a near-match as a
spill notice. This subsection is the contract's single rendered home; skills and agent
bodies point here.

Schema generation 31 is the ADR routing retirement migration. It accepts the retired cutoff and gap keys only while reading a schema-30-or-earlier lock, writes no replacement routing fields, and leaves ADR bytes untouched. A version-1 bridge attestation remains a frozen final-upgrade input; its payload is verified and discarded at the journaled lock replacement.
