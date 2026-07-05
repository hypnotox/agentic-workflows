## Invariant tagging

Give each machine-enforceable Invariants bullet an explicit slug —
``- `inv: <slug>` — …`` — and add a matching `` `invariant: <slug>` `` comment to a test that
exercises it. {{=awf:invariantMarkerSentence}} `awf check` (here `./x check`) and the standalone `awf invariants` (`./x invariants`)
fail when an **Implemented** ADR has a tagged slug with no backing test. Proposed/Accepted ADRs
are not yet enforced (tests land with implementation); Superseded ADRs are skipped. Bullets
without a slug are textual contracts, not machine-checked.
