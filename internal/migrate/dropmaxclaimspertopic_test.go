package migrate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// applyDropMaxClaimsPerTopic is the schema 27 -> 28 migration. Both in-repo trees
// carry a surviving sibling, so only these fixtures pin the case that separates
// this migration from the generation-25 removal it is modelled on: when the
// retired key is the block's only child the block is dropped outright rather than
// seeded (ADR-0194, safe because of ADR-0192).
// invariant: config/migrations-and-locks:claim-budget-key-dropped (TestApplyDropMaxClaimsPerTopic)
func TestApplyDropMaxClaimsPerTopic(t *testing.T) {
	const announcement = "drop-max-claims-per-topic: removed currentState.maxClaimsPerTopic\n"
	for _, tc := range []struct {
		name, src, want string
		wantAnnounce    bool
	}{
		{
			name:         "removes the key, keeps every sibling",
			src:          "prefix: ex\ncurrentState:\n  testGlobs:\n    - '**/*_test.go'\n  maxTopicsPerPath: 8\n  maxClaimsPerTopic: 20\n",
			want:         "prefix: ex\ncurrentState:\n  testGlobs:\n    - '**/*_test.go'\n  maxTopicsPerPath: 8\n",
			wantAnnounce: true,
		},
		{
			// The departure from generation 25: no seed, so the emptied block
			// goes with the key. ADR-0192 made block presence behaviourally
			// meaningless, so nothing is lost by letting it collapse.
			name:         "the sole child takes the block with it, unseeded",
			src:          "prefix: ex\ncurrentState:\n  maxClaimsPerTopic: 20\nskills: []\n",
			want:         "prefix: ex\nskills: []\n",
			wantAnnounce: true,
		},
		{
			name: "a tree without the key is a clean no-op",
			src:  "prefix: ex\ncurrentState:\n  maxTopicsPerPath: 8\n",
			want: "prefix: ex\ncurrentState:\n  maxTopicsPerPath: 8\n",
		},
		{
			name: "no currentState block at all is a clean no-op",
			src:  "prefix: ex\nskills: []\n",
			want: "prefix: ex\nskills: []\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			p := filepath.Join(root, ".awf", "config.yaml")
			testsupport.WriteFile(t, p, tc.src)
			var out bytes.Buffer
			if err := applyDropMaxClaimsPerTopic(root, &out); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(p)
			if err != nil { // coverage-ignore: the migration just wrote this path
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("config:\ngot  %q\nwant %q", got, tc.want)
			}
			if strings.Contains(string(got), "maxClaimsPerTopic") {
				t.Errorf("retired key survived: %q", got)
			}
			if announced := out.String() == announcement; announced != tc.wantAnnounce {
				t.Errorf("announcement = %v, want %v (output %q)", announced, tc.wantAnnounce, out.String())
			}
			// A replay must neither change the file again nor re-announce.
			var second bytes.Buffer
			if err := applyDropMaxClaimsPerTopic(root, &second); err != nil {
				t.Fatal(err)
			}
			again, rerr := os.ReadFile(p)
			if rerr != nil { // coverage-ignore: the path was read successfully a moment ago
				t.Fatal(rerr)
			}
			if string(again) != string(got) {
				t.Errorf("not idempotent: %q then %q", got, again)
			}
			if second.Len() != 0 {
				t.Errorf("replay re-announced: %q", second.String())
			}
		})
	}
}

func TestApplyDropMaxClaimsPerTopicRefusesMalformedYAML(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "prefix: [\ncurrentState:\n  maxClaimsPerTopic: 20\n")
	var out bytes.Buffer
	if err := applyDropMaxClaimsPerTopic(root, &out); err == nil {
		t.Fatal("malformed YAML must surface the parse error, not be swallowed")
	}
	if out.Len() != 0 {
		t.Errorf("a failed migration must announce nothing, got %q", out.String())
	}
}

// The forward-port branch is a separate function from the migration: awf check
// --staged reads the before-side config at its committed generation and ports it
// through ConfigForCurrentSchema so the current strict parser can read it. Without
// this branch the retired key survives and parsing fails on the very commit that
// removes it.
func TestConfigForCurrentSchemaDropsHistoricalMaxClaimsPerTopic(t *testing.T) {
	src := []byte("prefix: example\ncurrentState:\n  maxTopicsPerPath: 8\n  maxClaimsPerTopic: 20\n")
	got, err := ConfigForCurrentSchema(src, 27)
	if err != nil {
		t.Fatal(err)
	}
	want := "prefix: example\ncurrentState:\n  maxTopicsPerPath: 8\n"
	if string(got) != want {
		t.Fatalf("forward-ported config:\ngot  %q\nwant %q", got, want)
	}
	// Already-migrated bytes are unchanged, and a generation at 28 skips the
	// branch entirely.
	again, err := ConfigForCurrentSchema(got, 27)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != want {
		t.Fatalf("not idempotent: %q", again)
	}
	at28, err := ConfigForCurrentSchema(src, 28)
	if err != nil {
		t.Fatal(err)
	}
	if string(at28) != string(src) {
		t.Fatalf("generation 28 must not re-apply the removal, got %q", at28)
	}
}

func TestConfigForCurrentSchemaRefusesMalformedMaxClaimsYAML(t *testing.T) {
	if _, err := ConfigForCurrentSchema([]byte("prefix: [\ncurrentState:\n  maxClaimsPerTopic: 20\n"), 27); err == nil {
		t.Fatal("malformed YAML must surface the parse error, not be swallowed")
	}
}

func TestDropMaxClaimsPerTopicRegistered(t *testing.T) {
	if Current() != 28 {
		t.Fatalf("current generation = %d, want 28", Current())
	}
	if last := registry[len(registry)-1]; last.Name != "drop-max-claims-per-topic" {
		t.Fatalf("last registry entry = %q, want drop-max-claims-per-topic", last.Name)
	}
}
