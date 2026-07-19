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

Queries are active-only and read-only. Default output shows current topic title,
summary, claims, claim types, prose, and backing state. --history adds direct
Origin and Revised-by ADRs; --references adds direct incoming and outgoing claim
IDs; --coverage adds declared and effective scope plus configured marker sites.
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
	fmt.Fprintf(stdout, "%s %s\n", result.Kind, result.ID)
	if result.Title != "" {
		fmt.Fprintf(stdout, "Title: %s\nSummary: %s\n", result.Title, result.Summary)
	}
	fmt.Fprintln(stdout, "\n## Claims")
	for _, claim := range result.Claims {
		backing := string(claim.Backing)
		if backing == "" {
			backing = "none"
		}
		fmt.Fprintf(stdout, "\n%s [%s] [backing: %s]\n%s\n", claim.ID, claim.Type, backing, claim.Prose)
		if claim.Verify != "" {
			fmt.Fprintf(stdout, "Verify: %s\n", claim.Verify)
		}
	}
	if result.History != nil {
		fmt.Fprintln(stdout, "\n## History")
		for _, history := range result.History {
			fmt.Fprintf(stdout, "%s\n  Origin: ADR-%s (%s) %s\n", history.ClaimID, history.Origin.Number, history.Origin.Status, history.Origin.Title)
			for _, revision := range history.RevisedBy {
				fmt.Fprintf(stdout, "  Revised-by: ADR-%s (%s) %s\n", revision.Number, revision.Status, revision.Title)
			}
		}
	}
	if result.References != nil {
		fmt.Fprintln(stdout, "\n## References")
		for _, refs := range result.References {
			fmt.Fprintf(stdout, "%s\n  Incoming: %v\n  Outgoing: %v\n", refs.ClaimID, refs.Incoming, refs.Outgoing)
		}
	}
	if result.Coverage != nil {
		fmt.Fprintln(stdout, "\n## Coverage")
		if result.Coverage.DeclaredGlobal {
			fmt.Fprintln(stdout, "Declared: global")
		} else {
			fmt.Fprintf(stdout, "Declared paths: %v\n", result.Coverage.DeclaredPaths)
		}
		for _, selector := range result.Coverage.EffectiveSelectors {
			fmt.Fprintf(stdout, "Effective: domain %s + topic %s\n", selector.DomainPath, selector.TopicPath)
		}
		for _, site := range result.Coverage.MarkerSites {
			fmt.Fprintf(stdout, "Marker: %s:%d [%s] %s", site.Path, site.Line, site.Kind, site.ClaimID)
			if site.Note != "" {
				fmt.Fprintf(stdout, " - %s", site.Note)
			}
			fmt.Fprintln(stdout)
		}
	}
	return nil
}
