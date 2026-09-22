---
type: Project Topic
title: Release boundary
description: Release identity, pinned bootstrap behavior, native release verification, and publishing order.
paths:
  - 'internal/projector/VERSION'
  - '.awf/VERSION'
  - '.goreleaser.yaml'
  - '.github/workflows/release.yml'
  - 'tools/native-release-test/**'
  - 'tools/release-assets/**'
  - 'internal/docs/integration.md'
  - 'CHANGELOG.md'
  - 'MIGRATING-v0.50.md'
---

# Release boundary

`internal/projector/VERSION` is the binary release identity and the default pin rendered into `.awf/bootstrap.sh`. The same value is rendered into `.awf/VERSION` as a drift-checked record, not configuration or a binary-selection input. Binary SemVer does not describe topic or ownership compatibility.

The embedded `internal/projector/templates/downloader.sh.tmpl` owns platform selection, archive naming, checksum verification, extraction, and the `${XDG_CACHE_HOME:-$HOME/.cache}/awf/<version>/awf` cache. It renders the marked repository bootstrap, which honors the explicit `AWF_VERSION` override and prints a binary path, and the unmarked public `awf.sh`, which fixes this release's version and executes the verified binary with forwarded arguments. `AWF_VERSION=<target> ./awf render` explicitly selects a compatible update and rewrites the committed pin through the target binary.

Push the release commit on `main` and wait for its CI `gate` check to pass before pushing the version tag. GitHub's tag rule requires that hosted status on the tagged commit; a local gate run or an atomic branch-and-tag push does not satisfy it.

Build release candidates once, generate `dist/awf.sh` from the same tested source and embedded version, smoke the exact archives, checksums, and launcher on native platforms, and publish them unchanged. Native fixtures cover all embedded pages, five skills per harness, descriptor-free repeatable installation, preservation of authored agent files, version-record drift without binary-selection effects, empty-cache download, offline cache reuse, argument and exit propagation, and checksum rejection. Keep release history in root `CHANGELOG.md`.
