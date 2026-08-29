package config

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"gopkg.in/yaml.v3"
)

// Skeleton is the input to MarshalSkeleton: the fields a freshly-scaffolded
// .awf/config.yaml carries. Vars is typed map[string]string (not map[string]any)
// so a nil/null var value is unrepresentable - the scaffold seeds each var with an
// empty string, which marshals as `x: ""`. A nil interface would marshal as
// `x: null` and decode back to a nil value that renders as "<no value>", tripping
// the publication-safe check (ADR-0026 Decision 3).
type Skeleton struct {
	Prefix  string          `yaml:"prefix"`
	Profile catalog.Profile `yaml:"profile"`
	// IntegrationBranch is written explicitly because the key is required and
	// carries no in-code default (ADR-0202 Decision 6): a scaffold omitting it
	// would emit a config that fails its own validation on the next open.
	IntegrationBranch string            `yaml:"integrationBranch"`
	Vars              map[string]string `yaml:"vars"`
	Audit             *SkeletonAudit    `yaml:"audit,omitempty"`
	Bootstrap         *BootstrapConfig  `yaml:"bootstrap,omitempty"`
}

// SkeletonAudit is the audit block a scaffold can seed (ADR-0051): only
// allowedScopes - the one audit field init collects. Deliberately not
// *AuditConfig, whose zero-value fields would serialize as explicit settings.
type SkeletonAudit struct {
	AllowedScopes []string `yaml:"allowedScopes"`
}

// MarshalSkeleton renders a fresh config.yaml from s in the canonical awf format
// (two-space block style). It is the construction half of internal/config's
// ownership of config.yaml serialization (ADR-0026).
func MarshalSkeleton(s Skeleton) ([]byte, error) {
	return encode(s)
}

// AppendLocalDoc appends one strict local-document declaration without changing
// unrelated config structure. It refuses malformed existing declarations and duplicates.
func AppendLocalDoc(src []byte, doc LocalDoc) ([]byte, error) {
	if _, err := Parse("", src); err != nil {
		return nil, err
	}
	document, root, err := parseMapping(src)
	if err != nil { // coverage-ignore: Parse accepted the same YAML mapping immediately above
		return nil, err
	}
	value, index := mapValue(root, "localDocs")
	entry := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		strScalar("name"), strScalar(doc.Name),
		strScalar("title"), strScalar(doc.Title),
		strScalar("description"), strScalar(doc.Description),
	}}
	if value == nil {
		root.Content = append(root.Content, strScalar("localDocs"), &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{entry}})
	} else {
		if value.Kind != yaml.SequenceNode { // coverage-ignore: strict Parse rejects every non-sequence localDocs value before node mutation
			return nil, errors.New("config: localDocs must be a sequence")
		}
		for _, item := range value.Content {
			var existing LocalDoc
			if err := item.Decode(&existing); err != nil { // coverage-ignore: strict Parse decoded every existing item immediately above
				return nil, fmt.Errorf("config: malformed localDocs: %w", err)
			}
			if existing.Name == doc.Name {
				return nil, fmt.Errorf("config: duplicate localDocs name %q", doc.Name)
			}
		}
		value.Style = 0
		root.Content[index] = value
		value.Content = append(value.Content, entry)
	}
	out, err := encode(document)
	if err != nil { // coverage-ignore: decoded yaml.Node trees contain only encoder-supported values
		return nil, err
	}
	parsed, err := Parse("", out)
	if err != nil { // coverage-ignore: encoding a strict parsed config plus scalar entry cannot fail strict parsing
		return nil, err
	}
	if err := parsed.Validate(); err != nil {
		return nil, err
	}
	return out, nil
}

// SetArrayMember adds or removes name in the sequence under key in a config.yaml
// source, via a yaml.Node round-trip that preserves comments and every untouched
// key (ADR-0026). The edited sequence is normalized to block style, so a flow-style
// input (`key: [a, b]`) is accepted. Adding a member already present is a no-op;
// removing a member absent from the key (or a key absent on remove) errors.
func SetArrayMember(src []byte, key, name string, add bool) ([]byte, error) {
	doc, root, err := parseMapping(src)
	if err != nil {
		return nil, err
	}
	val, vi := mapValue(root, key)
	switch {
	case val == nil: // key absent
		if !add {
			return nil, fmt.Errorf("config: no %q entry under %q", name, key)
		}
		root.Content = append(root.Content, strScalar(key), blockSeq(name))
	case val.Kind == yaml.SequenceNode:
		val.Style = 0 // normalize flow -> block
		idx := seqIndex(val, name)
		switch {
		case add:
			if idx < 0 {
				val.Content = append(val.Content, strScalar(name))
			}
		case idx < 0:
			return nil, fmt.Errorf("config: no %q entry under %q", name, key)
		default:
			val.Content = append(val.Content[:idx], val.Content[idx+1:]...)
		}
	default: // bare `key:` (null value)
		if !add {
			return nil, fmt.Errorf("config: no %q entry under %q", name, key)
		}
		root.Content[vi] = blockSeq(name)
	}
	return encode(doc)
}

// parseMapping decodes src into a YAML document and returns the document plus its
// root mapping node - the shared preamble of every awf-owned config.yaml edit.
func parseMapping(src []byte) (*yaml.Node, *yaml.Node, error) {
	doc := &yaml.Node{}
	if err := yaml.Unmarshal(src, doc); err != nil {
		return nil, nil, fmt.Errorf("config: parse: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, nil, errors.New("config: not a YAML mapping")
	}
	return doc, doc.Content[0], nil
}

// encode is the single funnel for awf-owned config.yaml serialization: a yaml.v3
// encoder fixed at two-space indentation. Both MarshalSkeleton (construction) and
// SetArrayMember (mutation) route through it, so the on-disk format has exactly one
// definition.
func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil { // coverage-ignore: encode receives a Skeleton or a yaml.Node decoded from valid YAML; only unrepresentable Go types (chan/func) fail, which neither holds
		return nil, err
	}
	_ = enc.Close()
	return buf.Bytes(), nil
}

func mapValue(m *yaml.Node, key string) (*yaml.Node, int) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1], i + 1
		}
	}
	return nil, -1
}

func seqIndex(seq *yaml.Node, name string) int {
	for i, n := range seq.Content {
		if n.Value == name {
			return i
		}
	}
	return -1
}

func strScalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

func blockSeq(name string) *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{strScalar(name)}}
}
