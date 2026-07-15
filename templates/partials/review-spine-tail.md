## Dedup rule

When multiple lenses flag the same `location` for the same underlying issue, emit one finding and keep the most specific `suggested_fix`. Do not discard findings with different locations even if the root cause is the same.

## Review procedure

1. {{ with .data.readStep }}{{ . }}{{ else }}Read the artifact in full. Read every doc it references by name.{{ end }}
1. Run all universal lenses plus any project-specific focus items.
1. Dedup overlapping findings.
1. Classify each finding as mechanical / reasoned / user-decision.
1. Emit the digest (see format below). Report findings only: do not edit, commit, or re-review the artifact; the dispatching skill applies fixes and runs a single verify pass.

## Digest format

```
{{ with .data.digestLabel }}{{ . }}{{ else }}Review{{ end }} summary:
{{ with .data.digestSummary }}{{ . }}{{ else }}- Summary: <one line>{{ end }}

{{ with .data.digestLabel }}{{ . }}{{ else }}Review{{ end }} review complete (N lenses, M findings).
- Findings by classification: mechanical K, reasoned L, user-decision P
  1. <user-decision finding, if any>
```

Target ~80 words for the {{ with .data.digestLabel }}{{ . }}{{ else }}review{{ end }} summary (range 50–100 words). This digest reports findings; the dispatching skill applies the mechanical and reasoned fixes, escalates the user-decision findings, and runs a single verify pass.
