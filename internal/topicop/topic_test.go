package topicop

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestUsageErrorMessage(t *testing.T) {
	if got := (&UsageError{Message: "usage"}).Error(); got != "usage" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestRunValidatesBeforeStateAndFallsBackStatic(t *testing.T) {
	_, err := Run(context.Background(), t.TempDir(), Input{Selector: "bad"}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "expected <domain>/<topic>") {
		t.Fatalf("syntax error = %v", err)
	}
	detail, err := Run(context.Background(), t.TempDir(), Input{Selector: "domain/topic"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	document, err := detail.Document()
	var out bytes.Buffer
	if err == nil {
		err = presentation.Render(&out, document)
	}
	if err != nil || !strings.Contains(out.String(), "static not inside an awf project") {
		t.Fatalf("static detail = %q, %v", out.String(), err)
	}
}
