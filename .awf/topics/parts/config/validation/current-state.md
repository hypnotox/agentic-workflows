Config-tree validation rules: names, path globs, targets, and anchoring.

## Claims

### `invariant: commit-policy`

An optional commitPolicy block is structurally validated for an exact full-OID baseline, exact identity pairs, and, when signing is required, supported OpenSSH signer records; absent policy preserves existing behavior.
Backing: test

### `invariant: domain-name-validated`

Config validation rejects a domain name that contains a path separator (a forward or back slash) or a .. segment.
Backing: test

### `invariant: domain-path-globs-valid`

Working-tree and staged domain sidecars reject empty, duplicate, or malformed anchored path selectors when they load. Historical audit projection omits domain sidecars and does not validate them.
Backing: test

### `invariant: hooks-commands-resolvable`

Config validation for sync and check fails when `vars.gateCmd` is unset because the always-rendered hook payloads run the project gate; the error names the exact var to set. The always-rendered runner supplies every awf-verb fallback, so checkCmd and commitGateCmd carry no separate validation arm.
Backing: test

### `invariant: pathglob-anchored`

pathglob.Match is an anchored full-path doublestar match against a slash-separated repo-relative path: a bare star-dot-go pattern does not match cmd/a.go, a leading double-star form matches both a.go and cmd/a.go, and cmd followed by double-star matches every file under cmd/; no production matcher matches against a basename.
Backing: test

### `invariant: testglobs-anchored-validated`

Config validation rejects a testGlobs pattern that is malformed or contains no path separator, applying the same anchored-glob rule used for the invariant marker source globs.
Backing: test
