package migrate

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The migration materializes the required-explicit key visibly, leaves a config
// that already carries one byte-identical, and announces exactly one line. The
// claim names the generation as well as the behaviour, so the marked test pins
// the registry entry too.
// invariant: config/configuration:integration-branch-explicit (TestIntegrationBranchMigration)
func TestIntegrationBranchMigration(t *testing.T) {
	const announce = "integration-branch-explicit: set integrationBranch: main\n"
	var found *Migration
	for i := range registry {
		if registry[i].To == integrationBranchGeneration {
			found = &registry[i]
		}
	}
	if found == nil || found.Name != "integration-branch-explicit" || found.OwnsSchemaStamp {
		t.Fatalf("generation %d migration = %+v, want name integration-branch-explicit without a schema stamp", integrationBranchGeneration, found)
	}
	for _, tc := range []struct {
		name       string
		cfg        string
		wantOutput string
		wantValue  string
	}{
		{
			name:       "absent key is written visibly",
			cfg:        "prefix: ex\nskills: [tdd]\nagents: []\n",
			wantOutput: announce,
			wantValue:  "integrationBranch: main",
		},
		{
			name:       "a present key is left alone",
			cfg:        "prefix: ex\nintegrationBranch: trunk\nskills: [tdd]\nagents: []\n",
			wantOutput: "",
			wantValue:  "integrationBranch: trunk",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := closeFixture(t, tc.cfg, nil)
			var out bytes.Buffer
			if err := applyIntegrationBranch(root, &out); err != nil {
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
			if !strings.Contains(string(cfg), tc.wantValue) {
				t.Errorf("config missing %q:\n%s", tc.wantValue, cfg)
			}
			// Idempotence: a second run prints nothing and changes nothing.
			var second bytes.Buffer
			if err := applyIntegrationBranch(root, &second); err != nil {
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

// A historical config below this generation is ported forward with the key
// materialized. Without this branch every staged check would fail to load HEAD
// with "integrationBranch must not be empty" until the introducing commit aged
// out of the diff, because the key is required and has no in-code default.
// invariant: config/configuration:integration-branch-explicit (TestConfigForCurrentSchemaSeedsHistoricalIntegrationBranch)
func TestConfigForCurrentSchemaSeedsHistoricalIntegrationBranch(t *testing.T) {
	src := []byte("prefix: example\nskills: []\n")
	got, err := ConfigForCurrentSchema(src, integrationBranchGeneration-1)
	if err != nil {
		t.Fatal(err)
	}
	want := "prefix: example\nskills: []\nintegrationBranch: main\n"
	if string(got) != want {
		t.Fatalf("forward-ported config:\ngot  %q\nwant %q", got, want)
	}
	// Idempotent, and a config already naming a branch keeps its own value.
	again, err := ConfigForCurrentSchema(got, integrationBranchGeneration-1)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != want {
		t.Fatalf("not idempotent: %q", again)
	}
	own := []byte("prefix: example\nintegrationBranch: trunk\n")
	kept, err := ConfigForCurrentSchema(own, integrationBranchGeneration-1)
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != string(own) {
		t.Fatalf("a present branch must survive the port-forward, got %q", kept)
	}
	// At or past the generation the branch is skipped entirely.
	atGen, err := ConfigForCurrentSchema(src, integrationBranchGeneration)
	if err != nil {
		t.Fatal(err)
	}
	if string(atGen) != string(src) {
		t.Fatalf("generation %d must not re-apply the seed, got %q", integrationBranchGeneration, atGen)
	}
}

// Malformed YAML surfaces the parse error rather than being swallowed.
func TestConfigForCurrentSchemaRefusesMalformedIntegrationBranchYAML(t *testing.T) {
	if _, err := ConfigForCurrentSchema([]byte("prefix: [\n"), integrationBranchGeneration-1); err == nil {
		t.Fatal("malformed YAML must surface the parse error, not be swallowed")
	}
}

// An absent config is a no-op (idempotent re-run safe, the editConfig skeleton).
func TestIntegrationBranchNoConfigNoop(t *testing.T) {
	if err := applyIntegrationBranch(t.TempDir(), io.Discard); err != nil {
		t.Fatalf("absent config must be a no-op, got %v", err)
	}
}

// A malformed config surfaces the load error rather than mutating anything.
func TestIntegrationBranchMalformedConfig(t *testing.T) {
	root := closeFixture(t, ": : not valid : :\n", nil)
	if err := applyIntegrationBranch(root, io.Discard); err == nil {
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
