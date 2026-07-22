package topic

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"

	awfrender "github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

type TopicRenderModel struct{ Title, Summary, Applicability, Part string }
type TopicListItem struct{ Slug, Title, Summary, Link string }
type IndexRenderModel struct {
	Domain string
	Topics []TopicListItem
}
type NavigationModel struct {
	IndexLink string
	Topics    []TopicListItem
}

func BuildTopicModel(t Topic, domainPaths []string, markers MarkerIndex, currentPaths []string) TopicRenderModel {
	return TopicRenderModel{Title: t.Metadata.Title, Summary: t.Metadata.Summary, Applicability: applicabilitySummary(t.ID.String(), ApplicabilityForTopic(t, domainPaths, markers, currentPaths)), Part: t.Part}
}
func BuildIndexModel(domain string, topics []Topic) IndexRenderModel {
	items := topicItems(topics, "")
	return IndexRenderModel{Domain: domain, Topics: items}
}
func BuildNavigationModel(domain string, topics []Topic) NavigationModel {
	return NavigationModel{IndexLink: "../topics/" + domain + "/index.md", Topics: topicItems(topics, "../topics/"+domain+"/")}
}
func topicItems(topics []Topic, prefix string) []TopicListItem {
	items := make([]TopicListItem, 0, len(topics))
	for _, t := range topics {
		items = append(items, TopicListItem{Slug: t.ID.Slug, Title: t.Metadata.Title, Summary: t.Metadata.Summary, Link: prefix + t.ID.Slug + ".md"})
	}
	slices.SortFunc(items, func(a, b TopicListItem) int {
		if c := strings.Compare(a.Title, b.Title); c != 0 {
			return c
		}
		if c := strings.Compare(a.Summary, b.Summary); c != 0 {
			return c
		}
		return strings.Compare(a.Slug, b.Slug)
	})
	return items
}

// applicabilitySummary renders the selectors-only applicability paragraph: the
// concrete matched paths and marker sites stay out of committed docs and live
// in `awf topic <id> --coverage` (ADR-0147).
func applicabilitySummary(id string, a TopicApplicability) string {
	drilldown := fmt.Sprintf("Run `awf topic %s --coverage` for current matched paths and marker sites.", id)
	if a.DeclaredGlobal {
		return fmt.Sprintf("Global topic within owning domain selectors %s. %s", selectorList(a.DomainPaths), drilldown)
	}
	return fmt.Sprintf("Owning domain selectors: %s. Topic selectors: %s. Both domain and topic selectors must match. %s", selectorList(a.DomainPaths), selectorList(a.TopicPaths), drilldown)
}

// selectorList renders a backticked selector list, degrading to the word none
// when the list is empty so the paragraph stays coherent prose.
func selectorList(globs []string) string {
	if len(globs) == 0 {
		return "none"
	}
	return "`" + strings.Join(globs, "`, `") + "`"
}

func RenderTopic(model TopicRenderModel) (string, error) {
	stripped, err := awfrender.StripAuthoringComments(model.Part)
	if err != nil {
		return "", err
	}
	return executeRaw("topics/topic.md.tmpl", map[string]any{"Title": model.Title, "Summary": model.Summary, "Applicability": model.Applicability}, strings.TrimRight(stripped, "\r\n"))
}
func RenderIndex(model IndexRenderModel) (string, error) {
	return execute("topics/index.md.tmpl", map[string]any{"Domain": model.Domain, "Topics": model.Topics})
}
func executeRaw(tid string, data map[string]any, raw string) (string, error) {
	const sent = "\x00awf:topic-part\x00"
	src, err := templateSource(tid)
	if err != nil {
		return "", err
	}
	src = strings.Replace(src, "{{ .Part }}", sent, 1)
	return awfrender.Execute(src, data, map[string]string{sent: raw}, tid)
}
func execute(tid string, data map[string]any) (string, error) {
	src, err := templateSource(tid)
	if err != nil {
		return "", err
	}
	return awfrender.Execute(src, data, nil, tid)
}
func templateSource(tid string) (string, error) {
	b, err := fs.ReadFile(templates.FS, tid)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", tid, err)
	}
	s, err := awfrender.StripAuthoringComments(string(b))
	if err != nil { // coverage-ignore: compile-time embedded topic templates contain only well-formed authoring comments
		return "", err
	}
	return s, nil
}
