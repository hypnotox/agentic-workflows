package project

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
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

// ADR-0090: the committed example adopter is kept deterministic through ./x -
// sync re-renders it from source; check repository authority, note-, and
// test-gates it. The example is its own Go module so the enclosing ./...
// sweeps never see it; this test pins the wiring so it cannot be silently
// dropped.
//
// invariant: tooling/quality-gates:example-adopter-checked (TestExampleAdopterWiring)
// invariant: tooling/quality-gates:example-zero-notes (TestExampleAdopterWiring)
// invariant: tooling/quality-gates:example-module-isolated (TestExampleAdopterWiring)
func TestExampleAdopterWiring(t *testing.T) {
	assertExampleDecisionRouting(t)
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
		`out="$(cd examples/sundial && "$bindir/awf" check repo)"`,
		`grep -q '^note: '`,
		`(cd examples/sundial && go test ./...)`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("x lost the example-adopter step %q (ADR-0090)", want)
		}
	}
	failureStart := strings.Index(script, `if ! out="$(cd examples/sundial && "$bindir/awf" check repo)"; then`)
	if failureStart < 0 {
		t.Fatal("x lost the failing example-check branch")
	}
	failureEnd := strings.Index(script[failureStart:], "\n    fi")
	if failureEnd < 0 || !strings.Contains(script[failureStart:failureStart+failureEnd], "exit 1") {
		t.Fatal("a failed example check no longer fails ./x check")
	}
	exampleCfg, err := config.Load("../../examples/sundial/.awf")
	if err != nil {
		t.Fatalf("load example config: %v", err)
	}
	if exampleCfg.ProseGate == nil || !exampleCfg.ProseGate.Enabled || exampleCfg.MemoryCite == nil || !exampleCfg.MemoryCite.Enabled {
		t.Fatalf("example gates are not both enabled: prose=%+v memory=%+v", exampleCfg.ProseGate, exampleCfg.MemoryCite)
	}
	if _, err := os.Stat("../../examples/sundial/go.mod"); err != nil {
		t.Errorf("examples/sundial must stay its own Go module (ADR-0090): %v", err)
	}
}

func assertExampleDecisionRouting(t *testing.T) {
	t.Helper()
	read := func(path string) string {
		raw, err := os.ReadFile(filepath.Join("../../examples/sundial", path))
		if err != nil {
			t.Fatalf("read sundial %s: %v", path, err)
		}
		return string(raw)
	}
	for _, target := range []string{".pi", ".claude"} {
		writingPlans := read(filepath.Join(target, "skills", "sundial-writing-plans", "SKILL.md"))
		for _, want := range []string{"implementation directives", "paths, commands, task order, rollout batches, and ordinary test transactions"} {
			if !strings.Contains(writingPlans, want) {
				t.Errorf("%s writing-plans missing decision routing %q", target, want)
			}
		}
		proposingADR := read(filepath.Join(target, "skills", "sundial-proposing-adr", "SKILL.md"))
		for _, want := range []string{"remains meaningful after implementation", "preserve exactly the frontmatter emitted by `awf new adr`"} {
			if !strings.Contains(proposingADR, want) {
				t.Errorf("%s proposing-adr missing decision routing %q", target, want)
			}
		}
		reviewer := read(filepath.Join(target, "agents", "adr-reviewer.md"))
		for _, want := range []string{"post-implementation", "counterfactual", "mechanism itself is load-bearing", "reasoned finding"} {
			if !strings.Contains(reviewer, want) {
				t.Errorf("%s ADR reviewer missing semantic routing %q", target, want)
			}
		}
		if strings.Contains(reviewer, "## Doc-currency checklist") {
			t.Errorf("%s ADR reviewer retains implementation-inventory checklist", target)
		}
	}
	guide := read("AGENTS.md")
	if !strings.Contains(guide, "Route settled content by authority lifetime") || strings.Contains(guide, "<no value>") {
		t.Errorf("Sundial AGENTS decision routing is missing or contains unresolved-value residue")
	}
}

func TestSundialConfirmedEffortBoundary(t *testing.T) {
	readExample := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join("../../examples/sundial", path))
		if err != nil {
			t.Fatalf("read sundial %s: %v", path, err)
		}
		return string(raw)
	}
	assertOrdered := func(label, body string, wants ...string) {
		t.Helper()
		position := 0
		for _, want := range wants {
			next := strings.Index(body[position:], want)
			if next < 0 {
				t.Errorf("%s missing ordered phrase %q", label, want)
				return
			}
			position += next + len(want)
		}
	}
	assertBoundaryDoesNotCreate := func(label, body, boundary string) {
		t.Helper()
		start := strings.Index(body, boundary)
		if start < 0 {
			t.Errorf("%s lost boundary %q", label, boundary)
			return
		}
		if strings.Contains(body[start:], "awf effort new") {
			t.Errorf("%s creates missing ownership after %q", label, boundary)
		}
	}
	assertDiscoveryOwner := func(label, body, mutationBoundary string) {
		t.Helper()
		assertOrdered(label, body,
			"**Mandatory first-creation confirmation.**",
			"Discovery creates no effort",
			"`Outcome: <concrete non-minimal outcome>`",
			"`Effort title: <proposed title>`",
			"`Effort slug: <proposed-short-slug>`",
			"Ask the user to confirm creation",
			"end the turn without creating an effort",
			"clear response in a later turn",
			"confirms all three fields",
			"awf effort new --slug <confirmed-slug> \"<confirmed-title>\"",
		)
		if !strings.Contains(body, "fixed identity") || !strings.Contains(body, "without title reconfirmation") {
			t.Errorf("%s does not preserve fixed-identity resume without reconfirmation", label)
		}
		creation := strings.Index(body, "awf effort new --slug <confirmed-slug> \"<confirmed-title>\"")
		mutation := strings.Index(body, mutationBoundary)
		if mutationBoundary == "" || mutation < 0 || creation < 0 || creation >= mutation {
			t.Errorf("%s must complete confirmed creation before mutation boundary %q", label, mutationBoundary)
		}
	}
	assertDownstream := func(label, body, boundary string) {
		t.Helper()
		if strings.Contains(body, "awf effort new") {
			t.Errorf("%s creates an effort instead of requiring confirmed ownership", label)
		}
		end := strings.Index(body, boundary)
		if end < 0 {
			t.Errorf("%s lost pre-mutation boundary %q", label, boundary)
			return
		}
		preMutation := strings.ToLower(body[:end])
		if !strings.Contains(preMutation, "already-confirmed") && !strings.Contains(preMutation, "existing confirmed effort") {
			t.Errorf("%s pre-mutation contract does not establish confirmed ownership", label)
		}
		for _, want := range []string{"mandatory first-creation three-field confirmation", "never creates a missing effort"} {
			if !strings.Contains(preMutation, want) {
				t.Errorf("%s pre-mutation ownership contract is missing %q", label, want)
			}
		}
		if !strings.Contains(preMutation, "fixed identity") || !strings.Contains(preMutation, "without title reconfirmation") {
			t.Errorf("%s pre-mutation contract does not preserve fixed-identity resume without reconfirmation", label)
		}
	}
	for _, target := range []string{".pi", ".claude"} {
		readSkill := func(name string) string {
			return readExample(filepath.Join(target, "skills", "sundial-"+name, "SKILL.md"))
		}
		brainstorming := readSkill("brainstorming")
		assertDiscoveryOwner(target+" brainstorming", brainstorming, "5. **Present the design in sections")
		assertBoundaryDoesNotCreate(target+" brainstorming final approval", brainstorming, "**Mandatory approval check-in.**")
		reviewingADR := readSkill("reviewing-adr")
		assertBoundaryDoesNotCreate(target+" ADR final approval", reviewingADR, "**Mandatory approval check-in.**")
		writingPlans := readSkill("writing-plans")
		assertBoundaryDoesNotCreate(target+" routine checkpoint", writingPlans, "**Routine checkpoint.**")
		discoveryBoundaries := map[string]string{
			"debugging":          "5. **Isolate with a failing test",
			"roadmap-graduation": "### 4. Graduate in a single commit",
		}
		for name, boundary := range discoveryBoundaries {
			assertDiscoveryOwner(target+" "+name, readSkill(name), boundary)
		}
		downstreamBoundaries := map[string]string{
			"tdd":           "1. Run `awf context",
			"proposing-adr": "1. **Scaffold the file",
			"writing-plans": "1. **Confirm scope with the user",
		}
		for name, boundary := range downstreamBoundaries {
			assertDownstream(target+" "+name, readSkill(name), boundary)
		}
		orienting := readSkill("orienting")
		if strings.Contains(orienting, "awf effort new") || !strings.Contains(orienting, "never creates an effort") || !strings.Contains(orienting, "fixed identity") {
			t.Errorf("%s orienting must never create and must validate fixed-identity resume", target)
		}
		exploring := readSkill("exploring")
		if strings.Contains(exploring, "awf effort new") || !strings.Contains(exploring, "never creates an effort") || !strings.Contains(exploring, "report-only") {
			t.Errorf("%s exploring must never create and must remain report-only", target)
		}

		entries, err := os.ReadDir(filepath.Join("../../examples/sundial", target, "skills"))
		if err != nil {
			t.Fatalf("list sundial %s skills: %v", target, err)
		}
		for _, entry := range entries {
			body := readExample(filepath.Join(target, "skills", entry.Name(), "SKILL.md"))
			if strings.Contains(body, "<no value>") {
				t.Errorf("%s/%s contains an unresolved-value token", target, entry.Name())
			}
		}
	}
	for _, target := range []string{".pi", ".claude"} {
		path := filepath.Join("../..", target, "skills", "awf-retrospective", "SKILL.md")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read project retrospective %s: %v", target, err)
		}
		assertDownstream("project "+target+" retrospective", string(raw), "2. **Reflect and record worthy observations")
	}
	workflow := readExample("docs/workflow.md")
	for _, want := range []string{"Discovery creates no effort", "existing effort resumes under its fixed identity", "newly discovered outcome cannot silently reuse"} {
		if !strings.Contains(workflow, want) {
			t.Errorf("sundial workflow missing %q", want)
		}
	}
	guide := readExample("AGENTS.md")
	for _, want := range []string{"proposed effort title", "proposed short effort slug", "clear response in a later turn confirming all three fields", "`awf effort new --slug <confirmed-slug> \"<confirmed-title>\"`", "only for work inside its confirmed outcome"} {
		if !strings.Contains(guide, want) {
			t.Errorf("sundial guide missing %q", want)
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
	for _, dropped := range []string{"activeMdRegenCmd", "checkCmd", "commitGateCmd"} {
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

	runnerRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(runnerRoot, "x"), x, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeBin := filepath.Join(runnerRoot, "bin")
	if err := os.Mkdir(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(runnerRoot, "go.log")
	fakeGo := "#!/usr/bin/env bash\nprintf '%s\\n' \"$*\" >>\"$GO_LOG\"\n"
	if err := os.WriteFile(filepath.Join(fakeBin, "go"), []byte(fakeGo), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (int, string) {
		t.Helper()
		if err := os.WriteFile(logPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", append([]string{"./x"}, args...)...)
		cmd.Dir = runnerRoot
		cmd.Env = append(os.Environ(), "PATH="+fakeBin+":"+os.Getenv("PATH"), "GO_LOG="+logPath)
		stderr := new(strings.Builder)
		cmd.Stderr = stderr
		err := cmd.Run()
		if err == nil {
			return 0, stderr.String()
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), stderr.String()
		}
		t.Fatalf("run Sundial x: %v", err)
		return 0, ""
	}
	for _, args := range [][]string{{"gate", "full"}, {"gate", "unknown"}, {"gate", "extra", "arg"}} {
		status, stderr := run(args...)
		logged, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		if status != 2 || !strings.Contains(stderr, "usage:") || len(logged) != 0 {
			t.Errorf("Sundial x %v: status=%d stderr=%q log=%q", args, status, stderr, logged)
		}
	}
	if status, stderr := run("test", "-run", "TestOne"); status != 0 || stderr != "" {
		t.Errorf("Sundial x test forwarding: status=%d stderr=%q", status, stderr)
	}
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(logged) != "test ./... -run TestOne\n" {
		t.Errorf("Sundial x test log=%q", logged)
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
