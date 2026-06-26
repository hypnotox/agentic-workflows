## Current state

The render engine is a marker-section overlay (`<!-- awf:section -->`) layered onto Go `text/template` with `missingkey=zero`, guarded by a hard publication-safety check that rejects any unresolved-variable placeholder. Doc paths are awf-given via the `.layout` namespace (never `.vars`); skills cite docs through `.layout.docs.*`/`.layout.workflowRef`, and a doc-gated skill is suppressed when its doc is disabled. Catalog docs ship static default content with a per-doc section taxonomy; domain docs are the one data-driven exception, injecting a generated index as forced body.
