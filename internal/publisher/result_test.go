package publisher

import (
	"strings"
	"testing"
)

func TestPreparationPublicationRequiresPublisherBinding(t *testing.T) {
	unbound := Preparation{}
	if _, err := unbound.Sync(); err == nil || !strings.Contains(err.Error(), "unbound preparation") {
		t.Fatalf("unbound sync error = %v", err)
	}
	if _, err := unbound.Initialize(InitAuthority{}); err == nil || !strings.Contains(err.Error(), "unbound preparation") {
		t.Fatalf("unbound initialize error = %v", err)
	}
	if _, err := unbound.InitCollisions(); err == nil || !strings.Contains(err.Error(), "unbound preparation") {
		t.Fatalf("unbound collision error = %v", err)
	}
}

func TestResultDefensivelyProjectsCommittedMutations(t *testing.T) {
	result := newResult([]Backup{{Path: "one", Bak: "one.awf-bak"}}, []Change{{Path: "two", Cause: "added"}}, []string{"three"})
	backups := result.Backups()
	changes := result.Changes()
	pruned := result.Pruned()
	backups[0].Path = "mutated"
	changes[0].Path = "mutated"
	pruned[0] = "mutated"
	if got := result.Backups()[0].Path; got != "one" {
		t.Fatalf("backup projection mutated Result: %q", got)
	}
	if got := result.Changes()[0].Path; got != "two" {
		t.Fatalf("change projection mutated Result: %q", got)
	}
	if got := result.Pruned()[0]; got != "three" {
		t.Fatalf("prune projection mutated Result: %q", got)
	}
}

func TestValidatePublicationArtifactRefusals(t *testing.T) {
	for _, content := range []string{
		"---\nname: [\n---\n",
		"plain text",
		"---\nname: \"\"\ndescription: description\n---\n",
		"---\nname: name\ndescription: \"\"\n---\n",
	} {
		if err := validatePublicationArtifact([]byte(content), MarkdownAgentDialect); err == nil {
			t.Fatalf("accepted invalid publication artifact %q", content)
		}
	}
}
