package migrate

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The backfill enables orienting exactly where brainstorming is enabled and
// orienting is not, announces the addition, and leaves every other config
// byte-identical; a re-run is a no-op.
// invariant: config/migrations-and-locks:orienting-skill-backfill
func TestOrientingBackfill(t *testing.T) {
	const announce = "orienting-skill-backfill: enabled skill orienting (brainstorming is enabled)\n"
	for _, tc := range []struct {
		name          string
		cfg           string
		wantOutput    string
		wantOrienting bool
	}{
		{
			name:          "brainstorming without orienting gains it",
			cfg:           "prefix: ex\nskills: [brainstorming]\nagents: [grounding-checker]\n",
			wantOutput:    announce,
			wantOrienting: true,
		},
		{
			name:          "both already enabled is untouched",
			cfg:           "prefix: ex\nskills: [brainstorming, orienting]\nagents: [grounding-checker]\n",
			wantOutput:    "",
			wantOrienting: true,
		},
		{
			name:          "no brainstorming is untouched",
			cfg:           "prefix: ex\nskills: [tdd]\nagents: []\n",
			wantOutput:    "",
			wantOrienting: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := closeFixture(t, tc.cfg, nil)
			var out bytes.Buffer
			if err := applyOrientingSkillBackfill(root, &out); err != nil {
				t.Fatalf("apply: %v", err)
			}
			if out.String() != tc.wantOutput {
				t.Errorf("output = %q, want %q", out.String(), tc.wantOutput)
			}
			cfg, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantOutput == "" && string(cfg) != tc.cfg {
				t.Errorf("no-op case changed the config:\n%s", cfg)
			}
			gotOrienting := strings.Contains(string(cfg), "orienting")
			if gotOrienting != tc.wantOrienting {
				t.Errorf("orienting present = %v, want %v:\n%s", gotOrienting, tc.wantOrienting, cfg)
			}
			// Idempotence: a second run prints nothing and changes nothing.
			var second bytes.Buffer
			if err := applyOrientingSkillBackfill(root, &second); err != nil {
				t.Fatalf("re-apply: %v", err)
			}
			again, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if second.Len() != 0 || !bytes.Equal(cfg, again) {
				t.Errorf("re-run must be a silent byte-identical no-op: output=%q config=\n%s", second.String(), again)
			}
		})
	}
}

// An absent config is a no-op (idempotent re-run safe, the editConfig skeleton).
func TestOrientingBackfillNoConfigNoop(t *testing.T) {
	if err := applyOrientingSkillBackfill(t.TempDir(), io.Discard); err != nil {
		t.Fatalf("absent config must be a no-op, got %v", err)
	}
}

// A malformed config surfaces the load error rather than mutating anything.
func TestOrientingBackfillMalformedConfig(t *testing.T) {
	root := closeFixture(t, ": : not valid : :\n", nil)
	if err := applyOrientingSkillBackfill(root, io.Discard); err == nil {
		t.Fatal("expected a parse error for a malformed config")
	}
	cfg, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(cfg) != ": : not valid : :\n" {
		t.Errorf("malformed config must not be mutated:\n%s", cfg)
	}
}
