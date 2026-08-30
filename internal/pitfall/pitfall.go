// Package pitfall owns authored pitfall identity and source semantics.
package pitfall

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
	"gopkg.in/yaml.v3"
)

const SourceDir = ".awf/docs/pitfalls"

var slugRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Entry struct {
	Slug, SourcePath, Title string
	Domains                 []string
	Related                 []int
	Body                    string
	Source                  []byte
}

type Corpus struct{ entries []Entry }

func New(entries []Entry) Corpus { return Corpus{entries: slices.Clone(entries)} }
func (c Corpus) All() []Entry    { return slices.Clone(c.entries) }
func (c Corpus) Len() int        { return len(c.entries) }

const scaffoldBody = "Describe the durable hazard, its consequence, and the safer practice.\n"

// Scaffold derives one canonical authored source from the current corpus.
func (c Corpus) Scaffold(rawTitle string) (Entry, []byte, error) {
	title := strings.TrimSpace(rawTitle)
	if title == "" {
		return Entry{}, nil, errors.New("pitfall title is empty")
	}
	if strings.ContainsAny(rawTitle, "\r\n") {
		return Entry{}, nil, errors.New("pitfall title contains CR or LF")
	}
	used := make(map[string]bool, len(c.entries))
	for _, existing := range c.entries {
		if EqualTitle(title, existing.Title) {
			return Entry{}, nil, fmt.Errorf("pitfall title %q duplicates %s under Unicode whitespace and simple folding", title, existing.SourcePath)
		}
		used[existing.Slug] = true
	}
	slug, err := AllocateSlug(title, used)
	if err != nil {
		return Entry{}, nil, err
	}
	entry := Entry{Slug: slug, SourcePath: SourceDir + "/" + slug + ".md", Title: title, Body: scaffoldBody}
	serialized, err := Serialize(entry)
	if err != nil {
		return Entry{}, nil, err
	}
	return entry, serialized, nil
}

type SourceFile struct {
	Path    string
	Bytes   []byte
	Regular bool
}

type metadata struct {
	Title   *string
	Domains presentNode
	Related presentNode
}

type presentNode struct {
	present bool
	node    yaml.Node
}

func (m *metadata) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return errors.New("pitfall metadata must be a mapping")
	}
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i].Value, node.Content[i+1]
		if seen[key] {
			return fmt.Errorf("duplicate pitfall metadata key %q", key)
		}
		seen[key] = true
		switch key {
		case "title":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return errors.New("pitfall title must be a string scalar")
			}
			title := value.Value
			m.Title = &title
		case "domains":
			m.Domains = presentNode{present: true, node: *value}
		case "related":
			m.Related = presentNode{present: true, node: *value}
		default:
			return fmt.Errorf("field %s not found in type pitfall.metadata", key)
		}
	}
	return nil
}

// Load parses and validates a complete injected direct-leaf source set.
func Load(files []SourceFile) (Corpus, error) {
	entries := make([]Entry, 0, len(files))
	seenPaths := map[string]bool{}
	for _, f := range files {
		e, err := Parse(f)
		if err != nil {
			return Corpus{}, err
		}
		if seenPaths[e.SourcePath] {
			return Corpus{}, fmt.Errorf("%s: duplicate source path", e.SourcePath)
		}
		seenPaths[e.SourcePath] = true
		for _, prior := range entries {
			if EqualTitle(e.Title, prior.Title) {
				return Corpus{}, fmt.Errorf("%s: title %q duplicates %s under Unicode whitespace and simple folding", e.SourcePath, e.Title, prior.SourcePath)
			}
		}
		entries = append(entries, e)
	}
	slices.SortFunc(entries, func(a, b Entry) int { return strings.Compare(a.Slug, b.Slug) })
	return New(entries), nil
}

func Parse(f SourceFile) (Entry, error) {
	clean := path.Clean(f.Path)
	if f.Path == "" || path.IsAbs(f.Path) || f.Path != clean || strings.Contains(f.Path, `\`) {
		return Entry{}, fmt.Errorf("%s: pitfall source path must already be canonical slash-relative form", f.Path)
	}
	prefix := SourceDir + "/"
	name, ok := strings.CutPrefix(f.Path, prefix)
	if !ok || name == "" || strings.Contains(name, "/") {
		return Entry{}, fmt.Errorf("%s: pitfall source must be a direct child of %s", f.Path, SourceDir)
	}
	if !f.Regular {
		return Entry{}, fmt.Errorf("%s: pitfall source must be a regular file", f.Path)
	}
	if path.Ext(name) != ".md" {
		return Entry{}, fmt.Errorf("%s: pitfall source must use the .md extension", f.Path)
	}
	slug := strings.TrimSuffix(name, ".md")
	if slug == "index" {
		return Entry{}, fmt.Errorf("%s: slug index is reserved", f.Path)
	}
	if !slugRE.MatchString(slug) {
		return Entry{}, fmt.Errorf("%s: invalid pitfall slug %q", f.Path, slug)
	}
	var m metadata
	body, found, err := frontmatter.ParseStrict(f.Bytes, &m)
	if err != nil {
		return Entry{}, fmt.Errorf("%s: %w", f.Path, err)
	}
	if !found {
		return Entry{}, fmt.Errorf("%s: missing frontmatter", f.Path)
	}
	if m.Title == nil {
		return Entry{}, fmt.Errorf("%s: missing required title", f.Path)
	}
	title := strings.TrimSpace(*m.Title)
	if title == "" {
		return Entry{}, fmt.Errorf("%s: title is empty", f.Path)
	}
	if strings.ContainsAny(*m.Title, "\r\n") {
		return Entry{}, fmt.Errorf("%s: title contains CR or LF", f.Path)
	}
	if strings.TrimSpace(string(body)) == "" {
		return Entry{}, fmt.Errorf("%s: body is empty", f.Path)
	}
	domains, err := decodeOptionalStringList(f.Path, "domains", m.Domains)
	if err != nil {
		return Entry{}, err
	}
	related, err := decodeOptionalIntegerList(f.Path, "related", m.Related)
	if err != nil {
		return Entry{}, err
	}
	if err := validateStrings(f.Path, "domains", domains); err != nil {
		return Entry{}, err
	}
	seenRelated := map[int]bool{}
	for _, n := range related {
		if n <= 0 {
			return Entry{}, fmt.Errorf("%s: related entries must be positive ADR numbers", f.Path)
		}
		if seenRelated[n] {
			return Entry{}, fmt.Errorf("%s: duplicate related entry %d", f.Path, n)
		}
		seenRelated[n] = true
	}
	return Entry{Slug: slug, SourcePath: f.Path, Title: title, Domains: domains, Related: related, Body: string(body), Source: slices.Clone(f.Bytes)}, nil
}

func optionalSequence(source, field string, value presentNode) ([]*yaml.Node, error) {
	if !value.present {
		return nil, nil
	}
	if value.node.Kind != yaml.SequenceNode || len(value.node.Content) == 0 {
		return nil, fmt.Errorf("%s: explicitly supplied %s must be a nonempty list", source, field)
	}
	return value.node.Content, nil
}

func decodeOptionalStringList(source, field string, value presentNode) ([]string, error) {
	nodes, err := optionalSequence(source, field, value)
	if err != nil || nodes == nil {
		return nil, err
	}
	values := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
			return nil, fmt.Errorf("%s: %s entries must be string scalars", source, field)
		}
		values = append(values, node.Value)
	}
	return values, nil
}

func decodeOptionalIntegerList(source, field string, value presentNode) ([]int, error) {
	nodes, err := optionalSequence(source, field, value)
	if err != nil || nodes == nil {
		return nil, err
	}
	values := make([]int, 0, len(nodes))
	for _, node := range nodes {
		if node.Kind != yaml.ScalarNode || node.Tag != "!!int" {
			return nil, fmt.Errorf("%s: %s entries must be integer scalars", source, field)
		}
		var number int
		if err := node.Decode(&number); err != nil {
			return nil, fmt.Errorf("%s: %s entries must be integer scalars: %w", source, field, err)
		}
		values = append(values, number)
	}
	return values, nil
}

func validateStrings(source, field string, values []string) error {
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" || strings.TrimSpace(value) != value {
			return fmt.Errorf("%s: %s entries must be nonempty and trimmed", source, field)
		}
		if seen[value] {
			return fmt.Errorf("%s: duplicate %s entry %q", source, field, value)
		}
		seen[value] = true
	}
	return nil
}

// EqualTitle applies Go simple Unicode folding after Unicode whitespace collapse.
func EqualTitle(a, b string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(a), " "), strings.Join(strings.Fields(b), " "))
}

// AllocateSlug chooses the first free deterministic ASCII slug.
func AllocateSlug(title string, used map[string]bool) (string, error) {
	var b strings.Builder
	hyphen := false
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if hyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			hyphen = false
		} else {
			hyphen = true
		}
	}
	base := strings.Trim(b.String(), "-")
	if base == "" {
		return "", fmt.Errorf("title %q has no ASCII slug characters", title)
	}
	if base == "index" {
		return "", fmt.Errorf("title %q produces reserved slug index", title)
	}
	for n := 1; ; n++ {
		candidate := base
		if n > 1 {
			candidate += "-" + strconv.Itoa(n)
		}
		if !used[candidate] {
			return candidate, nil
		}
	}
}

// Serialize emits the canonical V1 source form.
func Serialize(e Entry) ([]byte, error) {
	if strings.TrimSpace(e.Body) == "" {
		return nil, fmt.Errorf("pitfall %q body is empty", e.Title)
	}
	m := struct {
		Title   string   `yaml:"title"`
		Domains []string `yaml:"domains,omitempty,flow"`
		Related []int    `yaml:"related,omitempty,flow"`
	}{e.Title, e.Domains, e.Related}
	y, err := yaml.Marshal(m)
	if err != nil {
		return nil, err
	}
	body := e.Body
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return append(append([]byte("---\n"), y...), append([]byte("---\n"), []byte(body)...)...), nil
}

const commonmarkPunct = `!"#$%&'()*+,-./:;<=>?@[\]^_` + "`" + `{|}~`

func EscapeTitle(title string) string {
	var b strings.Builder
	for _, r := range title {
		switch r {
		case '\r':
			b.WriteString("&#13;")
			continue
		case '\n':
			b.WriteString("&#10;")
			continue
		}
		if r == '\\' || strings.ContainsRune(commonmarkPunct, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
func EscapeHeading(title string) string   { return EscapeTitle(title) }
func EscapeLinkLabel(title string) string { return EscapeTitle(title) }
func EscapeTableCell(s string) string     { return EscapeTitle(s) }
