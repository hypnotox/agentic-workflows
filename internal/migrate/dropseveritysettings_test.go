package migrate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// applyDropSeveritySettings is the schema 24 -> 25 migration. Both in-repo trees
// set the two keys, so the upgrade run exercises the happy path, but only these
// fixtures pin the sibling-preservation and per-key announcement contract.
// invariant: config/migrations-and-locks:severity-keys-dropped
func TestApplyDropSeveritySettings(t *testing.T) {
	// The sibling block is a full currentState: the migration must leave sources,
	// testGlobs, and both maxima byte-identical while removing only the two ranks.
	const siblings = "currentState:\n  sources:\n    - globs: ['**/*.md']\n      marker: '#'\n" +
		"  testGlobs:\n    - '**/*_test.go'\n  maxTopicsPerPath: 8\n  maxClaimsPerTopic: 20\n"
	cases := []struct {
		name, src, want string
		wantKeys        []string
		wantSeed        bool
	}{
		{
			name: "removes both keys, keeps every sibling",
			src: "prefix: ex\ncurrentState:\n  sources:\n    - globs: ['**/*.md']\n      marker: '#'\n" +
				"  testGlobs:\n    - '**/*_test.go'\n  topicCoverage: error\n  topicFanout: warn\n" +
				"  maxTopicsPerPath: 8\n  maxClaimsPerTopic: 20\n",
			want:     "prefix: ex\n" + siblings,
			wantKeys: []string{"topicCoverage", "topicFanout"},
		},
		{
			name:     "announces only the key actually present",
			src:      "prefix: ex\ncurrentState:\n  topicFanout: off\n  maxTopicsPerPath: 8\n",
			want:     "prefix: ex\ncurrentState:\n  maxTopicsPerPath: 8\n",
			wantKeys: []string{"topicFanout"},
		},
		{
			name: "a tree carrying neither key is a clean no-op",
			src:  "prefix: ex\n" + siblings,
			want: "prefix: ex\n" + siblings,
		},
		{
			name: "no currentState block at all is a clean no-op",
			src:  "prefix: ex\nskills: []\n",
			want: "prefix: ex\nskills: []\n",
		},
		{
			// The removals would empty the block. At generation 25 an absent
			// block suppressed BOTH checks (ADR-0183 item 1), so the explicit
			// default budget was seeded instead of letting the block collapse.
			// ADR-0192 removed that gate, so the seed is now inert but harmless;
			// the migration is frozen and keeps writing it.
			name:     "sole children seed the default budget rather than collapsing",
			src:      "prefix: ex\ncurrentState:\n  topicCoverage: warn\n  topicFanout: off\nskills: []\n",
			want:     "prefix: ex\ncurrentState:\n  maxTopicsPerPath: 8\nskills: []\n",
			wantKeys: []string{"topicCoverage", "topicFanout"},
			wantSeed: true,
		},
		{
			name:     "a lone surviving sibling needs no seed",
			src:      "prefix: ex\ncurrentState:\n  topicCoverage: warn\n  maxClaimsPerTopic: 5\n",
			want:     "prefix: ex\ncurrentState:\n  maxClaimsPerTopic: 5\n",
			wantKeys: []string{"topicCoverage"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			p := filepath.Join(root, ".awf", "config.yaml")
			testsupport.WriteFile(t, p, tc.src)
			var out bytes.Buffer
			if err := applyDropSeveritySettings(root, &out); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(p)
			if err != nil { // coverage-ignore: the migration just wrote this path
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("config:\ngot  %q\nwant %q", got, tc.want)
			}
			// One announcement per key actually removed, and none for a key that
			// was already absent.
			for _, key := range []string{"topicCoverage", "topicFanout"} {
				line := "drop-severity-settings: removed currentState." + key + "\n"
				want := strings.Contains(strings.Join(tc.wantKeys, ","), key)
				if got := strings.Contains(out.String(), line); got != want {
					t.Errorf("announcement for %s = %v, want %v (output %q)", key, got, want, out.String())
				}
			}
			seedLine := "drop-severity-settings: set currentState.maxTopicsPerPath to 8"
			if got := strings.Contains(out.String(), seedLine); got != tc.wantSeed {
				t.Errorf("seed announcement = %v, want %v (output %q)", got, tc.wantSeed, out.String())
			}
			wantLines := len(tc.wantKeys)
			if tc.wantSeed {
				wantLines++
			}
			if n := strings.Count(out.String(), "\n"); n != wantLines {
				t.Errorf("announced %d lines, want %d: %q", n, wantLines, out.String())
			}
			// A replay must neither change the file again nor re-announce.
			var second bytes.Buffer
			if err := applyDropSeveritySettings(root, &second); err != nil {
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

// A non-canonically-formatted source: the round-trip re-indents surviving keys,
// so the claim promises value preservation rather than byte preservation
// (ADR-0185). Asserting the surviving keys and values by content rather than by
// exact bytes is what makes that clause falsifiable: a migration that dropped or
// altered a sibling value would fail here, while a pure re-indent does not.
// invariant: config/migrations-and-locks:severity-keys-dropped
func TestApplyDropSeveritySettingsPreservesValuesNotLayout(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, ".awf", "config.yaml")
	testsupport.WriteFile(t, p, "prefix: ex\ncurrentState:\n    topicCoverage: error\n"+
		"    testGlobs: ['**/*_test.go']\n    maxTopicsPerPath:   8\n    maxClaimsPerTopic: 20\n")
	if err := applyDropSeveritySettings(root, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if err != nil { // coverage-ignore: the migration just wrote this path
		t.Fatal(err)
	}
	cfg, err := config.Parse(filepath.Join(root, ".awf"), got)
	if err != nil {
		t.Fatalf("migrated config must parse: %v", err)
	}
	if cfg.CurrentState == nil {
		t.Fatal("currentState block must survive: three siblings remain")
	}
	if n := len(cfg.CurrentState.TestGlobs); n != 1 || cfg.CurrentState.TestGlobs[0] != "**/*_test.go" {
		t.Errorf("testGlobs = %#v, want the configured value intact", cfg.CurrentState.TestGlobs)
	}
	if cfg.CurrentState.MaxTopicsPerPath == nil || *cfg.CurrentState.MaxTopicsPerPath != 8 {
		t.Errorf("maxTopicsPerPath = %v, want the configured 8 intact", cfg.CurrentState.MaxTopicsPerPath)
	}
	if cfg.CurrentState.MaxClaimsPerTopic == nil || *cfg.CurrentState.MaxClaimsPerTopic != 20 {
		t.Errorf("maxClaimsPerTopic = %v, want the configured 20 intact", cfg.CurrentState.MaxClaimsPerTopic)
	}
	if cfg.Prefix != "ex" {
		t.Errorf("prefix = %q, want the configured value intact", cfg.Prefix)
	}
}

func TestApplyDropSeveritySettingsNoConfig(t *testing.T) {
	var out bytes.Buffer
	if err := applyDropSeveritySettings(t.TempDir(), &out); err != nil {
		t.Fatalf("an absent config.yaml must be a no-op, got %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("nothing to remove must announce nothing, got %q", out.String())
	}
}

func TestApplyDropSeveritySettingsMalformedConfig(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "currentState: [a, b\n")
	if err := applyDropSeveritySettings(root, &bytes.Buffer{}); err == nil {
		t.Fatal("a malformed config must surface the parse error, not be swallowed")
	}
}

// A committed config at generation 24 is forward-ported in memory before the
// strict parser sees it, so the staged check can compare a historical HEAD that
// still carries both keys against a migrated index. Without the byte-level
// branch in ConfigForCurrentSchema the removal would make every staged check
// fail to parse HEAD until the removing commit itself aged out of the diff.
func TestConfigForCurrentSchemaDropsHistoricalSeveritySettings(t *testing.T) {
	src := []byte("prefix: example\ncurrentState:\n  topicCoverage: error\n  topicFanout: warn\n  maxTopicsPerPath: 8\n")
	got, err := ConfigForCurrentSchema(src, 24)
	if err != nil {
		t.Fatal(err)
	}
	want := "prefix: example\ncurrentState:\n  maxTopicsPerPath: 8\n"
	if string(got) != want {
		t.Fatalf("forward-ported config:\ngot  %q\nwant %q", got, want)
	}
	// Already-migrated bytes are unchanged, and a generation at or past 25 skips
	// the branch entirely.
	again, err := ConfigForCurrentSchema(got, 24)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != want {
		t.Fatalf("not idempotent: %q", again)
	}
	at25, err := ConfigForCurrentSchema(src, 25)
	if err != nil {
		t.Fatal(err)
	}
	if string(at25) != string(src) {
		t.Fatalf("generation 25 must not re-apply the removal, got %q", at25)
	}
}

func TestConfigForCurrentSchemaRefusesMalformedSeverityYAML(t *testing.T) {
	if _, err := ConfigForCurrentSchema([]byte("prefix: [\ncurrentState:\n  topicCoverage: error\n"), 24); err == nil {
		t.Fatal("malformed YAML must surface the parse error, not be swallowed")
	}
}
