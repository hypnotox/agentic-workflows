package config

import (
	"bytes"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Skeleton is the input to MarshalSkeleton: the fields a freshly-scaffolded
// .awf/config.yaml carries. Vars is typed map[string]string (not map[string]any)
// so a nil/null var value is unrepresentable — the scaffold seeds each var with an
// empty string, which marshals as `x: ""`. A nil interface would marshal as
// `x: null` and decode back to a nil value that renders as "<no value>", tripping
// the publication-safe check (ADR-0026 Decision 3).
type Skeleton struct {
	Prefix     string            `yaml:"prefix"`
	Vars       map[string]string `yaml:"vars"`
	Skills     []string          `yaml:"skills"`
	Agents     []string          `yaml:"agents"`
	Docs       []string          `yaml:"docs"`
	Audit      *SkeletonAudit    `yaml:"audit,omitempty"`
	Invariants *InvariantConfig  `yaml:"invariants,omitempty"`
	Bootstrap  *BootstrapConfig  `yaml:"bootstrap,omitempty"`
	Hooks      *HooksConfig      `yaml:"hooks,omitempty"`
}

// SkeletonAudit is the audit block a scaffold can seed (ADR-0051): only
// allowedScopes — the one audit field init collects. Deliberately not
// *AuditConfig, whose zero-value fields would serialize as explicit settings.
type SkeletonAudit struct {
	AllowedScopes []string `yaml:"allowedScopes"`
}

// CatalogTrim optionally overrides which catalog skills/docs a scaffolded config
// enables (ADR-0029 catalog trim). A nil *CatalogTrim — or a nil dimension within
// it — means "no selection: keep the curated-core default"; a non-nil dimension is
// the verbatim, fully-deselectable enable set (an empty slice deselects all).
type CatalogTrim struct {
	Skills *[]string
	Docs   *[]string
}

// MarshalSkeleton renders a fresh config.yaml from s in the canonical awf format
// (two-space block style). It is the construction half of internal/config's
// ownership of config.yaml serialization (ADR-0026).
func MarshalSkeleton(s Skeleton) ([]byte, error) {
	return encode(s)
}

// SetArrayMember adds or removes name in the sequence under key in a config.yaml
// source, via a yaml.Node round-trip that preserves comments and every untouched
// key (ADR-0026). The edited sequence is normalized to block style, so a flow-style
// input (`key: [a, b]`) is accepted. Adding a member already present is a no-op;
// removing a member absent from the key (or a key absent on remove) errors.
// invariant: config-mutation-roundtrip
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
			// invariant: remove-block-scoped
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

// SetArray sets the sequence under key to exactly values, creating the key if it
// is absent and replacing it otherwise, via a yaml.Node round-trip that preserves
// comments and every untouched key (ADR-0026). Used where the whole list is
// computed rather than edited member-by-member — the targets array carries a Load
// default, so an absent on-disk key must be materialized as the full resolved list,
// not appended to (ADR-0037).
func SetArray(src []byte, key string, values []string) ([]byte, error) {
	doc, root, err := parseMapping(src)
	if err != nil {
		return nil, err
	}
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, v := range values {
		seq.Content = append(seq.Content, strScalar(v))
	}
	if _, vi := mapValue(root, key); vi >= 0 {
		root.Content[vi] = seq
	} else {
		root.Content = append(root.Content, strScalar(key), seq)
	}
	return encode(doc)
}

// RemoveKey deletes the top-level mapping entry under key from a config.yaml
// source via a yaml.Node round-trip that preserves comments and every untouched
// key (ADR-0026). Removing an absent key is a no-op (returns src unchanged), so a
// schema migration can re-run safely.
func RemoveKey(src []byte, key string) ([]byte, error) {
	doc, root, err := parseMapping(src)
	if err != nil {
		return nil, err
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			root.Content = append(root.Content[:i], root.Content[i+2:]...)
			return encode(doc)
		}
	}
	return src, nil
}

// SetMappingScalar sets child to a bool value under a top-level mapping at key in
// a config.yaml source, creating the key's mapping (and the child) if absent, via a
// yaml.Node round-trip that preserves comments and every untouched key (ADR-0026).
// It is the nested-scalar analog of SetArray (which writes a sequence): the
// bootstrap enable entry is `bootstrap.enabled: <bool>`, not an enable array, so it
// needs a mapping-scalar writer rather than SetArrayMember. An existing scalar under
// key/child is overwritten.
func SetMappingScalar(src []byte, key, child string, value bool) ([]byte, error) {
	doc, root, err := parseMapping(src)
	if err != nil {
		return nil, err
	}
	boolStr := "false"
	if value {
		boolStr = "true"
	}
	val, _ := mapValue(root, key)
	if val == nil || val.Kind != yaml.MappingNode {
		m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
			strScalar(child), boolScalar(boolStr),
		}}
		if val == nil {
			root.Content = append(root.Content, strScalar(key), m)
		} else {
			_, vi := mapValue(root, key)
			root.Content[vi] = m
		}
		return encode(doc)
	}
	if cv, _ := mapValue(val, child); cv != nil {
		cv.Tag, cv.Value, cv.Style = "!!bool", boolStr, 0
	} else {
		val.Content = append(val.Content, strScalar(child), boolScalar(boolStr))
	}
	return encode(doc)
}

func boolScalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: v}
}

// parseMapping decodes src into a YAML document and returns the document plus its
// root mapping node — the shared preamble of every awf-owned config.yaml edit.
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
// invariant: config-serialization-owned
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
