// Package frontmatter splits and parses YAML frontmatter delimited by leading
// "---" lines in markdown content. It is the single home for this concern.
package frontmatter

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Split separates a leading YAML frontmatter block (opened and closed by a line
// containing exactly "---") from the body. When content has no such block, found
// is false and body is the original content unchanged.
func Split(content []byte) (yamlBlock []byte, body []byte, found bool) {
	lines := bytes.SplitAfter(content, []byte("\n")) // keep line terminators
	if len(lines) == 0 || strings.TrimRight(string(lines[0]), "\r\n") != "---" {
		return nil, content, false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(string(lines[i]), "\r\n") == "---" {
			return bytes.Join(lines[1:i], nil), bytes.Join(lines[i+1:], nil), true
		}
	}
	return nil, content, false // no closing delimiter
}

// Parse splits frontmatter from content and unmarshals the YAML block into out.
// When no frontmatter is present, found is false and out is left unchanged.
func Parse(content []byte, out any) (body []byte, found bool, err error) {
	yamlBlock, body, found := Split(content)
	if !found {
		return body, false, nil
	}
	if err := yaml.Unmarshal(yamlBlock, out); err != nil {
		return body, true, fmt.Errorf("parse frontmatter: %w", err)
	}
	return body, true, nil
}
