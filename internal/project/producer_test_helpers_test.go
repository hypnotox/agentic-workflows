package project

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type agent struct{ Name, Description, Body string }

func encodeMarkdownAgent(a agent) (string, error) {
	if strings.TrimSpace(a.Name) == "" || strings.ContainsAny(a.Name, "\r\n") {
		return "", errors.New("invalid agent name")
	}
	if strings.TrimSpace(a.Description) == "" {
		return "", fmt.Errorf("agent %q description is empty", a.Name)
	}
	description := a.Description
	if !yamlPlainSafe(description) {
		description = strconv.Quote(description)
	}
	return "---\nname: " + a.Name + "\ndescription: " + description + "\n---\n\n" + a.Body, nil
}
func yamlPlainSafe(s string) bool {
	return s != "" && !strings.HasPrefix(s, "-") && !strings.ContainsAny(s, "\"'[]{}#&*!|>@`") && !strings.Contains(s, ": ")
}
