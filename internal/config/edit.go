package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

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

// SidecarEdit describes one leaf-only YAML sidecar mutation.
type SidecarEdit struct {
	Field string
	Mode  string
	Value any
}

// EditSidecar applies a leaf mutation without rebuilding unrelated YAML nodes.
// It returns source bytes, whether the sidecar remains present, and whether bytes changed.
func EditSidecar(src []byte, edit SidecarEdit) ([]byte, bool, bool, error) {
	parts := strings.Split(edit.Field, ".")
	if edit.Field == "" {
		return nil, false, false, fmt.Errorf("config: empty sidecar field")
	}
	var doc *yaml.Node
	var root *yaml.Node
	var err error
	if len(src) == 0 {
		doc = &yaml.Node{Kind: yaml.DocumentNode}
		root = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		doc.Content = []*yaml.Node{root}
	} else if doc, root, err = parseMapping(src); err != nil {
		return nil, false, false, err
	}
	parents := []*yaml.Node{root}
	current := root
	for _, key := range parts[:len(parts)-1] {
		v, _ := mapValue(current, key)
		if v == nil {
			v = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			current.Content = append(current.Content, strScalar(key), v)
		}
		if v.Kind != yaml.MappingNode {
			return nil, false, false, fmt.Errorf("config: sidecar field %q has intermediate mapping conflict", edit.Field)
		}
		current = v
		parents = append(parents, current)
	}
	key := parts[len(parts)-1]
	old, idx := mapValue(current, key)
	switch edit.Mode {
	case "reset":
		if old == nil {
			return src, len(src) != 0, false, nil
		}
		current.Content = append(current.Content[:idx-1], current.Content[idx+1:]...)
		for i := len(parents) - 1; i > 0; i-- {
			p := parents[i]
			if len(p.Content) != 0 {
				break
			}
			parent := parents[i-1]
			for j := 1; j < len(parent.Content); j += 2 {
				if parent.Content[j] == p {
					parent.Content = append(parent.Content[:j-1], parent.Content[j+1:]...)
					break
				}
			}
		}
		if len(root.Content) == 0 {
			return nil, false, true, nil
		}
	case "value":
		n, err := valueNode(edit.Value)
		if err != nil {
			return nil, false, false, err
		}
		if old != nil && reflect.DeepEqual(nodeValue(old), nodeValue(n)) {
			return src, true, false, nil
		}
		if old == nil {
			current.Content = append(current.Content, strScalar(key), n)
		} else {
			current.Content[idx] = n
		}
	case "add", "remove":
		n, err := valueNode(edit.Value)
		if err != nil {
			return nil, false, false, err
		}
		var seq *yaml.Node
		if old == nil {
			if edit.Mode == "remove" {
				return src, len(src) != 0, false, nil
			}
			seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
			current.Content = append(current.Content, strScalar(key), seq)
		} else {
			if old.Kind != yaml.SequenceNode {
				return nil, false, false, fmt.Errorf("config: sidecar field %q is not a list", edit.Field)
			}
			seq = old
		}
		found := -1
		for i, v := range seq.Content {
			if reflect.DeepEqual(nodeValue(v), nodeValue(n)) {
				found = i
				break
			}
		}
		if edit.Mode == "add" {
			if found >= 0 {
				return src, true, false, nil
			}
			seq.Style = 0
			seq.Content = append(seq.Content, n)
		} else {
			if found < 0 {
				return src, true, false, nil
			}
			seq.Style = 0
			seq.Content = append(seq.Content[:found], seq.Content[found+1:]...)
		}
	default:
		return nil, false, false, fmt.Errorf("config: unknown sidecar edit mode %q", edit.Mode)
	}
	out, err := encode(doc)
	if err != nil {
		return nil, false, false, err
	}
	return out, true, true, nil
}
func valueNode(v any) (*yaml.Node, error) {
	b, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	var d yaml.Node
	if err := yaml.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	if len(d.Content) == 0 {
		return nil, fmt.Errorf("config: empty value")
	}
	return d.Content[0], nil
}
func nodeValue(n *yaml.Node) any { var v any; _ = n.Decode(&v); return v }

// DecodeJSONValue decodes exactly one JSON value. JSON's decoder rejects trailing documents.
func DecodeJSONValue(s string) (any, error) {
	var v any
	dec := json.NewDecoder(strings.NewReader(s))
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("trailing JSON value")
		}
		return nil, err
	}
	return v, nil
}
