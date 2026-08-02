// Package execution tests the closed capability-planned execution mechanism.
package execution

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// invariant: code-design/execution-planning:closed-step-selection (TestClosedStepSelection)
func TestClosedStepSelection(t *testing.T) {
	cases := []struct {
		name   string
		system System
		want   definitionErrorKind
	}{
		{"duplicate requirement", System{Requirements: []Requirement{{ID: "r"}, {ID: "r"}}}, definitionDuplicateRequirement},
		{"duplicate step", System{Steps: []Step{{ID: "s"}, {ID: "s"}}}, definitionDuplicateStep},
		{"unknown foundation", System{Foundations: []RequirementID{"missing"}}, definitionUnknownFoundation},
		{"unknown dependency", System{Requirements: []Requirement{{ID: "r", Dependencies: []RequirementID{"missing"}}}}, definitionUnknownDependency},
		{"cycle", System{Requirements: []Requirement{{ID: "a", Dependencies: []RequirementID{"b"}}, {ID: "b", Dependencies: []RequirementID{"a"}}}}, definitionDependencyCycle},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Prepare(context.Background(), tc.system, nil)
			var got *definitionError
			if !errors.As(err, &got) || got.kind != tc.want {
				t.Fatalf("Prepare error = %v, want definition error kind %q", err, tc.want)
			}
		})
	}

	_, err := Prepare(context.Background(), System{Steps: []Step{{ID: "known"}}}, []StepID{"missing"})
	assertDefinitionKind(t, err, definitionUnknownRequestedStep)

	preparedBeforeValidation := false
	_, err = Prepare(context.Background(), System{
		Requirements: []Requirement{{ID: "r", Prepare: func(context.Context) error { preparedBeforeValidation = true; return nil }}, {ID: "r"}},
	}, nil)
	assertDefinitionKind(t, err, definitionDuplicateRequirement)
	if preparedBeforeValidation {
		t.Fatal("static validation prepared a requirement")
	}

	prepared := mustPrepare(t, System{
		Requirements: []Requirement{{ID: "r"}},
		Steps:        []Step{{ID: "later", Requirements: requirements("r")}, {ID: "first", Requirements: requirements("r")}},
		Bind:         bindActions(map[StepID]Action{"later": noAction, "first": noAction}),
	}, []StepID{"first", "later"})
	if got, want := prepared.steps, []StepID{"later", "first"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selected order = %v, want %v", got, want)
	}

	secondaryPrepared, bound := false, false
	prepared, err = Prepare(context.Background(), System{
		Requirements: []Requirement{{ID: "secondary", Prepare: func(context.Context) error { secondaryPrepared = true; return nil }}},
		Steps:        []Step{{ID: "selected", Requirements: requirements("missing")}},
		Bind:         func([]StepID) ([]BoundAction, error) { bound = true; return nil, nil },
	}, []StepID{"selected"})
	if prepared != nil {
		t.Fatal("unknown resolved requirement returned a prepared execution")
	}
	assertDefinitionKind(t, err, definitionUnknownResolved)
	if secondaryPrepared || bound {
		t.Fatalf("unknown resolved requirement prepared secondary = %v, bound = %v", secondaryPrepared, bound)
	}

	var bindingEvents []string
	prepared, err = Prepare(context.Background(), System{
		Requirements: []Requirement{{ID: "secondary", Prepare: record(&bindingEvents, "prepare")}},
		Steps:        []Step{{ID: "selected", Requirements: requirements("secondary")}},
		Bind: func([]StepID) ([]BoundAction, error) {
			bindingEvents = append(bindingEvents, "bind")
			return []BoundAction{{Step: "wrong", Run: noAction}}, nil
		},
	}, []StepID{"selected"})
	if prepared != nil {
		t.Fatal("wrong binding identity returned a prepared execution")
	}
	assertDefinitionKind(t, err, definitionInvalidBinding)
	if want := []string{"prepare", "bind"}; !reflect.DeepEqual(bindingEvents, want) {
		t.Fatalf("binding events = %v, want %v", bindingEvents, want)
	}

	assertBindingValidation(t)
}

// invariant: code-design/execution-planning:requirements-prepared-once (TestRequirementsPreparedOnce)
func TestRequirementsPreparedOnce(t *testing.T) {
	var events []string
	system := System{
		Requirements: []Requirement{
			{ID: "late", Dependencies: []RequirementID{"base"}, Prepare: record(&events, "late")},
			{ID: "base", Prepare: record(&events, "base")},
			{ID: "foundation", Dependencies: []RequirementID{"base"}, Prepare: record(&events, "foundation")},
			{ID: "other", Dependencies: []RequirementID{"base"}, Prepare: record(&events, "other")},
		},
		Foundations: []RequirementID{"foundation"},
		Steps: []Step{
			{ID: "one", Requirements: resolving(&events, "resolve-one", "late", "base")},
			{ID: "two", Requirements: resolving(&events, "resolve-two", "other", "late")},
		},
		Bind: func(steps []StepID) ([]BoundAction, error) {
			events = append(events, "bind")
			return []BoundAction{{Step: steps[0], Run: noAction}, {Step: steps[1], Run: noAction}}, nil
		},
	}
	mustPrepare(t, system, []StepID{"two", "one"})
	if want := []string{"base", "foundation", "resolve-one", "resolve-two", "late", "other", "bind"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}

	stages := []struct {
		name           string
		sys            System
		bindingFailure bool
	}{
		{"validation", System{Requirements: []Requirement{{ID: "r"}, {ID: "r"}}, Steps: []Step{{ID: "s"}}}, false},
		{"foundation", System{Requirements: []Requirement{{ID: "f", Prepare: failing("foundation")}}, Foundations: []RequirementID{"f"}, Steps: []Step{{ID: "s"}}}, false},
		{"resolution", System{Steps: []Step{{ID: "s", Requirements: failingRequirements("resolution")}}}, false},
		{"resolved identity", System{Steps: []Step{{ID: "s", Requirements: requirements("missing")}}}, false},
		{"secondary", System{Requirements: []Requirement{{ID: "r", Prepare: failing("secondary")}}, Steps: []Step{{ID: "s", Requirements: requirements("r")}}}, false},
		{"binding", System{Steps: []Step{{ID: "s"}}, Bind: func([]StepID) ([]BoundAction, error) { return nil, errors.New("binding") }}, true},
	}
	for _, tc := range stages {
		t.Run(tc.name, func(t *testing.T) {
			actionRan := false
			if !tc.bindingFailure {
				tc.sys.Bind = func([]StepID) ([]BoundAction, error) {
					return []BoundAction{{Step: "s", Run: func(context.Context) error { actionRan = true; return nil }}}, nil
				}
			}
			prepared, err := Prepare(context.Background(), tc.sys, []StepID{"s"})
			if err == nil {
				t.Fatal("Prepare succeeded")
			}
			if prepared != nil {
				t.Fatal("failed readiness returned a prepared execution")
			}
			if actionRan {
				t.Fatal("failed readiness executed an action")
			}
		})
	}
}

func assertBindingValidation(t *testing.T) {
	t.Helper()
	_, err := Prepare(context.Background(), System{Steps: []Step{{ID: "one"}}}, []StepID{"one"})
	assertDefinitionKind(t, err, definitionInvalidBinding)

	base := System{Steps: []Step{{ID: "one"}, {ID: "two"}}}
	cases := []struct {
		name    string
		actions []BoundAction
	}{
		{"missing", []BoundAction{{Step: "one", Run: noAction}}},
		{"extra", []BoundAction{{Step: "one", Run: noAction}, {Step: "two", Run: noAction}, {Step: "three", Run: noAction}}},
		{"duplicate", []BoundAction{{Step: "one", Run: noAction}, {Step: "one", Run: noAction}}},
		{"wrong identity", []BoundAction{{Step: "two", Run: noAction}, {Step: "one", Run: noAction}}},
		{"nil action", []BoundAction{{Step: "one"}, {Step: "two", Run: noAction}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			system := base
			system.Bind = func([]StepID) ([]BoundAction, error) { return tc.actions, nil }
			_, err := Prepare(context.Background(), system, []StepID{"one", "two"})
			assertDefinitionKind(t, err, definitionInvalidBinding)
		})
	}

	t.Run("binder mutates selected identities", func(t *testing.T) {
		system := base
		system.Bind = func(selected []StepID) ([]BoundAction, error) {
			selected[0] = "two"
			return []BoundAction{{Step: "two", Run: noAction}, {Step: "two", Run: noAction}}, nil
		}
		_, err := Prepare(context.Background(), system, []StepID{"one", "two"})
		assertDefinitionKind(t, err, definitionInvalidBinding)
	})

	bindErr := errors.New("binder failed")
	_, err = Prepare(context.Background(), System{
		Steps: []Step{{ID: "one"}},
		Bind:  func([]StepID) ([]BoundAction, error) { return nil, bindErr },
	}, []StepID{"one"})
	if !errors.Is(err, bindErr) || !strings.Contains(err.Error(), "[one]") {
		t.Fatalf("binding error = %v, want selected identity and wrapped cause", err)
	}
}

// invariant: code-design/execution-planning:explicit-step-failure-policy (TestExplicitStepFailurePolicy)
func TestExplicitStepFailurePolicy(t *testing.T) {
	boom := errors.New("boom")
	var calls []string
	prepared := mustPrepare(t, System{
		Steps: []Step{{ID: "second"}, {ID: "first"}, {ID: "third"}},
		Bind: bindActions(map[StepID]Action{
			"second": record(&calls, "second"),
			"first":  func(context.Context) error { calls = append(calls, "first"); return boom },
			"third":  record(&calls, "third"),
		}),
	}, []StepID{"first", "second", "third"})

	outcomes, err := prepared.Run(context.Background(), StopOnFailure)
	if err != nil || !reflect.DeepEqual(calls, []string{"second", "first"}) {
		t.Fatalf("stop outcomes = %v, error = %v, calls = %v", outcomes, err, calls)
	}
	if got := outcomeSteps(outcomes); !reflect.DeepEqual(got, []StepID{"second", "first"}) || !errors.Is(outcomes[1].Err, boom) {
		t.Fatalf("stop outcomes = %#v", outcomes)
	}

	calls = nil
	outcomes, err = prepared.Run(context.Background(), ContinueOnFailure)
	if err != nil || !reflect.DeepEqual(calls, []string{"second", "first", "third"}) || len(outcomes) != 3 {
		t.Fatalf("continue outcomes = %v, error = %v, calls = %v", outcomes, err, calls)
	}
	if _, err := prepared.Run(context.Background(), FailurePolicy(99)); err == nil {
		t.Fatal("unsupported policy succeeded")
	}

	assertRunCancellation(t)
}

func assertRunCancellation(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	prepared := mustPrepare(t, System{Steps: []Step{{ID: "one"}}, Bind: bindActions(map[StepID]Action{"one": noAction})}, []StepID{"one"})
	outcomes, err := prepared.Run(ctx, StopOnFailure)
	if len(outcomes) != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancel outcomes = %v, error = %v", outcomes, err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	prepared = mustPrepare(t, System{Steps: []Step{{ID: "one"}, {ID: "two"}}, Bind: bindActions(map[StepID]Action{
		"one": func(context.Context) error { cancel(); return nil }, "two": noAction,
	})}, []StepID{"one", "two"})
	outcomes, err = prepared.Run(ctx, ContinueOnFailure)
	if got := outcomeSteps(outcomes); !reflect.DeepEqual(got, []StepID{"one"}) || !errors.Is(err, context.Canceled) {
		t.Fatalf("between-action outcomes = %v, error = %v", outcomes, err)
	}

	boom := errors.New("action failed")
	ctx, cancel = context.WithCancel(context.Background())
	prepared = mustPrepare(t, System{Steps: []Step{{ID: "one"}}, Bind: bindActions(map[StepID]Action{
		"one": func(context.Context) error { cancel(); return boom },
	})}, []StepID{"one"})
	outcomes, err = prepared.Run(ctx, ContinueOnFailure)
	if len(outcomes) != 1 || !errors.Is(outcomes[0].Err, boom) || !errors.Is(err, context.Canceled) {
		t.Fatalf("failing cancellation outcomes = %v, error = %v", outcomes, err)
	}
}

func requirements(ids ...RequirementID) func(context.Context) ([]RequirementID, error) {
	return func(context.Context) ([]RequirementID, error) { return ids, nil }
}
func failingRequirements(message string) func(context.Context) ([]RequirementID, error) {
	return func(context.Context) ([]RequirementID, error) { return nil, errors.New(message) }
}
func record(events *[]string, name string) Action {
	return func(context.Context) error { *events = append(*events, name); return nil }
}
func resolving(events *[]string, name string, ids ...RequirementID) func(context.Context) ([]RequirementID, error) {
	return func(context.Context) ([]RequirementID, error) { *events = append(*events, name); return ids, nil }
}
func failing(message string) func(context.Context) error {
	return func(context.Context) error { return errors.New(message) }
}
func bindActions(actions map[StepID]Action) Binder {
	return func(steps []StepID) ([]BoundAction, error) {
		out := make([]BoundAction, len(steps))
		for i, step := range steps {
			out[i] = BoundAction{Step: step, Run: actions[step]}
			if actions == nil {
				out[i].Run = noAction
			}
		}
		return out, nil
	}
}
func noAction(context.Context) error { return nil }
func mustPrepare(t *testing.T, system System, requested []StepID) *Prepared {
	t.Helper()
	prepared, err := Prepare(context.Background(), system, requested)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	return prepared
}
func assertDefinitionKind(t *testing.T, err error, want definitionErrorKind) {
	t.Helper()
	var got *definitionError
	if !errors.As(err, &got) || got.kind != want {
		t.Fatalf("error = %v, want definition error kind %q", err, want)
	}
}
func outcomeSteps(outcomes []Outcome) []StepID {
	steps := make([]StepID, len(outcomes))
	for i, outcome := range outcomes {
		steps[i] = outcome.Step
	}
	return steps
}

func TestDefinitionErrorText(t *testing.T) {
	for _, err := range []*definitionError{
		{kind: definitionDuplicateRequirement, requirement: "r"},
		{kind: definitionDuplicateStep, step: "s"},
		{kind: definitionUnknownFoundation, referencedRequirement: "r"},
		{kind: definitionUnknownDependency, requirement: "r", referencedRequirement: "dependency"},
		{kind: definitionDependencyCycle, requirement: "r"},
		{kind: definitionUnknownRequestedStep, referencedStep: "s"},
		{kind: definitionUnknownResolved, step: "s", referencedRequirement: "r"},
		{kind: definitionInvalidBinding, step: "s", referencedStep: "bound"},
		{kind: definitionErrorKind("unknown")},
	} {
		if err.Error() == "" {
			t.Fatal("empty definition error")
		}
	}
	if got := firstUnordered([]Requirement{{ID: "r"}}, []RequirementID{"r"}); got != "" {
		t.Fatalf("first unordered = %q, want empty", got)
	}
}
