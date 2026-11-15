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
  - 'internal/docs/migrate.md'
---

# Release boundary

`internal/projector/VERSION` is the binary release identity and the default pin rendered into `.awf/bootstrap.sh`. The same value is rendered into `.awf/VERSION` as a drift-checked record, not configuration or a binary-selection input. Binary SemVer does not describe topic or ownership compatibility.

The embedded `internal/projector/templates/downloader.sh.tmpl` owns platform selection, archive naming, checksum verification, extraction, and the `${XDG_CACHE_HOME:-$HOME/.cache}/awf/<version>/awf` cache. It renders the marked repository bootstrap, which honors the explicit `AWF_VERSION` override and prints a binary path, and the unmarked public `awf.sh`, which fixes this release's version and executes the verified binary with forwarded arguments. `AWF_VERSION=<target> ./awf render` explicitly selects a compatible update and rewrites the committed pin through the target binary.

GitHub's tag rule requires CI's hosted `gate` status on the tagged commit, so a version tag is pushed only after that check has passed for the release commit on `main`; a local gate run or an atomic branch-and-tag push does not satisfy it.

`.github/workflows/release.yml` builds the candidate archives and checksums once, generates `dist/awf.sh` from the same tested source and embedded version, runs `tools/native-release-test/run.sh` against those exact files on Linux amd64 and macOS arm64, and publishes them unchanged. The native fixture starts from an empty cache so first download through the public launcher stays covered; a preseeded cache would not exercise it. Release history is in root `CHANGELOG.md`; transitions that need adopter action are also described in `internal/docs/migrate.md`.
