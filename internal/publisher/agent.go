package publisher

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// agent is the output-format-neutral rendering result for an agent artifact.
// Its body is Markdown instructions, independently of the target encoder.
type agent struct {
	Name        string
	Description string
	Body        string
}

func validateAgent(a agent) error {
	if strings.TrimSpace(a.Name) == "" {
		return errors.New("agent name is empty")
	}
	if strings.ContainsAny(a.Name, "\r\n") {
		return errors.New("agent name contains a newline")
	}
	if strings.TrimSpace(a.Description) == "" {
		return fmt.Errorf("agent %q description is empty", a.Name)
	}
	return nil
}

// encodeMarkdownAgent produces the YAML-frontmatter Markdown dialect used by
// Markdown-native agent targets.
func encodeMarkdownAgent(a agent) (string, error) {
	if err := validateAgent(a); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("---\nname: ")
	b.WriteString(a.Name)
	b.WriteString("\n")
	if strings.Contains(a.Description, "\n") {
		b.WriteString("description: >\n")
		for _, line := range strings.Split(a.Description, "\n") {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	} else {
		b.WriteString("description: ")
		if yamlPlainSafe(a.Description) {
			b.WriteString(a.Description)
		} else {
			b.WriteString(strconv.Quote(a.Description))
		}
		b.WriteString("\n")
	}
	b.WriteString("---\n\n")
	b.WriteString(a.Body)
	return b.String(), nil
}

func yamlPlainSafe(s string) bool {
	if s == "" || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "?") || strings.HasPrefix(s, ":") {
		return false
	}
	return !strings.ContainsAny(s, "\"'[]{}#&*!|>@`") && !strings.Contains(s, ": ")
}
