## Current state

Pi renders native discoverable skills, the subagent extension, and the lifecycle-independent handoff extension. Every enabled standard and local skill uses the normal `.pi/skills/<prefix>-<name>/SKILL.md` layout; no router or hidden workflow body participates in selection.

The topic producer renders each valid pair to `<docsDir>/topics/<domain>/<topic>.md`, emits a title-and-summary-sorted `<docsDir>/topics/<domain>/index.md`, and adds compact topic navigation to the owning domain page. Generated output follows the normal manifest, drift, and prune lifecycle.

The render engine overlays authored convention parts onto embedded templates with publication-safe missing-key rendering. Catalog workflow profiles provide kind, purpose, trigger, and optional advisory neighbors. Every enabled skill is independently discoverable; advisory neighbors do not create enablement edges or required transitions.

The catalog-derived mandatory Maintainable Code Design guide renders as an extensible plain singleton with a document-map artifact.

Pi extension entrypoints check their required runtime APIs before registering hooks. Handoff accepts exactly one effort-owned memory path, `.awf/efforts/<slug>/memory.md`, validating the slug grammar, lexical containment, no-follow components, ownership, the bounded singly-linked leaf, an `Effort: <slug>` first line, and unchanged identity across validation, alongside bounded kickoff input. It remains lifecycle-independent: it does not parse effort state, select or assign an effort, mutate memory, or invoke the awf binary.
