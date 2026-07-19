// Package bridge implements the migration-only current-state readiness adapter.
package bridge

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

const ApprovalPath = ".awf/current-state-migration.yaml"

var invariantKeyRE = regexp.MustCompile(`^ADR-[0-9]{4}#[a-z0-9]+(?:-[a-z0-9]+)*$`)
var destinationRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*/[a-z0-9]+(?:-[a-z0-9]+)*:[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Approval struct {
	Key         string `json:"key"`
	Destination string `json:"destination"`
}
type Approvals struct {
	Present bool
	Entries []Approval
}

func ParseApprovals(data []byte, present bool) (Approvals, error) {
	if !present {
		return Approvals{}, nil
	}
	var doc yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&doc); err != nil {
		return Approvals{Present: true}, fmt.Errorf("parse migration approvals: %w", err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return Approvals{Present: true}, errors.New("migration approvals must contain one YAML document")
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return Approvals{Present: true}, errors.New("migration approvals must be a mapping")
	}
	root := doc.Content[0]
	seen := map[string]bool{}
	versionOK := false
	var sequence *yaml.Node
	for i := 0; i < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			return Approvals{Present: true}, errors.New("migration approval field names must be strings")
		}
		if seen[key.Value] {
			return Approvals{Present: true}, fmt.Errorf("duplicate migration approval field %q", key.Value)
		}
		seen[key.Value] = true
		switch key.Value {
		case "version":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!int" || value.Value != "1" {
				return Approvals{Present: true}, errors.New("migration approval version must be integer 1")
			}
			versionOK = true
		case "invariantApprovals":
			if value.Kind != yaml.SequenceNode {
				return Approvals{Present: true}, errors.New("invariantApprovals must be a sequence")
			}
			sequence = value
		default:
			return Approvals{Present: true}, fmt.Errorf("unknown migration approval field %q", key.Value)
		}
	}
	if !versionOK || sequence == nil || len(seen) != 2 {
		return Approvals{Present: true}, errors.New("migration approvals require exactly version and invariantApprovals")
	}
	out := Approvals{Present: true}
	keys := map[string]bool{}
	for n, item := range sequence.Content {
		if item.Kind != yaml.MappingNode {
			return out, fmt.Errorf("invariantApprovals[%d] must be a mapping", n)
		}
		fields := map[string]string{}
		for i := 0; i < len(item.Content); i += 2 {
			key, value := item.Content[i], item.Content[i+1]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return out, fmt.Errorf("invariantApprovals[%d] fields and values must be string scalars", n)
			}
			if key.Value != "key" && key.Value != "destination" {
				return out, fmt.Errorf("invariantApprovals[%d] has unknown field %q", n, key.Value)
			}
			if _, duplicate := fields[key.Value]; duplicate {
				return out, fmt.Errorf("invariantApprovals[%d] duplicates field %q", n, key.Value)
			}
			if value.Value == "" {
				return out, fmt.Errorf("invariantApprovals[%d].%s must not be empty", n, key.Value)
			}
			fields[key.Value] = value.Value
		}
		if len(fields) != 2 || fields["key"] == "" || fields["destination"] == "" {
			return out, fmt.Errorf("invariantApprovals[%d] requires exactly key and destination", n)
		}
		if !invariantKeyRE.MatchString(fields["key"]) {
			return out, fmt.Errorf("invariantApprovals[%d] has malformed key %q", n, fields["key"])
		}
		if !destinationRE.MatchString(fields["destination"]) {
			return out, fmt.Errorf("invariantApprovals[%d] has malformed destination %q", n, fields["destination"])
		}
		if keys[fields["key"]] {
			return out, fmt.Errorf("duplicate invariant approval key %q", fields["key"])
		}
		keys[fields["key"]] = true
		out.Entries = append(out.Entries, Approval{Key: fields["key"], Destination: fields["destination"]})
	}
	return out, nil
}

func LoadApprovals(root string) (Approvals, error) {
	data, err := os.ReadFile(root + "/" + ApprovalPath)
	if errors.Is(err, os.ErrNotExist) {
		return ParseApprovals(nil, false)
	}
	if err != nil {
		return Approvals{}, err
	}
	return ParseApprovals(data, true)
}
