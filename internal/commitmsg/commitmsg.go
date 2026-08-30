// Package commitmsg cleans Git commit messages.
package commitmsg

import "strings"

// Message is a Git-cleaned commit message and its first surviving subject line.
type Message struct {
	Text    string
	Subject string
}

// Clean applies Git's default strip cleanup and returns the recorded message.
func Clean(raw []byte) Message {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			if trimmed == "# ------------------------ >8 ------------------------" {
				break
			}
			continue
		}
		cleaned = append(cleaned, line)
	}
	for len(cleaned) > 0 && strings.TrimSpace(cleaned[len(cleaned)-1]) == "" {
		cleaned = cleaned[:len(cleaned)-1]
	}
	msg := Message{Text: strings.Join(cleaned, "\n")}
	for _, line := range cleaned {
		if strings.TrimSpace(line) != "" {
			msg.Subject = strings.TrimRight(line, " \t")
			break
		}
	}
	return msg
}
