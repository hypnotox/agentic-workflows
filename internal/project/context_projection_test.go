package project

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// invariant: tooling/cli:context-full-authority-packet
func TestContextConciseAndFullProjectionBoundaries(t *testing.T) {
	files := ctxFiles()
	files["internal/foo/y.go"] = "package foo\n// touches-state: alpha/one:stable - direct implementation\n// touches-state: alpha/one:order - direct ordering\n// touches-state: alpha/one:aaa - direct early claim\n"
	part := strings.Replace(files[".awf/topics/parts/alpha/one/current-state.md"], "Origin: ADR-0001\nBacking: unbacked", "Origin: ADR-0001\nReferences: alpha/one:order\nBacking: unbacked", 1)
	files[".awf/topics/parts/alpha/one/current-state.md"] = part + "\n### `rule: aaa`\nEarly alphabetic claim.\nOrigin: ADR-0001\n"
	p := csRepo(t, ctxConfig, files)
	concise, err := p.ContextFor([]string{"internal/foo/y.go"})
	if err != nil {
		t.Fatal(err)
	}
	full, err := p.ContextForFull([]string{"internal/foo/y.go"})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := p.ContextForFullGitSelection([]string{"internal/foo/y.go"})
	if err != nil || selected.Requests[0].Status != RequestGitSelected || selected.Projection != ContextFull {
		t.Fatalf("full Git selection = %#v, %v", selected, err)
	}
	if concise.Projection != ContextConcise || full.Projection != ContextFull {
		t.Fatalf("projections = %q, %q", concise.Projection, full.Projection)
	}
	conciseTopic := concise.Paths[0].Topics[0]
	fullTopic := full.Paths[0].Topics[0]
	if len(conciseTopic.DirectClaims) != 3 || conciseTopic.DirectClaims[0].ID != "alpha/one:aaa" || conciseTopic.DirectClaims[1].ID != "alpha/one:order" || conciseTopic.DirectClaims[2].ID != "alpha/one:stable" || conciseTopic.OmittedClaimCount != 1 || conciseTopic.Full != nil {
		t.Fatalf("concise topic = %#v", conciseTopic)
	}
	if len(conciseTopic.DirectClaims[2].References.Incoming) != 0 || len(conciseTopic.DirectClaims[2].References.Outgoing) != 0 {
		t.Fatalf("concise references leaked: %#v", conciseTopic.DirectClaims[2].References)
	}
	if fullTopic.Full == nil || len(fullTopic.Full.Claims) != 4 || len(fullTopic.DirectClaims) != 3 {
		t.Fatalf("full topic = %#v", fullTopic)
	}
	encoded, err := json.Marshal(concise)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"full"`) {
		t.Fatalf("concise JSON contains full key: %s", encoded)
	}
	for _, claim := range fullTopic.Full.Claims {
		if claim.Sites == nil || claim.References.Incoming == nil || claim.References.Outgoing == nil {
			t.Fatalf("full claim has nil collections: %#v", claim)
		}
	}
	if got := nonNilStrings(nil); got == nil || len(got) != 0 {
		t.Fatalf("nil string projection = %#v", got)
	}
	if got := strings.Join(nonNilStrings([]string{"b", "a", "a"}), ","); got != "a,b" {
		t.Fatalf("sorted string projection = %q", got)
	}
	history := &topic.ClaimHistory{RemovedBy: &topic.ADRHistory{Number: "0002"}}
	if got := claimStateForOperation("update", "alpha/one:gone", "applied", topic.Corpus{}, history); got != "historically-removed" {
		t.Fatalf("removed state = %q", got)
	}
	if got := claimStateForOperation("remove", "alpha/one:gone", "applied", topic.Corpus{}, nil); got != "historically-removed" {
		t.Fatalf("applied remove state = %q", got)
	}
}
