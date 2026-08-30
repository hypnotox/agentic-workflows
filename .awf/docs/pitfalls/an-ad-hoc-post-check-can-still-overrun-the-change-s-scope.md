---
title: "An ad hoc post-check can still overrun the change's scope"
domains: ["tooling"]
---
Material plan post-checks now name their population, exclusions, lifecycle snapshot, and
expected terminal set or authorized residual findings. Checks outside that contract can
still read a wider unit than the rule governs, count grandfathered findings the change must
preserve, and demand an impossible zero. ADR-0115 exposed the shape: a heading-only cleanup
was checked by scanning whole append-only ADR files whose bodies intentionally retained
banned punctuation. For ad hoc checks and non-plan artifacts, scope the probe itself to the
governed unit (line, literal, heading, or population); explanatory prose around a wider probe
does not narrow what it measures.

Related decisions: [ADR-0115](../decisions/0115-ban-typographic-punctuation-substitutes-in-emitted-prose.md)
