// Package topic parses and validates current-state topic inputs.
package topic

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"gopkg.in/yaml.v3"
)

var (
	kebabRE        = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	claimHeadingRE = regexp.MustCompile("^### `((?:rule|invariant)): ([a-z0-9]+(?:-[a-z0-9]+)*)`$")
	adrRE          = regexp.MustCompile(`^ADR-([0-9]{4})$`)
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
	NoBacking   Backing = ""
	TestBacking Backing = "test"
	Unbacked    Backing = "unbacked"
)

type Claim struct {
	ID, Slug              string
	Type                  ClaimType
	Prose                 string
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
	Title   string   `yaml:"title"`
	Summary string   `yaml:"summary"`
	Paths   []string `yaml:"paths"`
	Applies string   `yaml:"applies"`
}

func ParseMetadata(path string, data []byte) (TopicID, Metadata, error) {
	id, err := idFromMetadataPath(path)
	if err != nil {
		return TopicID{}, Metadata{}, err
	}
	var raw metadataYAML
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return TopicID{}, Metadata{}, fmt.Errorf("parse topic metadata %s: %w", filepath.ToSlash(path), err)
	}
	m := Metadata{Title: strings.TrimSpace(raw.Title), Summary: strings.TrimSpace(raw.Summary), Paths: raw.Paths, Applies: raw.Applies}
	if m.Title == "" {
		return id, m, errors.New("topic title must not be empty")
	}
	if m.Summary == "" || strings.ContainsAny(m.Summary, "\r\n") {
		return id, m, errors.New("topic summary must be one nonempty line")
	}
	if (len(m.Paths) > 0) == (m.Applies != "") {
		return id, m, errors.New("topic must declare exactly one of nonempty paths or applies: global")
	}
	if m.Applies != "" && m.Applies != "global" {
		return id, m, fmt.Errorf("topic applies must be global; got %q", m.Applies)
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

func idFromMetadataPath(path string) (TopicID, error) {
	clean := filepath.ToSlash(filepath.Clean(path))
	seg := strings.Split(clean, "/")
	if len(seg) < 4 || seg[len(seg)-3] != "metadata" || filepath.Ext(seg[len(seg)-1]) != ".yaml" {
		return TopicID{}, fmt.Errorf("topic metadata path %q must end in metadata/<domain>/<topic>.yaml", clean)
	}
	id := TopicID{seg[len(seg)-2], strings.TrimSuffix(seg[len(seg)-1], ".yaml")}
	if !kebabRE.MatchString(id.Domain) || !kebabRE.MatchString(id.Slug) {
		return TopicID{}, fmt.Errorf("topic identity %q must use lowercase kebab-case components", id.String())
	}
	return id, nil
}

func ParsePart(id TopicID, path string, data []byte) (Topic, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
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
	origin, err := need("Origin: ")
	if err != nil {
		return Claim{}, err
	}
	m := adrRE.FindStringSubmatch(origin)
	if m == nil {
		return Claim{}, fmt.Errorf("origin must be ADR-NNNN; got %q", origin)
	}
	c.Origin = m[1]
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
	for _, p := range []string{"Origin: ", "Revised-by: ", "References: ", "Backing: ", "Verify: "} {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}
func reservedMetadata(line string) bool {
	for _, p := range []string{"Origin", "Revised-by", "References", "Backing", "Verify"} {
		if strings.HasPrefix(line, p+":") || strings.HasPrefix(line, p+" ") {
			return true
		}
	}
	return false
}
func parseADRList(v string) ([]string, error) {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		m := adrRE.FindStringSubmatch(p)
		if m == nil {
			return nil, fmt.Errorf("expected ADR-NNNN; got %q", p)
		}
		if seen[m[1]] {
			return nil, fmt.Errorf("duplicate ADR-%s", m[1])
		}
		seen[m[1]] = true
		out = append(out, m[1])
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

func readMetadata(path string) (TopicID, Metadata, error) {
	b, err := os.ReadFile(path)
	if err != nil { // coverage-ignore: callers pass files just discovered by WalkDir; failure requires a concurrent filesystem race
		return TopicID{}, Metadata{}, err
	}
	return ParseMetadata(path, b)
}
