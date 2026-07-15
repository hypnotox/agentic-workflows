## Invariant tagging

Declare each machine-enforceable Invariants bullet with an explicit slug in one of two forms: a
backed ``- `invariant: <slug>` — …`` for a test-proven property, or an
``- `unbacked-invariant: <slug>` — …. **Verify:** …`` for a reasoned contract with no automatic test
(the `Verify:` note says how to confirm it by hand). Back a backed slug with a matching
`` `invariant: <slug>` `` proof comment on a test ({{=awf:invariantMarkerSentence}}) and, when
`invariants.testGlobs` is configured, that proof must live in a test file. A separate advisory
`` `touches-invariant: <slug> — <note>` `` marker records a related production site and never backs.
`awf check` (here `./x check`) and the standalone `awf invariants` (`./x invariants`) fail when an
**Implemented** ADR declares a backed slug with no proof, declares an unbacked slug that a proof
marker contradicts, or declares an unbacked slug with no `Verify:` note. Proposed/Accepted ADRs are
not yet enforced (tests land with implementation); Superseded ADRs are skipped. Bullets without a
slug are textual contracts, not machine-checked.
