package migrate

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// The fixtures below spell the retired command names deliberately: they are what
// the migration rewrites. This file and renameretiredcommands.go are the one
// sanctioned home for those literals, and both are excluded from the repo-wide
// sweep that forbids them elsewhere.
func TestRewriteRetiredCommand(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
		wantChanged    bool
	}{
		{"bare awf sync", "awf sync", "awf render", true},
		{"runner-relative invariants", "./awf invariants", "./awf check invariants", true},
		{"absolute path prose-gate", "/usr/local/bin/awf prose-gate", "/usr/local/bin/awf check prose", true},
		{"memory-gate", "./awf memory-gate", "./awf check memory", true},
		{"commit-gate keeps trailing args", "./awf commit-gate \"$1\"", "./awf check commit \"$1\"", true},
		{"trailing args survive verbatim", "awf sync --force -v", "awf render --force -v", true},
		{"inner whitespace preserved", "./awf   prose-gate", "./awf   check prose", true},
		{"project runner untouched", "./x check", "./x check", false},
		{"non-awf first token untouched", "make gate", "make gate", false},
		{"non-awf runner naming a retired word untouched", "./x sync", "./x sync", false},
		{"retired word in prose untouched", "echo run awf sync first", "echo run awf sync first", false},
		{"retired word later in a pipeline untouched", "./x check && awf sync", "./x check && awf sync", false},
		{"awf with a live subcommand untouched", "./awf render", "./awf render", false},
		{"awf alone untouched", "awf", "awf", false},
		{"a token merely ending in the name untouched", "awf resync", "awf resync", false},
		{"empty value untouched", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := rewriteRetiredCommand(tc.in)
			if got != tc.want || changed != tc.wantChanged {
				t.Errorf("rewriteRetiredCommand(%q) = (%q, %v), want (%q, %v)", tc.in, got, changed, tc.want, tc.wantChanged)
			}
		})
	}
}

// The migration rewrites every matching var in one pass, leaves a sibling naming
// another runner alone, and preserves comments and untouched keys.
func TestRenameRetiredCommandsMigratesVars(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, ".awf", "config.yaml")
	testsupport.WriteFile(t, cfg, "prefix: ex\nvars:\n  proseGateCmd: ./awf prose-gate # keep\n"+
		"  memoryGateCmd: ./awf memory-gate\n  commitGateCmd: ./awf commit-gate\n"+
		"  activeMdRegenCmd: ./awf sync\n  checkCmd: ./x check\n  gateCmd: ./x gate\n")
	if err := applyRenameRetiredCommands(root, io.Discard); err != nil {
		t.Fatalf("applyRenameRetiredCommands: %v", err)
	}
	out, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := "prefix: ex\nvars:\n  proseGateCmd: ./awf check prose # keep\n" +
		"  memoryGateCmd: ./awf check memory\n  commitGateCmd: ./awf check commit\n" +
		"  activeMdRegenCmd: ./awf render\n  checkCmd: ./x check\n  gateCmd: ./x gate\n"
	if string(out) != want {
		t.Errorf("migrated config:\n got %q\nwant %q", out, want)
	}
}

// Re-running the migration after it has already ported a config is a no-op, so a
// replay from a degraded lock cannot double-rewrite.
func TestRenameRetiredCommandsIsIdempotent(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, ".awf", "config.yaml")
	src := "prefix: ex\nvars:\n  proseGateCmd: ./awf check prose\n"
	testsupport.WriteFile(t, cfg, src)
	if err := applyRenameRetiredCommands(root, io.Discard); err != nil {
		t.Fatalf("applyRenameRetiredCommands: %v", err)
	}
	out, _ := os.ReadFile(cfg)
	if string(out) != src {
		t.Errorf("already-migrated config rewritten:\n got %q\nwant %q", out, src)
	}
}

// A config with no vars block, a null var value, and a non-string var value each
// pass through untouched rather than faulting.
func TestRenameRetiredCommandsToleratesNonStringVars(t *testing.T) {
	for _, src := range []string{
		"prefix: ex\nskills: [tdd]\n",
		"prefix: ex\nvars:\n  proseGateCmd:\n",
		"prefix: ex\nvars: {}\n",
	} {
		root := t.TempDir()
		cfg := filepath.Join(root, ".awf", "config.yaml")
		testsupport.WriteFile(t, cfg, src)
		if err := applyRenameRetiredCommands(root, io.Discard); err != nil {
			t.Fatalf("applyRenameRetiredCommands(%q): %v", src, err)
		}
		out, _ := os.ReadFile(cfg)
		if string(out) != src {
			t.Errorf("config %q rewritten to %q", src, out)
		}
	}
}

func TestRenameRetiredCommandsAbsentConfig(t *testing.T) {
	if err := applyRenameRetiredCommands(t.TempDir(), io.Discard); err != nil {
		t.Errorf("applyRenameRetiredCommands with no .awf/config.yaml should be a no-op, got %v", err)
	}
}

// A config too malformed to decode is left to the strict parse to report, so the
// migration returns it unchanged rather than erroring here.
func TestRenameRetiredCommandsMalformedConfig(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, ".awf", "config.yaml")
	src := "vars: [bad\n"
	testsupport.WriteFile(t, cfg, src)
	if err := applyRenameRetiredCommands(root, io.Discard); err != nil {
		t.Errorf("malformed config should pass through, got %v", err)
	}
	out, _ := os.ReadFile(cfg)
	if string(out) != src {
		t.Errorf("malformed config rewritten to %q", out)
	}
}

// The rename-retired-commands migration is the schema 19 tip.
func TestRenameRetiredCommandsIsCurrent(t *testing.T) {
	if Current() != 19 {
		t.Errorf("Current() = %d, want 19", Current())
	}
}
