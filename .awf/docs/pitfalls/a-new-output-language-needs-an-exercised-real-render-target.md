---
title: "A new output language needs an exercised real render target"
domains: ["rendering"]
tags: ["editable-sections", "provenance-markers", "command-runner"]
---
ADR-0101 fixed the shell runner gaps that Markdown-only tests missed: provenance comments are
syntax-aware and executable targets retain their mode. The residual hazard begins when a
rendering primitive gains another output language. Render a real target in that language and
exercise it as an adopter will (execute, parse, or validate syntax) before finalizing the
design. Existing Markdown and shell coverage cannot prove the new format's comment syntax,
file mode, escaping, or other language-specific properties.
