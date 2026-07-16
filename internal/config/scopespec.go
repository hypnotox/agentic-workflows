package config

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ScopeSpec is one allowed commit scope: a name and an optional human meaning.
// In config a scope is written either as a bare string (name only) or a
// {name, meaning} mapping (ADR-0056).
type ScopeSpec struct {
	Name    string `yaml:"name"`
	Meaning string `yaml:"meaning"`
}

// UnmarshalYAML accepts either a scalar node (the bare-string form → empty
// meaning) or a strict mapping node. invariant: scope-config-dual-form
func (s *ScopeSpec) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		s.Name, s.Meaning = n.Value, ""
		return nil
	}
	if n.Kind != yaml.MappingNode {
		return errors.New("scope entry must be a string or a {name, meaning} mapping")
	}
	// Strictness is NOT inherited: yaml.v3 Node.Decode spins up a fresh
	// non-strict decoder, so the parent's KnownFields(true) does not apply here.
	// Enforce the closed key set explicitly (a mapping node's Content is a flat
	// key,value,key,value... list).
	for i := 0; i+1 < len(n.Content); i += 2 {
		if k := n.Content[i].Value; k != "name" && k != "meaning" {
			return fmt.Errorf("scope mapping has unknown key %q (allowed: name, meaning)", k)
		}
	}
	type raw ScopeSpec // avoid recursion
	var r raw
	if err := n.Decode(&r); err != nil {
		return err
	}
	if r.Name == "" {
		return errors.New("scope mapping requires a non-empty name")
	}
	*s = ScopeSpec(r)
	return nil
}
