---
title: "An ad hoc empty scan still needs proof that the probe ran"
tags: ["verification-discipline"]
---
Material plan post-checks now require a success sentinel or checked exit status before empty
output proves absence. The residual hazard is an ad hoc probe outside that contract. A
compound command that errors mid-sequence can silently skip a later scan: during the
ADR-0082 brainstorm, a failed separator meant a second grep never ran, yet its missing output
was read as "no matches" until grounding found two hits. For interactive investigations and
other non-plan probes, run scans separately or check an explicit sentinel or exit status;
treat an unobserved success path as unrun, not clean.
