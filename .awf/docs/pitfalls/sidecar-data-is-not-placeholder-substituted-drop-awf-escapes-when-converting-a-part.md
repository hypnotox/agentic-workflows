---
title: "Sidecar `data` is not placeholder-substituted, drop `{{=awf:...}}` escapes when converting a part"
domains: ["rendering"]
related: [89, 99]
---
A raw convention part is run through awf's `{{=awf:...}}` sandbox substitution before Go templating
(ADR-0057), so a part that *documents* a placeholder token backslash-escapes it (`\{{=awf:key}}`)
to render the literal token. A sidecar-derived doc's `data` value is different: the transform hands
it to the template as a plain string spliced in via Go `{{ . }}`, with no awf-placeholder pass over
it; so a `\{{=awf:...}}` escape copied verbatim from a raw part into `data.<key>` renders the
backslash literally, and an unescaped `{{=awf:...}}` renders as the literal token (never substituted).
This bit the ADR-0099 pitfalls conversion: the 40-entry hand-migration copied entries.md's escaped
tokens straight into `data.pitfalls`, and the backslashes leaked into the rendered doc; `awf check`
stayed clean (the output is exactly what the data says), so only diffing the rendered file against
the deleted part exposed it. When converting a part-based doc to the sidecar model, strip the
`\{{=awf:` escapes; verify by rendering and reading the output, not by trusting a clean check.
