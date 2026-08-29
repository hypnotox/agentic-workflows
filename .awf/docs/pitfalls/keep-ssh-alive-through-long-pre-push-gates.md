---
title: Keep SSH alive through long pre-push gates
domains: ["tooling"]
tags: ["git-hooks", "release-pipeline"]
---
An SSH push opens its transport before the pre-push hook completes, so a long full gate can leave the connection idle until the remote closes it after verification. Keep the complete hook and use per-command SSH server-alive settings for the push; do not bypass the gate or mistake intentional conservative range diagnostics for the transport failure.
