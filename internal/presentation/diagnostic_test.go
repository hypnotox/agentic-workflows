package presentation

import (
	"bytes"
	"testing"
)

func TestMutationDocument(t *testing.T) {
	value, _ := Prose("changed")
	for _, mutation := range []Mutation{
		{},
		{Status: " "},
		{Status: "ok", Identity: []Field{{}}},
		{Status: "ok", Changes: []MutationChange{{}}},
		{Status: "ok", Changes: []MutationChange{{Label: "changed"}}},
		{Status: "ok", Changes: []MutationChange{{Label: " ", Values: []Value{value}}}},
		{Status: "ok", Changes: []MutationChange{{Label: "changed", Values: []Value{{}}}}},
		{Status: "ok", Notes: []Value{{}}},
		{Status: "ok", NextActions: []Value{{}}},
	} {
		if _, err := mutation.Document(); err == nil {
			t.Fatalf("invalid mutation accepted: %#v", mutation)
		}
	}
	empty, err := (Mutation{Status: "completed"}).Document()
	if err != nil {
		t.Fatal(err)
	}
	var emptyOut bytes.Buffer
	if err := Render(&emptyOut, empty); err != nil {
		t.Fatal(err)
	}
	if emptyOut.String() != "status: completed\n" {
		t.Fatalf("empty mutation output = %q", emptyOut.String())
	}
	identityValue, _ := Prose("demo")
	identity, _ := NewField("effort", identityValue)
	note, _ := Prose("ownership note")
	next, _ := Prose("continue")
	document, err := (Mutation{
		Status: "completed", Identity: []Field{identity},
		Changes: []MutationChange{{Label: "changed", Values: []Value{value}}},
		Notes:   []Value{note}, NextActions: []Value{next},
	}).Document()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Render(&out, document); err != nil {
		t.Fatal(err)
	}
	const want = "status: completed\n\nmutation:\n  identity:\n    effort: demo\n  changes:\n    changed:\n      changed\n  notes:\n    ownership note\n  next actions:\n    step 1: continue\n"
	if out.String() != want {
		t.Fatalf("mutation output = %q, want %q", out.String(), want)
	}
}

func TestDiagnostic(t *testing.T) {
	value, _ := Prose("yes")
	changed, _ := NewField("index", value)
	step, _ := Prose("repair")
	document, err := (Diagnostic{Condition: "stopped", State: "operation", Changed: []Field{changed}, Cause: "failed", Steps: []Value{step}}).Document()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Render(&out, document); err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range []Diagnostic{
		{}, {Condition: " "}, {Condition: "ok", State: " "}, {Condition: "ok", Cause: " "},
		{Condition: "ok", Changed: []Field{{}}}, {Condition: "ok", Steps: []Value{{}}},
	} {
		if _, err := diagnostic.Document(); err == nil {
			t.Fatal("invalid diagnostic accepted")
		}
	}
}
