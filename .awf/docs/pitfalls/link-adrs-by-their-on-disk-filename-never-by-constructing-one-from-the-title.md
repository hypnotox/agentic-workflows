---
title: "Link ADRs by their on-disk filename, never by constructing one from the title"
domains: ["adr-system"]
---
An ADR's kebab filename is derived from its title at `awf new adr` time, but retellings
drift ("convention-parts-raw-not-templated" vs the actual "convention-parts-are-raw-input");
three invented link targets landed in ADR-0087's first draft (2026-07-10) and survived to
the verify pass because the ADR-0020 dead-link scan covers awf-managed *rendered* docs
only, not `docs/decisions/`. `ls docs/decisions/ | grep <number>` first, then link.
