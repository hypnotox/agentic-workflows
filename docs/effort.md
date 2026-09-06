# Efforts, plans, decisions, and worktrees

AWF provides optional local effort memory and create-only Markdown scaffolds. It does not decide when work is ready, interpret authored content, manage Git, or discover worktrees.

## Keep effort memory

Create an effort from the primary checkout when continuity materially helps:

```sh
./awf effort new <slug>
```

The command creates ignored local memory at `.awf/efforts/<slug>/memory.md`. Keep the intended outcome and success criteria, current state, next actions, artifact references, selected attributed decision evidence, and completion evidence current. Distinguish direct quotations, summaries, proposals, and agreed decisions. Retain only evidence useful to continuing the work, not complete sessions. Detailed criteria may live in a referenced plan.

Use the same primary checkout to inspect or finish the effort:

```sh
./awf effort list
./awf effort show <slug>
./awf effort finish <slug>
```

`finish` only moves the complete resident to `.awf/effort-archive/<slug>` without replacement. It does not judge readiness, compare criteria, interpret memory, or clean Git state.

## Use plans and ADRs independently

Plans and architecture decision records are separate optional aids. Complexity can warrant a plan; a material durable choice can warrant an ADR; an effort can need either, both, or neither.

A plan scaffold requires an existing active effort and stays beside its memory:

```sh
./awf plan new <effort-slug>
# .awf/efforts/<effort-slug>/plan.md
```

An ADR can stand alone and does not require an effort:

```sh
./awf adr new <slug>
# docs/decisions/<slug>.md
```

Both commands create files exclusively and never replace an existing destination. The files are author-owned Markdown. AWF does not parse them, include them in projection, or perform a Git action. You may also author either file directly without using the scaffold.

## Keep files in the correct checkout

Without a worktree, the primary and implementation checkout are the same directory. With a worktree:

- keep effort memory and plans in the primary checkout;
- keep the ADR and implementation in the implementation checkout;
- record both locations in memory and handoffs;
- run effort and plan commands from the primary checkout root;
- run ADR and implementation Git commands from the implementation checkout root;
- run worktree creation, integration, removal, and branch cleanup from the primary checkout root.

An absolute path to a repository wrapper does not change the command's working directory. AWF does not create or inspect worktrees. When isolation helps, manage one with native Git:

```sh
git worktree add -b awf/<slug> .awf/worktrees/<slug>
```

Worktrees are optional; this location is a convention rather than AWF-managed topology.

## Complete decisions and efforts

An ADR has one working copy in `docs/decisions`. Commit the chosen decision and its rationale during the effort. After implementation is verified, incorporate its durable substance and useful rationale into the applicable current topics. Remove the ADR in a later implementation/topic-update commit, and preserve both commits through the repository's ordinary non-squash integration.

Retrieve the historical decision when needed:

```sh
git log --full-history -- docs/decisions/<slug>.md
git show <decision-commit>:docs/decisions/<slug>.md
```

Before finishing an effort, compare actual results with its success criteria. Record verification and report unmet criteria, deviations, or a changed goal rather than rewriting the original goal as success. Update applicable topics and retire integrated ADRs as described above.

After non-squash integration, clean up an isolated worktree explicitly from the primary checkout, then finish the effort:

```sh
git worktree remove .awf/worktrees/<slug>
git branch -d awf/<slug>
./awf effort finish <slug>
```
