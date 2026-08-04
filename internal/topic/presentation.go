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
	fields := []presentation.Field{topicField("identity", result.Kind+" "+result.ID)}
	if result.HistoricalOnly {
		fields = append(fields, topicField("historical-only", "no active claim"))
	}
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
			nodes = append(nodes, topicSection("claim", append([]presentation.Node{topicField("identity", claim.ID)}, claimNodes...)...))
		}
		sections = append(sections, topicSection("claims", nodes...))
	}
	if result.History != nil {
		nodes := make([]presentation.Node, 0, len(result.History))
		for _, history := range result.History {
			historyNodes := []presentation.Node{topicField("identity", history.ClaimID)}
			if history.LegacyBaseline {
				historyNodes = append(historyNodes, topicField("origin", "legacy baseline not retained in active authority"))
			} else if history.Origin != nil {
				historyNodes = append(historyNodes, topicField("origin", historyText(*history.Origin)))
			}
			for _, revision := range history.RevisedBy {
				historyNodes = append(historyNodes, topicField("revised-by", historyText(revision)))
			}
			if history.RemovedBy != nil {
				historyNodes = append(historyNodes, topicField("removed-by", historyText(*history.RemovedBy)))
			}
			nodes = append(nodes, topicSection("claim", historyNodes...))
		}
		if len(nodes) > 0 {
			sections = append(sections, topicSection("history", nodes...))
		}
	}
	if result.References != nil {
		nodes := make([]presentation.Node, 0, len(result.References))
		for _, refs := range result.References {
			nodes = append(nodes, topicSection("claim", topicField("identity", refs.ClaimID), topicField("incoming", topicList(refs.Incoming)), topicField("outgoing", topicList(refs.Outgoing))))
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
		nodes = append(nodes, topicField("domain-paths", topicList(a.DomainPaths)), topicField("topic-paths", topicList(a.TopicPaths)), topicField("selector-rule", "both domain and topic selectors must match"), topicField("matched-paths", topicList(a.MatchedPaths)))
		for _, site := range a.MarkerSites {
			text := fmt.Sprintf("%s:%d | %s | %s", site.Path, site.Line, site.Kind, site.ClaimID)
			if site.Note != "" {
				text += " | " + site.Note
			}
			nodes = append(nodes, topicField("marker", text))
		}
		sections = append(sections, topicSection("coverage", nodes...))
	}
	return presentation.Detail{Fields: fields, Sections: sections}
}

func historyText(history ADRHistory) string {
	return fmt.Sprintf("ADR-%s | %s | %s", history.Number, history.Status, history.Title)
}
func topicList(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}
func topicValue(text string) (presentation.Value, error) { return presentation.Prose(text) }
func topicField(label, text string) presentation.Field {
	value, err := topicValue(text)
	if err != nil { // coverage-ignore: topic parser values cannot contain invalid presentation whitespace
		panic(err)
	}
	field, err := presentation.NewField(label, value)
	if err != nil { // coverage-ignore: this mapper owns each literal label
		panic(err)
	}
	return field
}
func topicSection(label string, nodes ...presentation.Node) presentation.Section {
	section, err := presentation.NewSection(label, nodes...)
	if err != nil { // coverage-ignore: every call supplies at least one mapped node
		panic(err)
	}
	return section
}
