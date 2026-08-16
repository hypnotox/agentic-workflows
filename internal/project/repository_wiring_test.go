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

// ADR-0281: the Pi-extension gate lane runs directly on the pinned host Node
// runtime. Its reusable lockfile installation is checkout-local, while each test
// run gets a narrow temporary source copy. These static assertions bind the
// runner's durable boundaries without requiring an installed NVM or npm.
func TestPiExtensionHostRunnerWorkerSeams(t *testing.T) {
	// The worker boundary is executable with fake commands, so ordinary Go tests
	// need neither a developer NVM nor npm nor Node.
	root := t.TempDir()
	tool := filepath.Join(root, "tools", "pi-extension-test")
	for _, dir := range []string{filepath.Join(root, ".pi/extensions"), filepath.Join(root, ".pi/agents"), filepath.Join(root, ".pi/skills"), filepath.Join(tool, "tests"), filepath.Join(tool, "fixtures"), filepath.Join(tool, "node_modules/.bin")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".nvmrc"), []byte("v24.19.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{
		".pi/extensions/example.ts":            "// @ts-nocheck\nexport const x = 1\n",
		".pi/unrelated-secret":                 "must not enter the workspace\n",
		"root-only":                            "must not enter the workspace\n",
		"tools/pi-extension-test/package.json": "{}\n", "tools/pi-extension-test/package-lock.json": "{}\n", "tools/pi-extension-test/tsconfig.json": "{}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, bin := range []string{"c8", "tsx"} {
		if err := os.WriteFile(filepath.Join(tool, "node_modules/.bin", bin), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// The fake compiler makes an ordering mutation observable and rejects source
	// outside the intentionally narrow workspace.
	tsc := "#!/bin/sh\nif grep -R '// @ts-nocheck' .pi/extensions >/dev/null || [ -e .pi/unrelated-secret ] || [ -e root-only ]; then exit 9; fi\n"
	if err := os.WriteFile(filepath.Join(tool, "node_modules/.bin/tsc"), []byte(tsc), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tool, ".host-deps-fingerprint.ok"), []byte("fingerprint\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(root, "fake-bin")
	if err := os.Mkdir(fake, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, "node"), []byte("#!/bin/sh\nif [ \"$1\" = --version ]; then echo v24.19.0; elif [ \"$1\" = - ]; then body=$(cat); case \"$body\" in *spawnSync*) echo \"${FAKE_FINGERPRINT:-fingerprint}\";; *) find .pi/extensions -name '*.ts' -exec sed -i '/^\\/\\/ @ts-nocheck$/d' {} +;; esac; else exit 0; fi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	npm := "#!/bin/sh\ncase \"$1\" in --version) echo 1;; ci) [ \"${FAKE_NPM_FAIL:-0}\" = 1 ] && exit 7; echo ci >>\"$AWF_FAKE_NPM_COUNT\"; mkdir -p node_modules/.bin; for bin in c8 tsx; do printf '#!/bin/sh\\nexit 0\\n' >node_modules/.bin/$bin; chmod +x node_modules/.bin/$bin; done; printf '%s' \"$FAKE_TSC\" >node_modules/.bin/tsc; chmod +x node_modules/.bin/tsc;; *) exit 2;; esac\n"
	if err := os.WriteFile(filepath.Join(fake, "npm"), []byte(npm), 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(tool, "run.sh")
	raw, err := os.ReadFile("../../tools/pi-extension-test/run.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, raw, 0o755); err != nil {
		t.Fatal(err)
	}
	count := filepath.Join(root, "npm-ci-count")
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(), "AWF_PI_TEST_ROOT="+root, "AWF_PI_TEST_SKIP_NVM=1", "AWF_PI_TEST_WORKER=1", "AWF_FAKE_NPM_COUNT="+count, "FAKE_TSC="+tsc, "PATH="+fake+":"+os.Getenv("PATH"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("warm worker failed: %v\n%s", err, output)
	}
	original, err := os.ReadFile(filepath.Join(root, ".pi/extensions/example.ts"))
	if err != nil || !strings.Contains(string(original), "// @ts-nocheck") {
		t.Fatalf("worker mutated source: %q, %v", original, err)
	}
	// Mutation confirmation: moving the execution invocation after tsc makes the
	// fake compiler see ts-nocheck and fail, rather than merely matching its
	// function declaration in a static order check.
	late := filepath.Join(tool, "late.sh")
	mutated := strings.Replace(string(raw), "  strip_ts_nocheck\n  node_modules/.bin/tsc", "  node_modules/.bin/tsc\n  strip_ts_nocheck", 1)
	if mutated == string(raw) {
		t.Fatal("strip-order mutation did not apply")
	}
	if err := os.WriteFile(late, []byte(mutated), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("bash", late)
	cmd.Env = append(os.Environ(), "AWF_PI_TEST_ROOT="+root, "AWF_PI_TEST_SKIP_NVM=1", "AWF_PI_TEST_WORKER=1", "AWF_FAKE_NPM_COUNT="+count, "FAKE_TSC="+tsc, "PATH="+fake+":"+os.Getenv("PATH"))
	if output, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("late strip mutation unexpectedly passed:\n%s", output)
	}
	leaks, err := filepath.Glob(filepath.Join(tool, ".host-deps.*"))
	if err != nil || len(leaks) != 0 {
		t.Fatalf("temporary marker leaks: %v, %v", leaks, err)
	}
	// A changed fingerprint invalidates the marker and performs a cold install.
	cmd = exec.Command("bash", script)
	cmd.Env = append(os.Environ(), "AWF_PI_TEST_ROOT="+root, "AWF_PI_TEST_SKIP_NVM=1", "AWF_PI_TEST_WORKER=1", "AWF_FAKE_NPM_COUNT="+count, "FAKE_TSC="+tsc, "FAKE_FINGERPRINT=changed", "PATH="+fake+":"+os.Getenv("PATH"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fingerprint invalidation failed: %v\n%s", err, output)
	}
	installs, err := os.ReadFile(count)
	if err != nil || strings.Count(string(installs), "ci\n") != 1 {
		t.Fatalf("fingerprint change must reinstall once: %q, %v", installs, err)
	}
	// A failed cold install publishes neither a success marker nor a temporary one.
	cmd = exec.Command("bash", script)
	cmd.Env = append(os.Environ(), "AWF_PI_TEST_ROOT="+root, "AWF_PI_TEST_SKIP_NVM=1", "AWF_PI_TEST_WORKER=1", "AWF_FAKE_NPM_COUNT="+count, "FAKE_TSC="+tsc, "FAKE_FINGERPRINT=failed", "FAKE_NPM_FAIL=1", "PATH="+fake+":"+os.Getenv("PATH"))
	if output, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("failed install unexpectedly passed:\n%s", output)
	}
	markers, err := filepath.Glob(filepath.Join(tool, ".host-deps-*.ok"))
	if err != nil || len(markers) != 0 {
		t.Fatalf("failed install left success markers: %v, %v", markers, err)
	}
	leaks, err = filepath.Glob(filepath.Join(tool, ".host-deps.*"))
	if err != nil || len(leaks) != 0 {
		t.Fatalf("failed install left temporary markers: %v, %v", leaks, err)
	}
	// The CI control bypasses NVM selection, not the exact pin oracle.
	if err := os.WriteFile(filepath.Join(fake, "node"), []byte("#!/bin/sh\nif [ \"$1\" = --version ]; then echo v0.0.0; else exit 0; fi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("bash", script)
	cmd.Env = append(os.Environ(), "AWF_PI_TEST_ROOT="+root, "AWF_PI_TEST_SKIP_NVM=1", "AWF_PI_TEST_WORKER=1", "PATH="+fake+":"+os.Getenv("PATH"))
	if output, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(output), "does not match v24.19.0") {
		t.Fatalf("CI exact-version rejection: %v\n%s", err, output)
	}
	if err := os.WriteFile(filepath.Join(fake, "node"), []byte("#!/bin/sh\nif [ \"$1\" = --version ]; then echo v24.19.0; elif [ \"$1\" = - ]; then cat >/dev/null; echo fingerprint; else exit 0; fi\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Local NVM remains authoritative by default and provides pin guidance.
	nvm := filepath.Join(root, "nvm")
	if err := os.Mkdir(nvm, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvm, "nvm.sh"), []byte("nvm() { return 1; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("bash", script)
	cmd.Env = append(os.Environ(), "AWF_PI_TEST_ROOT="+root, "AWF_PI_TEST_NVM_DIR="+nvm, "AWF_PI_TEST_SKIP_NVM=0", "AWF_PI_TEST_WORKER=1", "PATH="+fake+":"+os.Getenv("PATH"))
	if output, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(output), "nvm install v24.19.0") {
		t.Fatalf("missing NVM guidance: %v\n%s", err, output)
	}
}

// invariant: tooling/quality-gates:pi-extension-container-gate (TestPiExtensionHostLaneGateWiring)
func TestPiExtensionHostLaneGateWiring(t *testing.T) {
	rawX, err := os.ReadFile("../../x")
	if err != nil {
		t.Fatalf("read x: %v", err)
	}
	script := string(rawX)
	if !strings.Contains(script, "usage: ./x pi-test <run>") || strings.Contains(script, "pi-test <run|reset>") {
		t.Error("./x pi-test must offer only run")
	}
	if !strings.Contains(script, "tools/pi-extension-test/run.sh \"$action\"") {
		t.Error("./x must invoke the host Pi runner")
	}
	raw, err := os.ReadFile("../../tools/pi-extension-test/run.sh")
	if err != nil {
		t.Fatalf("read host runner: %v", err)
	}
	sh := string(raw)
	pinned, err := os.ReadFile("../../.nvmrc")
	if err != nil || string(pinned) != "v24.19.0\n" {
		t.Errorf(".nvmrc = %q, err=%v", pinned, err)
	}
	for _, want := range []string{
		"nvm install $pinned_node", ".nvmrc", "npm ci --ignore-scripts",
		"package-lock.json", "node_modules", "node --version", "spawnSync(\"npm\"",
		"os.platform()", "os.arch()", "AWF_PI_TEST_SKIP_NVM", "lockrun", ".host-lane.lock", "AWF_PI_TEST_WORKER=1",
		"mktemp -d", "cp -a \"$root/.pi/extensions\"", "cp -a \"$root/.pi/agents\"",
		"cp -a \"$root/.pi/skills\"", "ln -s \"$tool_dir/node_modules\" \"$workspace/node_modules\"",
		"readFileSync(path)", "field(`file:", "bytes.length", "// @ts-nocheck", "tsc -p tools/pi-extension-test/tsconfig.json",
		"--statements=100 --lines=100 --functions=100 --branches=100", "tmp_marker=\"\"", "cleanup_worker", "readdirSync", "localeCompare",
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("host runner missing %q", want)
		}
	}
	for _, banned := range []string{"docker", "reset)", "Dockerfile", "sed -i", "sha256sum", "sort -z"} {
		if strings.Contains(strings.ToLower(sh), strings.ToLower(banned)) {
			t.Errorf("host runner retains %q", banned)
		}
	}
	if _, err := os.Stat("../../tools/pi-extension-test/container.sh"); !errors.Is(err, fs.ErrNotExist) {
		t.Error("container.sh must be removed")
	}
	if _, err := os.Stat("../../tools/pi-extension-test/Dockerfile"); !errors.Is(err, fs.ErrNotExist) {
		t.Error("Dockerfile must be removed")
	}
}

// The generated Pi extension carries `// @ts-nocheck` on the line after its
// provenance banner so adopter IDEs stay quiet without a resolvable
// `@types/node`, and the host gate strips that exact directive before
// `tsc` so the static type-check still covers the real extension code. Neither
// half stands alone: a missing strip leaves the lane green while `tsc` silently
// skips the file, so only this static assertion enforces the coupling.
//
// invariant: rendering/pi-workflows:pi-extension-editor-quiet-strip (TestPiExtensionEditorQuietStrip)
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
	raw, err := os.ReadFile("../../tools/pi-extension-test/run.sh")
	if err != nil {
		t.Fatalf("read run.sh: %v", err)
	}
	sh := string(raw)

	// Index inside the prepare heredoc only. Indexing the whole script would
	// match the explanatory comment above it, which names the superseded copy
	// command, and an ordering assertion anchored to prose cannot fail.
	prepStart := strings.Index(sh, "copy_workspace()")
	if prepStart < 0 {
		t.Fatal("run.sh must build a prepare command")
	}
	prepare := sh[prepStart:]
	prepEnd := strings.Index(prepare, "\n}")
	if prepEnd < 0 {
		t.Fatal("the workspace copy function must close")
	}
	prepare = prepare[:prepEnd]

	copyAt := strings.Index(prepare, `cp -a "$root/.pi/extensions"`)
	// Match the calls in the execution subshell, not either function declaration.
	executionStart := strings.Index(sh, "(\n  cd \"$workspace\"")
	if executionStart < 0 {
		t.Fatal("the host runner must execute the workspace subshell")
	}
	execution := sh[executionStart:]
	stripAt := strings.Index(execution, "\n  strip_ts_nocheck\n")
	compileAt := strings.Index(execution, "\n  node_modules/.bin/tsc -p tools/pi-extension-test/tsconfig.json")
	if copyAt < 0 {
		t.Fatal("the host runner must copy extension source into the ephemeral workspace")
	}
	if stripAt < 0 {
		t.Fatal("the host runner must strip exactly the ts-nocheck directive")
	}
	if compileAt < 0 {
		t.Fatal("the host runner must run the TypeScript compiler")
	}
	if stripAt >= compileAt {
		t.Errorf("the host runner must strip before tsc in the execution subshell, got strip=%d tsc=%d", stripAt, compileAt)
	}

	// The claim quantifies over EVERY governed extension file, so the scope that
	// feeds the strip is as load-bearing as the strip itself: narrowing the find
	// would leave some file unstripped while the sed literal above still matched.
	if !strings.Contains(sh, `const root = ".pi/extensions"`) || !strings.Contains(sh, "readdirSync(dir, { withFileTypes: true })") {
		t.Error("the portable strip must recursively cover every governed extension TypeScript file (ADR-0148)")
	}
}
