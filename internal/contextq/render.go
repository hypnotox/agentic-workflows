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
	fields := []presentation.Field{field("context", header), literalField("selection", selectionText(res))}
	requestNodes := make([]presentation.Node, 0, len(res.Requests))
	if len(res.Requests) == 0 {
		requestNodes = append(requestNodes, field("status", "none"))
	}
	for _, request := range res.Requests {
		nodes := []presentation.Node{literalField("argument", request.Argument)}
		if request.Exact != nil {
			nodes = append(nodes, literalField("file", request.Exact.Path))
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
					nodes = append(nodes, literalField("members", strings.Join(group.Members, ", ")))
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
	sections = append(sections, section("authority", authorityNodes(res.Topics, facets)...))
	return renderDetail(presentation.Detail{Fields: fields, Sections: sections})
}

// RenderUncoveredText maps coverage result semantics into the shared Detail shape.
func RenderUncoveredText(res UncoveredResult, header string) string {
	fields := []presentation.Field{field("context", header)}
	nodes := []presentation.Node{}
	if len(res.ScanRoots) > 0 {
		nodes = append(nodes, literalField("scan-roots", strings.Join(res.ScanRoots, ", ")))
	}
	if len(res.Uncovered) == 0 && len(res.Unowned) == 0 {
		nodes = append(nodes, field("result", "all scanned paths are owned and covered by a scoped topic"))
	}
	if len(res.Uncovered) > 0 {
		records := make([]presentation.Record, 0, len(res.Uncovered))
		for _, entry := range res.Uncovered {
			records = append(records, record(literal(entry.Path), literal(entry.Domain)))
		}
		nodes = append(nodes, recordGroup("uncovered", []string{"path", "domain"}, records...))
	}
	if len(res.Unowned) > 0 {
		records := make([]presentation.Record, 0, len(res.Unowned))
		for _, entry := range res.Unowned {
			records = append(records, record(literal(entry.Path), prose(strconv.Itoa(entry.UnownedCount)), prose(strconv.Itoa(entry.ExcludedCount))))
		}
		nodes = append(nodes, recordGroup("unowned", []string{"path", "unowned-files", "excluded-files"}, records...))
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
		nodes = append(nodes, literalField("nested-root", impact.NestedRoot))
	}
	if impact.TargetInsideRepository != nil {
		nodes = append(nodes, field("symlink-target-inside-repository", strconv.FormatBool(*impact.TargetInsideRepository)))
	}
	if len(impact.Provenance) == 0 {
		nodes = append(nodes, field("provenance", "none"))
	}
	for _, provenance := range impact.Provenance {
		nodes = append(nodes, literalField("provenance", provenance.Role+" | "+provenance.Identity))
		if containsFacet(facets, FacetArtifacts) {
			for _, item := range provenance.Sources {
				nodes = append(nodes, literalField("source", item.Path+" | "+item.Label))
			}
			for _, item := range provenance.Outputs {
				nodes = append(nodes, literalField("output", item.Path+" | "+item.Label))
			}
			for _, item := range provenance.Navigation {
				nodes = append(nodes, literalField("navigate", item.Path+" | "+item.Label))
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
	nodes = append(nodes, literalField("domains", listText(domains)), literalField("topics", listText(topics)))
	for _, warning := range impact.Warnings {
		nodes = append(nodes, field("warning", string(warning)))
	}
	if impact.ADR != nil {
		adr := impact.ADR
		nodes = append(nodes, literalField("adr", fmt.Sprintf("ADR-%s | %s | %s | %s", adr.Number, adr.Title, adr.Status, adr.Mutability)), field("authority-role", adr.AuthorityRole))
		for _, operation := range adr.Operations {
			nodes = append(nodes, literalField("operation", fmt.Sprintf("%s | %s | %s | %s", operation.Operation, operation.Claim, operation.Progress, operation.ClaimState)))
			if operation.Detail != nil && operation.Detail.Current != nil {
				nodes = append(nodes, claimFields(*operation.Detail.Current, facets)...)
			}
			if operation.Detail != nil && operation.Detail.History != nil && operation.Detail.History.RemovedBy != nil {
				nodes = append(nodes, literalField("removal-history", "removed by ADR-"+operation.Detail.History.RemovedBy.Number))
			}
		}
	}
	return nodes
}

func authorityNodes(topics []topicImpact, facets []ContextFacet) []presentation.Node {
	if len(topics) == 0 {
		return []presentation.Node{field("topics", "none")}
	}
	nodes := []presentation.Node{}
	topicRecords := make([]presentation.Record, 0, len(topics))
	selectorRecords := []presentation.Record{}
	claimGroups := []struct {
		label  string
		claims func(topicImpact) []contextClaimImpact
	}{
		{"direct-claims", func(topic topicImpact) []contextClaimImpact { return topic.Direct }},
		{"invariants", func(topic topicImpact) []contextClaimImpact { return topic.Invariants }},
		{"additional-claims", func(topic topicImpact) []contextClaimImpact { return topic.Additional }},
		{"referenced-claims", func(topic topicImpact) []contextClaimImpact { return topic.Referenced }},
	}
	claimRecords := make([][]presentation.Record, len(claimGroups))
	sourceRecords, evidenceRecords, referenceRecords := []presentation.Record{}, []presentation.Record{}, []presentation.Record{}
	pendingRecords, pendingSummaryRecords := []presentation.Record{}, []presentation.Record{}
	for _, topic := range topics {
		topicRecords = append(topicRecords, record(literal(topic.ID), prose(topic.Title), prose(topic.Summary), prose(strconv.Itoa(topic.Counts.Invariants)), prose(strconv.Itoa(topic.Counts.Rules))))
		if selectors := topic.Selectors; selectors != nil {
			topicPaths := listText(selectors.TopicPaths)
			if selectors.DeclaredGlobal {
				topicPaths = "global"
			}
			selectorRecords = append(selectorRecords, record(literal(topic.ID), literal(listText(selectors.DomainPaths)), literal(topicPaths), prose("both domain and topic selectors must match")))
		}
		for i, group := range claimGroups {
			for _, claim := range group.claims(topic) {
				claimValues := []presentation.Value{literal(topic.ID), literal(claim.ID), prose(claim.Type), prose(claim.Summary)}
				if containsFacet(facets, FacetEvidence) {
					claimValues = append(claimValues, prose(claim.Backing), prose(orNone(claim.Verify)))
				}
				claimRecords[i] = append(claimRecords[i], record(claimValues...))
				if containsFacet(facets, FacetRelationships) {
					for _, source := range claim.Sources {
						sourceRecords = append(sourceRecords, record(literal(topic.ID), literal(claim.ID), prose(strconv.Itoa(source.RequestIndex)), prose(strings.Join(source.Kinds, ", "))))
					}
				}
				if containsFacet(facets, FacetEvidence) {
					for _, evidence := range claim.Evidence {
						sites := make([]string, 0, len(evidence.Sites))
						for _, site := range evidence.Sites {
							sites = append(sites, fmt.Sprintf("%s:%d", site.Path, site.Line))
						}
						evidenceRecords = append(evidenceRecords, record(literal(topic.ID), literal(claim.ID), prose(evidence.Kind), prose(strconv.Itoa(evidence.Count)), literal(listText(sites))))
					}
				}
				if containsFacet(facets, FacetReferences) {
					referenceRecords = append(referenceRecords, record(literal(topic.ID), literal(claim.ID), literal(listText(claim.Incoming)), literal(listText(claim.Outgoing))))
				}
			}
		}
		for _, pending := range topic.Pending.Operations {
			pendingRecords = append(pendingRecords, record(literal(topic.ID), literal(pending.ADR), prose(pending.Op), literal(pending.Claim), prose(pending.Progress)))
		}
		if len(topic.Pending.Operations) == 0 && topic.Pending.OperationCount > 0 {
			pendingSummaryRecords = append(pendingSummaryRecords, record(literal(topic.ID), prose(strconv.Itoa(topic.Pending.OperationCount)), literal(strings.Join(topic.Pending.ADRs, ", ")), prose(strconv.Itoa(topic.Pending.AdditionalADRCount))))
		}
	}
	nodes = append(nodes, recordGroup("topics", []string{"identity", "title", "summary", "invariants", "rules"}, topicRecords...))
	if len(selectorRecords) > 0 {
		nodes = append(nodes, recordGroup("selectors", []string{"topic", "domain-paths", "topic-paths", "rule"}, selectorRecords...))
	}
	for i, group := range claimGroups {
		if len(claimRecords[i]) > 0 {
			schema := []string{"topic", "identity", "type", "summary"}
			if containsFacet(facets, FacetEvidence) {
				schema = append(schema, "backing", "verify")
			}
			nodes = append(nodes, recordGroup(group.label, schema, claimRecords[i]...))
		}
	}
	if len(sourceRecords) > 0 {
		nodes = append(nodes, recordGroup("claim-sources", []string{"topic", "claim", "request", "kinds"}, sourceRecords...))
	}
	if len(evidenceRecords) > 0 {
		nodes = append(nodes, recordGroup("claim-evidence", []string{"topic", "claim", "kind", "count", "sites"}, evidenceRecords...))
	}
	if len(referenceRecords) > 0 {
		nodes = append(nodes, recordGroup("claim-references", []string{"topic", "claim", "incoming", "outgoing"}, referenceRecords...))
	}
	if len(pendingRecords) > 0 {
		nodes = append(nodes, recordGroup("pending-operations", []string{"topic", "adr", "operation", "claim", "progress"}, pendingRecords...))
	}
	if len(pendingSummaryRecords) > 0 {
		nodes = append(nodes, recordGroup("pending-summary", []string{"topic", "operation-count", "adrs", "additional-adrs"}, pendingSummaryRecords...))
	}
	return nodes
}
func containsFacet(facets []ContextFacet, want ContextFacet) bool {
	for _, facet := range facets {
		if facet == want {
			return true
		}
	}
	return false
}

func claimFields(claim contextClaimImpact, facets []ContextFacet) []presentation.Node {
	values := []presentation.Value{literal(claim.ID), prose(claim.Type), prose(claim.Summary)}
	schema := []string{"identity", "type", "summary"}
	if containsFacet(facets, FacetEvidence) {
		values = append(values, prose(claim.Backing), prose(orNone(claim.Verify)))
		schema = append(schema, "backing", "verify")
	}
	nodes := []presentation.Node{recordGroup("claim", schema, record(values...))}
	if containsFacet(facets, FacetRelationships) && len(claim.Sources) > 0 {
		records := make([]presentation.Record, 0, len(claim.Sources))
		for _, source := range claim.Sources {
			records = append(records, record(prose(strconv.Itoa(source.RequestIndex)), prose(strings.Join(source.Kinds, ", "))))
		}
		nodes = append(nodes, recordGroup("claim-sources", []string{"request", "kinds"}, records...))
	}
	if containsFacet(facets, FacetEvidence) && len(claim.Evidence) > 0 {
		records := make([]presentation.Record, 0, len(claim.Evidence))
		for _, evidence := range claim.Evidence {
			sites := make([]string, 0, len(evidence.Sites))
			for _, site := range evidence.Sites {
				sites = append(sites, fmt.Sprintf("%s:%d", site.Path, site.Line))
			}
			records = append(records, record(prose(evidence.Kind), prose(strconv.Itoa(evidence.Count)), literal(listText(sites))))
		}
		nodes = append(nodes, recordGroup("claim-evidence", []string{"kind", "count", "sites"}, records...))
	}
	if containsFacet(facets, FacetReferences) {
		nodes = append(nodes, recordGroup("claim-references", []string{"incoming", "outgoing"}, record(literal(listText(claim.Incoming)), literal(listText(claim.Outgoing)))))
	}
	return nodes
}

func relationshipFields(relationships contextRelationships) []presentation.Node {
	nodes := []presentation.Node{}
	if len(relationships.State) > 0 {
		nodes = append(nodes, literalField("state", strings.Join(relationships.State, ", ")))
	}
	if len(relationships.Touches) > 0 {
		nodes = append(nodes, literalField("touches", strings.Join(relationships.Touches, ", ")))
	}
	if len(relationships.Proofs) > 0 {
		nodes = append(nodes, literalField("proofs", strings.Join(relationships.Proofs, ", ")))
	}
	return nodes
}
func listText(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

func orNone(text string) string {
	if text == "" {
		return "none"
	}
	return text
}
func literal(text string) presentation.Value {
	value, err := presentation.Literal(orNone(text))
	if err != nil { // coverage-ignore: mapper inputs are normalized context identities
		panic(err)
	}
	return value
}
func prose(text string) presentation.Value {
	value, err := presentation.Prose(orNone(text))
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
func literalField(label, text string) presentation.Field {
	result, err := presentation.NewField(label, literal(text))
	if err != nil { // coverage-ignore: this mapper owns each literal label and uses a validated value
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
