// Package docs provides the adopter guides embedded in the AWF binary.
package docs

import "embed"

// pages is the complete public documentation set.
//
//go:embed overview.md integration.md agents.md topics.md effort.md changes.md adr.md completion.md
var pages embed.FS

var pageFiles = map[string]string{
	"":            "overview.md",
	"integration": "integration.md",
	"agents":      "agents.md",
	"topics":      "topics.md",
	"effort":      "effort.md",
	"changes":     "changes.md",
	"adr":         "adr.md",
	"completion":  "completion.md",
}

// Page returns one named documentation page. The empty name selects the overview.
func Page(name string) ([]byte, bool) {
	path, ok := pageFiles[name]
	if !ok {
		return nil, false
	}
	body, err := pages.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return body, true
}
