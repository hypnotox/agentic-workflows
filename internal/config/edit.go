package config

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skeleton is the input to MarshalSkeleton: the fields a freshly-scaffolded
// .awf/config.yaml carries. Vars is typed map[string]string (not map[string]any)
// so a nil/null var value is unrepresentable - the scaffold seeds each var with an
// empty string, which marshals as `x: ""`. A nil interface would marshal as
// `x: null` and decode back to a nil value that renders as "<no value>", tripping
// the publication-safe check (ADR-0026 Decision 3).
type Skeleton struct {
	Prefix    string            `yaml:"prefix"`
	Vars      map[string]string `yaml:"vars"`
	Skills    []string          `yaml:"skills"`
	Agents    []string          `yaml:"agents"`
	Docs      []string          `yaml:"docs"`
	Audit     *SkeletonAudit    `yaml:"audit,omitempty"`
	Bootstrap *BootstrapConfig  `yaml:"bootstrap,omitempty"`
	Hooks     *HooksConfig      `yaml:"hooks,omitempty"`
}

// SkeletonAudit is the audit block a scaffold can seed (ADR-0051): only
// allowedScopes - the one audit field init collects. Deliberately not
// *AuditConfig, whose zero-value fields would serialize as explicit settings.
type SkeletonAudit struct {
	AllowedScopes []string `yaml:"allowedScopes"`
}

// CatalogTrim optionally overrides which catalog skills/docs a scaffolded config
// enables (ADR-0029 catalog trim). A nil *CatalogTrim - or a nil dimension within
// it - means "no selection: keep the curated-core default"; a non-nil dimension is
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
// touches-state: config/configuration:config-mutation-roundtrip - yaml.Node add/remove round-trip; proof in edit_test.go
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
			// touches-state: config/configuration:remove-block-scoped - block-scoped sequence removal; proof in edit_test.go
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
// computed rather than edited member-by-member - the targets array carries a Load
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

// RemoveMappingKey removes child from the mapping at top-level key, preserving
// comments and every untouched key via the same yaml.Node round-trip as the
// other editors (ADR-0026). When the removal empties the parent mapping, the
// parent key goes too, so a retired setting leaves no vestigial block behind.
// An absent parent, a non-mapping parent, or an absent child is a no-op
// (returns src unchanged), so a schema migration can re-run safely.
func RemoveMappingKey(src []byte, key, child string) ([]byte, error) {
	doc, root, err := parseMapping(src)
	if err != nil {
		return nil, err
	}
	val, _ := mapValue(root, key)
	if val == nil || val.Kind != yaml.MappingNode {
		return src, nil
	}
	for i := 0; i+1 < len(val.Content); i += 2 {
		if val.Content[i].Value != child {
			continue
		}
		val.Content = append(val.Content[:i], val.Content[i+2:]...)
		if len(val.Content) == 0 {
			for j := 0; j+1 < len(root.Content); j += 2 {
				if root.Content[j].Value == key {
					root.Content = append(root.Content[:j], root.Content[j+2:]...)
					break
				}
			}
		}
		return encode(doc)
	}
	return src, nil
}

func boolScalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: v}
}

// SeedVarKey adds `name: ""` under the top-level vars: mapping when the key is
// absent, creating the mapping if needed, via the same comment-preserving
// yaml.Node round-trip as SetArrayMember (ADR-0026). A present key - set,
// empty, or null - is left untouched and src is returned unchanged: presence
// is the open-to-do signal and absence the deliberate decline (ADR-0087), so
// seeding must never overwrite either state.
func SeedVarKey(src []byte, name string) ([]byte, error) {
	doc, root, err := parseMapping(src)
	if err != nil {
		return nil, err
	}
	val, _ := mapValue(root, "vars")
	if val != nil && val.Kind == yaml.MappingNode {
		if existing, _ := mapValue(val, name); existing != nil {
			return src, nil
		}
		val.Content = append(val.Content, strScalar(name), emptyStrScalar())
		return encode(doc)
	}
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		strScalar(name), emptyStrScalar(),
	}}
	if val == nil {
		root.Content = append(root.Content, strScalar("vars"), m)
	} else {
		_, vi := mapValue(root, "vars")
		root.Content[vi] = m
	}
	return encode(doc)
}

// emptyStrScalar marshals as `""` - the seeded open-to-do value; a bare scalar
// would decode as null and violate ADR-0026 Decision 3.
func emptyStrScalar() *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "", Style: yaml.DoubleQuotedStyle}
}

// A GlobRewrite records one no-slash glob scalar AnchorNoSlashGlobs anchored:
// Key is the config location, From the original pattern (rewritten to
// `**/<From>`).
type GlobRewrite struct {
	Key  string
	From string
}

// AnchorNoSlashGlobs rewrites every no-slash glob scalar under
// invariants.sources[].globs and audit.dependencyManifests to `**/<pattern>`,
// preserving comments and untouched keys (ADR-0026) and reporting the rewrites
// performed. Slashed patterns are left alone, so the rewrite is idempotent;
// absent keys are a no-op. It is the nested-sequence editor the schema-7
// anchored-globs migration (ADR-0077) consumes - the sequence analog of
// SetMappingScalar.
func AnchorNoSlashGlobs(src []byte) ([]byte, []GlobRewrite, error) {
	doc, root, err := parseMapping(src)
	if err != nil {
		return nil, nil, err
	}
	var rewrites []GlobRewrite
	if inv, _ := mapValue(root, "invariants"); inv != nil && inv.Kind == yaml.MappingNode {
		if srcs, _ := mapValue(inv, "sources"); srcs != nil && srcs.Kind == yaml.SequenceNode {
			for _, s := range srcs.Content {
				if s.Kind != yaml.MappingNode {
					continue
				}
				if globs, _ := mapValue(s, "globs"); globs != nil && globs.Kind == yaml.SequenceNode {
					rewrites = append(rewrites, anchorSeq(globs, "invariants.sources.globs")...)
				}
			}
		}
	}
	if aud, _ := mapValue(root, "audit"); aud != nil && aud.Kind == yaml.MappingNode {
		if dm, _ := mapValue(aud, "dependencyManifests"); dm != nil && dm.Kind == yaml.SequenceNode {
			rewrites = append(rewrites, anchorSeq(dm, "audit.dependencyManifests")...)
		}
	}
	out, err := encode(doc)
	return out, rewrites, err
}

// anchorSeq rewrites each non-empty no-slash scalar member of seq to `**/<value>`
// and reports the rewrites under key.
// touches-state: config/configuration:glob-migration-anchored - no-slash glob anchoring rewrite; proof in edit_test.go
func anchorSeq(seq *yaml.Node, key string) []GlobRewrite {
	var rewrites []GlobRewrite
	for _, n := range seq.Content {
		if n.Kind == yaml.ScalarNode && n.Value != "" && !strings.Contains(n.Value, "/") {
			rewrites = append(rewrites, GlobRewrite{Key: key, From: n.Value})
			n.Value = "**/" + n.Value
		}
	}
	return rewrites
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
// touches-state: config/configuration:config-serialization-owned - single config.yaml serialization funnel; proof in edit_test.go
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
