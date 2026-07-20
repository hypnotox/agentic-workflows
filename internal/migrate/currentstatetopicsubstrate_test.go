package migrate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// applyCurrentStateTopicSubstrate is the schema 13 -> 14 migration: it retires the
// top-level `invariants` config block the current strict config.Config no longer
// accepts. This repo's config carries no such block, so these fixtures are its
// direct coverage.
func TestApplyCurrentStateTopicSubstrate(t *testing.T) {
	cases := []struct {
		name, src, want string
		wantMsg         bool
	}{
		{
			name:    "removes the block, keeps siblings",
			src:     "prefix: ex\ninvariants:\n  sources:\n    - globs: ['*.go']\n      marker: '//'\nskills: []\n",
			want:    "prefix: ex\nskills: []\n",
			wantMsg: true,
		},
		{
			name: "no invariants block is a silent no-op",
			src:  "prefix: ex\nskills: []\n",
			want: "prefix: ex\nskills: []\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			cfgPath := filepath.Join(root, ".awf", "config.yaml")
			testsupport.WriteFile(t, cfgPath, tc.src)
			var out bytes.Buffer
			if err := applyCurrentStateTopicSubstrate(root, &out); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(cfgPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("config = %q, want %q", got, tc.want)
			}
			if msg := strings.Contains(out.String(), "removed the retired top-level invariants block"); msg != tc.wantMsg {
				t.Errorf("announce = %v, want %v (output %q)", msg, tc.wantMsg, out.String())
			}
			// Idempotent: re-running removes nothing and announces nothing.
			var second bytes.Buffer
			if err := applyCurrentStateTopicSubstrate(root, &second); err != nil {
				t.Fatal(err)
			}
			if second.Len() != 0 {
				t.Errorf("replay re-announced: %q", second.String())
			}
		})
	}
}

func TestApplyCurrentStateTopicSubstrateNoConfig(t *testing.T) {
	var out bytes.Buffer
	if err := applyCurrentStateTopicSubstrate(t.TempDir(), &out); err != nil {
		t.Fatalf("an absent config.yaml must be a no-op, got %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("nothing to remove must announce nothing, got %q", out.String())
	}
}

func TestApplyCurrentStateTopicSubstrateMalformedConfig(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "invariants: [a, b\n")
	if err := applyCurrentStateTopicSubstrate(root, &bytes.Buffer{}); err == nil {
		t.Fatal("a malformed config must surface the parse error, not be swallowed")
	}
}
