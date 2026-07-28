## Current state

Pi renders native discoverable skills, the subagent extension, and the effort-independent handoff extension. Every enabled standard and local skill uses the normal `.pi/skills/<prefix>-<name>/SKILL.md` layout; no router or hidden workflow body participates in selection.

The topic producer renders each valid pair to `<docsDir>/topics/<domain>/<topic>.md`, emits a title-and-summary-sorted `<docsDir>/topics/<domain>/index.md`, and adds compact topic navigation to the owning domain page. Generated output follows the normal manifest, drift, and prune lifecycle.

The render engine overlays authored convention parts onto embedded templates with publication-safe missing-key rendering. Catalog workflow profiles provide kind, purpose, trigger, and optional advisory neighbors. Every enabled skill is independently discoverable; advisory neighbors do not create enablement edges or required transitions.

Pi extension entrypoints check their required runtime APIs before registering hooks. Handoff accepts optional confined regular-file memory and bounded kickoff input, but does not parse effort records, select effort state, or invoke the awf binary.
