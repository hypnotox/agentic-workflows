package contextq

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// RenderContextText maps the typed context result into the shared Detail shape.
// Contextq owns this semantic mapping; presentation owns syntax and rendering.
func RenderContextText(res ContextResult, header string, facets []ContextFacet) string {
	fields := []presentation.Field{field("context", header), field("selection", selectionText(res))}
	requestNodes := make([]presentation.Node, 0, len(res.Requests))
	if len(res.Requests) == 0 {
		requestNodes = append(requestNodes, field("status", "none"))
	}
	for _, request := range res.Requests {
		nodes := []presentation.Node{field("argument", request.Argument)}
		if request.Exact != nil {
			nodes = append(nodes, field("file", request.Exact.Path))
			nodes = append(nodes, impactFields(request.Exact.Context, facets)...)
			nodes = append(nodes, relationshipFields(request.Exact.Context.Relationships)...)
		}
		if request.Directory != nil {
			d := request.Directory
			excluded := 0
			for _, c := range d.Excluded {
				excluded += c.Count
			}
			nodes = append(nodes, field("directory", fmt.Sprintf("%d included | %d excluded", d.Included, excluded)))
			for _, c := range d.Excluded {
				nodes = append(nodes, field("excluded", fmt.Sprintf("%s=%d", c.Classification, c.Count)))
			}
			for i, group := range d.Groups {
				nodes = append(nodes, field("group", fmt.Sprintf("%d | %d files", i+1, group.Count)))
				if len(group.Members) > 0 {
					nodes = append(nodes, field("members", strings.Join(group.Members, ", ")))
				}
				nodes = append(nodes, impactFields(group.Context, facets)...)
			}
			if containsFacet(facets, FacetRelationships) {
				nodes = append(nodes, relationshipFields(d.Relationships)...)
			}
		}
		requestNodes = append(requestNodes, section(fmt.Sprintf("request-%d", request.Index), nodes...))
	}
	sections := []presentation.Section{section("requests", requestNodes...)}
	authority := make([]presentation.Node, 0, 1)
	if len(res.Topics) == 0 {
		authority = append(authority, field("topics", "none"))
	} else {
		values := make([]presentation.Value, 0, len(res.Topics))
		for _, topic := range res.Topics {
			values = append(values, prose(topicText(topic)))
		}
		authority = append(authority, list("topics", values...))
	}
	sections = append(sections, section("authority", authority...))
	return renderDetail(presentation.Detail{Fields: fields, Sections: sections})
}

// RenderUncoveredText maps coverage result semantics into the shared Detail shape.
func RenderUncoveredText(res UncoveredResult, header string) string {
	fields := []presentation.Field{field("context", header)}
	nodes := []presentation.Node{}
	if len(res.ScanRoots) > 0 {
		nodes = append(nodes, field("scan-roots", strings.Join(res.ScanRoots, ", ")))
	}
	if len(res.Uncovered) == 0 && len(res.Unowned) == 0 {
		nodes = append(nodes, field("result", "all scanned paths are owned and covered by a scoped topic"))
	}
	if len(res.Uncovered) > 0 {
		records := make([]presentation.Record, 0, len(res.Uncovered))
		for _, entry := range res.Uncovered {
			records = append(records, record(prose(entry.Path), prose(entry.Domain)))
		}
		nodes = append(nodes, recordGroup("uncovered", []string{"path", "domain"}, records...))
	}
	if len(res.Unowned) > 0 {
		values := make([]presentation.Value, 0, len(res.Unowned))
		for _, entry := range res.Unowned {
			text := entry.Path
			if entry.Path == "." || strings.HasSuffix(entry.Path, "/") {
				text += " | " + countNoun(entry.UnownedCount, "unowned file")
				if entry.ExcludedCount > 0 {
					text += fmt.Sprintf(" | %s excluded from coverage beneath", countNoun(entry.ExcludedCount, "file"))
				}
			}
			values = append(values, prose(text))
		}
		nodes = append(nodes, list("unowned", values...))
	}
	return renderDetail(presentation.Detail{Fields: fields, Sections: []presentation.Section{section("coverage", nodes...)}})
}

func selectionText(res ContextResult) string {
	if res.Selection == SelectionRange {
		return "range " + res.Range
	}
	return string(res.Selection)
}

func impactFields(impact contextPathImpact, facets []ContextFacet) []presentation.Node {
	nodes := []presentation.Node{field("classification", string(impact.Classification))}
	if impact.NestedRoot != "" {
		nodes = append(nodes, field("nested-root", impact.NestedRoot))
	}
	if impact.TargetInsideRepository != nil {
		nodes = append(nodes, field("symlink-target-inside-repository", strconv.FormatBool(*impact.TargetInsideRepository)))
	}
	if len(impact.Provenance) == 0 {
		nodes = append(nodes, field("provenance", "none"))
	}
	for _, provenance := range impact.Provenance {
		nodes = append(nodes, field("provenance", provenance.Role+" | "+provenance.Identity))
		if containsFacet(facets, FacetArtifacts) {
			for _, item := range provenance.Sources {
				nodes = append(nodes, field("source", item.Path+" | "+item.Label))
			}
			for _, item := range provenance.Outputs {
				nodes = append(nodes, field("output", item.Path+" | "+item.Label))
			}
			for _, item := range provenance.Navigation {
				nodes = append(nodes, field("navigate", item.Path+" | "+item.Label))
			}
		}
	}
	domains := []string{}
	for _, domain := range impact.Domains {
		domains = append(domains, domain.Name)
	}
	topics := []string{}
	for _, topic := range impact.Topics {
		topics = append(topics, topic.ID)
	}
	nodes = append(nodes, field("domains", listText(domains)), field("topics", listText(topics)))
	for _, warning := range impact.Warnings {
		nodes = append(nodes, field("warning", string(warning)))
	}
	if impact.ADR != nil {
		adr := impact.ADR
		nodes = append(nodes, field("adr", fmt.Sprintf("ADR-%s | %s | %s | %s", adr.Number, adr.Title, adr.Status, adr.Mutability)), field("authority-role", adr.AuthorityRole))
		for _, operation := range adr.Operations {
			nodes = append(nodes, field("operation", fmt.Sprintf("%s | %s | %s | %s", operation.Operation, operation.Claim, operation.Progress, operation.ClaimState)))
			if operation.Detail != nil && operation.Detail.Current != nil {
				nodes = append(nodes, claimFields(*operation.Detail.Current)...)
			}
			if operation.Detail != nil && operation.Detail.History != nil && operation.Detail.History.RemovedBy != nil {
				nodes = append(nodes, field("removal-history", "removed by ADR-"+operation.Detail.History.RemovedBy.Number))
			}
		}
	}
	return nodes
}

func topicText(topic topicImpact) string {
	parts := []string{topic.ID, topic.Title, topic.Summary, fmt.Sprintf("invariants=%d", topic.Counts.Invariants), fmt.Sprintf("rules=%d", topic.Counts.Rules)}
	if topic.Selectors != nil {
		selectors := topic.Selectors
		topicPaths := listText(selectors.TopicPaths)
		if selectors.DeclaredGlobal {
			topicPaths = "global"
		}
		parts = append(parts, "domain="+listText(selectors.DomainPaths), "topic="+topicPaths)
	}
	for _, claims := range [][]contextClaimImpact{topic.Direct, topic.Invariants, topic.Additional, topic.Referenced} {
		for _, claim := range claims {
			parts = append(parts, claimText(claim))
		}
	}
	for _, pending := range topic.Pending.Operations {
		parts = append(parts, fmt.Sprintf("ADR-%s %s %s %s", pending.ADR, pending.Op, pending.Claim, pending.Progress))
	}
	if len(topic.Pending.Operations) == 0 && topic.Pending.OperationCount > 0 {
		parts = append(parts, fmt.Sprintf("%d operations from %s", topic.Pending.OperationCount, strings.Join(topic.Pending.ADRs, ", ")))
	}
	return strings.Join(parts, " | ")
}

func claimText(claim contextClaimImpact) string {
	parts := []string{claim.ID, claim.Type, claim.Summary, claim.Backing, claim.Verify}
	for _, source := range claim.Sources {
		parts = append(parts, fmt.Sprintf("request %d %s", source.RequestIndex, strings.Join(source.Kinds, ", ")))
	}
	for _, evidence := range claim.Evidence {
		if len(evidence.Sites) == 0 {
			parts = append(parts, fmt.Sprintf("%s %d sites", evidence.Kind, evidence.Count))
		} else {
			for _, site := range evidence.Sites {
				parts = append(parts, fmt.Sprintf("%s %s:%d", evidence.Kind, site.Path, site.Line))
			}
		}
	}
	if len(claim.Incoming) > 0 {
		parts = append(parts, "incoming "+strings.Join(claim.Incoming, ", "))
	}
	if len(claim.Outgoing) > 0 {
		parts = append(parts, "outgoing "+strings.Join(claim.Outgoing, ", "))
	}
	return strings.Join(parts, " | ")
}
func containsFacet(facets []ContextFacet, want ContextFacet) bool {
	for _, facet := range facets {
		if facet == want {
			return true
		}
	}
	return false
}

func claimFields(claim contextClaimImpact) []presentation.Node {
	return []presentation.Node{field("claim", claimText(claim))}
}

func relationshipFields(relationships contextRelationships) []presentation.Node {
	nodes := []presentation.Node{}
	if len(relationships.State) > 0 {
		nodes = append(nodes, field("state", strings.Join(relationships.State, ", ")))
	}
	if len(relationships.Touches) > 0 {
		nodes = append(nodes, field("touches", strings.Join(relationships.Touches, ", ")))
	}
	if len(relationships.Proofs) > 0 {
		nodes = append(nodes, field("proofs", strings.Join(relationships.Proofs, ", ")))
	}
	return nodes
}
func countNoun(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
func listText(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}
func prose(text string) presentation.Value {
	value, err := presentation.Prose(text)
	if err != nil { // coverage-ignore: every mapper input is normalized from parsed context semantics
		panic(err)
	}
	return value
}
func field(label, text string) presentation.Field {
	value := prose(text)
	result, err := presentation.NewField(label, value)
	if err != nil { // coverage-ignore: this mapper owns each literal label and uses a validated value
		panic(err)
	}
	return result
}
func list(label string, values ...presentation.Value) presentation.List {
	result, err := presentation.NewList(label, values...)
	if err != nil { // coverage-ignore: callers construct nonempty lists from typed result slices
		panic(err)
	}
	return result
}
func section(label string, nodes ...presentation.Node) presentation.Section {
	result, err := presentation.NewSection(label, nodes...)
	if err != nil { // coverage-ignore: callers construct sections with at least one mapped node
		panic(err)
	}
	return result
}
func record(values ...presentation.Value) presentation.Record {
	result, err := presentation.NewRecord(values...)
	if err != nil { // coverage-ignore: callers provide nonempty validated semantic fields
		panic(err)
	}
	return result
}
func recordGroup(label string, schema []string, records ...presentation.Record) presentation.RecordGroup {
	result, err := presentation.NewRecordGroup(label, schema, records...)
	if err != nil { // coverage-ignore: callers provide a fixed schema and matching nonempty records
		panic(err)
	}
	return result
}
func renderDetail(detail presentation.Detail) string {
	document, err := detail.Document()
	if err != nil { // coverage-ignore: Detail is assembled only through the validated helpers above
		return ""
	}
	var out strings.Builder
	if presentation.Render(&out, document) != nil { // coverage-ignore: a validated document cannot fail buffer rendering
		return ""
	}
	return out.String()
}
