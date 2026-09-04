package publisher

import (
	"errors"
	"strings"
	"testing"
)

func TestPublisherRefusesASecondPublicationAttempt(t *testing.T) {
	operation := &Publisher{}
	if err := operation.beginMutation(); err != nil {
		t.Fatal(err)
	}
	if err := operation.beginMutation(); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("second mutation error = %v", err)
	}
}

func TestResultDefensivelyProjectsCommittedMutations(t *testing.T) {
	result := newResult([]Backup{{Path: "one", Bak: "one.awf-bak"}}, []Change{{Path: "two", Cause: "added"}}, []string{"three"}, []Effect{{Kind: "replace", Path: "two", Recovery: "rerun"}})
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

func TestInvalidSkillArtifactDiagnostic(t *testing.T) {
	cause := errors.New("missing frontmatter")
	err := invalidSkillArtifact(".pi/skills/awf-effort/SKILL.md", cause)
	if got, want := err.Error(), "invalid skill artifact in .pi/skills/awf-effort/SKILL.md: missing frontmatter"; got != want {
		t.Fatalf("diagnostic = %q, want %q", got, want)
	}
	if !errors.Is(err, cause) {
		t.Fatal("diagnostic does not retain cause")
	}
}

func TestValidatePublicationArtifactRefusals(t *testing.T) {
	for _, content := range []string{
		"---\nname: [\n---\n",
		"plain text",
		"---\nname: \"\"\ndescription: description\n---\n",
		"---\nname: name\ndescription: \"\"\n---\n",
	} {
		if err := validatePublicationArtifact([]byte(content)); err == nil {
			t.Fatalf("accepted invalid publication artifact %q", content)
		}
	}
}
