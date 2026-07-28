package catalog

import (
	"reflect"
	"testing"
)

func TestValidateWorkflowMappingsUsesClosedFixedBodyKinds(t *testing.T) {
	if err := ValidateWorkflowMappings(nil); err == nil {
		t.Fatal("nil catalog accepted")
	}
	for _, tc := range []struct {
		name string
		cat  *Catalog
	}{
		{"unmapped", &Catalog{Skills: map[string]SkillSpec{"skill": {}}}},
		{"unknown-kind", &Catalog{Skills: map[string]SkillSpec{"skill": {Workflow: &WorkflowMapping{Kind: "other"}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateWorkflowMappings(tc.cat); err == nil {
				t.Fatal("invalid mapping accepted")
			}
		})
	}
	if err := ValidateWorkflowMappings(Standard); err != nil {
		t.Fatalf("standard mapping validation: %v", err)
	}
	for _, kind := range []WorkflowKind{WorkflowChain, WorkflowTask, WorkflowSupport} {
		if !validWorkflowKind(kind) {
			t.Errorf("validWorkflowKind(%q) = false", kind)
		}
	}
	if validWorkflowKind("unknown") {
		t.Fatal("unknown workflow kind accepted")
	}
}

func TestWorkflowMappingsForEnabledSkillsRejectsInvalidInputs(t *testing.T) {
	if _, err := WorkflowMappingsForSkills(nil, nil); err == nil {
		t.Fatal("nil catalog accepted")
	}
	base := &Catalog{Skills: map[string]SkillSpec{
		"chain":   {Workflow: &WorkflowMapping{Kind: WorkflowChain}},
		"task":    {Workflow: &WorkflowMapping{Kind: WorkflowTask}},
		"support": {Workflow: &WorkflowMapping{Kind: WorkflowSupport}},
		"missing": {},
		"invalid": {Workflow: &WorkflowMapping{Kind: "invalid"}},
	}}
	for _, tc := range []struct {
		name    string
		enabled []string
	}{
		{"duplicate", []string{"chain", "chain"}},
		{"stale", []string{"stale"}},
		{"unmapped", []string{"missing"}},
		{"invalid", []string{"invalid"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := WorkflowMappingsForSkills(base, tc.enabled); err == nil {
				t.Fatal("invalid enabled workflow set accepted")
			}
		})
	}
	got, err := WorkflowMappingsForSkills(base, []string{"support", "chain", "task"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]WorkflowMapping{
		"chain":   {Kind: WorkflowChain},
		"task":    {Kind: WorkflowTask},
		"support": {Kind: WorkflowSupport},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("workflow mappings = %#v, want %#v", got, want)
	}
	got["chain"] = WorkflowMapping{Kind: WorkflowTask}
	if base.Skills["chain"].Workflow.Kind != WorkflowChain {
		t.Fatal("returned mapping aliases catalog metadata")
	}
}
