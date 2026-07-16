package project

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// agent is the output-format-neutral rendering result for an agent artifact.
// Its body is Markdown instructions, independently of the target encoder.
type agent struct {
	Name        string
	Description string
	Body        string
}

// codexAgentProfile is the complete supported Codex agent-profile schema.
type codexAgentProfile struct {
	Name                  string `toml:"name"`
	Description           string `toml:"description"`
	DeveloperInstructions string `toml:"developer_instructions"`
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

// encodeTOMLAgent produces and then decodes the complete typed Codex profile.
// The encoder consumes structured agent data directly, never Markdown output.
func validateTOMLAgent(content []byte) error {
	var profile codexAgentProfile
	meta, err := toml.Decode(string(content), &profile)
	if err != nil {
		return fmt.Errorf("parse TOML agent profile: %w", err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) != 0 {
		return fmt.Errorf("unknown TOML agent profile keys %v", undecoded)
	}
	return validateAgent(agent{Name: profile.Name, Description: profile.Description, Body: profile.DeveloperInstructions})
}

func encodeTOMLAgent(a agent) (string, error) {
	if err := validateAgent(a); err != nil {
		return "", err
	}
	want := codexAgentProfile{
		Name:                  a.Name,
		Description:           a.Description,
		DeveloperInstructions: a.Body,
	}
	var b bytes.Buffer
	if err := toml.NewEncoder(&b).Encode(want); err != nil { // coverage-ignore: bytes.Buffer writes cannot fail
		return "", fmt.Errorf("encode Codex agent profile: %w", err)
	}
	out := b.String()
	if err := validateTOMLAgent([]byte(out)); err != nil { // coverage-ignore: encoding this typed profile always produces a valid profile
		return "", fmt.Errorf("validate Codex agent profile: %w", err)
	}
	return out, nil
}
