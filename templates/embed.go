// Package templates embeds the standard's template tree (catalog.yaml, skills, agents, hooks, docs).
package templates

import "embed"

//go:embed catalog.yaml skills hooks agents agents-doc docs domains claude
var FS embed.FS
