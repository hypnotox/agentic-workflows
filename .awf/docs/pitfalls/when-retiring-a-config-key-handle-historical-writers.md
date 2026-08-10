---
title: "When retiring a config key, handle historical writers"
domains: ["config"]
tags: ["schema-migration", "verification-discipline"]
---
Add every newly retired key to both the retired-key removal ledger and the forward-port proof.
If an older migration writes the key, strip it while loading historical migration input as
well; a current-schema forward-port branch alone does not protect that adopter path.
