## Finding schema

Every finding must have all six fields:

```
{
  focus:          string,   // which lens flagged this
  severity:       "blocker" | "concern" | "nit",
  location:       string,   // file:line, quoted phrase, section name, or "<path> (missing)"
  issue:          string,   // one-sentence summary of what is wrong
  suggested_fix:  string,   // concrete fix or escalation note
  classification: "mechanical" | "reasoned" | "user-decision"
}
```

Every finding must cite a **specific location**; "the {{ with .data.reviewSubject }}{{ . }}{{ else }}artifact{{ end }} generally" is not a valid location.

## Classification rules

Classify by what acting on the finding requires, not by severity:

- **mechanical** — the answer is unambiguous from existing rules, docs, or code; the fix is direct.
- **reasoned** — a good answer can be reached by reading the relevant code or docs, but judgment is required; a one-line rationale is warranted. For deferred-to-follow-up cases, the rationale is prefixed with `Deferred to <name>:`.
- **user-decision** — a genuine design fork or unresolved ambiguity that should not be decided unilaterally; escalate.

Severity is informational only; the dispatching skill routes by classification kind.
