package effort

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
	"gopkg.in/yaml.v3"
)

const maxMemoryBytes = 1 << 20
const notYetUpdated = "Not yet updated."

// MemoryMetadata is the closed, mutable header of an owned memory file.
type MemoryMetadata struct {
	Effort  string `json:"effort"`
	Phase   string `json:"phase"`
	Next    string `json:"next"`
	Updated string `json:"updated"`
}

// MemoryUpdate names the mutable fields requested by a structured update.
type MemoryUpdate struct {
	Phase *string
	Next  *string
}

// InvalidMemoryError reports a memory which could not safely be updated.  A
// caller can use NextAction directly without trying to interpret parser prose.
type InvalidMemoryError struct {
	Slug       string
	NextAction string
	Err        error
}

func (e *InvalidMemoryError) Error() string {
	return fmt.Sprintf("invalid memory for effort %q: %v; changed bytes: no; next action: %s", e.Slug, e.Err, e.NextAction)
}
func (e *InvalidMemoryError) Unwrap() error { return e.Err }

type memoryDocument struct {
	metadata MemoryMetadata
	body     []byte
	legacy   bool
	identity string
	boundary bool
	invalid  map[string]bool
	err      error
}

// readMemoryIdentity deliberately checks only the immutable binding used by
// commands which must remain available despite damaged checkpoint metadata.
func readMemoryIdentity(raw []byte, slug string) error {
	if len(raw) > maxMemoryBytes {
		return errors.New("memory exceeds 1 MiB")
	}
	if !utf8.Valid(raw) {
		return errors.New("memory is not valid UTF-8")
	}
	if bytes.HasPrefix(raw, []byte("Effort: "+slug+"\n")) {
		return nil
	}
	block, _, found := frontmatter.Split(raw)
	if !found {
		return errors.New("memory has no matching effort identity")
	}
	effort, err := yamlIdentity(block)
	if err != nil {
		return err
	}
	if effort != slug {
		return errors.New("memory has no matching effort identity")
	}
	return nil
}

func readMemoryMetadata(raw []byte, slug string) (MemoryMetadata, []byte, error) {
	doc := inspectMemory(raw, slug)
	if doc.err != nil {
		return MemoryMetadata{}, nil, doc.err
	}
	return doc.metadata, doc.body, nil
}

func inspectMemory(raw []byte, slug string) memoryDocument {
	if len(raw) > maxMemoryBytes {
		return memoryDocument{err: errors.New("memory exceeds 1 MiB")}
	}
	if !utf8.Valid(raw) {
		return memoryDocument{err: errors.New("memory is not valid UTF-8")}
	}
	block, body, found := frontmatter.Split(raw)
	if found {
		return inspectCanonical(block, body, slug)
	}
	return inspectLegacy(raw, slug)
}

func inspectCanonical(block, body []byte, slug string) memoryDocument {
	mapping, err := yamlMapping(block)
	if err != nil {
		return memoryDocument{err: err}
	}
	doc := memoryDocument{body: body, boundary: true, invalid: map[string]bool{}}
	known := map[string]bool{"effort": true, "phase": true, "next": true, "updated": true}
	seen := map[string]bool{}
	for i := 0; i < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i], mapping.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Style&yaml.TaggedStyle != 0 || value.Kind == yaml.AliasNode || value.Style&yaml.TaggedStyle != 0 {
			return memoryDocument{err: errors.New("memory YAML contains aliases, tags, or non-string keys")}
		}
		if seen[key.Value] {
			return memoryDocument{err: errors.New("duplicate memory YAML key")}
		}
		seen[key.Value] = true
		if !known[key.Value] {
			return memoryDocument{err: errors.New("canonical memory metadata contains an unknown key")}
		}
		if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
			if key.Value == "effort" {
				return memoryDocument{err: errors.New("memory effort identity must be a string")}
			}
			doc.invalid[key.Value] = true
			continue
		}
		switch key.Value {
		case "effort":
			doc.identity = value.Value
		case "phase":
			doc.metadata.Phase = value.Value
		case "next":
			doc.metadata.Next = value.Value
		case "updated":
			doc.metadata.Updated = value.Value
		}
	}
	doc.metadata.Effort = doc.identity
	if doc.identity != slug {
		doc.err = errors.New("memory effort identity does not match directory")
		return doc
	}
	for _, key := range []string{"effort", "phase", "next", "updated"} {
		if !seen[key] {
			doc.invalid[key] = true
		}
	}
	if _, missing := doc.invalid["phase"]; !missing && validateMemoryMutable(doc.metadata.Phase) != nil {
		doc.invalid["phase"] = true
	}
	if _, missing := doc.invalid["next"]; !missing && validateMemoryMutable(doc.metadata.Next) != nil {
		doc.invalid["next"] = true
	}
	if _, missing := doc.invalid["updated"]; !missing && validateUpdated(doc.metadata.Updated, false) != nil {
		doc.invalid["updated"] = true
	}
	if len(doc.invalid) != 0 {
		doc.err = errors.New("invalid canonical memory metadata")
	}
	return doc
}

func inspectLegacy(raw []byte, slug string) memoryDocument {
	prefixEnd := bytes.Index(raw, []byte("\n\n"))
	if prefixEnd < 0 {
		return memoryDocument{err: errors.New("legacy memory has no exact four-line boundary")}
	}
	lines := strings.Split(string(raw[:prefixEnd]), "\n")
	if len(lines) != 4 {
		return memoryDocument{err: errors.New("legacy memory must have exactly four metadata lines")}
	}
	keys := []string{"Effort", "Phase", "Next", "Updated"}
	values := make([]string, 4)
	for i, key := range keys {
		prefix := key + ": "
		if !strings.HasPrefix(lines[i], prefix) {
			return memoryDocument{err: errors.New("legacy memory metadata is malformed")}
		}
		values[i] = strings.TrimPrefix(lines[i], prefix)
	}
	doc := memoryDocument{metadata: MemoryMetadata{Effort: values[0], Phase: values[1], Next: values[2], Updated: values[3]}, body: raw[prefixEnd+2:], legacy: true, identity: values[0], boundary: true, invalid: map[string]bool{}}
	if doc.identity != slug {
		doc.err = errors.New("memory effort identity does not match directory")
		return doc
	}
	if validateMemoryMutable(doc.metadata.Phase) != nil {
		doc.invalid["phase"] = true
	}
	if validateMemoryMutable(doc.metadata.Next) != nil {
		doc.invalid["next"] = true
	}
	if validateUpdated(doc.metadata.Updated, true) != nil {
		doc.invalid["updated"] = true
	}
	if len(doc.invalid) != 0 {
		doc.err = errors.New("invalid legacy memory metadata")
	}
	return doc
}

// yamlIdentity intentionally reads no mutable values. It is the compatibility
// boundary for list/show/finish, not a weaker form of metadata validation.
func yamlIdentity(block []byte) (string, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(block))
	var doc yaml.Node
	if err := decoder.Decode(&doc); err != nil {
		return "", fmt.Errorf("parse memory YAML: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return "", errors.New("memory YAML must be one mapping")
	}
	mapping := doc.Content[0]
	var effort string
	found := false
	for i := 0; i < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i], mapping.Content[i+1]
		if key.Kind == yaml.ScalarNode && key.Tag == "!!str" && key.Value == "effort" {
			if found {
				return "", errors.New("duplicate memory YAML effort key")
			}
			if key.Style&yaml.TaggedStyle != 0 || value.Kind != yaml.ScalarNode || value.Tag != "!!str" || value.Style&yaml.TaggedStyle != 0 {
				return "", errors.New("memory effort identity must be an untagged string")
			}
			effort, found = value.Value, true
		}
	}
	if !found {
		return "", errors.New("memory has no effort identity")
	}
	return effort, nil
}

func yamlMapping(block []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(block))
	var doc yaml.Node
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse memory YAML: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("multiple YAML documents")
		}
		return nil, fmt.Errorf("parse memory YAML: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("memory YAML must be one mapping")
	}
	return doc.Content[0], nil
}

func validateMemoryMutable(value string) error {
	if !utf8.ValidString(value) || len(value) > 500 || strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n") {
		return errors.New("memory mutable value must be a nonblank single line of at most 500 UTF-8 bytes")
	}
	return nil
}

func validateUpdated(value string, legacy bool) error {
	if legacy && value == notYetUpdated {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.Location() != time.UTC || !strings.HasSuffix(value, "Z") || parsed.Format(time.RFC3339Nano) != value {
		return errors.New("memory updated must be RFC3339Nano UTC")
	}
	return nil
}

func formatMemoryTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func encodeMemory(metadata MemoryMetadata, body []byte) ([]byte, error) {
	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, pair := range [][2]string{{"effort", metadata.Effort}, {"phase", metadata.Phase}, {"next", metadata.Next}, {"updated", metadata.Updated}} {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: pair[0]},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: pair[1]},
		)
	}
	encoded, err := yaml.Marshal(mapping)
	if err != nil { // coverage-ignore: yaml.Marshal of the fixed scalar-only mapping node cannot fail
		return nil, fmt.Errorf("encode memory YAML: %w", err)
	}
	return append(append([]byte("---\n"), encoded...), append([]byte("---\n"), body...)...), nil
}
