## Procedure

1. **Reflect on the session.** Gather its signals: the `awf-reviewing-impl` findings, the pitfalls or friction hit while implementing, and any issue that came up more than once.

2. **Record the worthy observations.** A first-occurrence pitfall or tricky area is *recorded* (rung 3 or 4 below), not promoted — the record is the memory the next retrospective reads to detect recurrence.

3. **Promote recurring, codifiable observations** to the strongest rung each can support (see the ladder). Verify a candidate genuinely recurs and is worth the effort before promoting.

4. **Update the adopter changelog.** If the effort changed anything adopter-facing — rendered template output, `awf` CLI behavior, or the config/lock schema — confirm it is recorded under the standing `## [Unreleased]` section of `changelog/CHANGELOG.md`, grouped by Breaking changes / Features / Bug fixes / Others (ADR-0041). Entries are supposed to land with the change itself; this step is the catch-net so a release cut never starts from an empty section.

5. **Note where each landed** in the session summary, so the loop is visible.

6. **Delete the effort's working-memory file** (`.awf/memory/<effort-slug>.md`), if one exists — the chain is complete and the ADR/plan/commits are the durable record. Working memory never outlives its effort.
