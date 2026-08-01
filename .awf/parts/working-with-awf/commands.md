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

Bare invocation numbers a single pending record. Several pending records require an explicit list naming every one of them, in an order that numbers a record before any record that revises what it adds; a partial list is refused, because a corpus half-numbered has no legal state to be in. The command prints one `<slug> -> NNNN` line per assignment for the integration commit message, and it does not precondition on a green check, so an unrelated finding cannot deadlock the one command that resolves the corpus.

A number, once assigned, never changes. If another integration lands between your numbering and your own integration, the numbering is unmade rather than corrected: reset it, take the new state, and number again. The command detects that shape (duplicate numbers with no pending record left) and refuses with the recipe rather than guessing:

```
git reset --hard HEAD~1 && git merge <integrationBranch> && awf adr number
```

For `awf context`, bare directories provide tier-0 census, compact grouping, provenance, topic counts, and bounded pending orientation; bare exact, staged, and range-selected files additionally provide tier-1 `State`, `Touches`, and `Proofs` relationships from actual markers. The eight named facets expand directory relationships, non-direct authority, evidence, selectors, references, pending operations, or artifacts; only `artifacts` refines groups, and `--full` is their union. Output above 8,192 bytes retains secure caller-owned spill delivery.
