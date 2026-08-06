package migrate

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const groundingBackfillChange = "grounding-skill-backfill: enabled skill grounding (standard brainstorming is enabled)"

// invariant: config/migrations-and-locks:grounding-skill-backfill (TestGroundingSkillBackfill)
func TestGroundingSkillBackfill(t *testing.T) {
	for _, tc := range []struct {
		name      string
		config    string
		files     map[string]string
		wantAdd   bool
		collision bool
	}{
		{"standard brainstorming", "prefix: ex\nintegrationBranch: main\nskills: [brainstorming]\nagents: [grounding-checker]\n", nil, true, false},
		{"local brainstorming", "prefix: ex\nintegrationBranch: main\nskills: [brainstorming]\nagents: [grounding-checker]\n", map[string]string{"skills/brainstorming.yaml": "local: true\n"}, false, false},
		{"already selected", "prefix: ex\nintegrationBranch: main\nskills: [brainstorming, grounding]\nagents: [grounding-checker]\n", nil, false, false},
		{"local collision", "prefix: ex\nintegrationBranch: main\nskills: [brainstorming]\nagents: [grounding-checker]\n", map[string]string{"skills/grounding.yaml": "local: true\n"}, false, true},
		{"enabled local collision", "prefix: ex\nintegrationBranch: main\nskills: [brainstorming, grounding]\nagents: [grounding-checker]\n", map[string]string{"skills/grounding.yaml": "local: true\n"}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := closeFixture(t, tc.config, tc.files)
			configPath := filepath.Join(root, ".awf", "config.yaml")
			before := mustRead(t, configPath)
			sidecarsBefore := readFixtureFiles(t, root, tc.files)
			var changes Changes
			migrationErr := applyGroundingSkillBackfill(root, &changes)
			var collision *GroundingSkillCollisionError
			if errors.As(migrationErr, &collision) != tc.collision {
				t.Fatalf("collision error = %v, want collision %v", migrationErr, tc.collision)
			}
			after := mustRead(t, configPath)
			if tc.wantAdd {
				cfg, err := loadForMigration(root)
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Contains(cfg.Skills, "grounding") {
					t.Fatalf("skills = %#v, want exact grounding member", cfg.Skills)
				}
				items := changes.Items()
				if len(items) != 1 || items[0].Text != groundingBackfillChange {
					t.Fatalf("changes = %#v", items)
				}
			} else {
				if !bytes.Equal(before, after) {
					t.Fatalf("no-op or refusal changed config\nbefore: %s\nafter: %s", before, after)
				}
				if len(changes.Items()) != 0 {
					t.Fatalf("no-op or refusal announced changes: %#v", changes.Items())
				}
			}
			assertFixtureFiles(t, root, sidecarsBefore)
			if tc.collision && collision.Path != "skills/grounding.yaml" {
				t.Errorf("collision path = %q", collision.Path)
			}
			if migrationErr == nil {
				var again Changes
				if err := applyGroundingSkillBackfill(root, &again); err != nil {
					t.Fatal(err)
				}
				if len(again.Items()) != 0 || !bytes.Equal(after, mustRead(t, configPath)) {
					t.Error("rerun is not silent and byte-identical")
				}
			}
		})
	}
}

func TestGroundingSkillBackfillFailuresDoNotMutate(t *testing.T) {
	t.Run("absent config", func(t *testing.T) {
		if err := applyGroundingSkillBackfill(t.TempDir(), &Changes{}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("stat failure", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "not-a-directory")
		if err := os.WriteFile(root, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := applyGroundingSkillBackfill(root, &Changes{}); err == nil {
			t.Fatal("expected stat failure")
		}
	})
	for _, tc := range []struct {
		name   string
		config string
		files  map[string]string
	}{
		{"malformed config", "prefix: [\n", nil},
		{"malformed brainstorming sidecar", "prefix: ex\nintegrationBranch: main\nskills: [brainstorming]\nagents: [grounding-checker]\n", map[string]string{"skills/brainstorming.yaml": "local: [\n"}},
		{"malformed grounding sidecar", "prefix: ex\nintegrationBranch: main\nskills: [brainstorming]\nagents: [grounding-checker]\n", map[string]string{"skills/grounding.yaml": "local: [\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := closeFixture(t, tc.config, tc.files)
			path := filepath.Join(root, ".awf", "config.yaml")
			before := mustRead(t, path)
			sidecars := readFixtureFiles(t, root, tc.files)
			var changes Changes
			if err := applyGroundingSkillBackfill(root, &changes); err == nil {
				t.Fatal("expected failure")
			}
			if !bytes.Equal(before, mustRead(t, path)) || len(changes.Items()) != 0 {
				t.Fatal("failure mutated config or announced success")
			}
			assertFixtureFiles(t, root, sidecars)
		})
	}
	t.Run("atomic write failure", func(t *testing.T) {
		root := closeFixture(t, "prefix: ex\nintegrationBranch: main\nskills: [brainstorming]\nagents: [grounding-checker]\n", nil)
		path := filepath.Join(root, ".awf", "config.yaml")
		before := mustRead(t, path)
		boom := errors.New("write failed")
		var changes Changes
		err := applyGroundingSkillBackfillWith(root, &changes, configEditor{writeAtomic: func(string, []byte) error { return boom }})
		if !errors.Is(err, boom) || !bytes.Equal(before, mustRead(t, path)) || len(changes.Items()) != 0 {
			t.Fatalf("write failure = %v, changes = %#v", err, changes.Items())
		}
	})
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
		if diagnostic.State != "operation" || !strings.Contains(diagnostic.Condition, err.Path) || diagnostic.Cause != "" {
			t.Fatalf("diagnostic = %#v", diagnostic)
		}
		if len(diagnostic.Changed) != len(changes) || len(diagnostic.Steps) != len(changes)+2 {
			t.Fatalf("diagnostic = %#v", diagnostic)
		}
	}
	if _, diagnosticErr := err.Diagnostic([]Change{{Text: "\n"}}); diagnosticErr == nil {
		t.Fatal("invalid prior change must not be silently discarded")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func readFixtureFiles(t *testing.T, root string, files map[string]string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for name := range files {
		out[name] = mustRead(t, filepath.Join(root, ".awf", name))
	}
	return out
}

func assertFixtureFiles(t *testing.T, root string, want map[string][]byte) {
	t.Helper()
	for name, before := range want {
		if after := mustRead(t, filepath.Join(root, ".awf", name)); !bytes.Equal(before, after) {
			t.Errorf("sidecar %s changed\nbefore: %s\nafter: %s", name, before, after)
		}
	}
}
