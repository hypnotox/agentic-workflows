---
title: "Raw-byte historical-record surgery must bound every scan to the frontmatter window"
domains: ["config"]
---
Frozen migrations that edit historical decision frontmatter as raw bytes must bound every
regex scan to the frontmatter window. Two window bugs bit during the generation-10 migration. First,
the last frontmatter line's terminating newline IS the newline that opens the closing
fence (`\n---`), so a window ending at the fence index excludes it, and a regex demanding
a trailing newline silently misses a key that happens to be the last frontmatter line;
the fix is the +1 window (`strings.Index(raw[3:], "\n---") + 3 + 1`, safe after
the migration's structural preflight proves the fence exists). Second, a scan not bounded to the window at all
matched a column-0 `related:` line in a decision *body* (a quoted frontmatter example) and
would have silently body-edited the target where the loud no-line failure was owed; the
implementation review caught it. When touching frontmatter by regex, compute the window
once and bound every scan to it; preserve regression tests for both edges with the frozen
migration.
