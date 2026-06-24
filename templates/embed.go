package templates

import "embed"

//go:embed catalog.yaml skills hooks agents
var FS embed.FS
