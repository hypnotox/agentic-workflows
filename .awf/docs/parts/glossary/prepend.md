Project jargon and what each term means; start here when a term is unfamiliar. Sorted by term.

- **Anchor claim:** The ability of a ledger event to be the target of a payload anchor reference. Owned exclusively by `trajectory_closed` events keyed on `payload.anchorId`, declared in the protocol descriptor's required `anchorClaimKinds` vocabulary; references resolve causally forward only, and ambiguity within the claiming set is an `ambiguous-anchor` violation. The envelope `piAnchorId` is observation-location metadata, never a claim.
- **Applicability evidence:** Separate owning-domain and topic selectors and the rule that both must match; the concrete current matched paths and marker sites live in `awf topic <id> --coverage`, while rendered docs carry selectors only and context carries the matched-path count. It is not symbolic glob intersection.
- **Artifact role:** One closed attribution kind assigned from loaded project authorities, with source, output, and navigation links.
- **Concise context:** The default orientation projection: each applicable topic once with its uncapped claim-ID roster, the full detail of directly marked claims, and an explicit detail-omission line with drilldown, plus per-path classification and attribution.
- **Effective path:** One unique file path selected literally or by request-directory expansion and classified independently.
- **Full authority packet:** The untruncated `awf context --full` projection containing every applicable current claim once per topic, with backing, direct sites and references, scopes, matched-path counts with coverage drilldowns, pending operations, and artifact navigation.
- **Primary classification:** Exactly one precedence-ordered context status for an effective path, such as covered, generated output, symlink, or not found.
- **Request path:** A normalized user or Git-selected query retained separately from the effective paths it selects.
