// Package templates embeds the standard's template tree (skills, agents, docs, bootstrap).
package templates

import "embed"

//go:embed all:skills all:agents agents-doc all:docs domains claude adr-readme adr-template plans-readme plans-template bootstrap hooks runner partials memory
var FS embed.FS
