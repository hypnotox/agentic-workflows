// Package templates embeds the standard's template tree (skills, agents, docs, bootstrap).
package templates

import "embed"

//go:embed skills agents agents-doc docs domains claude adr-readme adr-template plans-readme bootstrap hooks partials
var FS embed.FS
