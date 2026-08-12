---
title: "When retiring a config key, handle historical writers"
domains: ["config"]
tags: ["schema-migration", "verification-discipline"]
---
Add every newly retired key to the production retired-key removal ledger consumed by the
independent forward-port census and proof. If an older migration writes the key, strip it while
loading historical migration input as well; current-schema forward-porting alone does not protect
that adopter path.
