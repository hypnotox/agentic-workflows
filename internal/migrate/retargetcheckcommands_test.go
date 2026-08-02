package migrate

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"gopkg.in/yaml.v3"
)

func TestVarsMappingHasMerge(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      bool
	}{
		{"malformed", "[", false},
		{"empty", "", false},
		{"scalar", "plain\n", false},
		{"no vars", "prefix: example\n", false},
		{"non-mapping vars", "vars: []\n", false},
		{"ordinary vars", "vars:\n  gateCmd: ./x gate\n", false},
		{"merged vars", "vars:\n  defaults: &defaults\n    gateCmd: ./x gate\n  <<: *defaults\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := varsMappingHasMerge([]byte(tc.src)); got != tc.want {
				t.Errorf("varsMappingHasMerge(%q) = %v, want %v", tc.src, got, tc.want)
			}
		})
	}
}

func TestRewriteCheckCommand(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
		changed, clear bool
	}{
		{"prose", "./awf check prose", "./awf check repo prose", true, false},
		{"memory with spacing", "awf  check\tmemory --all", "awf  check\trepo\tmemory --all", true, false},
		{"commit trailing argument", "/usr/bin/awf check commit \"$1\"", "/usr/bin/awf check staged commit \"$1\"", true, false},
		{"invariants clears", "awf check invariants", "awf check invariants", false, true},
		{"bare old invariants untouched", "awf invariants", "awf invariants", false, false},
		{"other runner untouched", "./x check prose", "./x check prose", false, false},
		{"already retargeted", "./awf check repo prose", "./awf check repo prose", false, false},
		{"later invocation untouched", "echo awf check prose", "echo awf check prose", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, changed, removeVar := rewriteCheckCommand(tc.in)
			if got != tc.want || changed != tc.changed || removeVar != tc.clear {
				t.Errorf("rewriteCheckCommand(%q) = (%q, %v, %v), want (%q, %v, %v)", tc.in, got, changed, removeVar, tc.want, tc.changed, tc.clear)
			}
		})
	}
}

func TestRetargetCheckCommandsRegisteredAndForwardPorted(t *testing.T) {
	if Current() != retargetCheckCommandsGeneration {
		t.Fatalf("Current() = %d, want retarget generation %d", Current(), retargetCheckCommandsGeneration)
	}
	src := []byte("prefix: example\nvars:\n  proseGateCmd: arbitrary\n  helper: ./awf check prose\n")
	got, err := ConfigForCurrentSchema(src, retargetCheckCommandsGeneration-1)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Vars map[string]any `yaml:"vars"`
	}
	if err := yaml.Unmarshal(got, &doc); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"helper": "./awf check repo prose"}
	if !reflect.DeepEqual(doc.Vars, want) {
		t.Errorf("forward-ported vars = %#v, want %#v", doc.Vars, want)
	}
}

func TestRetargetCheckCommandsMigratesVars(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, ".awf", "config.yaml")
	testsupport.WriteFile(t, cfg, `prefix: example
vars:
  proseGateCmd: arbitrary # retired whatever it holds
  memoryGateCmd: ./awf check repo memory
  proseElsewhere: ./awf check prose --strict
  memoryElsewhere: awf check memory
  commitGateCmd: ./awf check commit "$1"
  oldReport: /opt/awf check invariants
  otherRunner: ./x check prose
  gateCmd: ./x gate
`)
	var out bytes.Buffer
	if err := applyRetargetCheckCommands(root, &out); err != nil {
		t.Fatalf("applyRetargetCheckCommands: %v", err)
	}
	got := readMigratedVars(t, cfg)
	want := map[string]any{
		"proseElsewhere":  "./awf check repo prose --strict",
		"memoryElsewhere": "awf check repo memory",
		"commitGateCmd":   "./awf check staged commit \"$1\"",
		"otherRunner":     "./x check prose",
		"gateCmd":         "./x gate",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("vars = %#v, want %#v", got, want)
	}
	for _, wantLine := range []string{
		"removed retired vars.proseGateCmd",
		"removed retired vars.memoryGateCmd",
		"retargeted vars.proseElsewhere",
		"retargeted vars.memoryElsewhere",
		"retargeted vars.commitGateCmd",
		"removed vars.oldReport naming retired check invariants",
	} {
		if !strings.Contains(out.String(), wantLine) {
			t.Errorf("migration output missing %q:\n%s", wantLine, out.String())
		}
	}

	before, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := applyRetargetCheckCommands(root, &out); err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	after, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) || out.Len() != 0 {
		t.Errorf("replay changed config or reported work: before %q after %q output %q", before, after, out.String())
	}
}

func TestRetargetCheckCommandsMaterializesMergedVars(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, ".awf", "config.yaml")
	testsupport.WriteFile(t, cfg, `prefix: example
vars:
  defaults: &defaults
    proseGateCmd: arbitrary
    memoryGateCmd: ./awf check memory
    commitGateCmd: ./awf check commit "$1"
    helper: ./awf check prose --strict
    other: ./x check memory
  <<: *defaults
`)
	if err := applyRetargetCheckCommands(root, io.Discard); err != nil {
		t.Fatal(err)
	}
	got := readMigratedVars(t, cfg)
	if _, ok := got["proseGateCmd"]; ok {
		t.Fatalf("merged proseGateCmd survived: %#v", got)
	}
	if _, ok := got["memoryGateCmd"]; ok {
		t.Fatalf("merged memoryGateCmd survived: %#v", got)
	}
	for key, want := range map[string]string{
		"commitGateCmd": "./awf check staged commit \"$1\"",
		"helper":        "./awf check repo prose --strict",
		"other":         "./x check memory",
	} {
		if got[key] != want {
			t.Errorf("vars.%s = %#v, want %q", key, got[key], want)
		}
	}
	if raw, err := os.ReadFile(cfg); err != nil {
		t.Fatal(err)
	} else if strings.Contains(string(raw), "<<:") {
		t.Fatalf("vars merge survived materialization:\n%s", raw)
	}
}

func TestRetargetCheckCommandsMaterializesAliasedCommands(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, ".awf", "config.yaml")
	testsupport.WriteFile(t, cfg, `anchors:
  prose: &prose ./awf check prose
  memory: &memory awf check memory --strict
  commit: &commit /opt/awf check commit "$1"
vars:
  proseHelper: *prose
  memoryHelper: *memory
  commitHelper: *commit
`)
	if err := applyRetargetCheckCommands(root, io.Discard); err != nil {
		t.Fatal(err)
	}
	got := readMigratedVars(t, cfg)
	for key, want := range map[string]string{
		"proseHelper":  "./awf check repo prose",
		"memoryHelper": "awf check repo memory --strict",
		"commitHelper": "/opt/awf check staged commit \"$1\"",
	} {
		if got[key] != want {
			t.Errorf("vars.%s = %#v, want %q", key, got[key], want)
		}
	}
}

func TestRetargetCheckCommandsForeignShapes(t *testing.T) {
	for _, tc := range []struct {
		name, src string
	}{
		{"malformed", "vars: [bad\n"},
		{"no vars", "prefix: example\n"},
		{"non-string", "vars:\n  helper: [awf, check, prose]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			cfg := filepath.Join(root, ".awf", "config.yaml")
			testsupport.WriteFile(t, cfg, tc.src)
			if err := applyRetargetCheckCommands(root, io.Discard); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.src {
				t.Errorf("foreign shape changed from %q to %q", tc.src, got)
			}
		})
	}
}

func TestRetargetCheckCommandsClearsAliasedInvariantAndAbsentConfig(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, ".awf", "config.yaml")
	testsupport.WriteFile(t, cfg, "anchors:\n  command: &cmd ./awf check invariants\nvars:\n  helper: *cmd\n")
	if err := applyRetargetCheckCommands(root, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := readMigratedVars(t, cfg); len(got) != 0 {
		t.Fatalf("aliased invariant var survived: %#v", got)
	}
	if err := applyRetargetCheckCommands(t.TempDir(), io.Discard); err != nil {
		t.Fatalf("absent config: %v", err)
	}
}

func readMigratedVars(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Vars map[string]any `yaml:"vars"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Vars
}
