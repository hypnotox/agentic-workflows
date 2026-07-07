Project jargon and what each term means; start here when a term is unfamiliar. Sorted by term.

| Term | Meaning |
|---|---|
| promotion ladder | `awf-retrospective`'s routing rule for a recurring, codifiable observation: promote it to the strongest rung it can support — an invariant (via ADR, `inv:` slug plus backing comment), a gate test or lint rule, a code-review focus item, or a pitfalls note (ADR-0067). |
| resync | The plan↔ADR reconciliation pass (`awf-reviewing-plan-resync`) that runs before implementation when both a plan and at least one ADR exist; it re-runs the scope-completeness and doc-currency lenses to catch plan-vs-finalised-ADR(s) drift and loops until they converge. |
| retrospective | The terminal workflow-chain step (`awf-retrospective`), run by the main thread after `awf-reviewing-impl`: it reflects on the session, records worthy observations, and promotes recurring findings up the promotion ladder toward deterministic checks (ADR-0067). |
| working memory | The per-effort session-state file `.awf/memory/<effort-slug>.md` — evolving design brief, chain position (`Phase:`/`Next:` lines), handoff log — checkpointed by the chain skills and kept out of version control by the always-rendered self-ignoring `.awf/memory/.gitignore`; never committed or cited by an ADR, plan, or commit message, and deleted by the retrospective when the chain terminates (ADR-0069). |
