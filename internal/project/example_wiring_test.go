package project

import (
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

// TestSundialCurrentStateMigrated pins the committed sundial fixture as a
// current-state adopter after the Plan 4 cutover: it carries a currentState
// config block (not legacy invariants), an authored topic corpus, sits at schema
// generation 14, and its committed lock records no bridge attestation. These
// properties hold identically in the preparation slice and after the final
// cutover, so the sealed contract stays green across both. Reading the fixture
// as data (never executing the binary) keeps this a static contract alongside
// the other example-wiring assertions.

// ADR-0090: the committed example adopter is kept deterministic through ./x -
// sync re-renders it from source; check drift-, invariant-, note-, and
// test-gates it. The example is its own Go module so the enclosing ./...
// sweeps never see it; this test pins the wiring so it cannot be silently
// dropped.
//
// invariant: tooling/quality-gates:example-adopter-checked (TestExampleAdopterWiring)
// invariant: tooling/quality-gates:example-zero-notes (TestExampleAdopterWiring)
// invariant: tooling/quality-gates:example-module-isolated (TestExampleAdopterWiring)
func TestExampleAdopterWiring(t *testing.T) {
	raw, err := os.ReadFile("../../x")
	if err != nil {
		t.Fatalf("read x: %v", err)
	}
	script := string(raw)
	for _, want := range []string{
		`context)`,
		`./awf context "$@" >"$capture"`,
		`go run ./cmd/contextspilllog --root "$PWD"`,
		`resolve or promote the issue`,
		`|check|context|`,
		`(cd examples/sundial && "$bindir/awf" render)`,
		`out="$(cd examples/sundial && "$bindir/awf" check)"`,
		`grep -q '^note: '`,
		`(cd examples/sundial && "$bindir/awf" check invariants)`,
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

// ADR-0198: the Pi-extension gate lane runs the extension suite inside a
// content-fingerprinted ephemeral Docker environment, so a contributor needs no
// host Node or npm, and it keeps an explicit reset cleanup command. Every
// property below is load-bearing for the claim and none is observable from the
// suite's own result: a lane that silently reverted to a persistent
// path-keyed container, or to a fingerprint that varies by checkout path, would
// still go green while orphaning one container, volume, and image per worktree.
//
// invariant: tooling/quality-gates:pi-extension-container-gate (TestPiExtensionContainerGateWiring)
func TestPiExtensionContainerGateWiring(t *testing.T) {
	rawX, err := os.ReadFile("../../x")
	if err != nil {
		t.Fatalf("read x: %v", err)
	}
	script := string(rawX)
	if !strings.Contains(script, "tools/pi-extension-test/container.sh run") {
		t.Error("./x gate must wire the pi-extension lane (ADR-0198)")
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

// The sundial example adopts the wrapper split (ADR-0156): the runner singleton
// renders the pure `awf` forwarder (default bootstrap-then-PATH body, no in-place
// region), its project verbs live in a hand-written `./x` outside the render set,
// and its config carries none of the awf-verb command vars, so it dogfoods the
// rendered defaults a fresh adopter gets.
//
// invariant: rendering/companion-scripts:runner-example-adopted (TestExampleAdoptsRunner)
func TestExampleAdoptsRunner(t *testing.T) {
	cfg, err := os.ReadFile("../../examples/sundial/.awf/config.yaml")
	if err != nil {
		t.Fatalf("read example config: %v", err)
	}
	if !strings.Contains(string(cfg), "runner:") {
		t.Error("the sundial example must enable the runner singleton (ADR-0156)")
	}
	for _, dropped := range []string{"activeMdRegenCmd", "checkCmd", "commitGateCmd", "proseGateCmd", "memoryGateCmd"} {
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
// invariant: rendering/pi-workflows:pi-extension-editor-quiet-strip (TestPiExtensionEditorQuietStrip)
func TestPiExtensionEditorQuietStrip(t *testing.T) {
	// Enumerate from the target descriptor, not from a directory walk, and check
	// BOTH rendered roots. A walk cannot notice a governed file that stopped
	// being rendered, and reading only this repository's root would miss the
	// example adopter entirely, so the claim's "every governed file" would hold
	// vacuously over whatever happened to be on disk.
	governed := map[string]bool{}
	for _, out := range piTarget.Outputs {
		if strings.HasSuffix(out.Path, ".ts") {
			governed[out.Path] = true
		}
	}
	if len(governed) == 0 {
		t.Fatal("the Pi target declares no governed TypeScript extension output")
	}
	roots := []string{"../..", "../../examples/sundial"}
	for _, root := range roots {
		for rel := range governed {
			path := root + "/" + rel
			raw, err := os.ReadFile(path)
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
		extRoot := root + "/.pi/extensions"
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
			if !governed[filepath.ToSlash(rel)] {
				t.Errorf("%s is stripped by the harness but is not a declared Pi target output", path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", extRoot, err)
		}
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
