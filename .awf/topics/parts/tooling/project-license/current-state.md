The project-license contract owns the canonical license bytes, public license references, and release-package inclusion. Its verifier distinguishes project licensing from third-party dependency metadata.

## Claims

### `invariant: project-license-agpl`

The root project uses the exact canonical AGPL-3.0-only license bytes and matching README badge and footer, and every GoReleaser archive includes that license. The release-time verifier rejects obsolete project MIT references while excluding dependency-license metadata and retained third-party notices from the project-license classification.
Origin: ADR-0227
Backing: test
