---
paths:
  - 'internal/projector/VERSION'
  - '.goreleaser.yaml'
  - '.github/workflows/release.yml'
  - 'tools/native-release-test/**'
  - 'CHANGELOG.md'
  - 'MIGRATING-v0.50.md'
---

# Release boundary

`internal/projector/VERSION` is the binary release identity and the default pin rendered into `.awf/bootstrap.sh`. Binary SemVer does not describe source compatibility and is not stored in project metadata.

The bootstrap downloads the matching Linux or Darwin, amd64 or arm64 archive, verifies it against the published checksum, and caches the executable by version. `AWF_VERSION=<target> ./awf render` explicitly selects a same-format update and rewrites the committed pin through the target binary.

Build release candidates once, smoke the exact candidates on native platforms, and publish them unchanged. Keep release history in root `CHANGELOG.md`.
