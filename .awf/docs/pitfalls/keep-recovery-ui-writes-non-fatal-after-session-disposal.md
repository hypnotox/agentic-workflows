---
title: "Keep recovery UI writes non-fatal after session disposal"
domains: ["rendering"]
---
In extension failure recovery, swallow UI-write failures so the original error remains
authoritative, and run the real-Pi lane for changes that cross session teardown. The fake
harness does not model disposed-session UI behavior.
