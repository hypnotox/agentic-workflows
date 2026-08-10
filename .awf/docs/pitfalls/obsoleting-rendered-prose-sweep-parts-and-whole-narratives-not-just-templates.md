---
title: "Obsoleting rendered prose: sweep parts and whole narratives, not just templates"
domains: ["rendering"]
tags: ["convention-parts", "template-overlay"]
---
Fixing a template's default sentence does not reach a project whose convention part
*overrides* that section; awf's own `.awf/parts/agents-doc/awf-setup.md` kept ADR-0081's
obsoleted "disable them as a unit" instruction past the template rewrite, a sync, and three
review passes (caught only by the impl review's dogfood check, 2026-07-09). The same session
also left a domain current-state narrative self-contradicting by *appending* the correction
while an earlier clause still asserted the old behavior. When a change obsoletes prose, grep
the repo for the phrase being retired (`templates/`, `.awf/**/parts/`, and the rendered
outputs will surface every carrier, parts included) and rewrite stale clauses in place;
an appended correction beside a surviving old claim is worse than either alone.
