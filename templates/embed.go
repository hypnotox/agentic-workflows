// Package templates embeds the standard's template tree (catalog.yaml, skills, agents, docs, bootstrap).
package templates

import "embed"

//go:embed catalog.yaml skills agents agents-doc docs domains claude adr-readme adr-template plans-readme bootstrap hooks partials
var FS embed.FS
