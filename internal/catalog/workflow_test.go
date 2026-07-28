package catalog

import "testing"

func TestWorkflowProfilesAreCompleteAndAdvisory(t *testing.T) {
	if err := ValidateWorkflowProfiles(nil); err == nil {
		t.Fatal("nil catalog accepted")
	}
	if err := ValidateWorkflowProfiles(Standard); err != nil {
		t.Fatalf("standard profile validation: %v", err)
	}
	for _, kind := range []WorkflowKind{WorkflowChain, WorkflowTask, WorkflowSupport} {
		if !validWorkflowKind(kind) {
			t.Errorf("validWorkflowKind(%q) = false", kind)
		}
	}
}

func TestWorkflowProfileRejectsUnknownSelfAndDuplicateNeighbors(t *testing.T) {
	base := func(profile WorkflowProfile) *Catalog {
		return &Catalog{Skills: map[string]SkillSpec{
			"one": {Profile: profile},
			"two": {Profile: WorkflowProfile{Kind: WorkflowTask, Purpose: "two", Trigger: "two"}},
		}}
	}
	for _, tc := range []struct {
		name string
		p    WorkflowProfile
	}{
		{"incomplete", WorkflowProfile{Kind: WorkflowTask}},
		{"unknown kind", WorkflowProfile{Kind: "unknown", Purpose: "one", Trigger: "one"}},
		{"unknown neighbor", WorkflowProfile{Kind: WorkflowTask, Purpose: "one", Trigger: "one", UsuallyFollows: []string{"missing"}}},
		{"self neighbor", WorkflowProfile{Kind: WorkflowTask, Purpose: "one", Trigger: "one", UsuallyFollows: []string{"one"}}},
		{"duplicate neighbor", WorkflowProfile{Kind: WorkflowTask, Purpose: "one", Trigger: "one", UsuallyFollows: []string{"two", "two"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateWorkflowProfiles(base(tc.p)); err == nil {
				t.Fatal("invalid profile accepted")
			}
		})
	}
}
