package migrate

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// invariant: config/migrations-and-locks:grounding-skill-backfill (TestGroundingSkillBackfill)
func TestGroundingSkillBackfill(t *testing.T) {
	for _, tc := range []struct {
		name      string
		config    string
		files     map[string]string
		want      string
		collision bool
	}{
		{"standard brainstorming", "prefix: ex\nintegrationBranch: main\nskills: [brainstorming]\nagents: [grounding-checker]\n", nil, "grounding", false},
		{"local brainstorming", "prefix: ex\nintegrationBranch: main\nskills: [brainstorming]\nagents: [grounding-checker]\n", map[string]string{"skills/brainstorming.yaml": "local: true\n"}, "", false},
		{"already selected", "prefix: ex\nintegrationBranch: main\nskills: [brainstorming, grounding]\nagents: [grounding-checker]\n", nil, "", false},
		{"local collision", "prefix: ex\nintegrationBranch: main\nskills: [brainstorming]\nagents: [grounding-checker]\n", map[string]string{"skills/grounding.yaml": "local: true\n"}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := closeFixture(t, tc.config, tc.files)
			before, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			var changes Changes
			migrationErr := applyGroundingSkillBackfill(root, &changes)
			var collision *GroundingSkillCollisionError
			if errors.As(migrationErr, &collision) != tc.collision {
				t.Fatalf("collision error = %v, want %v", err, tc.collision)
			}
			after, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" && !bytes.Equal(before, after) {
				t.Errorf("no-op or refusal changed config\n%s", after)
			}
			if tc.want != "" && !bytes.Contains(after, []byte(tc.want)) {
				t.Errorf("config missing %q: %s", tc.want, after)
			}
			if tc.collision && collision.Path != "skills/grounding.yaml" {
				t.Errorf("collision path = %q", collision.Path)
			}
			var again Changes
			if migrationErr == nil {
				if err := applyGroundingSkillBackfill(root, &again); err != nil {
					t.Fatal(err)
				}
				final, _ := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
				if len(again.Items()) != 0 || !bytes.Equal(after, final) {
					t.Error("rerun is not silent and byte-identical")
				}
			}
		})
	}
}

func TestGroundingSkillBackfillNoConfig(t *testing.T) {
	if err := applyGroundingSkillBackfill(t.TempDir(), &Changes{}); err != nil {
		t.Fatal(err)
	}
}

func TestGroundingSkillCollisionDiagnostic(t *testing.T) {
	err := &GroundingSkillCollisionError{Path: "skills/grounding.yaml"}
	if got := err.Error(); got != "project-local grounding occupies standard skill name: skills/grounding.yaml" {
		t.Fatalf("Error() = %q", got)
	}
	for _, changes := range [][]Change{nil, {{Text: "earlier migration changed config"}}} {
		diagnostic, diagnosticErr := err.Diagnostic(changes)
		if diagnosticErr != nil {
			t.Fatal(diagnosticErr)
		}
		if diagnostic.State != "operation" || diagnostic.Condition != "project-local grounding occupies the standard name" {
			t.Fatalf("diagnostic = %#v", diagnostic)
		}
		if len(diagnostic.Steps) != len(changes)+2 {
			t.Fatalf("steps = %#v", diagnostic.Steps)
		}
	}
}
