package presentation

import (
	"bytes"
	"testing"
)

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
