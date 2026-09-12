---
paths:
{{range .Selectors}}  - {{quote .}}
{{end}}---

# {{.Name}}

Adapt or omit sections. Remove prompts and content that does not help this document serve its purpose.

State the focused purpose of this topic.

## Current behavior and structure

Explain current behavior, ownership boundaries, and relationships that matter to future changes, not an inventory of files and functions.

## Constraints and practical implications

Explain current constraints and what they mean for changes in this area. Keep useful local explanations and link active ADRs rather than repeating their full rationale.
