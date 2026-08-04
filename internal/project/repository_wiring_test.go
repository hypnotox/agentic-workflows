package project

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The legacy sweep in container.sh is the one piece of ADR-0198 that can destroy
// something: it removes containers, and a wrong predicate kills a gate running in
// another checkout. Review found two defects in it that source-substring matching
// could not have caught, so it is exercised behaviourally through the script's own
// AWF_PI_TEST_DOCKER seam. The stub records every removal instead of performing one.
func TestPiExtensionLegacySweepPredicate(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("bash unavailable: %v", err)
	}
	dir := t.TempDir()
	live := filepath.Join(dir, "live")
	if err := os.MkdirAll(live, 0o755); err != nil {
		t.Fatalf("make live source path: %v", err)
	}
	removed := filepath.Join(dir, "removed.log")
	stub := filepath.Join(dir, "docker")
	script := `#!/usr/bin/env bash
[ "$1" = info ] && exit 0
if [ "$1" = ps ]; then printf 'runlive\nrundead\nstopped\nvanished\n'; exit 0; fi
if [ "$1" = inspect ]; then
  id="${!#}"
  [ "$id" = vanished ] && exit 1
  case "$3" in
    *State.Running*) if [ "$id" = stopped ]; then echo false; else echo true; fi ;;
    *) if [ "$id" = rundead ]; then echo ` + filepath.Join(dir, "gone") + `; else echo ` + live + `; fi ;;
  esac
  exit 0
fi
if [ "$1" = rm ]; then echo "${!#}" >>` + removed + `; exit 0; fi
exit 0
`
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("write docker stub: %v", err)
	}
	cmd := exec.Command("bash", "../../tools/pi-extension-test/container.sh", "reset")
	cmd.Env = append(os.Environ(), "AWF_PI_TEST_DOCKER="+stub)
	if out, err := cmd.CombinedOutput(); err != nil {
		// A sweep that aborts leaves every later object unreaped, and an earlier
		// defect made it abort silently on exactly the "vanished" case below.
		t.Fatalf("reset must survive a container that vanishes mid-sweep: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(removed)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read removal log: %v", err)
	}
	got := map[string]bool{}
	for _, id := range strings.Fields(string(raw)) {
		got[id] = true
	}
	// ADR-0198 item 9: remove only when provably unused, and fail closed otherwise.
	for id, want := range map[string]bool{
		"runlive":  false, // running at a live path: could be backing another checkout's gate
		"rundead":  true,  // recorded path is gone, so no new gate can start against it
		"stopped":  true,  // not running
		"vanished": false, // undescribable, so assumed live
	} {
		if got[id] != want {
			t.Errorf("legacy sweep removed=%v for %q, want removed=%v", got[id], id, want)
		}
	}
}

// ADR-0198: the Pi-extension gate lane runs the extension suite inside a
// content-fingerprinted ephemeral Docker environment, so a contributor needs no
// host Node or npm, and it keeps an explicit reset cleanup command. Every
// property below is load-bearing for the claim and none is observable from the
// suite's own result: a lane that silently reverted to a persistent
// path-keyed container, or to a fingerprint that varies by checkout path, would
// still go green while orphaning one container, volume, and image per worktree.
//
// invariant: tooling/quality-gates:pi-extension-container-gate (TestPiExtensionContainerGateWiring)
// invariant: rendering/pi-runtime:pi-extension-target-render (TestPiExtensionContainerGateWiring)
func TestPiExtensionContainerGateWiring(t *testing.T) {
	rawX, err := os.ReadFile("../../x")
	if err != nil {
		t.Fatalf("read x: %v", err)
	}
	script := string(rawX)
	gateStart := strings.Index(script, "  gate)")
	if gateStart < 0 {
		t.Fatal("./x must retain a gate arm")
	}
	gateEnd := strings.Index(script[gateStart:], "\n    ;;")
	if gateEnd < 0 {
		t.Fatal("./x must retain a closed gate arm")
	}
	gateArm := script[gateStart : gateStart+gateEnd]
	const smoke = "run_gate_step pi-runtime-smoke run_pi_runtime_smoke"
	if strings.Count(gateArm, smoke) != 1 {
		t.Errorf("./x gate must wire one explicit uncached pi-extension smoke, count=%d", strings.Count(gateArm, smoke))
	}
	for _, want := range []string{
		"env -u AWF_PI_RUNTIME_SMOKE go test ./...",
		"env AWF_PI_RUNTIME_SMOKE=1 go test -json ./internal/project -run '^TestPiRealRuntimeSmoke$' -count=1",
		`"Action":"pass".*"Test":"TestPiRealRuntimeSmoke"`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("./x Pi runtime ownership lost %q", want)
		}
	}
	if strings.Contains(gateArm, "tools/pi-extension-test/container.sh run") {
		t.Error("./x gate must not invoke the Pi container beside the explicit runtime smoke")
	}
	if !strings.Contains(script, "usage: ./x pi-test <run|reset>") {
		t.Error("./x pi-test must offer exactly run and reset (ADR-0198)")
	}

	rawSh, err := os.ReadFile("../../tools/pi-extension-test/container.sh")
	if err != nil {
		t.Fatalf("read container.sh: %v", err)
	}
	sh := string(rawSh)
	for _, want := range []string{
		// Ephemeral: one throwaway container per invocation.
		`run --rm`,
		// The whole repository root stays read-only; the copy is what mutates.
		`type=bind,src=$root,dst=/source,readonly`,
		// Dependencies come from the image, so the lane creates no volume.
		`/opt/awf-pi-test/node_modules`,
		// The reset cleanup command survives.
		`reset)`,
		// The docker binary stays injectable for testing.
		`${AWF_PI_TEST_DOCKER:-docker}`,
		// The standalone context entrypoint is explicitly measured at full coverage.
		`--include='.pi/extensions/awf-context-usage/index.ts'`,
		`--lines=100 --functions=100 --branches=100`,
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("container.sh lost %q (ADR-0198)", want)
		}
	}
	for _, banned := range []string{
		// A persistent container in any form reintroduces the orphan class.
		`docker create`,
		`" create`,
		`" start`,
		// A named volume is a per-path durable object.
		`volume create`,
		// stop was removed with the persistent container.
		`stop)`,
	} {
		if strings.Contains(sh, banned) {
			t.Errorf("container.sh must not reintroduce %q: the lane is ephemeral (ADR-0198)", banned)
		}
	}

	// The fingerprint must hash file CONTENTS. Hashing sha256sum's output embeds
	// each file's absolute path, which silently keys the image per checkout.
	start := strings.Index(sh, "hash_files()")
	if start < 0 {
		t.Fatal("container.sh must define hash_files (ADR-0198)")
	}
	body := sh[start:]
	end := strings.Index(body, "}")
	if end < 0 {
		t.Fatal("hash_files must be a closed shell function")
	}
	fingerprint := body[:end]
	if !strings.Contains(fingerprint, "cat ") {
		t.Error("hash_files must hash file contents, not sha256sum output (ADR-0198)")
	}
	if strings.Contains(fingerprint, `sha256sum "$tool_dir`) {
		t.Error("hash_files must not hash sha256sum's path-bearing output (ADR-0198)")
	}
	if strings.Contains(sh, "repo_hash") {
		t.Error("container.sh must not key any object on the repository path (ADR-0198)")
	}
}

// The generated Pi extension carries `// @ts-nocheck` on the line after its
// provenance banner so adopter IDEs stay quiet without a resolvable
// `@types/node`, and the container gate strips that exact directive before
// `tsc` so the static type-check still covers the real extension code. Neither
// half stands alone: a missing strip leaves the lane green while `tsc` silently
// skips the file, so only this static assertion enforces the coupling.
//
// invariant: rendering/pi-workflows:pi-extension-editor-quiet-strip (TestPiExtensionEditorQuietStrip)
// invariant: rendering/pi-runtime:pi-extension-target-render (TestPiExtensionEditorQuietStrip)
func TestPiExtensionEditorQuietStrip(t *testing.T) {
	// Enumerate from the target descriptor, not from a directory walk. A walk
	// cannot notice a governed file that stopped being rendered. The temporary
	// authored adopter keeps this assertion independent of a committed example.
	governed := map[string]TargetOutput{}
	for _, out := range piTarget.Outputs {
		if strings.HasSuffix(out.Path, ".ts") {
			governed[out.Path] = out
		}
	}
	if len(governed) == 0 {
		t.Fatal("the Pi target declares no governed TypeScript extension output")
	}
	root := temporaryAuthoredAdopter(t)
	selectedSkills := map[string]bool{"tdd": true, "exploring": true}
	for rel, out := range governed {
		path := filepath.Join(root, filepath.FromSlash(rel))
		raw, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) && out.RequiresSkill != "" && !selectedSkills[out.RequiresSkill] {
			continue
		}
		if err != nil {
			t.Fatalf("read governed extension %s: %v", path, err)
		}
		lines := strings.Split(string(raw), "\n")
		if len(lines) < 2 {
			t.Fatalf("%s is too short to carry a banner and directive", path)
		}
		if !strings.HasPrefix(lines[0], "// GENERATED by awf") {
			t.Errorf("%s must open with the provenance banner, got %q", path, lines[0])
		}
		if lines[1] != "// @ts-nocheck" {
			t.Errorf("%s must carry the ts-nocheck directive on the line immediately after the banner, got %q", path, lines[1])
		}
	}
	// The other direction: the recursive strip covers every .ts under the
	// extensions root, so a file there that the descriptor does not declare
	// would be stripped without this test ever checking its directive.
	extRoot := filepath.Join(root, ".pi", "extensions")
	err := filepath.WalkDir(extRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".ts") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if _, ok := governed[filepath.ToSlash(rel)]; !ok {
			t.Errorf("%s is stripped by the harness but is not a declared Pi target output", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", extRoot, err)
	}

	// The harness must strip that exact directive in its ephemeral copy, after
	// the source copy and before the compiler. Order is the whole point: a strip
	// before the copy is overwritten, and a strip after tsc never runs in time,
	// and either way the lane stays green while tsc silently skips the file.
	raw, err := os.ReadFile("../../tools/pi-extension-test/container.sh")
	if err != nil {
		t.Fatalf("read container.sh: %v", err)
	}
	sh := string(raw)

	// Index inside the prepare heredoc only. Indexing the whole script would
	// match the explanatory comment above it, which names the superseded copy
	// command, and an ordering assertion anchored to prose cannot fail.
	prepStart := strings.Index(sh, "prepare_command=")
	if prepStart < 0 {
		t.Fatal("container.sh must build a prepare command")
	}
	prepare := sh[prepStart:]
	prepEnd := strings.Index(prepare, "\nCOMMAND\n")
	if prepEnd < 0 {
		t.Fatal("the prepare command must be a closed COMMAND heredoc")
	}
	prepare = prepare[:prepEnd]

	copyAt := strings.Index(prepare, "cp -a /source/.pi")
	stripAt := strings.Index(prepare, `sed -i "s|^// @ts-nocheck$||"`)
	compileAt := strings.Index(prepare, "tsc -p tools/pi-extension-test/tsconfig.json")
	if copyAt < 0 {
		t.Fatal("the prepare command must copy the extension source into the ephemeral working copy")
	}
	if stripAt < 0 {
		t.Fatal("the prepare command must strip exactly the ts-nocheck directive")
	}
	if compileAt < 0 {
		t.Fatal("the prepare command must run the TypeScript compiler")
	}
	if copyAt >= stripAt || stripAt >= compileAt {
		t.Errorf("the prepare command must strip after the source copy and before tsc, got copy=%d strip=%d tsc=%d", copyAt, stripAt, compileAt)
	}

	// The claim quantifies over EVERY governed extension file, so the scope that
	// feeds the strip is as load-bearing as the strip itself: narrowing the find
	// would leave some file unstripped while the sed literal above still matched.
	if !strings.Contains(prepare, `find .pi/extensions -type f -name '*.ts'`) {
		t.Error("the strip must cover every governed extension TypeScript file (ADR-0148)")
	}
}
