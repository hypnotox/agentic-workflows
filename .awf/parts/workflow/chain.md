### Triggers

| Trigger | Fires when |
|---|---|
| Brainstorming | A material choice or clarification needs resolution. |
| Continuity/effort | Durable continuity materially helps. |
| Grounding | Repository premises are broad or uncertain. |
| ADR | A choice is load-bearing or active claims change. |
| Plan | Sequencing, coordination, or resumability materially helps. |
| Implementation review | Independent assurance has value. |

### Boundary and ordering


- Use any native skill when its purpose fits.

- Evaluate brainstorming, continuity/effort, grounding, ADR, plan, and implementation-review need independently at intake, then re-evaluate only affected triggers when material facts change. A plan fires when sequencing, coordination, or resumability materially helps.

- A plan records and operationalizes approved choices rather than inventing speculative structure, checks, or work.

- Full ordinary plan review verifies every ADR from parsed plan-level `adrs:` links.

- After a substantive ADR amendment, review the ADR first and then every deterministically linked Proposed plan; if implementation started, reassess completed affected phases and renew assurance where needed.

- A plan correction that would contradict linked authority returns to ADR amendment and review first.

- Implementation review fires when independent assurance has value and uncertainty resolves toward review.

- Universal repository authority, documentation, verification, and commit obligations apply regardless of which mechanisms fire.

- An unresolved material decision, not the act of mutating production code, is what brainstorming stops for; it alone presents a proportionate outline and obtains explicit approval, and a routine change whose protected contract is already settled proceeds without that stop.

- Retained conversation, user-provenance effort Decision-log evidence, or an explicit request to execute a named plan whose Architecture summary supplies the outline each establish that boundary; delegated owners consume the parent-supplied boundary without another approval interaction.

- The approval boundary precedes ADR and plan authoring, while effort, grounding, ADR, plan, and review triggers remain independent.

- Core and Full are governance footprints of this one workflow, not different standards of correctness, autonomy, maintainability, or review quality; they add no depth controls, rigor modes, routers, classifiers, or runtime policy knobs.

### Ownership


- `effort-workflow` alone owns autonomous continuity-triggered effort creation through checkpoints, integration, divergence handling, pending artifact closure, managed-topology removal, retrospective routing, and finish.

- Work without continuity need carries no effort or memory.

- Existing efforts resume under fixed identity only inside their outcome, with one user-managed writer and report-only children.

- A distinct active effort is never silently reused: reason whether it is kept with a resumable checkpoint or discontinued after necessary context transfer, inspected safe cleanup or explicit intentional discard, and ordinary archival finish.

- `reviewing-impl` owns assurance only and returns effort-backed work to `effort-workflow`; after assurance settles or is explicitly skipped, the effort-free execution parent owns any applicable deferred ADR/plan terminal closure.

- Divergent integration activates review before topology removal.

- No line count, artifact type, bundled work label, classifier, checklist, router, or new runtime mechanism selects the workflow.
