// Package templates embeds the standard's template tree.
package templates

import "embed"

//go:embed all:skills agents-doc all:docs all:pitfalls domains topics claude bootstrap hooks runner partials efforts worktrees effort-archive
var FS embed.FS
