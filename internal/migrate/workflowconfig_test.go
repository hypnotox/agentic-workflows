package migrate

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestRetireWorkflowConfigBytesRemovesProfileAndUnsetRetiredVars(t *testing.T) {
	source := []byte("# keep\nprefix: example\nprofile: full\nintegrationBranch: main\nvars:\n  gateCmd: make gate\n  gateCmdFull: \"\"\n  commitGateCmd: null\n  activeMdRegenCmd: '  '\n  invariantTestPath:\n")
	got, removed, err := retireWorkflowConfigBytes(source)
	if err != nil {
		t.Fatal(err)
	}
	wantRemoved := []string{"vars.activeMdRegenCmd", "vars.commitGateCmd", "vars.gateCmdFull", "vars.invariantTestPath", "profile"}
	if !slices.Equal(removed, wantRemoved) {
		t.Fatalf("removed=%v, want %v", removed, wantRemoved)
	}
	for _, forbidden := range []string{"profile:", "gateCmdFull:", "commitGateCmd:", "activeMdRegenCmd:", "invariantTestPath:"} {
		if bytes.Contains(got, []byte(forbidden)) {
			t.Fatalf("retired key %q remains:\n%s", forbidden, got)
		}
	}
	if !bytes.Contains(got, []byte("# keep")) || !bytes.Contains(got, []byte("gateCmd: make gate")) {
		t.Fatalf("retained config lost:\n%s", got)
	}
	again, removedAgain, err := retireWorkflowConfigBytes(got)
	if err != nil || len(removedAgain) != 0 || !bytes.Equal(again, got) {
		t.Fatalf("idempotence removed=%v err=%v\ngot:\n%s", removedAgain, err, again)
	}
}

func TestRetireWorkflowConfigBytesRefusesMeaningfulRetiredOverrides(t *testing.T) {
	for _, key := range retiredWorkflowVars {
		t.Run(key, func(t *testing.T) {
			source := []byte("prefix: example\nprofile: core\nintegrationBranch: main\nvars:\n  " + key + ": custom behavior\n")
			got, removed, err := retireWorkflowConfigBytes(source)
			if err == nil || !strings.Contains(err.Error(), "vars."+key+" has a meaningful retired override") {
				t.Fatalf("got=%q removed=%v err=%v, want actionable refusal", got, removed, err)
			}
			if got != nil || removed != nil {
				t.Fatalf("refusal planned mutation: got=%q removed=%v", got, removed)
			}
		})
	}
}

func TestRetireWorkflowConfigMigrationPreservesModeAndDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	path := config.ConfigPath(root)
	source := "prefix: example\nprofile: core\nintegrationBranch: main\nvars:\n  gateCmdFull: \"\"\n  gateCmd: make gate\n"
	writeLock(t, root, 48)
	testsupport.WriteFile(t, path, source)
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}

	applied, changes, mutations, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(applied, []string{retireWorkflowConfigName, retirePitfallRelationsName}) || len(changes) != 2 || len(mutations) != 1 {
		t.Fatalf("applied=%v changes=%v mutations=%#v", applied, changes, mutations)
	}
	mutation := mutations[0]
	if mutation.Path != ".awf/config.yaml" || mutation.Mode != 0o640 || strings.Contains(string(mutation.Content), "profile:") || strings.Contains(string(mutation.Content), "gateCmdFull:") {
		t.Fatalf("mutation=%#v", mutation)
	}
	got, err := os.ReadFile(filepath.Clean(path))
	if err != nil || string(got) != source {
		t.Fatalf("Build wrote config: %q, %v", got, err)
	}
}

func TestConfigBytesForGenerationChainsWorkflowConfigRetirement(t *testing.T) {
	source := []byte("prefix: example\nprofile: full\nintegrationBranch: main\ntags: {old: metadata}\nvars: {gateCmd: make gate, gateCmdFull: \"\"}\n")
	got, err := ConfigBytesForGeneration(LiveSchemaFloor, source)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"profile:", "tags:", "gateCmdFull:"} {
		if bytes.Contains(got, []byte(forbidden)) {
			t.Fatalf("retired key %q remains:\n%s", forbidden, got)
		}
	}
	if !bytes.Contains(got, []byte("gateCmd: make gate")) {
		t.Fatalf("retained var lost:\n%s", got)
	}
}
