---
title: "Raw-byte ADR surgery must bound every scan to the frontmatter window"
domains: ["config"]
tags: ["schema-migration", "adr-parsing"]
related: [120]
---
The generation-10 retirement-tokens migration and the `adr-retired-key` check edit or
refuse ADR frontmatter as raw bytes, by regex. Two window bugs bit in one session. First,
the last frontmatter line's terminating newline IS the newline that opens the closing
fence (`\n---`), so a window ending at the fence index excludes it, and a regex demanding
a trailing newline silently misses a key that happens to be the last frontmatter line;
the fix is the +1 window (`strings.Index(raw[3:], "\n---") + 3 + 1`, safe after
`adr.ParseDir` proved the fence exists). Second, a scan not bounded to the window at all
matched a column-0 `related:` line in an ADR *body* (a quoted frontmatter example) and
would have silently body-edited the target where the loud no-line failure was owed; the
implementation review caught it. When touching frontmatter by regex, compute the window
once and bound every scan to it - both edges carry regression tests in
`internal/migrate/retirementtokens_test.go`.
