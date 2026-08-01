// Package commitmsg cleans Git commit messages and parses stale-ADR authorization trailers.
package commitmsg

import (
	"fmt"
	"strings"
)

const (
	versionKey = "AWF-Allow-Version"
	reasonKey  = "AWF-Allow-Reason"
)

// Message is a Git-cleaned commit message and its first surviving subject line.
type Message struct {
	Text    string
	Subject string
}

// Authorization permits one older authored ADR version for the reason recorded.
type Authorization struct {
	Version string
	Reason  string
}

// SyntaxError identifies malformed reserved authorization syntax by cleaned line.
type SyntaxError struct {
	Line   int
	Reason string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("cleaned commit message line %d: %s", e.Line, e.Reason)
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

// ParseAuthorizations parses the final trailer block and validates reserved pairs.
func ParseAuthorizations(msg Message, validVersion func(string) bool) ([]Authorization, error) {
	if msg.Text == "" {
		return nil, nil
	}
	lines := strings.Split(msg.Text, "\n")
	start := len(lines)
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "" {
			start = i + 1
			break
		}
	}

	type trailer struct {
		key, value string
		line       int
	}
	trailers := make([]trailer, 0, len(lines)-start)
	validBlock := start < len(lines)
	if validBlock {
		for i := start; i < len(lines); i++ {
			key, value, ok := parseTrailerLine(lines[i])
			if !ok {
				validBlock = false
				break
			}
			trailers = append(trailers, trailer{key: key, value: value, line: i + 1})
		}
	}
	if !validBlock {
		trailers = nil
		start = len(lines)
	}

	for i, line := range lines {
		if i >= start {
			continue
		}
		if reservedLine(line) {
			return nil, &SyntaxError{Line: i + 1, Reason: "reserved AWF-Allow trailer appears outside the final trailer block"}
		}
	}

	var out []Authorization
	for i := 0; i < len(trailers); i++ {
		t := trailers[i]
		if !strings.HasPrefix(t.key, "AWF-Allow-") {
			continue
		}
		if t.key != versionKey && t.key != reasonKey {
			return nil, &SyntaxError{Line: t.line, Reason: "unknown reserved trailer " + t.key}
		}
		if t.key != versionKey {
			return nil, &SyntaxError{Line: t.line, Reason: "AWF-Allow-Reason must immediately follow AWF-Allow-Version"}
		}
		if i+1 >= len(trailers) || trailers[i+1].key != reasonKey {
			return nil, &SyntaxError{Line: t.line, Reason: "AWF-Allow-Version must be immediately followed by AWF-Allow-Reason"}
		}
		version := strings.Trim(trailers[i].value, " \t\v\f\r")
		reason := strings.Trim(trailers[i+1].value, " \t\v\f\r")
		if version == "" || !validVersion(version) {
			return nil, &SyntaxError{Line: t.line, Reason: fmt.Sprintf("unknown authorization version %q", version)}
		}
		if reason == "" {
			return nil, &SyntaxError{Line: trailers[i+1].line, Reason: "AWF-Allow-Reason must be nonempty"}
		}
		out = append(out, Authorization{Version: version, Reason: reason})
		i++
	}
	return out, nil
}

func parseTrailerLine(line string) (key, value string, ok bool) {
	if line == "" || line[0] == ' ' || line[0] == '\t' {
		return "", "", false
	}
	key, value, ok = strings.Cut(line, ":")
	if !ok || key == "" || strings.ContainsAny(key, " \t") || !strings.HasPrefix(value, " ") {
		return "", "", false
	}
	return key, strings.TrimPrefix(value, " "), true
}

func reservedLine(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(trimmed, "AWF-Allow-")
}
