package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

const topicStaticReference = `awf topic (static: not inside an awf project)

Usage: awf topic <domain>/<topic>[:<claim>] [--history] [--references] [--coverage] [--json]

Queries are active by default and read-only. Default output shows current topic
title, summary, claims, claim types, prose, and backing state. --history adds
direct Origin, Revised-by, and Removed-by ADR operations and is the only mode
that resolves a removed claim identity. --references adds direct incoming and
outgoing claim IDs; --coverage adds scope plus configured marker sites.
`

// runTopic validates the selector before inspecting project state, then mirrors
// config/context's static-fallback and in-handler version-gate boundary. It has
// no writer dependency.
func runTopic(cwd, selector string, history, references, coverage, asJSON bool, stdout io.Writer) error {
	if _, _, err := topic.ParseSelector(selector); err != nil {
		return &usageErr{err.Error()}
	}
	if _, err := os.Stat(config.ConfigPath(cwd)); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		_, err = io.WriteString(stdout, topicStaticReference)
		return err
	}
	if err := gate(cwd); err != nil {
		return err
	}
	p, err := project.Open(cwd)
	if err != nil {
		return err
	}
	result, err := p.QueryTopic(selector, topic.QueryOptions{History: history, References: references, Coverage: coverage})
	if err != nil {
		return err
	}
	return printTopic(stdout, result, asJSON)
}

func printTopic(stdout io.Writer, result topic.QueryResult, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	var writeErr error
	write := func(context, format string, args ...any) {
		if writeErr != nil {
			return
		}
		if _, err := fmt.Fprintf(stdout, format, args...); err != nil {
			writeErr = fmt.Errorf("write human topic %s: %w", context, err)
		}
	}
	write("identity", "%s %s\n", result.Kind, result.ID)
	if result.HistoricalOnly {
		write("historical-only label", "historical only - no active claim\n")
	}
	if result.Title != "" {
		write("metadata", "Title: %s\nSummary: %s\n", result.Title, result.Summary)
	}
	write("claims heading", "\n## Claims\n")
	for _, claim := range result.Claims {
		write("claim", "\n%s [%s] [backing: %s]\n%s\n", claim.ID, claim.Type, claim.Backing, claim.Prose)
		if claim.Verify != "" {
			write("claim verification", "Verify: %s\n", claim.Verify)
		}
	}
	if result.History != nil {
		write("history heading", "\n## History\n")
		for _, history := range result.History {
			write("claim history", "%s\n  Origin: ADR-%s (%s) %s\n", history.ClaimID, history.Origin.Number, history.Origin.Status, history.Origin.Title)
			for _, revision := range history.RevisedBy {
				write("claim revision", "  Revised-by: ADR-%s (%s) %s\n", revision.Number, revision.Status, revision.Title)
			}
			if history.RemovedBy != nil {
				write("claim removal", "  Removed-by: ADR-%s (%s) %s\n", history.RemovedBy.Number, history.RemovedBy.Status, history.RemovedBy.Title)
			}
		}
	}
	if result.References != nil {
		write("references heading", "\n## References\n")
		for _, refs := range result.References {
			write("claim references", "%s\n  Incoming: %v\n  Outgoing: %v\n", refs.ClaimID, refs.Incoming, refs.Outgoing)
		}
	}
	if result.Coverage != nil {
		write("coverage heading", "\n## Coverage\n")
		if result.Coverage.DeclaredGlobal {
			write("global coverage", "Declared: global\n")
		} else {
			write("declared coverage paths", "Declared paths: %v\n", result.Coverage.DeclaredPaths)
		}
		for _, selector := range result.Coverage.EffectiveSelectors {
			write("effective coverage selector", "Effective: domain %s + topic %s\n", selector.DomainPath, selector.TopicPath)
		}
		for _, site := range result.Coverage.MarkerSites {
			write("coverage marker", "Marker: %s:%d [%s] %s", site.Path, site.Line, site.Kind, site.ClaimID)
			if site.Note != "" {
				write("coverage marker note", " - %s", site.Note)
			}
			write("coverage marker newline", "\n")
		}
	}
	return writeErr
}
