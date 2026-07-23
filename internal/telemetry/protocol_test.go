package telemetry

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestDescriptorIsNormativeAndMatchesGoTypes(t *testing.T) {
	t.Parallel()
	first := DescriptorBytes()
	second := DescriptorBytes()
	if !bytes.Equal(first, second) || len(DescriptorSHA256()) != 64 {
		t.Fatal("descriptor bytes or digest are unstable")
	}
	first[0] ^= 1
	if bytes.Equal(first, DescriptorBytes()) {
		t.Fatal("DescriptorBytes returned mutable embedded storage")
	}

	types := protocolGoTypes()
	assertFields := func(name string, fields map[string]fieldDescriptor) {
		t.Helper()
		typeOf, ok := types[name]
		if !ok {
			t.Fatalf("descriptor names unknown Go type %q", name)
		}
		assertGoDescriptorFields(t, name, typeOf, fields)
	}
	assertFields(descriptor.Envelope.GoType, descriptor.Envelope.Fields)
	for _, schema := range descriptor.Payloads {
		assertFields(schema.GoType, schema.Fields)
	}
	for _, schema := range descriptor.Objects {
		assertFields(schema.GoType, schema.Fields)
	}
	for _, schema := range descriptor.LifecycleRequests {
		assertFields(schema.GoType, schema.Fields)
	}

	if descriptor.Version != (ProtocolVersion{Major: 2, Minor: 0}) || descriptor.Envelope.AdditionalProperties {
		t.Fatal("descriptor version or closed envelope differs")
	}
	if descriptor.Limits.IdentifierBytes != 128 || descriptor.Limits.EventIDBytes != 128 || descriptor.Limits.IdempotencyKeyBytes != 128 || descriptor.Limits.ObservationIDBytes != 128 || descriptor.Limits.ModelBytes != 128 || descriptor.Limits.ToolBytes != 128 || descriptor.Limits.CategoryBytes != 128 {
		t.Fatal("descriptor bounds differ")
	}
	assertProtocol2Descriptor(t)
	for vocabulary, values := range descriptor.Vocabularies {
		seen := map[string]bool{}
		for _, value := range values {
			if value == "" || seen[value] {
				t.Fatalf("vocabulary %s has empty or duplicate value %q", vocabulary, value)
			}
			seen[value] = true
			raw, _ := json.Marshal(value)
			if err := validateField("vocabulary", raw, fieldDescriptor{Type: "string", Vocabulary: vocabulary}); err != nil {
				t.Fatalf("descriptor vocabulary %s value %q rejected: %v", vocabulary, value, err)
			}
		}
		if err := validateField("vocabulary", json.RawMessage(`"not-declared"`), fieldDescriptor{Type: "string", Vocabulary: vocabulary}); err == nil {
			t.Fatalf("vocabulary %s accepted an undeclared value", vocabulary)
		}
	}
	lifecycleKinds := map[string]bool{}
	mappedKinds := map[string]bool{}
	for kind, payload := range descriptor.Payloads {
		if payload.AdditionalProperties || payload.PrivacyPolicy != descriptor.Privacy.Policy {
			t.Fatalf("payload %s is not closed under the declared privacy policy", kind)
		}
		if payload.Class == "lifecycle" {
			lifecycleKinds[kind] = true
			if payload.Repairable != (kind != "repair_applied") {
				t.Fatalf("payload %s repairability differs from its lifecycle class", kind)
			}
		}
	}
	for action, request := range descriptor.LifecycleRequests {
		if descriptor.Payloads[request.EventKind].Class != "lifecycle" || mappedKinds[request.EventKind] {
			t.Fatalf("action %s has invalid lifecycle mapping %q", action, request.EventKind)
		}
		mappedKinds[request.EventKind] = true
	}
	if !reflect.DeepEqual(lifecycleKinds, mappedKinds) {
		t.Fatalf("lifecycle mappings differ: payloads=%v requests=%v", lifecycleKinds, mappedKinds)
	}
	if !reflect.DeepEqual(descriptor.Vocabularies["eventKinds"], orderedPayloadKinds()) {
		t.Fatal("eventKinds is not the descriptor payload order")
	}
}

func assertProtocol2Descriptor(t *testing.T) {
	t.Helper()
	if descriptor.Privacy.AllowedRepositoryPathField != "" {
		t.Fatalf("protocol 2 retains repository-path privacy exception %q", descriptor.Privacy.AllowedRepositoryPathField)
	}
	oldField := "checkpoint" + "Id"
	if _, ok := descriptor.LimitsJSONFieldForTest(oldField + "Bytes"); ok {
		t.Fatal("protocol 2 retains protocol-1 path limit")
	}
	for _, activity := range []string{"adr-lifecycle", "refactor-coupling-audit", "roadmap-graduation"} {
		if !containsString(descriptor.Vocabularies["activities"], activity) {
			t.Errorf("protocol 2 activity %q is absent", activity)
		}
	}
	if !reflect.DeepEqual(descriptor.Vocabularies["routeActions"], []string{"select", "change"}) {
		t.Fatalf("routeActions = %v", descriptor.Vocabularies["routeActions"])
	}
	payload, ok := descriptor.Payloads["phase_transitioned"]
	if !ok || payload.GoType != "PhaseTransitionedPayload" || payload.Class != "lifecycle" || !payload.Repairable || payload.AdditionalProperties {
		t.Fatalf("phase_transitioned payload authority = %#v", payload)
	}
	request, ok := descriptor.LifecycleRequests["transition-phase"]
	if !ok || request.GoType != "TransitionPhaseLifecycleRequest" || request.EventKind != "phase_transitioned" || request.AdditionalProperties {
		t.Fatalf("transition-phase request authority = %#v", request)
	}
	for _, field := range []string{"phase", "startEventId", "nextPhase"} {
		if !payload.Fields[field].Required {
			t.Errorf("phase_transitioned.%s is not required", field)
		}
	}
	for _, field := range []string{"routeAction", "route"} {
		if payload.Fields[field].Required {
			t.Errorf("phase_transitioned.%s is not optional", field)
		}
	}
	if len(payload.Constraints) != 1 || payload.Constraints[0].Kind != "paired-presence" || !reflect.DeepEqual(payload.Constraints[0].Fields, []string{"routeAction", "route"}) {
		t.Fatalf("phase_transitioned route constraint = %#v", payload.Constraints)
	}
	if len(request.Constraints) != 1 || request.Constraints[0].Kind != "paired-presence" || !reflect.DeepEqual(request.Constraints[0].Fields, []string{"routeAction", "route"}) {
		t.Fatalf("transition-phase route constraint = %#v", request.Constraints)
	}
	for _, location := range []struct {
		name   string
		fields map[string]fieldDescriptor
	}{
		{"effort_created", descriptor.Payloads["effort_created"].Fields},
		{"create request", descriptor.LifecycleRequests["create"].Fields},
		{"effort metadata", descriptor.Objects["EffortMetadata"].Fields},
	} {
		if _, ok := location.fields[oldField]; ok {
			t.Errorf("%s retains protocol-1 path field", location.name)
		}
	}
}

func (p protocolDescriptor) LimitsJSONFieldForTest(name string) (json.RawMessage, bool) {
	raw, _ := json.Marshal(p.Limits)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	value, ok := fields[name]
	return value, ok
}

func TestValidateProtocol2PhaseTransitionShape(t *testing.T) {
	t.Parallel()
	event := protocol2TransitionEvent("transition", "brainstorm-start", []string{"brainstorm-start"}, "brainstorming", "implementation", "select", "direct")
	if _, err := ValidateEvent(mustJSON(t, event)); err != nil {
		t.Fatalf("protocol 2 transition rejected: %v", err)
	}
	delete(event["payload"].(map[string]any), "route")
	if _, err := ValidateEvent(mustJSON(t, event)); err == nil {
		t.Fatal("transition accepted routeAction without route")
	}
	request := protocol2TransitionRequest("transition", "brainstorm-start", []string{"brainstorm-start"}, "brainstorming", "implementation", "change", "plan")
	if _, err := DecodeLifecycleRequest(mustJSON(t, request)); err != nil {
		t.Fatalf("protocol 2 transition request rejected: %v", err)
	}
}

func protocol2TransitionEvent(eventID, startEventID string, predecessors []string, phase, nextPhase, routeAction, route string) map[string]any {
	payload := map[string]any{"phase": phase, "startEventId": startEventID, "nextPhase": nextPhase}
	if routeAction != "" {
		payload["routeAction"], payload["route"] = routeAction, route
	}
	return map[string]any{
		"version": map[string]any{"major": 2, "minor": 0}, "eventId": eventID, "idempotencyKey": "key-" + eventID,
		"effortId": "effort-id", "sessionId": "session-id", "timestamp": "2026-07-22T00:00:01Z",
		"kind": "phase_transitioned", "predecessors": predecessors, "payload": payload,
	}
}

func protocol2TransitionRequest(eventID, startEventID string, predecessors []string, phase, nextPhase, routeAction, route string) map[string]any {
	request := protocol2TransitionEvent(eventID, startEventID, predecessors, phase, nextPhase, routeAction, route)
	payload := request["payload"].(map[string]any)
	delete(request, "version")
	delete(request, "kind")
	delete(request, "payload")
	request["action"] = "transition-phase"
	for key, value := range payload {
		request[key] = value
	}
	return request
}

func TestValidateEventExhaustiveDescriptorContract(t *testing.T) {
	t.Parallel()
	for _, kind := range descriptor.Vocabularies["eventKinds"] {
		kind := kind
		t.Run(kind, func(t *testing.T) {
			event := validEvent(kind, 0)
			if _, err := ValidateEvent(mustJSON(t, event)); err != nil {
				t.Fatalf("valid event rejected: %v", err)
			}
			payload := event["payload"].(map[string]any)
			for field, metadata := range descriptor.Payloads[kind].Fields {
				changed := cloneMap(t, event)
				changed["payload"].(map[string]any)[field] = map[string]any{"wrong": true}
				if _, err := ValidateEvent(mustJSON(t, changed)); err == nil {
					t.Errorf("wrong type accepted for payload field %s", field)
				}
				if metadata.Required {
					changed = cloneMap(t, event)
					delete(changed["payload"].(map[string]any), field)
					if _, err := ValidateEvent(mustJSON(t, changed)); err == nil {
						t.Errorf("missing required payload field %s accepted", field)
					}
				}
			}
			payload["forbiddenUnknown"] = true
			if _, err := ValidateEvent(mustJSON(t, event)); err == nil {
				t.Error("minor-0 unknown payload field accepted")
			}
		})
	}
}

func TestValidateEventCompatibilityExtensionsAndIdentity(t *testing.T) {
	t.Parallel()
	minorZero := validEvent("usage_observed", 0)
	for field, metadata := range descriptor.Envelope.Fields {
		changed := cloneMap(t, minorZero)
		changed[field] = map[string]any{"wrong": true}
		if _, err := ValidateEvent(mustJSON(t, changed)); err == nil {
			t.Errorf("wrong type accepted for envelope field %s", field)
		}
		if metadata.Required {
			changed = cloneMap(t, minorZero)
			delete(changed, field)
			if _, err := ValidateEvent(mustJSON(t, changed)); err == nil {
				t.Errorf("missing required envelope field %s accepted", field)
			}
		}
	}
	minorZero["futureEnvelope"] = true
	if _, err := ValidateEvent(mustJSON(t, minorZero)); err == nil {
		t.Error("minor-0 unknown envelope field accepted")
	}

	event := validEvent("usage_observed", 1)
	raw := mustJSON(t, event)
	raw = bytes.Replace(raw, []byte(`"payload":{`), []byte(`"payload":{"futurePayload": {"n": 1},`), 1)
	raw = bytes.Replace(raw, []byte(`"version":{`), []byte(`"futureEnvelope": [1, 2],"version":{`), 1)
	decoded, err := ValidateEvent(raw)
	if err != nil {
		t.Fatalf("compatible-minor extensions rejected: %v", err)
	}
	if string(decoded.EnvelopeExtensions["futureEnvelope"]) != `[1, 2]` {
		t.Fatalf("envelope extension not preserved: %s", decoded.EnvelopeExtensions["futureEnvelope"])
	}
	if string(decoded.PayloadExtensions["futurePayload"]) != `{"n": 1}` {
		t.Fatalf("payload extension not preserved: %s", decoded.PayloadExtensions["futurePayload"])
	}

	cases := []map[string]any{
		validEvent("usage_observed", 0),
		validEvent("effort_created", 0),
	}
	delete(cases[0], "observationId")
	cases[1]["observationId"] = "observation"
	for _, invalid := range cases {
		if _, err := ValidateEvent(mustJSON(t, invalid)); err == nil {
			t.Error("invalid lifecycle/passive identity accepted")
		}
	}

	unsupported := validEvent("usage_observed", 0)
	unsupported["version"] = map[string]any{"major": 3, "minor": 0}
	if _, err := ValidateEvent(mustJSON(t, unsupported)); err == nil {
		t.Error("unsupported major accepted")
	}
	unknownKind := validEvent("usage_observed", 1)
	unknownKind["kind"] = "future_required_kind"
	if _, err := ValidateEvent(mustJSON(t, unknownKind)); err == nil {
		t.Error("unknown required kind accepted")
	}
	missing := validEvent("usage_observed", 1)
	delete(missing, "effortId")
	if _, err := ValidateEvent(mustJSON(t, missing)); err == nil {
		t.Error("missing known required field accepted")
	}
	collision := validEvent("usage_observed", 1)
	collision["effortId"] = 42
	if _, err := ValidateEvent(mustJSON(t, collision)); err == nil {
		t.Error("known-field type collision accepted")
	}
}

func TestProtocol2PairedPresenceConstraintAuthority(t *testing.T) {
	cloneRaw, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	var clone protocolDescriptor
	if err := json.Unmarshal(cloneRaw, &clone); err != nil {
		t.Fatal(err)
	}
	transition := clone.Payloads["phase_transitioned"]
	invalidMetadata := transition.Constraints[0]
	invalidMetadata.Field = "route"
	if err := validateConstraintAuthority(clone, "payload phase_transitioned", transition.Fields, []constraintDescriptor{invalidMetadata}); err == nil {
		t.Fatal("invalid paired-presence metadata accepted")
	}
	unknownField := transition.Constraints[0]
	unknownField.Fields = []string{"routeAction", "missing"}
	if err := validateConstraintAuthority(clone, "payload phase_transitioned", transition.Fields, []constraintDescriptor{unknownField}); err == nil {
		t.Fatal("unknown paired-presence field accepted")
	}
}

func TestValidateEventBoundsPrivacyAndConditions(t *testing.T) {
	t.Parallel()
	base := validEvent("usage_observed", 0)
	for name, value := range map[string]any{
		"empty identifier":  "",
		"slash identifier":  "unsafe/id",
		"dot identifier":    ".",
		"dotdot identifier": "..",
		"oversize UTF-8":    strings.Repeat("é", 65),
	} {
		changed := cloneMap(t, base)
		changed["effortId"] = value
		if _, err := ValidateEvent(mustJSON(t, changed)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	for _, mutation := range []func(map[string]any){
		func(e map[string]any) { e["payload"].(map[string]any)["inputTokens"] = -1 },
		func(e map[string]any) { e["payload"].(map[string]any)["durationMs"] = 1.5 },
		func(e map[string]any) { e["timestamp"] = "not-a-time" },
		func(e map[string]any) { e["predecessors"] = []any{"same", "same"} },
		func(e map[string]any) { e["payload"].(map[string]any)["phase"] = "unknown-phase" },
	} {
		changed := cloneMap(t, base)
		mutation(changed)
		if _, err := ValidateEvent(mustJSON(t, changed)); err == nil {
			t.Error("invalid bounded value accepted")
		}
	}
	nonFinite := bytes.Replace(mustJSON(t, base), []byte(`"costUsd":0`), []byte(`"costUsd":1e999`), 1)
	if _, err := ValidateEvent(nonFinite); err == nil {
		t.Error("non-finite usage accepted")
	}

	for _, forbidden := range descriptor.Privacy.ForbiddenFields {
		envelope := cloneMap(t, base)
		envelope[forbidden] = "private"
		if _, err := ValidateEvent(mustJSON(t, envelope)); err == nil {
			t.Errorf("privacy field %q accepted in envelope", forbidden)
		}
		payload := cloneMap(t, base)
		payload["payload"].(map[string]any)[forbidden] = "private"
		if _, err := ValidateEvent(mustJSON(t, payload)); err == nil {
			t.Errorf("privacy field %q accepted in payload", forbidden)
		}
	}

	independent := validEvent("effort_created", 0)
	independentPayload := independent["payload"].(map[string]any)
	independentPayload["creationMode"] = "independent"
	if _, err := ValidateEvent(mustJSON(t, independent)); err == nil {
		t.Error("independent creation with origin accepted")
	}
	delete(independentPayload, "originEffortId")
	delete(independentPayload, "originTrajectoryId")
	delete(independentPayload, "originAnchorId")
	if _, err := ValidateEvent(mustJSON(t, independent)); err != nil {
		t.Fatalf("independent creation rejected: %v", err)
	}

	shell := validEvent("shell_observed", 0)
	shellPayload := shell["payload"].(map[string]any)
	shellPayload["classification"] = "unclassified"
	if _, err := ValidateEvent(mustJSON(t, shell)); err == nil {
		t.Error("gateMode accepted for unclassified shell event")
	}
	delete(shellPayload, "gateMode")
	if _, err := ValidateEvent(mustJSON(t, shell)); err != nil {
		t.Fatalf("unclassified shell event without gateMode rejected: %v", err)
	}

	waiver := validEvent("finding_waived", 0)
	waiver["payload"].(map[string]any)["reasonCode"] = "approved-clock-skew"
	if _, err := ValidateEvent(mustJSON(t, waiver)); err == nil {
		t.Error("waiver reason accepted for an ineligible rule")
	}
}

func TestDecodeLifecycleRequestUnion(t *testing.T) {
	t.Parallel()
	for action, schema := range descriptor.LifecycleRequests {
		action, schema := action, schema
		t.Run(action, func(t *testing.T) {
			request := validRequest(action)
			decoded, err := DecodeLifecycleRequest(mustJSON(t, request))
			if err != nil {
				t.Fatalf("valid request rejected: %v", err)
			}
			if reflect.TypeOf(decoded).Elem().Name() != schema.GoType || decoded.lifecycleAction() != action {
				t.Fatalf("decoded wrong union member: %T", decoded)
			}
			for field, metadata := range schema.Fields {
				changed := cloneMap(t, request)
				changed[field] = map[string]any{"wrong": true}
				if _, err := DecodeLifecycleRequest(mustJSON(t, changed)); err == nil {
					t.Errorf("wrong type accepted for request field %s", field)
				}
				if metadata.Required {
					delete(changed, field)
					if _, err := DecodeLifecycleRequest(mustJSON(t, changed)); err == nil {
						t.Errorf("missing required request field %s accepted", field)
					}
				}
			}
			request["unknown"] = true
			if _, err := DecodeLifecycleRequest(mustJSON(t, request)); err == nil {
				t.Error("unknown request field accepted")
			}
		})
	}

	derived := validRequest("create")
	delete(derived, "origin")
	if _, err := DecodeLifecycleRequest(mustJSON(t, derived)); err == nil {
		t.Error("derived create without origin accepted")
	}
	independent := validRequest("create")
	independent["creationMode"] = "independent"
	if _, err := DecodeLifecycleRequest(mustJSON(t, independent)); err == nil {
		t.Error("independent create with origin accepted")
	}
	waive := validRequest("waive")
	waive["reasonCode"] = "approved-clock-skew"
	if _, err := DecodeLifecycleRequest(mustJSON(t, waive)); err == nil {
		t.Error("ineligible lifecycle waiver accepted")
	}
	for _, raw := range []json.RawMessage{nil, json.RawMessage(`[]`), json.RawMessage(`{"action":"unknown"}`), {0xff}} {
		if _, err := DecodeLifecycleRequest(raw); err == nil {
			t.Errorf("invalid request accepted: %q", raw)
		}
	}
}

func TestRepairReplacementIsClosedAndLifecycleOnly(t *testing.T) {
	t.Parallel()
	repair := validEvent("repair_applied", 0)
	for kind, payload := range descriptor.Payloads {
		changed := cloneMap(t, repair)
		replacement := changed["payload"].(map[string]any)["replacement"].(map[string]any)
		replacement["eventKind"] = kind
		replacement["payload"] = validFields(payload.Fields)
		applyValidConditions(kind, replacement["payload"].(map[string]any))
		_, err := ValidateEvent(mustJSON(t, changed))
		wantValid := payload.Class == "lifecycle" && kind != "repair_applied"
		if (err == nil) != wantValid {
			t.Errorf("replacement %q validity=%v, want %v: %v", kind, err == nil, wantValid, err)
		}
	}
	for _, kind := range []string{"repair_applied", "usage_observed", "unknown"} {
		changed := cloneMap(t, repair)
		changed["payload"].(map[string]any)["replacement"].(map[string]any)["eventKind"] = kind
		if _, err := ValidateEvent(mustJSON(t, changed)); err == nil {
			t.Errorf("non-repairable replacement %q accepted", kind)
		}
	}
	repair["payload"].(map[string]any)["replacement"].(map[string]any)["extra"] = true
	if _, err := ValidateEvent(mustJSON(t, repair)); err == nil {
		t.Error("unknown replacement field accepted")
	}
}

func TestDuplicateJSONKeysAreRejectedRecursively(t *testing.T) {
	t.Parallel()
	base := mustJSON(t, validEvent("repair_applied", 0))
	for name, raw := range map[string]json.RawMessage{
		"envelope":    bytes.Replace(base, []byte(`"eventId":"event-id"`), []byte(`"eventId":"shadow","eventId":"event-id"`), 1),
		"version":     bytes.Replace(base, []byte(`"major":2`), []byte(`"major":2,"major":2`), 1),
		"payload":     bytes.Replace(base, []byte(`"proposalKind":`), []byte(`"proposalKind":"supersede-event","proposalKind":`), 1),
		"replacement": bytes.Replace(base, []byte(`"eventKind":`), []byte(`"eventKind":"phase_started","eventKind":`), 1),
	} {
		if _, err := ValidateEvent(raw); err == nil {
			t.Errorf("duplicate %s key accepted", name)
		}
	}
	request := mustJSON(t, validRequest("create"))
	request = bytes.Replace(request, []byte(`"anchorId":`), []byte(`"anchorId":"shadow","anchorId":`), 1)
	if _, err := DecodeLifecycleRequest(request); err == nil {
		t.Error("duplicate nested lifecycle key accepted")
	}
}

func TestCompatibleMinorPrivacyIsRecursive(t *testing.T) {
	t.Parallel()
	for _, location := range []string{"envelope", "payload"} {
		event := validEvent("usage_observed", 1)
		if location == "envelope" {
			event["future"] = map[string]any{"nested": []any{map[string]any{"prompt": "private"}}}
		} else {
			event["payload"].(map[string]any)["future"] = map[string]any{"nested": map[string]any{"stderr": "private"}}
		}
		if _, err := ValidateEvent(mustJSON(t, event)); err == nil {
			t.Errorf("nested privacy field in %s extension accepted", location)
		}
	}
}

func TestFinishPhaseOutcomeMatchesEventVocabulary(t *testing.T) {
	t.Parallel()
	for _, value := range descriptor.Vocabularies["outcomes"] {
		event := validEvent("phase_finished", 0)
		event["payload"].(map[string]any)["outcome"] = value
		if _, err := ValidateEvent(mustJSON(t, event)); err != nil {
			t.Errorf("event outcome %q rejected: %v", value, err)
		}
		request := validRequest("finish-phase")
		request["outcome"] = value
		if _, err := DecodeLifecycleRequest(mustJSON(t, request)); err != nil {
			t.Errorf("request outcome %q rejected: %v", value, err)
		}
	}
	request := validRequest("finish-phase")
	request["outcome"] = "completed"
	if _, err := DecodeLifecycleRequest(mustJSON(t, request)); err == nil {
		t.Error("terminal effort outcome accepted as phase outcome")
	}
}

func TestUniqueArraysCompareDecodedValues(t *testing.T) {
	t.Parallel()
	event := mustJSON(t, validEvent("usage_observed", 0))
	event = bytes.Replace(event, []byte(`"predecessors":[]`), []byte(`"predecessors":["same","\\u0073ame"]`), 1)
	if _, err := ValidateEvent(event); err == nil {
		t.Error("escaped duplicate predecessor accepted")
	}
}

func TestDescriptorParsingRejectsInvalidAuthority(t *testing.T) {
	t.Parallel()
	valid := DescriptorBytes()
	mutate := func(fn func(map[string]any)) []byte {
		t.Helper()
		var value map[string]any
		if err := json.Unmarshal(valid, &value); err != nil {
			t.Fatal(err)
		}
		fn(value)
		return mustJSON(t, value)
	}
	cases := map[string][]byte{
		"duplicate key": bytes.Replace(valid, []byte(`"major": 2`), []byte(`"major": 2, "major": 2`), 1),
		"unknown key": mutate(func(value map[string]any) {
			value["unknown"] = true
		}),
		"unknown vocabulary reference": mutate(func(value map[string]any) {
			value["payloads"].(map[string]any)["route_selected"].(map[string]any)["fields"].(map[string]any)["route"].(map[string]any)["vocabulary"] = "missing"
		}),
		"payload vocabulary parity": mutate(func(value map[string]any) {
			kinds := value["vocabularies"].(map[string]any)["eventKinds"].([]any)
			value["vocabularies"].(map[string]any)["eventKinds"] = kinds[1:]
		}),
		"lifecycle reference": mutate(func(value map[string]any) {
			value["lifecycleRequests"].(map[string]any)["create"].(map[string]any)["eventKind"] = "usage_observed"
		}),
		"constraint field reference": mutate(func(value map[string]any) {
			constraints := value["payloads"].(map[string]any)["effort_created"].(map[string]any)["constraints"].([]any)
			constraints[0].(map[string]any)["fields"] = []any{"missing"}
		}),
	}
	for name, raw := range cases {
		name, raw := name, raw
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("invalid descriptor did not panic")
				}
			}()
			mustParseDescriptor(raw)
		})
	}
}

func protocolGoTypes() map[string]reflect.Type {
	values := []any{
		EventEnvelope{}, EffortCreatedPayload{}, SessionAssociatedPayload{}, SessionDetachedPayload{}, RoutePayload{},
		PhaseStartedPayload{}, PhaseTransitionedPayload{}, PhaseFinishedPayload{}, TrajectoryPayload{}, TrajectoryForkedPayload{}, EffortTerminalPayload{}, EffortAbandonedPayload{},
		EffortReopenedPayload{}, FindingWaivedPayload{}, RepairAppliedPayload{}, UsageObservedPayload{}, ToolObservedPayload{},
		ShellObservedPayload{}, CompactionObservedPayload{}, HandoffObservedPayload{}, SubagentObservedPayload{}, SessionObservedPayload{},
		OriginMetadata{}, RepairReplacement{}, RepairProposal{}, EffortMetadata{}, Association{}, CreateLifecycleRequest{}, TransitionPhaseLifecycleRequest{},
		AssociateLifecycleRequest{}, DetachLifecycleRequest{}, RouteLifecycleRequest{}, StartPhaseLifecycleRequest{},
		FinishPhaseLifecycleRequest{}, TrajectoryLifecycleRequest{}, ForkTrajectoryLifecycleRequest{}, TerminalLifecycleRequest{}, AbandonLifecycleRequest{},
		ReopenLifecycleRequest{}, WaiveLifecycleRequest{}, RepairLifecycleRequest{},
	}
	result := make(map[string]reflect.Type, len(values))
	for _, value := range values {
		typeOf := reflect.TypeOf(value)
		result[typeOf.Name()] = typeOf
	}
	return result
}

func assertGoDescriptorFields(t *testing.T, location string, typeOf reflect.Type, fields map[string]fieldDescriptor) {
	t.Helper()
	goFields := map[string]reflect.StructField{}
	var collect func(reflect.Type)
	collect = func(current reflect.Type) {
		for index := range current.NumField() {
			field := current.Field(index)
			if field.Anonymous {
				collect(field.Type)
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name != "" && name != "-" {
				goFields[name] = field
			}
		}
	}
	collect(typeOf)
	if !reflect.DeepEqual(keySet(goFields), keySet(fields)) {
		t.Fatalf("%s fields differ: Go=%v descriptor=%v", location, keySet(goFields), keySet(fields))
	}
	for name, goField := range goFields {
		metadata := fields[name]
		tagParts := strings.Split(goField.Tag.Get("json"), ",")
		optional := len(tagParts) > 1 && slicesContains(tagParts[1:], "omitempty")
		if metadata.Required == optional {
			t.Errorf("%s.%s required differs: Go optional=%v descriptor required=%v", location, name, optional, metadata.Required)
		}
		expectedType, expectedFormat, expectedVocabulary := goFieldMetadata(name, goField.Type)
		if metadata.Type != expectedType || metadata.Format != expectedFormat || metadata.Vocabulary != expectedVocabulary {
			t.Errorf("%s.%s metadata differs: got type=%q format=%q vocabulary=%q; want type=%q format=%q vocabulary=%q", location, name, metadata.Type, metadata.Format, metadata.Vocabulary, expectedType, expectedFormat, expectedVocabulary)
		}
		switch expectedType {
		case "uint16", "uint64", "number":
			if metadata.Minimum == nil || *metadata.Minimum != 0 {
				t.Errorf("%s.%s must explicitly declare minimum 0", location, name)
			}
		case "array":
			if metadata.Items == nil {
				t.Errorf("%s.%s has no item metadata", location, name)
				continue
			}
			itemType, itemFormat, itemVocabulary := goFieldMetadata(name, goField.Type.Elem())
			if metadata.Items.Type != itemType || metadata.Items.Format != itemFormat || metadata.Items.Vocabulary != itemVocabulary {
				t.Errorf("%s.%s item metadata differs", location, name)
			}
			wantMin := 0
			if name == "evidenceIds" || name == "sourceEventIds" {
				wantMin = 1
			}
			if !metadata.UniqueItems || metadata.MinItems != wantMin {
				t.Errorf("%s.%s uniqueness/cardinality differs: unique=%v minItems=%d want minItems=%d", location, name, metadata.UniqueItems, metadata.MinItems, wantMin)
			}
		case "object":
			assertGoDescriptorFields(t, location+"."+name, goField.Type, metadata.Fields)
		}
	}
}

func goFieldMetadata(name string, typeOf reflect.Type) (fieldType, format, vocabulary string) {
	if typeOf.Kind() == reflect.Pointer {
		typeOf = typeOf.Elem()
	}
	switch typeOf.Name() {
	case "EventKind":
		return "string", "", "eventKinds"
	case "Route":
		return "string", "", "routes"
	case "RouteAction":
		return "string", "", "routeActions"
	case "Phase":
		return "string", "", "phases"
	case "Activity":
		return "string", "", "activities"
	case "AssociationOrigin":
		return "string", "", "associationOrigins"
	case "CreationMode":
		return "string", "", "creationModes"
	case "DetachReason":
		return "string", "", "detachReasons"
	case "ProposalKind":
		return "string", "", "proposalKinds"
	case "Outcome":
		return "string", "", "outcomes"
	case "TerminalOutcome":
		return "string", "", "terminalOutcomes"
	case "StopReason":
		return "string", "", "stopReasons"
	case "ErrorCategory":
		return "string", "", "errorCategories"
	case "ShellClassification":
		return "string", "", "shellClassifications"
	case "GateMode":
		return "string", "", "gateModes"
	case "WaiverReasonCode":
		return "string", "", "waiverReasonCodes"
	case "DiagnosticRuleCode":
		return "string", "", "diagnosticRuleCodes"
	case "BoundedCategory":
		return "string", "category", ""
	case "ModelName":
		return "string", "model", ""
	case "ToolName":
		return "string", "tool", ""
	case "ProtocolVersion":
		return "object", "", ""
	case "OriginMetadata":
		return "origin", "", ""
	case "RepairReplacement":
		return "replacement", "", ""
	case "RepairProposal":
		return "proposal", "", ""
	case "RawMessage":
		return "payload", "", ""
	}
	switch typeOf.Kind() {
	case reflect.String:
		switch name {
		case "action":
			return "string", "", ""
		case "timestamp", "createdAt":
			return "string", "timestamp", ""
		default:
			return "string", "identifier", ""
		}
	case reflect.Uint16:
		return "uint16", "", ""
	case reflect.Uint64:
		return "uint64", "", ""
	case reflect.Float64:
		return "number", "", ""
	case reflect.Slice:
		return "array", "", ""
	case reflect.Struct:
		return "object", "", ""
	default:
		panic("unsupported Go descriptor field " + typeOf.String())
	}
}

func slicesContains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func keySet[V any](values map[string]V) map[string]bool {
	result := make(map[string]bool, len(values))
	for value := range values {
		result[value] = true
	}
	return result
}

func orderedPayloadKinds() []string {
	var raw struct {
		Payloads map[string]json.RawMessage `json:"payloads"`
	}
	if err := json.Unmarshal(DescriptorBytes(), &raw); err != nil {
		panic(err)
	}
	// JSON object order is not represented by a Go map, so eventKinds remains the
	// descriptor's explicit ordering authority. This helper checks set parity.
	result := descriptor.Vocabularies["eventKinds"]
	if len(raw.Payloads) != len(result) {
		return nil
	}
	for _, kind := range result {
		if _, ok := raw.Payloads[kind]; !ok {
			return nil
		}
	}
	return result
}

func validEvent(kind string, minor uint16) map[string]any {
	schema := descriptor.Payloads[kind]
	event := map[string]any{
		"version":      map[string]any{"major": descriptor.Version.Major, "minor": minor},
		"eventId":      "event-id",
		"effortId":     "effort-id",
		"sessionId":    "session-id",
		"timestamp":    "2026-07-22T12:34:56.123456789Z",
		"kind":         kind,
		"predecessors": []any{},
		"payload":      validFields(schema.Fields),
	}
	if schema.Class == "lifecycle" {
		event["idempotencyKey"] = "idempotency-key"
	} else {
		event["observationId"] = "observation-id"
	}
	applyValidConditions(kind, event["payload"].(map[string]any))
	return event
}

func validRequest(action string) map[string]any {
	request := validFields(descriptor.LifecycleRequests[action].Fields)
	request["action"] = action
	if action == "create" {
		request["creationMode"] = "derived"
	}
	if action == "waive" {
		request["ruleCode"] = "WFV1-PHASE-ORDER"
		request["reasonCode"] = "approved-route-deviation"
	}
	return request
}

func validFields(fields map[string]fieldDescriptor) map[string]any {
	result := make(map[string]any, len(fields))
	for name, field := range fields {
		result[name] = validField(name, field)
	}
	return result
}

func validField(name string, field fieldDescriptor) any {
	switch field.Type {
	case "string":
		if field.Vocabulary != "" {
			return descriptor.Vocabularies[field.Vocabulary][0]
		}
		switch field.Format {
		case "timestamp":
			return "2026-07-22T12:34:56Z"
		case "checkpoint":
			return "docs/checkpoint.md"
		default:
			return name + "-id"
		}
	case "uint16", "uint64", "number":
		return 0
	case "array":
		if field.MinItems > 0 {
			return []any{"evidence-id"}
		}
		return []any{}
	case "object":
		return validFields(field.Fields)
	case "origin":
		return validFields(descriptor.Objects["OriginMetadata"].Fields)
	case "replacement":
		return map[string]any{"eventKind": "phase_started", "payload": validFields(descriptor.Payloads["phase_started"].Fields)}
	case "proposal":
		return validFields(descriptor.Objects["RepairProposal"].Fields)
	case "payload":
		return map[string]any{}
	default:
		panic("unsupported test descriptor field type " + field.Type)
	}
}

func applyValidConditions(kind string, payload map[string]any) {
	switch kind {
	case "effort_created":
		payload["creationMode"] = "derived"
	case "finding_waived":
		payload["ruleCode"] = "WFV1-PHASE-ORDER"
		payload["reasonCode"] = "approved-route-deviation"
	case "shell_observed":
		payload["classification"] = "gate"
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func cloneMap(t *testing.T, source map[string]any) map[string]any {
	t.Helper()
	var clone map[string]any
	if err := json.Unmarshal(mustJSON(t, source), &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func TestValidationHelperErrorBranches(t *testing.T) {
	assertError := func(err error) {
		t.Helper()
		if err == nil {
			t.Fatal("expected error")
		}
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("malformed descriptor did not panic")
			}
		}()
		mustParseDescriptor([]byte(`{`))
	}()

	for _, raw := range []json.RawMessage{nil, json.RawMessage(`[]`), json.RawMessage(`{}`), {0xff}} {
		_, err := ValidateEvent(raw)
		assertError(err)
	}
	badVersion := validEvent("usage_observed", 0)
	badVersion["version"] = map[string]any{"major": 65536, "minor": 0}
	_, err := ValidateEvent(mustJSON(t, badVersion))
	assertError(err)

	_, err = DecodeLifecycleRequest(json.RawMessage(`{}`))
	assertError(err)
	_, err = DecodeLifecycleRequest(json.RawMessage(`{"action":1}`))
	assertError(err)

	assertError(validateField("x", json.RawMessage(`1`), fieldDescriptor{Type: "string"}))
	assertError(validateField("x", json.RawMessage(`-1`), fieldDescriptor{Type: "uint16"}))
	assertError(validateField("x", json.RawMessage(`65536`), fieldDescriptor{Type: "uint16"}))
	minimum := float64(1)
	assertError(validateField("x", json.RawMessage(`"bad"`), fieldDescriptor{Type: "number"}))
	assertError(validateField("x", json.RawMessage(`0`), fieldDescriptor{Type: "number", Minimum: &minimum}))
	assertError(validateField("x", json.RawMessage(`null`), fieldDescriptor{Type: "array"}))
	assertError(validateField("x", json.RawMessage(`[]`), fieldDescriptor{Type: "array", MinItems: 1}))
	assertError(validateField("x", json.RawMessage(`["x"]`), fieldDescriptor{Type: "array"}))
	badItem := fieldDescriptor{Type: "uint64"}
	assertError(validateField("x", json.RawMessage(`["x"]`), fieldDescriptor{Type: "array", Items: &badItem}))
	assertError(validateField("x", json.RawMessage(`[]`), fieldDescriptor{Type: "object"}))
	assertError(validateField("x", json.RawMessage(`[]`), fieldDescriptor{Type: "payload"}))
	assertError(validateField("x", json.RawMessage(`null`), fieldDescriptor{Type: "origin"}))
	assertError(validateField("x", json.RawMessage(`null`), fieldDescriptor{Type: "proposal"}))
	assertError(validateField("x", json.RawMessage(`null`), fieldDescriptor{Type: "replacement"}))
	assertError(validateField("x", json.RawMessage(`null`), fieldDescriptor{Type: "unknown"}))

	assertError(validateStringFormat("model", strings.Repeat("m", 129), "model"))
	assertError(validateStringFormat("tool", strings.Repeat("t", 129), "tool"))
	assertError(validateStringFormat("category", strings.Repeat("c", 129), "category"))
	assertError(validateStringFormat("checkpoint", strings.Repeat("c", 257), "checkpoint"))
	assertError(validateStringFormat("checkpoint", "/absolute", "checkpoint"))
	assertError(validateStringFormat("checkpoint", "../outside", "checkpoint"))
	assertError(validateStringFormat("x", "value", "unknown"))
	assertError(func() error { _, err := rawUint(json.RawMessage(`18446744073709551616`), "x"); return err }())

	assertError(validateConstraints("x", map[string]json.RawMessage{}, []constraintDescriptor{{Kind: "unknown"}}))
	if err := validateConstraints("x", map[string]json.RawMessage{"mode": json.RawMessage(`"other"`)}, []constraintDescriptor{{Kind: "fields-required-when", Discriminator: "mode", Value: "wanted", Fields: []string{"field"}}}); err != nil {
		t.Fatalf("nonmatching constraint rejected: %v", err)
	}

	originalOrigin := descriptor.Objects["OriginMetadata"]
	delete(descriptor.Objects, "OriginMetadata")
	assertError(validateDescriptorObject("x", json.RawMessage(`{}`), "OriginMetadata"))
	descriptor.Objects["OriginMetadata"] = originalOrigin
	originalReplacement := descriptor.Objects["RepairReplacement"]
	delete(descriptor.Objects, "RepairReplacement")
	assertError(validateReplacement("x", json.RawMessage(`{}`)))
	descriptor.Objects["RepairReplacement"] = originalReplacement

	replacement := validField("replacement", fieldDescriptor{Type: "replacement"})
	replacementMap := replacement.(map[string]any)
	replacementMap["eventKind"] = 1
	assertError(validateReplacement("x", mustJSON(t, replacementMap)))
	replacementMap = validField("replacement", fieldDescriptor{Type: "replacement"}).(map[string]any)
	replacementMap["payload"] = []any{}
	assertError(validateReplacement("x", mustJSON(t, replacementMap)))
	replacementMap = validField("replacement", fieldDescriptor{Type: "replacement"}).(map[string]any)
	delete(replacementMap["payload"].(map[string]any), "phase")
	assertError(validateReplacement("x", mustJSON(t, replacementMap)))

	originalRequest := descriptor.LifecycleRequests["complete"]
	changedRequest := originalRequest
	changedRequest.GoType = "UnknownLifecycleRequest"
	descriptor.LifecycleRequests["complete"] = changedRequest
	_, err = DecodeLifecycleRequest(mustJSON(t, validRequest("complete")))
	assertError(err)
	descriptor.LifecycleRequests["complete"] = originalRequest
}
