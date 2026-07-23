package telemetry

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

//go:embed protocol.json
var descriptorJSON []byte

type protocolDescriptor struct {
	Version           ProtocolVersion                `json:"version"`
	Limits            descriptorLimits               `json:"limits"`
	Vocabularies      map[string][]string            `json:"vocabularies"`
	Privacy           privacyDescriptor              `json:"privacy"`
	WaiverRules       map[string][]string            `json:"waiverRules"`
	Envelope          objectDescriptor               `json:"envelope"`
	Payloads          map[string]payloadDescriptor   `json:"payloads"`
	LifecycleRequests map[string]lifecycleDescriptor `json:"lifecycleRequests"`
	Objects           map[string]objectDescriptor    `json:"objects"`
}

type descriptorLimits struct {
	IdentifierBytes     int `json:"identifierBytes"`
	EventIDBytes        int `json:"eventIdBytes"`
	IdempotencyKeyBytes int `json:"idempotencyKeyBytes"`
	ObservationIDBytes  int `json:"observationIdBytes"`
	ModelBytes          int `json:"modelBytes"`
	ToolBytes           int `json:"toolBytes"`
	CategoryBytes       int `json:"categoryBytes"`
}

type privacyDescriptor struct {
	Policy                     string   `json:"policy"`
	AllowedRepositoryPathField string   `json:"allowedRepositoryPathField"`
	ForbiddenFields            []string `json:"forbiddenFields"`
}

type objectDescriptor struct {
	GoType               string                     `json:"goType"`
	AdditionalProperties bool                       `json:"additionalProperties"`
	Fields               map[string]fieldDescriptor `json:"fields"`
	IdentityRule         string                     `json:"identityRule"`
}

type payloadDescriptor struct {
	GoType               string                     `json:"goType"`
	Class                string                     `json:"class"`
	Repairable           bool                       `json:"repairable"`
	PrivacyPolicy        string                     `json:"privacyPolicy"`
	AdditionalProperties bool                       `json:"additionalProperties"`
	Fields               map[string]fieldDescriptor `json:"fields"`
	Constraints          []constraintDescriptor     `json:"constraints"`
}

type lifecycleDescriptor struct {
	GoType               string                     `json:"goType"`
	EventKind            string                     `json:"eventKind"`
	AdditionalProperties bool                       `json:"additionalProperties"`
	Fields               map[string]fieldDescriptor `json:"fields"`
	Constraints          []constraintDescriptor     `json:"constraints"`
}

type constraintDescriptor struct {
	Kind          string   `json:"kind"`
	Discriminator string   `json:"discriminator"`
	Value         string   `json:"value"`
	Fields        []string `json:"fields"`
	Field         string   `json:"field"`
	RuleField     string   `json:"ruleField"`
	ReasonField   string   `json:"reasonField"`
}

type fieldDescriptor struct {
	Type                 string                     `json:"type"`
	Required             bool                       `json:"required"`
	Format               string                     `json:"format"`
	Vocabulary           string                     `json:"vocabulary"`
	Minimum              *float64                   `json:"minimum"`
	MinItems             int                        `json:"minItems"`
	UniqueItems          bool                       `json:"uniqueItems"`
	Items                *fieldDescriptor           `json:"items"`
	Fields               map[string]fieldDescriptor `json:"fields"`
	AdditionalProperties bool                       `json:"additionalProperties"`
}

var descriptor = mustParseDescriptor(descriptorJSON)

func mustParseDescriptor(raw []byte) protocolDescriptor {
	if !utf8.Valid(raw) {
		panic("invalid embedded telemetry protocol: not valid UTF-8")
	}
	if err := validateJSONStructure(raw, "descriptor", nil); err != nil {
		panic(fmt.Sprintf("invalid embedded telemetry protocol: %v", err))
	}
	var parsed protocolDescriptor
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		panic(fmt.Sprintf("invalid embedded telemetry protocol: %v", err))
	}
	if err := ensureJSONEOF(decoder); err != nil { // coverage-ignore: validated single-value JSON cannot retain trailing decoder input
		panic(fmt.Sprintf("invalid embedded telemetry protocol: %v", err))
	}
	if err := validateDescriptor(parsed); err != nil {
		panic(fmt.Sprintf("invalid embedded telemetry protocol: %v", err))
	}
	return parsed
}

func validateDescriptor(parsed protocolDescriptor) error {
	if parsed.Version.Major == 0 {
		return errors.New("version.major must be non-zero")
	}
	limits := reflect.ValueOf(parsed.Limits)
	for i := range limits.NumField() {
		if limits.Field(i).Int() <= 0 {
			return fmt.Errorf("limits.%s must be positive", limits.Type().Field(i).Name)
		}
	}
	if parsed.Privacy.Policy == "" || parsed.Privacy.AllowedRepositoryPathField != "" {
		return errors.New("privacy policy or allowed repository path field is invalid")
	}
	forbiddenFields := stringSet(parsed.Privacy.ForbiddenFields)
	if len(forbiddenFields) != len(parsed.Privacy.ForbiddenFields) || forbiddenFields[""] || forbiddenFields[parsed.Privacy.AllowedRepositoryPathField] {
		return errors.New("privacy forbidden fields are empty, duplicated, or conflict with the allowed path field")
	}
	if err := validateVocabularyAuthority(parsed); err != nil {
		return err
	}
	if parsed.Envelope.AdditionalProperties || parsed.Envelope.GoType != "EventEnvelope" || parsed.Envelope.IdentityRule != "lifecycle:idempotencyKey,passive:observationId" {
		return errors.New("envelope authority is not the closed EventEnvelope contract")
	}
	if err := validateDescriptorFields(parsed, "envelope", parsed.Envelope.Fields); err != nil {
		return err
	}
	if payload := parsed.Envelope.Fields["payload"]; !payload.Required || payload.Type != "payload" {
		return errors.New("envelope.payload must be the required payload discriminator")
	}

	eventKinds := stringSet(parsed.Vocabularies["eventKinds"])
	if !reflect.DeepEqual(eventKinds, descriptorKeySet(parsed.Payloads)) {
		return errors.New("eventKinds and payload declarations differ")
	}
	lifecycleKinds := make(map[string]bool)
	for kind, payload := range parsed.Payloads {
		if payload.AdditionalProperties || payload.GoType == "" || payload.PrivacyPolicy != parsed.Privacy.Policy {
			return fmt.Errorf("payloads.%s is not a closed typed payload under the privacy policy", kind)
		}
		if payload.Class != "lifecycle" && payload.Class != "passive" {
			return fmt.Errorf("payloads.%s has unknown class %q", kind, payload.Class)
		}
		wantRepairable := payload.Class == "lifecycle" && kind != "repair_applied"
		if payload.Repairable != wantRepairable {
			return fmt.Errorf("payloads.%s repairable metadata differs from lifecycle class", kind)
		}
		if payload.Class == "lifecycle" {
			lifecycleKinds[kind] = true
		}
		if err := validateDescriptorFields(parsed, "payloads."+kind, payload.Fields); err != nil {
			return err
		}
		if err := validateConstraintAuthority(parsed, "payloads."+kind, payload.Fields, payload.Constraints); err != nil {
			return err
		}
	}

	mappedLifecycleKinds := make(map[string]bool)
	for action, request := range parsed.LifecycleRequests {
		if request.AdditionalProperties || request.GoType == "" {
			return fmt.Errorf("lifecycleRequests.%s is not closed or typed", action)
		}
		payload, ok := parsed.Payloads[request.EventKind]
		if !ok || payload.Class != "lifecycle" {
			return fmt.Errorf("lifecycleRequests.%s references non-lifecycle event kind %q", action, request.EventKind)
		}
		if mappedLifecycleKinds[request.EventKind] {
			return fmt.Errorf("lifecycle event kind %q has multiple request mappings", request.EventKind)
		}
		mappedLifecycleKinds[request.EventKind] = true
		if err := validateDescriptorFields(parsed, "lifecycleRequests."+action, request.Fields); err != nil {
			return err
		}
		actionField, ok := request.Fields["action"]
		if !ok || !actionField.Required || actionField.Type != "string" || actionField.Format != "" || actionField.Vocabulary != "" {
			return fmt.Errorf("lifecycleRequests.%s.action is not the required discriminator", action)
		}
		if err := validateConstraintAuthority(parsed, "lifecycleRequests."+action, request.Fields, request.Constraints); err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(lifecycleKinds, mappedLifecycleKinds) {
		return errors.New("lifecycle request to payload mapping is incomplete")
	}

	for name, object := range parsed.Objects {
		if object.AdditionalProperties || object.GoType == "" || object.IdentityRule != "" {
			return fmt.Errorf("objects.%s is not closed and typed", name)
		}
		if err := validateDescriptorFields(parsed, "objects."+name, object.Fields); err != nil {
			return err
		}
	}
	for _, required := range []string{"OriginMetadata", "RepairReplacement", "RepairProposal", "EffortMetadata", "Association"} {
		if _, ok := parsed.Objects[required]; !ok {
			return fmt.Errorf("objects.%s is required", required)
		}
	}
	return validateWaiverAuthority(parsed)
}

func validateVocabularyAuthority(parsed protocolDescriptor) error {
	for name, values := range parsed.Vocabularies {
		if name == "" || len(values) == 0 {
			return fmt.Errorf("vocabulary %q is empty", name)
		}
		seen := make(map[string]bool, len(values))
		for _, value := range values {
			if value == "" || seen[value] {
				return fmt.Errorf("vocabulary %s has empty or duplicate value %q", name, value)
			}
			seen[value] = true
		}
	}
	for _, required := range []string{"eventKinds", "diagnosticRuleCodes", "waiverReasonCodes"} {
		if _, ok := parsed.Vocabularies[required]; !ok {
			return fmt.Errorf("vocabulary %s is required", required)
		}
	}
	return nil
}

func validateDescriptorFields(parsed protocolDescriptor, location string, fields map[string]fieldDescriptor) error {
	if len(fields) == 0 {
		return fmt.Errorf("%s.fields must not be empty", location)
	}
	for name, field := range fields {
		if name == "" {
			return fmt.Errorf("%s.fields contains an empty name", location)
		}
		if contains(parsed.Privacy.ForbiddenFields, name) {
			return fmt.Errorf("%s.fields contains privacy-forbidden field %q", location, name)
		}
		if err := validateFieldAuthority(parsed, location+".fields."+name, field); err != nil {
			return err
		}
	}
	return nil
}

func validateFieldAuthority(parsed protocolDescriptor, location string, field fieldDescriptor) error {
	if field.MinItems < 0 {
		return fmt.Errorf("%s.minItems must be non-negative", location)
	}
	switch field.Type {
	case "string":
		if field.Items != nil || len(field.Fields) != 0 || field.Minimum != nil || field.MinItems != 0 || field.UniqueItems || field.AdditionalProperties {
			return fmt.Errorf("%s has metadata incompatible with string", location)
		}
		if field.Format != "" && !contains([]string{"identifier", "checkpoint", "model", "tool", "category", "timestamp"}, field.Format) {
			return fmt.Errorf("%s has unknown format %q", location, field.Format)
		}
		if field.Vocabulary != "" {
			if _, ok := parsed.Vocabularies[field.Vocabulary]; !ok {
				return fmt.Errorf("%s references unknown vocabulary %q", location, field.Vocabulary)
			}
			if field.Format != "" {
				return fmt.Errorf("%s cannot declare both format and vocabulary", location)
			}
		}
	case "uint16", "uint64", "number":
		if field.Items != nil || len(field.Fields) != 0 || field.Format != "" || field.Vocabulary != "" || field.MinItems != 0 || field.UniqueItems || field.AdditionalProperties {
			return fmt.Errorf("%s has metadata incompatible with %s", location, field.Type)
		}
		if field.Minimum != nil && (*field.Minimum < 0 || math.IsNaN(*field.Minimum) || math.IsInf(*field.Minimum, 0)) {
			return fmt.Errorf("%s.minimum must be finite and non-negative", location)
		}
	case "array":
		if field.Items == nil || len(field.Fields) != 0 || field.Format != "" || field.Vocabulary != "" || field.Minimum != nil || field.AdditionalProperties {
			return fmt.Errorf("%s has invalid array metadata", location)
		}
		if err := validateFieldAuthority(parsed, location+".items", *field.Items); err != nil {
			return err
		}
	case "object":
		if field.Items != nil || field.Format != "" || field.Vocabulary != "" || field.Minimum != nil || field.MinItems != 0 || field.UniqueItems || field.AdditionalProperties {
			return fmt.Errorf("%s has invalid or open object metadata", location)
		}
		return validateDescriptorFields(parsed, location, field.Fields)
	case "payload", "replacement", "origin", "proposal":
		if field.Items != nil || len(field.Fields) != 0 || field.Format != "" || field.Vocabulary != "" || field.Minimum != nil || field.MinItems != 0 || field.UniqueItems || field.AdditionalProperties {
			return fmt.Errorf("%s has metadata incompatible with %s", location, field.Type)
		}
	default:
		return fmt.Errorf("%s has unsupported type %q", location, field.Type)
	}
	return nil
}

func validateConstraintAuthority(parsed protocolDescriptor, location string, fields map[string]fieldDescriptor, constraints []constraintDescriptor) error {
	for i, constraint := range constraints {
		constraintLocation := fmt.Sprintf("%s.constraints[%d]", location, i)
		requireField := func(name, role string) (fieldDescriptor, error) {
			field, ok := fields[name]
			if name == "" || !ok {
				return fieldDescriptor{}, fmt.Errorf("%s references unknown %s field %q", constraintLocation, role, name)
			}
			return field, nil
		}
		switch constraint.Kind {
		case "fields-required-when", "fields-forbidden-when":
			discriminator, err := requireField(constraint.Discriminator, "discriminator")
			if err != nil {
				return err
			}
			if discriminator.Type != "string" || !contains(parsed.Vocabularies[discriminator.Vocabulary], constraint.Value) || len(constraint.Fields) == 0 || constraint.Field != "" || constraint.RuleField != "" || constraint.ReasonField != "" {
				return fmt.Errorf("%s has invalid conditional metadata", constraintLocation)
			}
			for _, name := range constraint.Fields {
				if _, err := requireField(name, "conditional"); err != nil {
					return err
				}
			}
		case "field-allowed-when":
			discriminator, err := requireField(constraint.Discriminator, "discriminator")
			if err != nil {
				return err
			}
			if _, err := requireField(constraint.Field, "conditional"); err != nil {
				return err
			}
			if discriminator.Type != "string" || !contains(parsed.Vocabularies[discriminator.Vocabulary], constraint.Value) || len(constraint.Fields) != 0 || constraint.RuleField != "" || constraint.ReasonField != "" {
				return fmt.Errorf("%s has invalid allowed-when metadata", constraintLocation)
			}
		case "paired-presence":
			if len(constraint.Fields) != 2 || constraint.Discriminator != "" || constraint.Value != "" || constraint.Field != "" || constraint.RuleField != "" || constraint.ReasonField != "" {
				return fmt.Errorf("%s has invalid paired-presence metadata", constraintLocation)
			}
			for _, name := range constraint.Fields {
				if _, err := requireField(name, "paired"); err != nil {
					return err
				}
			}
		case "waiver-eligibility":
			rule, err := requireField(constraint.RuleField, "rule")
			if err != nil {
				return err
			}
			reason, err := requireField(constraint.ReasonField, "reason")
			if err != nil {
				return err
			}
			if rule.Vocabulary != "diagnosticRuleCodes" || reason.Vocabulary != "waiverReasonCodes" || constraint.Discriminator != "" || constraint.Value != "" || len(constraint.Fields) != 0 || constraint.Field != "" {
				return fmt.Errorf("%s has invalid waiver metadata", constraintLocation)
			}
		default:
			return fmt.Errorf("%s has unsupported kind %q", constraintLocation, constraint.Kind)
		}
	}
	return nil
}

func validateWaiverAuthority(parsed protocolDescriptor) error {
	rules := stringSet(parsed.Vocabularies["diagnosticRuleCodes"])
	reasons := stringSet(parsed.Vocabularies["waiverReasonCodes"])
	usedReasons := make(map[string]bool)
	for rule, allowed := range parsed.WaiverRules {
		if !rules[rule] || len(allowed) == 0 {
			return fmt.Errorf("waiverRules references unknown rule %q or has no reasons", rule)
		}
		seen := make(map[string]bool, len(allowed))
		for _, reason := range allowed {
			if !reasons[reason] || seen[reason] {
				return fmt.Errorf("waiverRules.%s has unknown or duplicate reason %q", rule, reason)
			}
			seen[reason] = true
			usedReasons[reason] = true
		}
	}
	if !reflect.DeepEqual(reasons, usedReasons) {
		return errors.New("waiver reason vocabulary and eligibility table differ")
	}
	return nil
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func descriptorKeySet[V any](values map[string]V) map[string]bool {
	result := make(map[string]bool, len(values))
	for key := range values {
		result[key] = true
	}
	return result
}

func DescriptorBytes() []byte {
	return append([]byte(nil), descriptorJSON...)
}

func DescriptorSHA256() string {
	sum := sha256.Sum256(DescriptorBytes())
	return hex.EncodeToString(sum[:])
}

func ValidateEvent(raw json.RawMessage) (EventEnvelope, error) {
	if !utf8.Valid(raw) {
		return EventEnvelope{}, errors.New("event is not valid UTF-8")
	}
	if err := validateJSONStructure(raw, "event", stringSet(descriptor.Privacy.ForbiddenFields)); err != nil {
		return EventEnvelope{}, err
	}
	root, err := rawObject(raw, "event")
	if err != nil {
		return EventEnvelope{}, err
	}
	versionRaw, ok := root["version"]
	if !ok {
		return EventEnvelope{}, errors.New("event: missing required field version")
	}
	var version ProtocolVersion
	if err := validateField("version", versionRaw, descriptor.Envelope.Fields["version"]); err != nil {
		return EventEnvelope{}, err
	}
	if err := json.Unmarshal(versionRaw, &version); err != nil { // coverage-ignore: descriptor validation already proved the two uint16 fields decode
		return EventEnvelope{}, fmt.Errorf("event.version: %w", err)
	}
	if version.Major != descriptor.Version.Major {
		return EventEnvelope{}, fmt.Errorf("event.version.major: unsupported protocol major %d", version.Major)
	}
	allowExtensions := version.Minor > descriptor.Version.Minor
	envelopeExtensions, err := validateObject("event", root, descriptor.Envelope.Fields, allowExtensions)
	if err != nil {
		return EventEnvelope{}, err
	}
	kind, err := rawString(root["kind"], "event.kind")
	if err != nil { // coverage-ignore: envelope validation already proved kind is a string
		return EventEnvelope{}, err
	}
	payloadSchema, ok := descriptor.Payloads[kind]
	if !ok { // coverage-ignore: envelope validation already proved kind belongs to descriptor eventKinds
		return EventEnvelope{}, fmt.Errorf("event.kind: unknown required kind %q", kind)
	}
	payload, err := rawObject(root["payload"], "event.payload")
	if err != nil { // coverage-ignore: envelope validation already proved payload is an object
		return EventEnvelope{}, err
	}
	payloadExtensions, err := validateObject("event.payload", payload, payloadSchema.Fields, allowExtensions)
	if err != nil {
		return EventEnvelope{}, err
	}
	if err := validateConstraints("event.payload", payload, payloadSchema.Constraints); err != nil {
		return EventEnvelope{}, err
	}
	if err := validateIdentityFields(payloadSchema.Class, root); err != nil {
		return EventEnvelope{}, err
	}
	var event EventEnvelope
	if err := json.Unmarshal(raw, &event); err != nil { // coverage-ignore: exhaustive descriptor validation proved every destination field decodes
		return EventEnvelope{}, fmt.Errorf("event: %w", err)
	}
	event.EnvelopeExtensions = envelopeExtensions
	event.PayloadExtensions = payloadExtensions
	return event, nil
}

func DecodeLifecycleRequest(raw json.RawMessage) (LifecycleRequest, error) {
	if !utf8.Valid(raw) {
		return nil, errors.New("lifecycle request is not valid UTF-8")
	}
	if err := validateJSONStructure(raw, "lifecycle request", stringSet(descriptor.Privacy.ForbiddenFields)); err != nil {
		return nil, err
	}
	root, err := rawObject(raw, "lifecycle request")
	if err != nil {
		return nil, err
	}
	actionRaw, ok := root["action"]
	if !ok {
		return nil, errors.New("lifecycle request: missing required field action")
	}
	action, err := rawString(actionRaw, "lifecycle request.action")
	if err != nil {
		return nil, err
	}
	schema, ok := descriptor.LifecycleRequests[action]
	if !ok {
		return nil, fmt.Errorf("lifecycle request.action: unknown action %q", action)
	}
	if _, err := validateObject("lifecycle request", root, schema.Fields, false); err != nil {
		return nil, err
	}
	if err := validateConstraints("lifecycle request", root, schema.Constraints); err != nil {
		return nil, err
	}
	var request LifecycleRequest
	switch schema.GoType {
	case "CreateLifecycleRequest":
		request = &CreateLifecycleRequest{}
	case "AssociateLifecycleRequest":
		request = &AssociateLifecycleRequest{}
	case "DetachLifecycleRequest":
		request = &DetachLifecycleRequest{}
	case "RouteLifecycleRequest":
		request = &RouteLifecycleRequest{}
	case "StartPhaseLifecycleRequest":
		request = &StartPhaseLifecycleRequest{}
	case "TransitionPhaseLifecycleRequest":
		request = &TransitionPhaseLifecycleRequest{}
	case "FinishPhaseLifecycleRequest":
		request = &FinishPhaseLifecycleRequest{}
	case "TrajectoryLifecycleRequest":
		request = &TrajectoryLifecycleRequest{}
	case "ForkTrajectoryLifecycleRequest":
		request = &ForkTrajectoryLifecycleRequest{}
	case "TerminalLifecycleRequest":
		request = &TerminalLifecycleRequest{}
	case "ReopenLifecycleRequest":
		request = &ReopenLifecycleRequest{}
	case "WaiveLifecycleRequest":
		request = &WaiveLifecycleRequest{}
	case "RepairLifecycleRequest":
		request = &RepairLifecycleRequest{}
	default:
		return nil, fmt.Errorf("lifecycle request.action: unsupported descriptor Go type %q", schema.GoType)
	}
	if err := json.Unmarshal(raw, request); err != nil { // coverage-ignore: request descriptor validation proved every destination field decodes
		return nil, fmt.Errorf("lifecycle request: %w", err)
	}
	return request, nil
}

func validateJSONStructure(raw []byte, location string, forbidden map[string]bool) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var walk func(string) error
	walk = func(current string) error {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("%s: invalid JSON: %w", current, err)
		}
		delim, isDelimiter := token.(json.Delim)
		if !isDelimiter {
			return nil
		}
		switch delim {
		case '{':
			seen := make(map[string]bool)
			for decoder.More() {
				nameToken, err := decoder.Token()
				if err != nil {
					return fmt.Errorf("%s: invalid object key: %w", current, err)
				}
				name, ok := nameToken.(string)
				if !ok { // coverage-ignore: encoding/json only returns string object keys
					return fmt.Errorf("%s: object key is not a string", current)
				}
				if seen[name] {
					return fmt.Errorf("%s: duplicate field %q", current, name)
				}
				seen[name] = true
				if forbidden[name] {
					return fmt.Errorf("%s: privacy-forbidden field %q", current, name)
				}
				if err := walk(current + "." + name); err != nil {
					return err
				}
			}
		case '[':
			for index := 0; decoder.More(); index++ {
				if err := walk(fmt.Sprintf("%s[%d]", current, index)); err != nil {
					return err
				}
			}
		default: // coverage-ignore: encoding/json emits only object or array delimiters at this validated position
			return fmt.Errorf("%s: unexpected JSON delimiter %q", current, delim)
		}
		if _, err := decoder.Token(); err != nil {
			return fmt.Errorf("%s: unterminated JSON value: %w", current, err)
		}
		return nil
	}
	if err := walk(location); err != nil {
		return err
	}
	return ensureJSONEOF(decoder)
}

func ensureJSONEOF(decoder *json.Decoder) error {
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return fmt.Errorf("trailing invalid JSON: %w", err)
	}
	return nil
}

func rawObject(raw json.RawMessage, location string) (map[string]json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%s: missing object", location)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("%s: expected object: %w", location, err)
	}
	if object == nil || strings.TrimSpace(string(raw))[0] != '{' {
		return nil, fmt.Errorf("%s: expected object", location)
	}
	return object, nil
}

func validateObject(location string, object map[string]json.RawMessage, fields map[string]fieldDescriptor, allowExtensions bool) (map[string]json.RawMessage, error) {
	extensions := make(map[string]json.RawMessage)
	for name, value := range object {
		field, known := fields[name]
		if !known {
			if contains(descriptor.Privacy.ForbiddenFields, name) { // coverage-ignore: structural privacy validation rejects this before object decoding
				return nil, fmt.Errorf("%s: privacy-forbidden field %q", location, name)
			}
			if !allowExtensions {
				return nil, fmt.Errorf("%s: unknown field %q", location, name)
			}
			extensions[name] = append(json.RawMessage(nil), value...)
			continue
		}
		if err := validateField(location+"."+name, value, field); err != nil {
			return nil, err
		}
	}
	for name, field := range fields {
		if field.Required {
			if _, present := object[name]; !present {
				return nil, fmt.Errorf("%s: missing required field %s", location, name)
			}
		}
	}
	return extensions, nil
}

func validateField(location string, raw json.RawMessage, field fieldDescriptor) error {
	switch field.Type {
	case "string":
		value, err := rawString(raw, location)
		if err != nil {
			return err
		}
		if field.Vocabulary != "" && !contains(descriptor.Vocabularies[field.Vocabulary], value) {
			return fmt.Errorf("%s: unknown %s value %q", location, field.Vocabulary, value)
		}
		return validateStringFormat(location, value, field.Format)
	case "uint16":
		value, err := rawUint(raw, location)
		if err != nil {
			return err
		}
		if value > math.MaxUint16 {
			return fmt.Errorf("%s: exceeds uint16", location)
		}
		return nil
	case "uint64":
		_, err := rawUint(raw, location)
		return err
	case "number":
		value, err := rawNumber(raw, location)
		if err != nil {
			return err
		}
		if field.Minimum != nil && value < *field.Minimum {
			return fmt.Errorf("%s: must be at least %v", location, *field.Minimum)
		}
		return nil
	case "array":
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil || values == nil || strings.TrimSpace(string(raw))[0] != '[' {
			return fmt.Errorf("%s: expected array", location)
		}
		if len(values) < field.MinItems {
			return fmt.Errorf("%s: requires at least %d item(s)", location, field.MinItems)
		}
		decodedValues := make([]any, 0, len(values))
		for index, value := range values {
			if field.Items == nil {
				return fmt.Errorf("%s: descriptor has no item schema", location)
			}
			if err := validateField(fmt.Sprintf("%s[%d]", location, index), value, *field.Items); err != nil {
				return err
			}
			if field.UniqueItems {
				var decoded any
				decoder := json.NewDecoder(bytes.NewReader(value))
				decoder.UseNumber()
				if err := decoder.Decode(&decoded); err != nil { // coverage-ignore: field validation already proved valid JSON
					return fmt.Errorf("%s[%d]: %w", location, index, err)
				}
				for _, prior := range decodedValues {
					if reflect.DeepEqual(prior, decoded) {
						return fmt.Errorf("%s: duplicate item", location)
					}
				}
				decodedValues = append(decodedValues, decoded)
			}
		}
		return nil
	case "object":
		object, err := rawObject(raw, location)
		if err != nil {
			return err
		}
		_, err = validateObject(location, object, field.Fields, false)
		return err
	case "payload":
		_, err := rawObject(raw, location)
		return err
	case "replacement":
		return validateReplacement(location, raw)
	case "origin":
		return validateDescriptorObject(location, raw, "OriginMetadata")
	case "proposal":
		return validateDescriptorObject(location, raw, "RepairProposal")
	default:
		return fmt.Errorf("%s: unsupported descriptor type %q", location, field.Type)
	}
}

func validateDescriptorObject(location string, raw json.RawMessage, name string) error {
	object, err := rawObject(raw, location)
	if err != nil {
		return err
	}
	schema, ok := descriptor.Objects[name]
	if !ok {
		return fmt.Errorf("%s: descriptor object %q is missing", location, name)
	}
	_, err = validateObject(location, object, schema.Fields, false)
	return err
}

func validateReplacement(location string, raw json.RawMessage) error {
	object, err := rawObject(raw, location)
	if err != nil {
		return err
	}
	schema, ok := descriptor.Objects["RepairReplacement"]
	if !ok {
		return fmt.Errorf("%s: replacement descriptor is missing", location)
	}
	if _, err := validateObject(location, object, schema.Fields, false); err != nil {
		return err
	}
	kind, err := rawString(object["eventKind"], location+".eventKind")
	if err != nil { // coverage-ignore: replacement object validation already proved eventKind is a string
		return err
	}
	payloadSchema, ok := descriptor.Payloads[kind]
	if !ok || payloadSchema.Class != "lifecycle" || kind == "repair_applied" {
		return fmt.Errorf("%s.eventKind: kind %q is not a non-recursive lifecycle replacement", location, kind)
	}
	payload, err := rawObject(object["payload"], location+".payload")
	if err != nil { // coverage-ignore: replacement object validation already proved payload is an object
		return err
	}
	if _, err := validateObject(location+".payload", payload, payloadSchema.Fields, false); err != nil {
		return err
	}
	return validateConstraints(location+".payload", payload, payloadSchema.Constraints)
}

func validateIdentityFields(class string, root map[string]json.RawMessage) error {
	_, hasIdempotency := root["idempotencyKey"]
	_, hasObservation := root["observationId"]
	if class == "lifecycle" && hasIdempotency && !hasObservation {
		return nil
	}
	if class == "passive" && hasObservation && !hasIdempotency {
		return nil
	}
	return fmt.Errorf("event: %s kind requires exactly its %s identity field", class, map[bool]string{true: "idempotencyKey", false: "observationId"}[class == "lifecycle"])
}

func validateConstraints(location string, object map[string]json.RawMessage, constraints []constraintDescriptor) error {
	for _, constraint := range constraints {
		switch constraint.Kind {
		case "fields-required-when", "fields-forbidden-when":
			value, _ := rawString(object[constraint.Discriminator], location+"."+constraint.Discriminator)
			if value != constraint.Value {
				continue
			}
			for _, field := range constraint.Fields {
				_, present := object[field]
				if constraint.Kind == "fields-required-when" && !present {
					return fmt.Errorf("%s: %s=%q requires %s", location, constraint.Discriminator, constraint.Value, field)
				}
				if constraint.Kind == "fields-forbidden-when" && present {
					return fmt.Errorf("%s: %s=%q forbids %s", location, constraint.Discriminator, constraint.Value, field)
				}
			}
		case "field-allowed-when":
			if _, present := object[constraint.Field]; !present {
				continue
			}
			value, _ := rawString(object[constraint.Discriminator], location+"."+constraint.Discriminator)
			if value != constraint.Value {
				return fmt.Errorf("%s: %s is allowed only when %s=%q", location, constraint.Field, constraint.Discriminator, constraint.Value)
			}
		case "paired-presence":
			_, left := object[constraint.Fields[0]]
			_, right := object[constraint.Fields[1]]
			if left != right {
				return fmt.Errorf("%s: %s and %s must be present together", location, constraint.Fields[0], constraint.Fields[1])
			}
		case "waiver-eligibility":
			rule, _ := rawString(object[constraint.RuleField], location+"."+constraint.RuleField)
			reason, _ := rawString(object[constraint.ReasonField], location+"."+constraint.ReasonField)
			if !contains(descriptor.WaiverRules[rule], reason) {
				return fmt.Errorf("%s.%s: %q is not allowed for %s", location, constraint.ReasonField, reason, rule)
			}
		default:
			return fmt.Errorf("%s: unsupported descriptor constraint %q", location, constraint.Kind)
		}
	}
	return nil
}

func validateStringFormat(location, value, format string) error {
	if value == "" {
		return fmt.Errorf("%s: must not be empty", location)
	}
	limit := descriptor.Limits.CategoryBytes
	switch format {
	case "identifier":
		limit = identifierLimit(location)
		if value == "." || value == ".." || strings.ContainsAny(value, "/\\") || strings.IndexByte(value, 0) >= 0 {
			return fmt.Errorf("%s: unsafe identifier", location)
		}
	case "model":
		limit = descriptor.Limits.ModelBytes
	case "tool":
		limit = descriptor.Limits.ToolBytes
	case "category", "":
	case "timestamp":
		if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
			return fmt.Errorf("%s: expected RFC3339Nano timestamp: %w", location, err)
		}
		return nil
	default:
		return fmt.Errorf("%s: unsupported string format %q", location, format)
	}
	if len([]byte(value)) > limit {
		return fmt.Errorf("%s: exceeds %d UTF-8 bytes", location, limit)
	}
	return nil
}

func identifierLimit(location string) int {
	name := location[strings.LastIndex(location, ".")+1:]
	switch name {
	case "eventId", "startEventId", "handoffEventId":
		return descriptor.Limits.EventIDBytes
	case "idempotencyKey":
		return descriptor.Limits.IdempotencyKeyBytes
	case "observationId":
		return descriptor.Limits.ObservationIDBytes
	default:
		return descriptor.Limits.IdentifierBytes
	}
}

func rawString(raw json.RawMessage, location string) (string, error) {
	var value string
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil || strings.TrimSpace(string(raw))[0] != '"' {
		return "", fmt.Errorf("%s: expected string", location)
	}
	return value, nil
}

func rawUint(raw json.RawMessage, location string) (uint64, error) {
	text := string(raw)
	if strings.ContainsAny(text, ".eE-+") {
		return 0, fmt.Errorf("%s: expected non-negative integer", location)
	}
	value, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: expected non-negative integer", location)
	}
	return value, nil
}

func rawNumber(raw json.RawMessage, location string) (float64, error) {
	value, err := strconv.ParseFloat(string(raw), 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
		return 0, fmt.Errorf("%s: expected finite number", location)
	}
	return value, nil
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
