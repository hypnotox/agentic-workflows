## Dedup rule

When multiple lenses flag the same `location` for the same underlying issue, emit one finding and keep the most specific `suggested_fix`. Do not discard findings with different locations even if the root cause is the same.

## Review procedure

1. {{ with .data.readStep }}{{ . }}{{ else }}Read the artifact in full. Read every doc it references by name.{{ end }}
1. Run all universal lenses plus any project-specific focus items.
1. Dedup overlapping findings.
1. Classify each finding as mechanical / reasoned / user-decision.
1. Apply mechanical and reasoned fixes directly{{ if .data.fixesAsCommits }} as new commits; run {{ with .vars.gateCmd }}`{{ . }}`{{ else }}the project's gate{{ end }} before each fix commit{{ end }}; note rationale for reasoned fixes.{{ if .data.fixesAsCommits }} Never `--amend` prior commits.{{ end }}
1. Re-review the updated artifact. Exit when: (a) no findings, (b) remaining findings are wording-only, or (c) the artifact is clean by inspection. **3-round soft cap**: after three rounds with remaining structural findings, surface the current state as `user-decision` findings and stop looping without explicit direction.
1. Emit the digest (see format below).

## Digest format

```
{{ with .data.digestLabel }}{{ . }}{{ else }}Review{{ end }} summary:
{{ with .data.digestSummary }}{{ . }}{{ else }}- Summary: <one line>{{ end }}

{{ with .data.digestLabel }}{{ . }}{{ else }}Review{{ end }} review complete (R rounds, N lenses, M findings).
- Mechanical fixes applied: K
- Reasoned fixes applied: L
- User decisions needed: P
  1. <question>
```

Target ~80 words for the {{ with .data.digestLabel }}{{ . }}{{ else }}review{{ end }} summary (range 50–100 words). When `P = 0`, the summary block is optional and the chain auto-proceeds. When `P > 0`, surface the digest and wait for the user's decisions before continuing.
