## Layout

Tests live beside their package (`internal/almanac`, `internal/schedule`): model
tests pin clamping and the polar collapse; schedule tests pin table shape.
`./x gate` runs them all with `go vet`; the invariant-backing comments under
`./internal/...` are checked by `./awf invariants`.
