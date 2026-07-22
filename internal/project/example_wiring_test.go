package project

import (
	"os"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
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
	if migrate.Current() != 16 {
		t.Fatalf("migrate.Current() = %d, want 16", migrate.Current())
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

// The sundial example adopts the managed runner: it enables the singleton and its
// rendered `x` carries the awf-owned dispatch (including the `context` verb its
// hand-written runner was missing, ADR-0092) and its ported project verbs in the
// in-place section.
//
// invariant: rendering/templates:runner-example-adopted
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

// invariant: tooling/cli:managed-runner-command-parity
func TestRepositoryRunnerForwardsEveryMetadataCommand(t *testing.T) {
	raw, err := os.ReadFile("../../x")
	if err != nil {
		t.Fatal(err)
	}
	arms := shellCaseArms(string(raw))
	for _, name := range clispec.ForwardedNames() {
		body, ok := arms[name]
		if !ok {
			t.Errorf("repository x has no case arm for forwarded command %q", name)
			continue
		}
		if !strings.Contains(body, "go run ./cmd/awf") || !strings.Contains(body, name) && !strings.Contains(body, `"$cmd"`) {
			t.Errorf("repository x arm %q does not delegate through go run ./cmd/awf: %s", name, body)
		}
	}
	for _, name := range []string{"init", "upgrade", "uninstall"} {
		if _, ok := arms[name]; ok {
			t.Errorf("repository x must not forward excluded command %q", name)
		}
	}
}

func shellCaseArms(script string) map[string]string {
	out := map[string]string{}
	lines := strings.Split(script, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasSuffix(line, ")") || line == "*)" {
			continue
		}
		var body []string
		for i++; i < len(lines) && strings.TrimSpace(lines[i]) != ";;"; i++ {
			body = append(body, strings.TrimSpace(lines[i]))
		}
		for _, raw := range strings.Split(strings.TrimSuffix(line, ")"), "|") {
			out[strings.TrimSpace(raw)] = strings.Join(body, "\n")
		}
	}
	return out
}

func TestExampleAdoptsRunner(t *testing.T) {
	cfg, err := os.ReadFile("../../examples/sundial/.awf/config.yaml")
	if err != nil {
		t.Fatalf("read example config: %v", err)
	}
	if !strings.Contains(string(cfg), "runner:") {
		t.Error("the sundial example must enable the runner singleton (ADR-0101)")
	}
	raw, err := os.ReadFile("../../examples/sundial/x")
	if err != nil {
		t.Fatalf("the sundial example must render x: %v", err)
	}
	x := string(raw)
	for _, want := range []string{
		"#!/usr/bin/env bash",
		`"$(bash .awf/bootstrap.sh)" "$cmd" "$@" ;;`, // awf-owned dispatch
		"context",      // the verb the hand-written runner was missing (ADR-0092)
		"go vet ./...", // sundial's ported gate verb, preserved in the in-place section
		"# awf:edit-in-place runner-project-verbs: ",
	} {
		if !strings.Contains(x, want) {
			t.Errorf("rendered examples/sundial/x missing %q", want)
		}
	}
}

// The generated Pi extension carries `// @ts-nocheck` on the line after its
// provenance banner so adopter IDEs stay quiet without a resolvable
// `@types/node`, and the container gate strips that exact directive before
// `tsc` so the static type-check still covers the real extension code. Neither
// half stands alone: a missing strip leaves the lane green while `tsc` silently
// skips the file, so only this static assertion enforces the coupling.
//
// invariant: rendering/templates:pi-extension-editor-quiet-strip
func TestPiExtensionEditorQuietStrip(t *testing.T) {
	for _, name := range []string{"awf-handoff/index.ts", "awf-subagents/index.ts", "awf-subagents/runner.ts"} {
		content := renderPiExtensionFile(t, name)
		lines := strings.Split(content, "\n")
		if len(lines) < 2 || lines[1] != "// @ts-nocheck" {
			t.Errorf("rendered %s must carry // @ts-nocheck on line 2, got:\n%s", name, content)
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
	for _, rel := range []string{"../../.pi/extensions/awf-handoff/index.ts", "../../.pi/extensions/awf-subagents/index.ts", "../../.pi/extensions/awf-subagents/runner.ts", "../../examples/sundial/.pi/extensions/awf-handoff/index.ts", "../../examples/sundial/.pi/extensions/awf-subagents/index.ts", "../../examples/sundial/.pi/extensions/awf-subagents/runner.ts"} {
		content, err := os.ReadFile(rel)
		if err != nil {
			t.Errorf("read governed Pi output %s: %v", rel, err)
			continue
		}
		lines := strings.Split(string(content), "\n")
		if len(lines) < 2 || lines[1] != "// @ts-nocheck" {
			t.Errorf("governed Pi output %s lacks line-2 directive", rel)
		}
	}
}
