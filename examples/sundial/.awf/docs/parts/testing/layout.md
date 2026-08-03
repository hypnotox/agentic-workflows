## Layout

Tests live beside their package (`internal/almanac`, `internal/schedule`): model
tests pin clamping and the polar collapse; schedule tests pin table shape.
`./x gate` runs them all with `go vet`; `awf check repo` validates the
invariant-backing comments under `./internal/...` as part of current-state authority. When enabled, commit-policy previews are tested through actual Git commit facts and retain disabled-policy success without claiming hook enforcement.
