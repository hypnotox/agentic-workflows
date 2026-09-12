---
paths:
  - 'internal/projector/VERSION'
  - '.goreleaser.yaml'
  - '.github/workflows/release.yml'
  - 'tools/native-release-test/**'
  - 'tools/release-assets/**'
  - 'internal/docs/integration.md'
  - 'CHANGELOG.md'
  - 'MIGRATING-v0.50.md'
---

# Release boundary

`internal/projector/VERSION` is the binary release identity and the default pin rendered into `.awf/bootstrap.sh`. Binary SemVer does not describe source compatibility and is not stored in project metadata.

One embedded shell template owns platform selection, archive naming, checksum verification, extraction, and the `${XDG_CACHE_HOME:-$HOME/.cache}/awf/<version>/awf` cache. It renders the marked repository bootstrap, which honors the explicit `AWF_VERSION` override and prints a binary path, and the unmarked public `awf.sh`, which fixes this release's version and executes the verified binary with forwarded arguments. `AWF_VERSION=<target> ./awf render` explicitly selects a same-format update and rewrites the committed pin through the target binary.

Build release candidates once, generate `dist/awf.sh` from the same tested source and embedded version, smoke the exact archives, checksums, and launcher on native platforms, and publish them unchanged. Native fixtures cover empty-cache download, offline cache reuse, argument and exit propagation, and checksum rejection. Keep release history in root `CHANGELOG.md`.
