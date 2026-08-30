package topic

import (
	"fmt"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// Detail maps every selected topic query result field into the shared
// presentation tree. Topic owns this semantic mapping; presentation owns
// validation and text rendering.
func (result QueryResult) Detail() presentation.Detail {
	fields := []presentation.Field{topicLiteralField("identity", result.Kind+" "+result.ID)}
	sections := []presentation.Section{}
	if result.Title != "" {
		sections = append(sections, topicSection("topic", topicField("title", result.Title), topicField("summary", result.Summary)))
	}
	if len(result.Claims) > 0 {
		nodes := make([]presentation.Node, 0, len(result.Claims))
		for _, claim := range result.Claims {
			claimNodes := []presentation.Node{topicField("type", string(claim.Type)), topicField("backing", string(claim.Backing)), topicField("prose", claim.Prose)}
			if claim.Verify != "" {
				claimNodes = append(claimNodes, topicField("verify", claim.Verify))
			}
			nodes = append(nodes, topicSection("claim", append([]presentation.Node{topicLiteralField("identity", claim.ID)}, claimNodes...)...))
		}
		sections = append(sections, topicSection("claims", nodes...))
	}
	if result.References != nil {
		nodes := make([]presentation.Node, 0, len(result.References))
		for _, refs := range result.References {
			nodes = append(nodes, topicSection("claim", topicLiteralField("identity", refs.ClaimID), topicLiteralField("incoming", topicList(refs.Incoming)), topicLiteralField("outgoing", topicList(refs.Outgoing))))
		}
		if len(nodes) > 0 {
			sections = append(sections, topicSection("references", nodes...))
		}
	}
	if result.Coverage != nil {
		a := result.Coverage.Applicability
		nodes := []presentation.Node{}
		if a.DeclaredGlobal {
			nodes = append(nodes, topicField("declared", "global"))
		}
		nodes = append(nodes, presentationTopicItems("domain-paths", a.DomainPaths), presentationTopicItems("topic-paths", a.TopicPaths), topicField("selector-rule", "both domain and topic selectors must match for ownership"), presentationTopicItems("applicable-paths", a.ApplicablePaths), presentationTopicItems("owned-paths", a.OwnedPaths))
		for _, site := range a.MarkerSites {
			text := fmt.Sprintf("%s:%d | %s | %s", site.Path, site.Line, site.Kind, site.ClaimID)
			if site.Note != "" {
				text += " | " + site.Note
			}
			nodes = append(nodes, topicLiteralField("marker", text))
		}
		sections = append(sections, topicSection("coverage", nodes...))
	}
	return presentation.Detail{Fields: fields, Sections: sections}
}

// StaticReferenceDetail maps the unadopted-project topic reference into the
// same presentation tree as ordinary topic output.
func StaticReferenceDetail() presentation.Detail {
	return presentation.Detail{
		Fields:   []presentation.Field{topicField("topic", "static not inside an awf project")},
		Sections: []presentation.Section{topicSection("reference", topicField("description", "Query active current-state topics and claims. Use references for direct claim IDs and coverage for scope and marker sites."))},
	}
}

func topicList(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}
func presentationTopicItems(label string, items []string) presentation.Node {
	if len(items) == 0 {
		return topicField(label, "none")
	}
	values := make([]presentation.Value, 0, len(items))
	for _, item := range items {
		value, err := topicLiteral(item)
		if err != nil {
			panic(err)
		}
		values = append(values, value)
	}
	list, err := presentation.NewList(label, values...)
	if err != nil {
		panic(err)
	}
	return list
}
func topicValue(text string) (presentation.Value, error)   { return presentation.Prose(text) }
func topicLiteral(text string) (presentation.Value, error) { return presentation.Literal(text) }
func topicField(label, text string) presentation.Field {
	value, err := topicValue(text)
	if err != nil {
		panic(err)
	}
	field, err := presentation.NewField(label, value)
	if err != nil {
		panic(err)
	}
	return field
}
func topicLiteralField(label, text string) presentation.Field {
	value, err := topicLiteral(text)
	if err != nil {
		panic(err)
	}
	field, err := presentation.NewField(label, value)
	if err != nil {
		panic(err)
	}
	return field
}
func topicSection(label string, nodes ...presentation.Node) presentation.Section {
	section, err := presentation.NewSection(label, nodes...)
	if err != nil {
		panic(err)
	}
	return section
}
