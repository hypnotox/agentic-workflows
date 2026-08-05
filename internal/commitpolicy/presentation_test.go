package commitpolicy

import (
	"errors"
	"testing"
)

func TestPresentation(t *testing.T) {
	for _, outcome := range []Outcome{{Refusal: &Refusal{Category: BaselineFailure, Observed: "missing", Actions: []string{"fix"}, Cause: errors.New("cause")}}, {}} {
		if _, err := Presentation(Policy{}, outcome); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Presentation(Policy{}, Outcome{Refusal: &Refusal{Category: BaselineFailure, Observed: "missing", Actions: []string{" "}}}); err == nil {
		t.Fatal("invalid action accepted")
	}
}
