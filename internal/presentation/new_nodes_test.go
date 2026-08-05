package presentation

import "testing"

func TestStepsValidation(t *testing.T) {
	nodeMarker{}.presentationNode()
	if _, err := NewSteps("steps", value{}); err == nil {
		t.Fatal("invalid value")
	}
	if _, err := NewSection("section", Steps{label: "steps"}); err == nil {
		t.Fatal("invalid steps")
	}
}
