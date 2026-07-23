The read-only orientation surfaces: awf context, awf topic, describe, and uncovered reporting.

## Claims

### `invariant: context-adr-operation-projection`

An explicit governed ADR path projects its parsed identity, lifecycle, mutability, non-authoritative prose role, and declaration-ordered Proposed, Remaining, Applied, or Canceled operation progress from canonical application batches; full mode adds only operation-linked current or removed claim history and marker detail.
Origin: ADR-0148
Backing: test

### `invariant: context-applicability-navigation`

Topic and context applicability share one evidence model that lists owning-domain selectors and topic selectors separately and states that both must match; awf topic --coverage reports the sorted concrete matched paths and marker sites, while context reports the matched-path count with a coverage drilldown, and neither claims symbolic glob intersection.
Origin: ADR-0148
Backing: test

### `invariant: context-default-excludes-history`

Concise path context renders full detail only for claims selected directly by exact-path state, touches-state, or invariant proof markers, reports every remaining rostered claim as an ID with a detail-omission line and topic drilldown, and never expands Implemented ADRs, historical plans, referenced claim bodies, or unrelated ADR history.
Origin: ADR-0148
Backing: unbacked
Verify: On a fixture with claim provenance, ADR tags and relations, linked plans, and claim references, grouped concise context reports the claim-ID roster and direct-claim detail but differs from both context --full and explicit awf topic <claim-id> --history output.

### `invariant: context-concise-projection`

Context assembles one topic entry per applicable topic per invocation: concise entries carry the uncapped current claim-ID roster, the full detail of exactly the marker-selected direct-claim union, and, when any rostered claim's detail is omitted, an explicit detail-omission line with the topic drilldown.
Origin: ADR-0153
Backing: test

### `invariant: context-full-authority-packet`

`context --full` renders every current claim's full detail and pending operations once per applicable topic with no detail-omission line, from the same non-recursive semantic model as the concise projection; managed complete-authority callers request `--full` explicitly.
Origin: ADR-0148
Revised-by: ADR-0153
Backing: test

### `invariant: context-known-artifact-navigation`

Known config, lock, manifest, template, convention-part, authored-data, topic-metadata, claim-part, decision-record, and managed-output artifacts receive deterministic role, source, output, and navigation attribution from loaded authorities rather than path-lookalike heuristics.
Origin: ADR-0148
Backing: test

### `invariant: context-output-parity`

The human-readable and --json renderings of `awf context` consume the same selected concise or full semantic result, so serialization changes presentation only and concise JSON contains no hidden full block.
Origin: ADR-0148
Backing: test

### `invariant: context-path-attribution`

Context preserves normalized request queries separately from sorted effective paths, records directory expansion status, and emits each unique effective path once with every sorted request that selected it and non-null result collections; a path's topic collection is an attribution of topic IDs and direct-claim IDs, while topic authority lives in the sorted invocation-level topic collection.
Origin: ADR-0148
Backing: test

### `invariant: context-path-classification`

Each effective path receives exactly one precedence-ordered classification: outside repository, nested adopter, generated output, symlink, context ignored, not found, covered, or eligible unowned; symlinks remain inert and report only lexical target containment.
Origin: ADR-0148
Backing: test

### `invariant: context-read-only`

The `awf context` command assembles each query from one selected working-tree or immutable index universe and writes no config, lock, output, or cache; staged config, lock, topic, marker, path, and artifact inputs never mix with dirty working bytes.
Origin: ADR-0148
Backing: test

### `invariant: context-static-fallback`

Run outside an adopted tree, where no config file is present, concise and full `awf context` both degrade to a successful static empty answer that states live classification and authority require adoption, mirroring `awf config`.
Origin: ADR-0148
Backing: test

### `invariant: describe-read-only`

awf init --describe prints the var descriptor set as JSON to stdout and creates no files under the target root.
Origin: ADR-0148
Backing: test

### `invariant: uncovered-collapses-directories`

In the coverage report, a directory all of whose scanned tracked descendants are owned by no domain is reported as that single topmost directory with a trailing slash, never as its individual files.
Origin: ADR-0148
Backing: test

### `invariant: uncovered-output-parity`

The human-readable and JSON renderings of the coverage report present the same uncovered and unowned sets, because both are printed from one assembled result.
Origin: ADR-0148
Backing: test
