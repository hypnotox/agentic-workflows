// Package pitfall owns authored pitfall identity and source semantics.
package pitfall

import (
	"bytes"
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
	Domains, Tags           []string
	Related                 []int
	Body                    string
	Source                  []byte
}

type Corpus struct{ entries []Entry }

func New(entries []Entry) Corpus { return Corpus{entries: slices.Clone(entries)} }
func (c Corpus) All() []Entry    { return slices.Clone(c.entries) }
func (c Corpus) Len() int        { return len(c.entries) }

type SourceFile struct {
	Path    string
	Bytes   []byte
	Regular bool
}

type metadata struct {
	Title   *string  `yaml:"title"`
	Domains []string `yaml:"domains,omitempty"`
	Tags    []string `yaml:"tags,omitempty"`
	Related []int    `yaml:"related,omitempty"`
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
	prefix := SourceDir + "/"
	name, ok := strings.CutPrefix(clean, prefix)
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
	if err := validateStrings(f.Path, "domains", m.Domains); err != nil {
		return Entry{}, err
	}
	if err := validateStrings(f.Path, "tags", m.Tags); err != nil {
		return Entry{}, err
	}
	seenRelated := map[int]bool{}
	for _, n := range m.Related {
		if n <= 0 {
			return Entry{}, fmt.Errorf("%s: related entries must be positive ADR numbers", f.Path)
		}
		if seenRelated[n] {
			return Entry{}, fmt.Errorf("%s: duplicate related entry %d", f.Path, n)
		}
		seenRelated[n] = true
	}
	return Entry{Slug: slug, SourcePath: clean, Title: title, Domains: slices.Clone(m.Domains), Tags: slices.Clone(m.Tags), Related: slices.Clone(m.Related), Body: string(body), Source: slices.Clone(f.Bytes)}, nil
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
		Tags    []string `yaml:"tags,omitempty,flow"`
		Related []int    `yaml:"related,omitempty,flow"`
	}{e.Title, e.Domains, e.Tags, e.Related}
	y, err := yaml.Marshal(m)
	if err != nil { // coverage-ignore: the closed metadata struct contains only scalar slices and yaml.Marshal cannot reject it
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

type RelativeLink struct{ Source, Destination string }

var inlineLinkRE = regexp.MustCompile(`!?\[[^\]\n]*\]\(\s*([^\s)]+)`)
var autoLinkRE = regexp.MustCompile(`<([^<>\s]+)>`)
var referenceRE = regexp.MustCompile(`(?m)^\s*\[[^\]\n]+\]:\s*(\S+)`)

// RelativeLinks detects path-relative Markdown destinations outside code.
func RelativeLinks(e Entry) []RelativeLink {
	text := maskCode(e.Body)
	var targets []string
	for _, re := range []*regexp.Regexp{inlineLinkRE, autoLinkRE, referenceRE} {
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			targets = append(targets, strings.Trim(m[1], "<>"))
		}
	}
	var out []RelativeLink
	for _, target := range targets {
		if isRelative(target) {
			out = append(out, RelativeLink{Source: e.SourcePath, Destination: target})
		}
	}
	return out
}

func isRelative(target string) bool {
	if target == "" || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "/") || strings.HasPrefix(target, "\\") || strings.HasPrefix(target, "//") {
		return false
	}
	colon := strings.IndexByte(target, ':')
	slash := strings.IndexByte(target, '/')
	return colon < 0 || (slash >= 0 && slash < colon)
}

func maskCode(s string) string {
	lines := strings.SplitAfter(s, "\n")
	fenced := false
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
			fenced = !fenced
			lines[i] = strings.Repeat(" ", len(line))
			continue
		}
		if fenced {
			lines[i] = strings.Repeat(" ", len(line))
			continue
		}
		var b bytes.Buffer
		inCode := false
		for _, r := range line {
			if r == '`' {
				inCode = !inCode
				b.WriteRune(' ')
				continue
			}
			if inCode {
				b.WriteRune(' ')
			} else {
				b.WriteRune(r)
			}
		}
		lines[i] = b.String()
	}
	return strings.Join(lines, "")
}
