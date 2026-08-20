---
title: Resolve review ranges from Git rather than transcribing SHAs
tags: ["verification-discipline"]
---
A review brief can name a plausible but nonexistent full commit SHA when an abbreviated SHA is expanded from memory. The reviewer may still inspect the working tree and return useful findings, but its claimed range and freshness are unverifiable, so the assurance must be renewed.

Resolve every review boundary immediately before dispatch with `git rev-parse <ref>`, then make the named range prove itself with `git diff --name-only <base>..<head>` or the required audit command. Copy the command output rather than completing a SHA by hand. After a merge or settlement commit, resolve both boundaries again before renewed assurance.
