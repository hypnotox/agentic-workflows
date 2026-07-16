// Package changelog embeds the hand-maintained CHANGELOG.md (ADR-0041). It is a
// top-level package, not internal/, purely because go:embed cannot embed a file
// outside its own package directory: mirrors templates/embed.go.
package changelog

import "embed"

//go:embed CHANGELOG.md
var FS embed.FS
