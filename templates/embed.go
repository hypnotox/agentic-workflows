// Package templates embeds the standard's template tree (catalog.yaml, skills, agents, hooks).
package templates

import "embed"

//go:embed catalog.yaml skills hooks agents
var FS embed.FS
