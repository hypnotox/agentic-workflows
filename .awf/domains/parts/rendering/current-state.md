## Current state

Pi renders native discoverable skills, the subagent extension, and the lifecycle-independent handoff extension. Workflow templates exclusively own mandatory checkpoint persistence, approval stops, safe resumability, and truthful handoff-log timing; Pi replacement is discretionary only after an eligible persisted boundary. Every enabled standard and local skill uses the normal `.pi/skills/<prefix>-<name>/SKILL.md` layout; no router or hidden workflow body participates in selection.

The topic producer renders each valid pair to `<docsDir>/topics/<domain>/<topic>.md`, emits a title-and-summary-sorted `<docsDir>/topics/<domain>/index.md`, and adds compact topic navigation to the owning domain page. Generated output follows the normal manifest, drift, and prune lifecycle.

The render engine overlays authored convention parts onto embedded templates with publication-safe missing-key rendering. Resident-root policy lives in `internal/resident` as the single production home of the root table, the resident path and kind predicates, and the anchoring `Roots` value the render core consumes; template identity derives only from the catalog and the kind-descriptor, singleton, and target declaration tables (ADR-0195). Catalog workflow profiles provide kind, purpose, trigger, and optional advisory neighbors. Every enabled skill is independently discoverable; advisory neighbors do not create enablement edges or required transitions.

The catalog-derived mandatory Maintainable Code Design guide renders as an extensible plain singleton with a document-map artifact.

Pi extension entrypoints check their required runtime APIs before registering hooks. Handoff remains lifecycle-independent: workflow templates own effort checkpoint, approval, safe-point, reorientation, and handoff-log policy; bounded kickoff carries instructions while replacement mechanics own no memory policy.
