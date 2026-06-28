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
	Prefix string            `yaml:"prefix"`
	Vars   map[string]string `yaml:"vars"`
	Skills []string          `yaml:"skills"`
	Agents []string          `yaml:"agents"`
	Hooks  []string          `yaml:"hooks"`
	Docs   []string          `yaml:"docs"`
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
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, fmt.Errorf("config: parse: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("config: not a YAML mapping")
	}
	root := doc.Content[0]
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
	return encode(&doc)
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
