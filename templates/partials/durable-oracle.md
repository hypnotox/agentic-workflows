Every behaviour-changing fix requires the strongest practical durable oracle. The normal and preferred path is an automated regression test observed failing for the right reason and then passing. When that path is impractical, state a concrete reason, preserve or improve verification strength, and retain the strongest safe, reproducible alternative. Never weaken expected behaviour. Never weaken verification strength. Fix the root cause rather than the symptom.

Use this evidence order as guidance, not a requirement to mechanically attempt every earlier option:

1. An automated regression test observed red then green.
2. A deterministic integration or reproduction harness.
3. A contract or invariant test that directly exercises the failure.
4. Use scripted, reproducible manual verification with recorded inputs and expected result.
5. An explicit explanation of why durable automation is unavailable, plus the strongest safe evidence that can be retained.

For a nondeterministic race, stress or invariant evidence may be the strongest practical oracle. For a destructive migration defect, use safe fixture or dry-run evidence rather than unsafe reproduction. An alternative is legal because the preferred path is impractical, not merely inconvenient, and its reason and retained evidence must make any verification-strength judgment reviewable.
