# invariants

## Current state

Each machine-enforceable ADR Invariant bullet carries an `inv: <slug>` tag backed by a `<marker> invariant: <slug>` comment in a source matching the project's configured `invariants.sources`. The checker is language-agnostic (filename globs + a literal marker) and enforce-by-default: an Implemented ADR with an unbacked — or unconfigured — slug fails `awf check`. Backing is opt-in per bullet; untagged bullets remain textual contracts.


## Decisions

### Implemented

- [ADR-0007: Invariant-Backing Tooling — `inv:` Tags and the `awf invariants` Checker](../decisions/0007-invariant-backing-tooling.md)
- [ADR-0008: Language-Agnostic Invariant Backing and a Polyglot Standard](../decisions/0008-language-agnostic-invariant-backing.md)

