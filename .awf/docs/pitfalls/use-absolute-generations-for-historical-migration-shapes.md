---
title: "Use absolute generations for historical migration shapes"
domains: ["config"]
tags: ["schema-migration", "version-authority"]
---
A tree layout detected by historical shape belongs to a fixed generation, not
`Current() +/- k`. Verify the absolute mapping against the migration registry whenever the
registry grows.
