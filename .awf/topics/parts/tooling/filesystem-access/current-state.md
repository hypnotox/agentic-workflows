The filesystem package owns production root-confined access, while the dedicated testsupport fixture owns the distinct kernel-backed controlled fault source.

## Claims

### `invariant: single-production-handle`

`internal/filesystem` is the only production home for deliberately composed root-confined filesystem access; it exports one concrete handle and no provider-owned interface, while historical direct filesystem effects remain bounded candidates until a concrete conversion adopts the handle.
Origin: ADR-0216
Backing: test

### `invariant: root-confined-paths`

The production handle accepts only valid slash-relative paths beneath its selected `os.Root`, refuses absolute, parent, and escaping-symlink access, returns slash-relative walk paths without following directory symlinks, and preserves wrapped error identity.
Origin: ADR-0216
Backing: test

### `invariant: single-fault-source`

`internal/testsupport/fsfixture` is the only standard-library-only kernel-backed controlled filesystem fault source; it delegates unselected operations to its real root, preserves caller-supplied error identity, and cites the durable distinct-source decision at its implementation site. Production import exclusion remains governed by `tooling/test-infrastructure:production-never-imports-test-support`.
Origin: ADR-0216
Backing: test
