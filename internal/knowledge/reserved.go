package knowledge

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func validateReserved(relative string, body []byte, metadata map[string]any, found bool) []string {
	index := strings.EqualFold(filepath.Base(relative), "index.md")
	var messages []string
	if found {
		if !index || filepath.ToSlash(filepath.Dir(relative)) != "docs" {
			messages = append(messages, "OKF reserved file must not contain frontmatter; only docs/index.md may declare okf_version")
		} else if len(metadata) != 1 || !nonemptyString(metadata["okf_version"]) {
			messages = append(messages, "OKF root index frontmatter may contain only a non-empty string okf_version")
		}
	} else if first, _, _ := bytes.Cut(body, []byte("\n")); string(bytes.TrimSuffix(first, []byte("\r"))) == "---" {
		messages = append(messages, "reserved file has an unterminated frontmatter block")
	}

	// Parse Markdown structure, not prose quality or listing completeness. A
	// heading-only index/log can describe an empty listing/history. Code blocks
	// and quoted examples are not top-level sections or entries.
	document := goldmark.DefaultParser().Parse(text.NewReader(body))
	headingSeen := false
	dateSeen := false
	previousDate := ""
	for node := document.FirstChild(); node != nil; node = node.NextSibling() {
		switch node := node.(type) {
		case *ast.Heading:
			heading := strings.TrimSpace(string(node.Lines().Value(body)))
			if !index {
				_, err := time.Parse("2006-01-02", heading)
				if err == nil {
					if previousDate != "" && heading > previousDate {
						messages = append(messages, "OKF log date groups must be newest first")
					}
					previousDate, dateSeen = heading, true
				} else if headingSeen || node.Level != 1 {
					messages = append(messages, fmt.Sprintf("OKF log date heading %q must use a valid YYYY-MM-DD date", heading))
				}
			}
			headingSeen = true
		case *ast.List:
			if index {
				if !headingSeen {
					messages = append(messages, "OKF index entries must be grouped under a heading")
				}
				_ = ast.Walk(node, func(entry ast.Node, entering bool) (ast.WalkStatus, error) {
					if entering && entry.Kind() == ast.KindListItem && !containsLink(entry) {
						messages = append(messages, "OKF index list entries must contain Markdown links")
					}
					return ast.WalkContinue, nil
				})
			} else {
				if !dateSeen {
					messages = append(messages, "OKF log entries must follow a YYYY-MM-DD date heading")
				}
				_ = ast.Walk(node, func(entry ast.Node, entering bool) (ast.WalkStatus, error) {
					if entering && entry != node && entry.Kind() == ast.KindList {
						messages = append(messages, "OKF log entries must form a flat list")
						return ast.WalkSkipChildren, nil
					}
					return ast.WalkContinue, nil
				})
			}
		}
	}
	if !headingSeen {
		messages = append(messages, "OKF reserved file requires a heading (index section or update log)")
	}
	return messages
}

func containsLink(node ast.Node) bool {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if child.Kind() == ast.KindLink || child.Kind() == ast.KindAutoLink {
			return true
		}
		// A child listing cannot supply its parent's entry link.
		if child.Kind() != ast.KindList && containsLink(child) {
			return true
		}
	}
	return false
}
