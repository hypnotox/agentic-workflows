// Package topic parses and validates current-state topic inputs.
package topic

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	awfrender "github.com/hypnotox/agentic-workflows/internal/render"
	"gopkg.in/yaml.v3"
)

var (
	kebabRE        = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	claimHeadingRE = regexp.MustCompile("^### `((?:rule|invariant)): ([a-z0-9]+(?:-[a-z0-9]+)*)`$")
	adrRE          = regexp.MustCompile(`^ADR-([a-z0-9]+(?:-[a-z0-9]+)*)$`)
	allDigitsRE    = regexp.MustCompile(`^[0-9]+$`)
	claimIDRE      = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*/[a-z0-9]+(?:-[a-z0-9]+)*:[a-z0-9]+(?:-[a-z0-9]+)*$`)
	headingRE      = regexp.MustCompile(`^#{1,3}(?: |$)`)
)

type TopicID struct{ Domain, Slug string }

func (id TopicID) String() string { return id.Domain + "/" + id.Slug }

type Metadata struct {
	Title, Summary string
	Paths          []string
	Applies        string
}
type ClaimType string

const (
	Rule      ClaimType = "rule"
	Invariant ClaimType = "invariant"
)

type Backing string

const (
	NoBacking         Backing = ""
	ExplicitNoBacking Backing = "none"
	TestBacking       Backing = "test"
	Unbacked          Backing = "unbacked"
)

type Claim struct {
	ID, Slug              string
	Type                  ClaimType
	Prose                 string
	Summary               string
	Origin                string
	RevisedBy, References []string
	Backing               Backing
	Verify                string
}
type Topic struct {
	ID                     TopicID
	Metadata               Metadata
	Intro, Part            string
	Claims                 []Claim
	MetadataPath, PartPath string
}

type metadataYAML struct {
	Title   string
	Summary string
	Paths   []string
	Applies string
}

func (m *metadataYAML) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return errors.New("topic metadata must be a mapping")
	}
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i].Value, node.Content[i+1]
		if seen[key] {
			return fmt.Errorf("field %s already set in topic metadata", key)
		}
		seen[key] = true
		switch key {
		case "title":
			if err := decodeMetadataString(value, &m.Title, "topic title"); err != nil {
				return err
			}
		case "summary":
			if err := decodeMetadataString(value, &m.Summary, "topic summary"); err != nil {
				return err
			}
		case "paths":
			if value.Kind != yaml.SequenceNode {
				return errors.New("topic paths must be a sequence of string scalars")
			}
			m.Paths = make([]string, len(value.Content))
			for j, item := range value.Content {
				if err := decodeMetadataString(item, &m.Paths[j], fmt.Sprintf("topic paths[%d]", j)); err != nil {
					return err
				}
			}
		case "applies":
			if err := decodeMetadataString(value, &m.Applies, "topic applies"); err != nil {
				return err
			}
		default:
			return fmt.Errorf("field %s not found in topic metadata", key)
		}
	}
	return nil
}

func decodeMetadataString(node *yaml.Node, dst *string, field string) error {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return fmt.Errorf("%s must be a string scalar", field)
	}
	*dst = node.Value
	return nil
}

func ParseMetadata(metadataRoot, path string, data []byte) (TopicID, Metadata, error) {
	id, err := idFromMetadataPath(metadataRoot, path)
	if err != nil {
		return TopicID{}, Metadata{}, err
	}
	var raw metadataYAML
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return TopicID{}, Metadata{}, fmt.Errorf("parse topic metadata %s: %w", filepath.ToSlash(path), err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple YAML documents are not allowed")
		}
		return TopicID{}, Metadata{}, fmt.Errorf("parse topic metadata %s: %w", filepath.ToSlash(path), err)
	}
	m := Metadata{Title: strings.TrimSpace(raw.Title), Summary: strings.TrimSpace(raw.Summary), Paths: raw.Paths, Applies: raw.Applies}
	if m.Title == "" {
		return id, m, errors.New("topic title must not be empty")
	}
	if m.Summary == "" || strings.ContainsAny(m.Summary, "\r\n") {
		return id, m, errors.New("topic summary must be one nonempty line")
	}
	if m.Applies != "" && m.Applies != "global" {
		return id, m, fmt.Errorf("topic applies must be global; got %q", m.Applies)
	}
	if len(m.Paths) == 0 && m.Applies == "" {
		return id, m, errors.New("topic must declare nonempty paths, applies: global, or both")
	}
	seen := map[string]bool{}
	for _, g := range m.Paths {
		if g == "" {
			return id, m, errors.New("topic path must not be empty")
		}
		if seen[g] {
			return id, m, fmt.Errorf("duplicate topic path %q", g)
		}
		seen[g] = true
		if err := pathglob.Validate(g); err != nil {
			return id, m, fmt.Errorf("topic path: %w", err)
		}
	}
	return id, m, nil
}

func idFromMetadataPath(metadataRoot, path string) (TopicID, error) {
	rel, err := filepath.Rel(metadataRoot, path)
	if err != nil { // coverage-ignore: metadataRoot and discovered paths share the project root and therefore the same volume
		return TopicID{}, fmt.Errorf("resolve topic metadata path %q: %w", filepath.ToSlash(path), err)
	}
	clean := filepath.ToSlash(rel)
	seg := strings.Split(clean, "/")
	if len(seg) != 2 || seg[0] == ".." || filepath.Ext(seg[1]) != ".yaml" {
		return TopicID{}, fmt.Errorf("topic metadata path %q must be exactly <domain>/<topic>.yaml below %q", clean, filepath.ToSlash(metadataRoot))
	}
	id := TopicID{seg[0], strings.TrimSuffix(seg[1], ".yaml")}
	if !kebabRE.MatchString(id.Domain) || !kebabRE.MatchString(id.Slug) {
		return TopicID{}, fmt.Errorf("topic identity %q must use lowercase kebab-case components", id.String())
	}
	return id, nil
}

func ParsePart(id TopicID, path string, data []byte) (Topic, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	semantic, err := awfrender.StripAuthoringComments(text)
	if err != nil {
		return Topic{}, err
	}
	lines := strings.Split(semantic, "\n")
	claimsAt := -1
	for i, line := range lines {
		if line == "## Claims" {
			if claimsAt >= 0 {
				return Topic{}, errors.New("topic part must contain exactly one ## Claims section")
			}
			claimsAt = i
		}
	}
	if claimsAt < 0 {
		return Topic{}, errors.New("topic part must end with a ## Claims section")
	}
	for _, line := range lines[claimsAt+1:] {
		if strings.HasPrefix(line, "## ") {
			return Topic{}, errors.New("## Claims must be the final level-two section")
		}
	}
	intro := strings.TrimSpace(strings.Join(lines[:claimsAt], "\n"))
	if intro == "" {
		return Topic{}, errors.New("topic part must contain explanatory prose before ## Claims")
	}
	region := lines[claimsAt+1:]
	var claims []Claim
	seen := map[string]bool{}
	for i := 0; i < len(region); {
		if strings.TrimSpace(region[i]) == "" {
			i++
			continue
		}
		m := claimHeadingRE.FindStringSubmatch(region[i])
		if m == nil {
			return Topic{}, fmt.Errorf("invalid content in Claims region at line %d: expected a canonical claim heading", claimsAt+i+2)
		}
		start := i
		i++
		for i < len(region) && claimHeadingRE.FindStringSubmatch(region[i]) == nil {
			if headingRE.MatchString(region[i]) {
				return Topic{}, fmt.Errorf("heading inside claim %q is not allowed", m[2])
			}
			i++
		}
		if seen[m[2]] {
			return Topic{}, fmt.Errorf("duplicate local claim slug %q", m[2])
		}
		seen[m[2]] = true
		claim, err := parseClaim(id, ClaimType(m[1]), m[2], region[start+1:i])
		if err != nil {
			return Topic{}, fmt.Errorf("claim %s:%s: %w", id.String(), m[2], err)
		}
		claims = append(claims, claim)
	}
	return Topic{ID: id, Intro: intro, Part: text, Claims: claims, PartPath: path}, nil
}

func parseClaim(id TopicID, typ ClaimType, slug string, lines []string) (Claim, error) {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	metaStart := len(lines)
	for metaStart > 0 && isMetadataLine(lines[metaStart-1]) {
		metaStart--
	}
	for _, line := range lines[:metaStart] {
		trim := strings.TrimSpace(line)
		if reservedMetadata(trim) {
			return Claim{}, fmt.Errorf("malformed or out-of-order reserved metadata %q", trim)
		}
	}
	prose := strings.TrimSpace(strings.Join(lines[:metaStart], "\n"))
	if prose == "" {
		return Claim{}, errors.New("claim prose must not be empty")
	}
	meta := lines[metaStart:]
	c := Claim{ID: id.String() + ":" + slug, Slug: slug, Type: typ, Prose: prose}
	pos := 0
	need := func(prefix string) (string, error) {
		if pos >= len(meta) || !strings.HasPrefix(meta[pos], prefix) {
			return "", fmt.Errorf("expected %s metadata", strings.TrimSuffix(prefix, ": "))
		}
		v := strings.TrimSpace(strings.TrimPrefix(meta[pos], prefix))
		pos++
		return v, nil
	}
	var err error
	if pos < len(meta) && strings.HasPrefix(meta[pos], "Summary: ") {
		c.Summary, err = need("Summary: ")
		if err != nil { // coverage-ignore: the identical prefix guard above makes need succeed
			return Claim{}, err
		}
		if c.Summary == "" || strings.ContainsAny(c.Summary, "\r\n") { // coverage-ignore: line tokenization and metadata recognition reject blank or multiline values before this semantic guard
			return Claim{}, errors.New("summary must be one nonempty line")
		}
		if len([]rune(c.Summary)) > 160 {
			return Claim{}, errors.New("summary must be at most 160 Unicode code points")
		}
	}
	origin, err := need("Origin: ")
	if err != nil {
		return Claim{}, err
	}
	c.Origin, err = parseADRRef(origin)
	if err != nil {
		return Claim{}, fmt.Errorf("origin must be ADR-NNNN or ADR-<slug>; got %q", origin)
	}
	if pos < len(meta) && strings.HasPrefix(meta[pos], "Revised-by: ") {
		v, _ := need("Revised-by: ")
		c.RevisedBy, err = parseADRList(v)
		if err != nil {
			return Claim{}, fmt.Errorf("revised-by: %w", err)
		}
	}
	if pos < len(meta) && strings.HasPrefix(meta[pos], "References: ") {
		v, _ := need("References: ")
		c.References, err = parseClaimList(v)
		if err != nil {
			return Claim{}, fmt.Errorf("references: %w", err)
		}
	}
	if typ == Rule {
		if pos != len(meta) {
			return Claim{}, errors.New("rules must not declare backing metadata")
		}
		return c, nil
	}
	v, err := need("Backing: ")
	if err != nil {
		return Claim{}, err
	}
	c.Backing = Backing(v)
	switch c.Backing {
	case TestBacking:
		if pos != len(meta) {
			return Claim{}, errors.New("test-backed invariant must not declare Verify")
		}
	case Unbacked:
		c.Verify, err = need("Verify: ")
		if err != nil {
			return Claim{}, err
		}
		if pos != len(meta) {
			return Claim{}, errors.New("unexpected metadata after Verify")
		}
	default:
		return Claim{}, fmt.Errorf("Backing must be test or unbacked; got %q", v)
	}
	return c, nil
}

func isMetadataLine(line string) bool {
	t := strings.TrimSpace(line)
	for _, p := range []string{"Summary: ", "Origin: ", "Revised-by: ", "References: ", "Backing: ", "Verify: "} {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}
func reservedMetadata(line string) bool {
	for _, p := range []string{"Summary", "Origin", "Revised-by", "References", "Backing", "Verify"} {
		if strings.HasPrefix(line, p+":") || strings.HasPrefix(line, p+" ") {
			return true
		}
	}
	return false
}

// parseADRRef reads one ADR provenance reference in either identity form: the
// four-digit number of a numbered record, or the slug of a pending record
// awaiting its number at integration (ADR-0202 item 10). A purely numeric token
// of any other length is neither form and is rejected, so a mistyped number can
// never be read as a slug.
func parseADRRef(s string) (string, error) {
	m := adrRE.FindStringSubmatch(s)
	if m == nil {
		return "", fmt.Errorf("expected ADR-NNNN or ADR-<slug>; got %q", s)
	}
	if allDigitsRE.MatchString(m[1]) && len(m[1]) != 4 {
		return "", fmt.Errorf("expected ADR-NNNN or ADR-<slug>; got %q", s)
	}
	return m[1], nil
}

func parseADRList(v string) ([]string, error) {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, p := range parts {
		ref, err := parseADRRef(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		if seen[ref] {
			return nil, fmt.Errorf("duplicate ADR-%s", ref)
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out, nil
}
func parseClaimList(v string) ([]string, error) {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if !claimIDRE.MatchString(p) {
			return nil, fmt.Errorf("invalid qualified claim ID %q", p)
		}
		if seen[p] {
			return nil, fmt.Errorf("duplicate claim reference %q", p)
		}
		seen[p] = true
		out = append(out, p)
	}
	return out, nil
}
