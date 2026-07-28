package telemetry

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

//go:embed protocol.json
var descriptorJSON []byte

var lowerUUIDv4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type descriptor struct {
	SchemaVersion int `json:"schemaVersion"`
	Limits        struct {
		IdentifierBytes, CategoryBytes int
		IntegerMaximum                 uint64
	} `json:"limits"`
}

var protocol = parseDescriptor()

func parseDescriptor() descriptor {
	var d descriptor
	if err := json.Unmarshal(descriptorJSON, &d); err != nil || d.SchemaVersion != SchemaVersion || d.Limits.IdentifierBytes == 0 || d.Limits.CategoryBytes == 0 || d.Limits.IntegerMaximum != maxSafeInteger {
		panic("invalid session telemetry descriptor")
	}
	return d
}
func DescriptorBytes() []byte { return append([]byte(nil), descriptorJSON...) }
func Identifier(value string) bool {
	return validText(value, protocol.Limits.IdentifierBytes) && value != "." && value != ".." && !strings.ContainsAny(value, "/\\\x00\r\n")
}
func category(value string) bool { return validText(value, protocol.Limits.CategoryBytes) }
func validText(value string, limit int) bool {
	return value != "" && utf8.ValidString(value) && len([]byte(value)) <= limit
}

// ValidateHeader validates one closed stream header. The filename identity check is made by the reader and writer.
func ValidateHeader(raw json.RawMessage) (Header, error) {
	_, err := closedObject(raw, []string{"record", "schemaVersion", "sessionId", "createdAt"})
	if err != nil {
		return Header{}, err
	}
	var h Header
	if err = json.Unmarshal(raw, &h); err != nil {
		return h, err
	}
	if h.Record != "header" || h.SchemaVersion != SchemaVersion || !Identifier(h.SessionID) || h.CreatedAt.IsZero() {
		return h, errors.New("invalid session header")
	}
	return h, nil
}
func ValidateObservation(raw json.RawMessage) (Observation, error) {
	root, err := closedObject(raw, []string{"record", "schemaVersion", "observationId", "timestamp", "kind", "payload"})
	if err != nil {
		return Observation{}, err
	}
	var o Observation
	if err = json.Unmarshal(raw, &o); err != nil {
		return o, err
	}
	if o.Record != "observation" || o.SchemaVersion != SchemaVersion || !lowerUUIDv4.MatchString(o.ObservationID) || o.Timestamp.IsZero() {
		return o, errors.New("invalid observation envelope")
	}
	if err := validatePayload(o.Kind, root["payload"]); err != nil {
		return o, err
	}
	o.Raw = append(o.Raw[:0], raw...)
	return o, nil
}
func closedObject(raw json.RawMessage, names []string) (map[string]json.RawMessage, error) {
	if !utf8.Valid(raw) {
		return nil, errors.New("record is not valid UTF-8")
	}
	if err := noDuplicateJSON(raw); err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&m); err != nil || m == nil {
		return nil, errors.New("record must be a JSON object")
	}
	if len(m) != len(names) {
		return nil, errors.New("record has unknown or missing field")
	}
	for _, n := range names {
		if _, ok := m[n]; !ok {
			return nil, fmt.Errorf("record missing %s", n)
		}
	}
	return m, nil
}
func noDuplicateJSON(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var walk func() error
	walk = func() error {
		t, e := d.Token()
		if e != nil {
			return e
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					k, e := d.Token()
					if e != nil {
						return e
					}
					s, ok := k.(string)
					if !ok || seen[s] {
						return errors.New("duplicate or invalid JSON field")
					}
					seen[s] = true
					if e = walk(); e != nil {
						return e
					}
				}
				_, e = d.Token()
				return e
			case '[':
				for d.More() {
					if e := walk(); e != nil {
						return e
					}
				}
				_, e = d.Token()
				return e
			}
		}
		return nil
	}
	if err := walk(); err != nil {
		return err
	}
	var trailing any
	if err := d.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}
func validatePayload(kind string, raw json.RawMessage) error {
	var names []string
	switch kind {
	case "usage":
		names = []string{"inputTokens", "outputTokens", "cacheReadTokens", "cacheWriteTokens", "costUsd"}
	case "tool":
		names = []string{"tool", "outcome", "durationMs"}
	case "gate":
		names = []string{"gate", "outcome", "durationMs"}
	case "subagent":
		names = []string{"role", "outcome", "queueWaitMs", "durationMs"}
	case "compaction":
		names = []string{"inputTokensBefore", "inputTokensAfter"}
	case "handoff":
		names = []string{"outcome", "durationMs"}
	default:
		return errors.New("unknown observation kind")
	}
	m, err := closedObject(raw, names)
	if err != nil {
		return err
	}
	integer := func(name string) error {
		var n json.Number
		if err := json.Unmarshal(m[name], &n); err != nil {
			return fmt.Errorf("%s must be an integer", name)
		}
		i, e := n.Int64()
		if e != nil || i < 0 || uint64(i) > maxSafeInteger {
			return fmt.Errorf("%s must be a non-negative safe integer", name)
		}
		return nil
	}
	outcome := func(name string) error {
		var s string
		if json.Unmarshal(m[name], &s) != nil || (s != "success" && s != "failure" && s != "cancelled") {
			return fmt.Errorf("%s has invalid outcome", name)
		}
		return nil
	}
	switch kind {
	case "usage":
		for _, n := range []string{"inputTokens", "outputTokens", "cacheReadTokens", "cacheWriteTokens"} {
			if err := integer(n); err != nil {
				return err
			}
		}
		var cost float64
		if json.Unmarshal(m["costUsd"], &cost) != nil || math.IsNaN(cost) || math.IsInf(cost, 0) || cost < 0 {
			return errors.New("costUsd must be finite and non-negative")
		}
	case "tool", "gate":
		categoryName := "tool"
		if kind == "gate" {
			categoryName = "gate"
		}
		var s string
		if json.Unmarshal(m[categoryName], &s) != nil || !category(s) {
			return fmt.Errorf("%s has invalid category", categoryName)
		}
		if err := outcome("outcome"); err != nil {
			return err
		}
		return integer("durationMs")
	case "subagent":
		var role string
		if json.Unmarshal(m["role"], &role) != nil || (role != "grounding" && role != "exploration" && role != "adr-review" && role != "plan-review" && role != "implementation" && role != "code-review") {
			return errors.New("role has invalid value")
		}
		if err := outcome("outcome"); err != nil {
			return err
		}
		for _, n := range []string{"queueWaitMs", "durationMs"} {
			if err := integer(n); err != nil {
				return err
			}
		}
	case "compaction":
		for _, n := range names {
			if err := integer(n); err != nil {
				return err
			}
		}
	case "handoff":
		if err := outcome("outcome"); err != nil {
			return err
		}
		return integer("durationMs")
	}
	return nil
}
