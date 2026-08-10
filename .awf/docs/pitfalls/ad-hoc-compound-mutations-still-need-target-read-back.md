---
title: "Ad hoc compound mutations still need target read-back"
tags: ["verification-discipline"]
---
Material plan post-checks now require reading back every target after a compound mutation.
Interactive and otherwise unplanned command chains remain exposed: a failed `cd <dir> &&
<edit>` can skip the edit while a later unchained command succeeds, making the block appear
green without its intended side effect. An `invariants:` append vanished this way during the
ADR-0090 session and only a target read-back exposed it. Outside the governed plan contract,
inspect each mutation target rather than trusting the compound command's overall result.
