package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
)

// TestSundialCurrentStateMigrated pins the committed sundial fixture as a
// current-state adopter after the Plan 4 cutover: it carries a currentState
// config block (not legacy invariants), an authored topic corpus, sits at schema
// generation 14, and its committed lock records no bridge attestation. These
// properties hold identically in the preparation slice and after the final
// cutover, so the sealed contract stays green across both. Reading the fixture
// as data (never executing the binary) keeps this a static contract alongside
// the other example-wiring assertions.
func TestSundialCurrentStateMigrated(t *testing.T) {
	if migrate.Current() != 18 {
		t.Fatalf("migrate.Current() = %d, want 18", migrate.Current())
	}
	lockPath := "../../examples/sundial/.awf/awf.lock"
	lock, err := manifest.Load(lockPath)
	if err != nil {
		t.Fatalf("read sundial lock: %v", err)
	}
	if lock.SchemaVersion != migrate.Current() {
		t.Errorf("sundial lock schemaVersion = %d, want %d", lock.SchemaVersion, migrate.Current())
	}
	if lock.BridgeAttestation != nil {
		t.Errorf("sundial committed lock must carry no bridge attestation")
	}
	rawLock, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read sundial lock bytes: %v", err)
	}
	if strings.Contains(string(rawLock), "bridgeAttestation") {
		t.Errorf("sundial lock must not carry a bridgeAttestation field")
	}
	cfg, err := os.ReadFile("../../examples/sundial/.awf/config.yaml")
	if err != nil {
		t.Fatalf("read sundial config: %v", err)
	}
	cfgStr := string(cfg)
	if !strings.Contains(cfgStr, "\ncurrentState:") {
		t.Errorf("sundial config must carry a currentState block after migration")
	}
	if strings.Contains(cfgStr, "\ninvariants:") {
		t.Errorf("sundial config must not keep a legacy invariants block after migration")
	}
	for _, present := range []string{
		"../../examples/sundial/.awf/topics/metadata/almanac/model.yaml",
		"../../examples/sundial/.awf/topics/parts/almanac/model/current-state.md",
		"../../examples/sundial/.awf/topics/metadata/cli/interface.yaml",
	} {
		if _, err := os.Stat(present); err != nil {
			t.Errorf("migrated sundial fixture must contain %q: %v", present, err)
		}
	}
}

// ADR-0090: the committed example adopter is kept deterministic through ./x -
// sync re-renders it from source; check drift-, invariant-, note-, and
// test-gates it. The example is its own Go module so the enclosing ./...
// sweeps never see it; this test pins the wiring so it cannot be silently
// dropped.
//
// invariant: tooling/quality-gates:example-adopter-checked
// invariant: tooling/quality-gates:example-zero-notes
// invariant: tooling/quality-gates:example-module-isolated
func TestExampleAdopterWiring(t *testing.T) {
	raw, err := os.ReadFile("../../x")
	if err != nil {
		t.Fatalf("read x: %v", err)
	}
	script := string(raw)
	for _, want := range []string{
		`(cd examples/sundial && "$bindir/awf" sync)`,
		`out="$(cd examples/sundial && "$bindir/awf" check)"`,
		`grep -q '^note: '`,
		`(cd examples/sundial && "$bindir/awf" invariants)`,
		`(cd examples/sundial && go test ./...)`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("x lost the example-adopter step %q (ADR-0090)", want)
		}
	}
	if _, err := os.Stat("../../examples/sundial/go.mod"); err != nil {
		t.Errorf("examples/sundial must stay its own Go module (ADR-0090): %v", err)
	}
}

// invariant: tooling/quality-gates:pi-extension-container-gate
func TestPiExtensionContainerGateWiring(t *testing.T) {
	raw, err := os.ReadFile("../../x")
	if err != nil {
		t.Fatalf("read x: %v", err)
	}
	script := string(raw)
	for _, want := range []string{
		"tools/pi-extension-test/container.sh run",
		"pi-test)",
		"<run|stop|reset>",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("x lost Pi extension test wiring %q", want)
		}
	}
	manager, err := os.ReadFile("../../tools/pi-extension-test/container.sh")
	if err != nil {
		t.Fatalf("read container manager: %v", err)
	}
	for _, want := range []string{
		`AWF_PI_TEST_DOCKER`,
		`dst=/source,readonly`,
		`--workdir /workspace/repo`,
		`pi-extension-test: Docker is required by ./x gate`,
	} {
		if !strings.Contains(string(manager), want) {
			t.Errorf("container manager lost contract %q", want)
		}
	}
}

// The sundial example adopts the wrapper split (ADR-0156): the runner singleton
// renders the pure `awf` forwarder (default bootstrap-then-PATH body, no in-place
// region), its project verbs live in a hand-written `./x` outside the render set,
// and its config carries none of the awf-verb command vars, so it dogfoods the
// rendered defaults a fresh adopter gets.
//
// invariant: rendering/companion-scripts:runner-example-adopted
func TestExampleAdoptsRunner(t *testing.T) {
	cfg, err := os.ReadFile("../../examples/sundial/.awf/config.yaml")
	if err != nil {
		t.Fatalf("read example config: %v", err)
	}
	if !strings.Contains(string(cfg), "runner:") {
		t.Error("the sundial example must enable the runner singleton (ADR-0156)")
	}
	for _, dropped := range []string{"activeMdRegenCmd", "checkCmd", "commitGateCmd", "proseGateCmd"} {
		if strings.Contains(string(cfg), dropped) {
			t.Errorf("the sundial config must carry no awf-verb command var %q: it dogfoods the rendered defaults", dropped)
		}
	}
	raw, err := os.ReadFile("../../examples/sundial/awf")
	if err != nil {
		t.Fatalf("the sundial example must render awf: %v", err)
	}
	wrapper := string(raw)
	for _, want := range []string{
		"#!/usr/bin/env bash",
		`if [ -f .awf/bootstrap.sh ] && pinned="$(bash .awf/bootstrap.sh)"; then`,
		`exec "$pinned" "$@"`,
		`exec awf "$@"`,
	} {
		if !strings.Contains(wrapper, want) {
			t.Errorf("rendered examples/sundial/awf missing %q", want)
		}
	}
	if strings.Contains(wrapper, "awf:edit-in-place") {
		t.Errorf("the rendered wrapper must carry no in-place region:\n%s", wrapper)
	}
	x, err := os.ReadFile("../../examples/sundial/x")
	if err != nil {
		t.Fatalf("the sundial example must keep a hand-written project runner x: %v", err)
	}
	for _, want := range []string{"gate)", "test)"} {
		if !strings.Contains(string(x), want) {
			t.Errorf("hand-written examples/sundial/x missing project verb arm %q", want)
		}
	}
	if strings.Contains(string(x), "GENERATED by awf") {
		t.Error("examples/sundial/x must be hand-written, outside the render set")
	}
}

// The generated Pi extension carries `// @ts-nocheck` on the line after its
// provenance banner so adopter IDEs stay quiet without a resolvable
// `@types/node`, and the container gate strips that exact directive before
// `tsc` so the static type-check still covers the real extension code. Neither
// half stands alone: a missing strip leaves the lane green while `tsc` silently
// skips the file, so only this static assertion enforces the coupling.
//
// invariant: rendering/pi-workflows:pi-extension-editor-quiet-strip
func TestPiExtensionEditorQuietStrip(t *testing.T) {
	want := map[string]bool{}
	for _, output := range piTarget.Outputs {
		if !strings.HasPrefix(output.Path, ".pi/extensions/") || filepath.Ext(output.Path) != ".ts" {
			t.Fatalf("Pi extension descriptor output is not governed TypeScript: %s", output.Path)
		}
		want[output.Path] = true
		name := strings.TrimPrefix(output.Path, ".pi/extensions/")
		content := renderPiExtensionFile(t, name)
		lines := strings.Split(content, "\n")
		if len(lines) < 2 || lines[1] != "// @ts-nocheck" {
			t.Errorf("rendered %s must carry // @ts-nocheck on line 2, got:\n%s", name, content)
		}
	}
	for label, root := range map[string]string{"root": "../..", "sundial": "../../examples/sundial"} {
		got := map[string]bool{}
		extensionRoot := filepath.Join(root, ".pi/extensions")
		err := filepath.WalkDir(extensionRoot, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			got[filepath.ToSlash(rel)] = true
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s governed Pi outputs: %v", label, err)
		}
		for path := range want {
			if !got[path] {
				t.Errorf("%s checkout is missing descriptor-owned Pi output %s", label, path)
				continue
			}
			content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
			if err != nil {
				t.Errorf("read %s governed Pi output %s: %v", label, path, err)
				continue
			}
			lines := strings.Split(string(content), "\n")
			if len(lines) < 2 || lines[1] != "// @ts-nocheck" {
				t.Errorf("%s governed Pi output %s lacks line-2 directive", label, path)
			}
		}
		for path := range got {
			if !want[path] {
				t.Errorf("%s checkout has governed Pi output absent from descriptor: %s", label, path)
			}
		}
	}
	raw, err := os.ReadFile("../../tools/pi-extension-test/container.sh")
	if err != nil {
		t.Fatalf("read container manager: %v", err)
	}
	manager := string(raw)
	strip := `find .pi/extensions -type f -name '*.ts' -print0 | sort -z | xargs -0 sed -i "s|^// @ts-nocheck$||"`
	stripAt := strings.Index(manager, strip)
	if stripAt < 0 {
		t.Fatal("container manager missing the @ts-nocheck strip stage")
	}
	cpAt := strings.Index(manager, "cp -a /source/. /workspace/repo/")
	if cpAt < 0 {
		t.Fatal("container manager missing the source copy stage")
	}
	tscAt := strings.Index(manager, "tsc -p tools/pi-extension-test/tsconfig.json")
	if tscAt < 0 {
		t.Fatal("container manager missing the tsc invocation")
	}
	if cpAt >= stripAt || stripAt >= tscAt {
		t.Error("the @ts-nocheck strip must run after the source copy and before tsc")
	}
	if !strings.Contains(manager, `--include='.pi/extensions/**/*.ts'`) {
		t.Error("container coverage does not include every Pi extension")
	}
	tsconfig, err := os.ReadFile("../../tools/pi-extension-test/tsconfig.json")
	if err != nil {
		t.Fatalf("read Pi extension tsconfig: %v", err)
	}
	if !strings.Contains(string(tsconfig), `"../../.pi/extensions/**/*.ts"`) {
		t.Error("Pi typecheck does not include every descriptor-owned extension")
	}
	for path := range want {
		if strings.Contains(manager, path) {
			t.Errorf("container command hard-codes descriptor member %s instead of using the generalized extension glob", path)
		}
	}
}
