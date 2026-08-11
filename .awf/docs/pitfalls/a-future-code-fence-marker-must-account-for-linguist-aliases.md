---
title: "A future code-fence marker must account for Linguist aliases"
domains: ["adr-system"]
tags: ["plan-taxonomy", "commit-gate"]
related: [111]
---
ADR-0111 settled the existing ```commit marker: `commit` remains a GitHub Linguist alias with
commit-message highlighting, the check validates it by default, and ```commit awf-ignore is
the explicit display-only opt-out. The residual hazard applies when choosing another
fence-based marker. Check its first info-string token against Linguist's language aliases
(`lib/linguist/languages.yml`) before adopting it, preserve useful highlighting when possible,
and prefer checked-by-default semantics with an explicit opt-out over a namespaced token that
silently loses highlighting and fails open.
