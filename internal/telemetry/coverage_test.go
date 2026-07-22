package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"testing"
	"time"
)

func cloneProtocolDescriptor(t *testing.T) protocolDescriptor {
	t.Helper()
	raw, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	var cloned protocolDescriptor
	if err := json.Unmarshal(raw, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func TestDescriptorAuthorityRejectsEveryMalformedAuthorityShape(t *testing.T) {
	mutations := []func(*protocolDescriptor){
		func(d *protocolDescriptor) { d.Version.Major = 0 },
		func(d *protocolDescriptor) { d.Limits.IdentifierBytes = 0 },
		func(d *protocolDescriptor) { d.Privacy.Policy = "" },
		func(d *protocolDescriptor) {
			d.Privacy.ForbiddenFields = append(d.Privacy.ForbiddenFields, d.Privacy.ForbiddenFields[0])
		},
		func(d *protocolDescriptor) { d.Vocabularies[""] = []string{"x"} },
		func(d *protocolDescriptor) { d.Vocabularies["eventKinds"] = append(d.Vocabularies["eventKinds"], "") },
		func(d *protocolDescriptor) { delete(d.Vocabularies, "diagnosticRuleCodes") },
		func(d *protocolDescriptor) { d.Envelope.AdditionalProperties = true },
		func(d *protocolDescriptor) { d.Envelope.Fields = nil },
		func(d *protocolDescriptor) {
			field := d.Envelope.Fields["payload"]
			field.Required = false
			d.Envelope.Fields["payload"] = field
		},
		func(d *protocolDescriptor) { delete(d.Payloads, "usage_observed") },
		func(d *protocolDescriptor) {
			payload := d.Payloads["usage_observed"]
			payload.GoType = ""
			d.Payloads["usage_observed"] = payload
		},
		func(d *protocolDescriptor) {
			payload := d.Payloads["usage_observed"]
			payload.Class = "other"
			d.Payloads["usage_observed"] = payload
		},
		func(d *protocolDescriptor) {
			payload := d.Payloads["usage_observed"]
			payload.Repairable = true
			d.Payloads["usage_observed"] = payload
		},
		func(d *protocolDescriptor) {
			request := d.LifecycleRequests["create"]
			request.GoType = ""
			d.LifecycleRequests["create"] = request
		},
		func(d *protocolDescriptor) {
			request := d.LifecycleRequests["create"]
			request.EventKind = "usage_observed"
			d.LifecycleRequests["create"] = request
		},
		func(d *protocolDescriptor) {
			request := d.LifecycleRequests["associate"]
			request.EventKind = d.LifecycleRequests["create"].EventKind
			d.LifecycleRequests["associate"] = request
		},
		func(d *protocolDescriptor) {
			request := d.LifecycleRequests["create"]
			request.Fields = nil
			d.LifecycleRequests["create"] = request
		},
		func(d *protocolDescriptor) {
			request := d.LifecycleRequests["create"]
			field := request.Fields["action"]
			field.Required = false
			request.Fields["action"] = field
			d.LifecycleRequests["create"] = request
		},
		func(d *protocolDescriptor) {
			request := d.LifecycleRequests["create"]
			request.Constraints = []constraintDescriptor{{Kind: "unknown"}}
			d.LifecycleRequests["create"] = request
		},
		func(d *protocolDescriptor) { delete(d.LifecycleRequests, "create") },
		func(d *protocolDescriptor) {
			object := d.Objects["Association"]
			object.IdentityRule = "bad"
			d.Objects["Association"] = object
		},
		func(d *protocolDescriptor) {
			object := d.Objects["Association"]
			object.Fields = nil
			d.Objects["Association"] = object
		},
		func(d *protocolDescriptor) { delete(d.Objects, "Association") },
		func(d *protocolDescriptor) { d.WaiverRules["missing-rule"] = []string{"approved-exception"} },
		func(d *protocolDescriptor) {
			for rule := range d.WaiverRules {
				d.WaiverRules[rule] = nil
				break
			}
		},
		func(d *protocolDescriptor) {
			for rule := range d.WaiverRules {
				d.WaiverRules[rule] = append(d.WaiverRules[rule], d.WaiverRules[rule][0])
				break
			}
		},
		func(d *protocolDescriptor) {
			d.Vocabularies["waiverReasonCodes"] = append(d.Vocabularies["waiverReasonCodes"], "unused-reason")
		},
	}
	for index, mutate := range mutations {
		candidate := cloneProtocolDescriptor(t)
		mutate(&candidate)
		if err := validateDescriptor(candidate); err == nil {
			t.Errorf("mutation %d accepted", index)
		}
	}
}

func TestDescriptorFieldAndConstraintAuthorityErrorMatrix(t *testing.T) {
	minimumNegative := -1.0
	minimumNaN := math.NaN()
	validString := fieldDescriptor{Type: "string"}
	fieldCases := []fieldDescriptor{
		{Type: "string", MinItems: -1},
		{Type: "string", UniqueItems: true},
		{Type: "string", Format: "bad"},
		{Type: "string", Vocabulary: "missing"},
		{Type: "string", Format: "identifier", Vocabulary: "eventKinds"},
		{Type: "uint64", Format: "identifier"},
		{Type: "number", Minimum: &minimumNegative},
		{Type: "number", Minimum: &minimumNaN},
		{Type: "array"},
		{Type: "array", Items: &fieldDescriptor{Type: "bad"}},
		{Type: "object", Items: &validString},
		{Type: "payload", Fields: map[string]fieldDescriptor{"x": validString}},
		{Type: "bad"},
	}
	for index, field := range fieldCases {
		if err := validateFieldAuthority(descriptor, "test", field); err == nil {
			t.Errorf("field case %d accepted", index)
		}
	}
	if err := validateDescriptorFields(descriptor, "test", map[string]fieldDescriptor{"": validString}); err == nil {
		t.Fatal("empty field name accepted")
	}
	forbidden := descriptor.Privacy.ForbiddenFields[0]
	if err := validateDescriptorFields(descriptor, "test", map[string]fieldDescriptor{forbidden: validString}); err == nil {
		t.Fatal("privacy-forbidden field accepted")
	}

	fields := map[string]fieldDescriptor{
		"disc":   {Type: "string", Vocabulary: "outcomes"},
		"target": {Type: "string"},
		"rule":   {Type: "string", Vocabulary: "diagnosticRuleCodes"},
		"reason": {Type: "string", Vocabulary: "waiverReasonCodes"},
	}
	constraints := []constraintDescriptor{
		{Kind: "fields-required-when", Discriminator: "missing", Value: "success", Fields: []string{"target"}},
		{Kind: "fields-required-when", Discriminator: "disc", Value: "missing", Fields: []string{"target"}},
		{Kind: "fields-forbidden-when", Discriminator: "disc", Value: "success", Fields: []string{"missing"}},
		{Kind: "field-allowed-when", Discriminator: "missing", Value: "success", Field: "target"},
		{Kind: "field-allowed-when", Discriminator: "disc", Value: "success", Field: "missing"},
		{Kind: "field-allowed-when", Discriminator: "disc", Value: "missing", Field: "target"},
		{Kind: "waiver-eligibility", RuleField: "missing", ReasonField: "reason"},
		{Kind: "waiver-eligibility", RuleField: "rule", ReasonField: "missing"},
		{Kind: "waiver-eligibility", RuleField: "target", ReasonField: "reason"},
		{Kind: "unknown"},
	}
	for index, constraint := range constraints {
		if err := validateConstraintAuthority(descriptor, "test", fields, []constraintDescriptor{constraint}); err == nil {
			t.Errorf("constraint case %d accepted", index)
		}
	}
}

func TestValidatedDecodeAndPlatformFailureSeams(t *testing.T) {
	for _, raw := range [][]byte{{0xff}, []byte("{} {}"), []byte("{} x")} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("invalid embedded descriptor %q did not panic", raw)
				}
			}()
			_ = mustParseDescriptor(raw)
		}()
	}
	if _, err := rawObject(nil, "empty"); err == nil {
		t.Fatal("empty object accepted")
	}
	if err := ensureJSONEOF(json.NewDecoder(errorReader{})); err == nil {
		t.Fatal("decoder read failure accepted")
	}
}

// errorReader makes the JSON decoder's trailing-token read fail with a non-EOF
// error, a branch bytes.Reader-backed production input cannot reach.
type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failure") }

func TestFilesystemFaultBranchesAreAsserted(t *testing.T) {
	ledger, metadata, _ := createTestEffort(t)
	original := ledger.ops.syncDir
	ledger.ops.syncDir = func(string) error { return errInjected }
	if err := ledger.syncDir(filepath.Join(ledger.paths.effort(metadata.EffortID), "sessions")); err == nil {
		t.Fatal("directory sync fault accepted")
	}
	ledger.ops.syncDir = original

	if _, err := lockLeaseOperations(context.Background(), filepath.Join(ledger.paths.leaseGuard(), "child")); err == nil {
		t.Fatal("invalid operation-guard path accepted")
	}
	unlock, err := lockLeaseOperations(context.Background(), ledger.paths.leaseGuard())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if _, err := lockLeaseOperations(ctx, ledger.paths.leaseGuard()); err == nil {
		t.Fatal("canceled operation-guard wait accepted")
	}
	if err := unlock(); err != nil {
		t.Fatal(err)
	}

	missing := filepath.Join(t.TempDir(), "missing")
	if err := syncDirectory(missing); err == nil {
		t.Fatal("missing directory sync accepted")
	}
	if _, err := NewLedger(""); err == nil {
		t.Fatal("empty project root accepted")
	}
}
