package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// hookFiles renders a project with the given config and returns the
// .awf/hooks/*.sh RenderedFiles keyed by payload name.
func hookFiles(t *testing.T, configYAML string) map[string]RenderedFile {
	t.Helper()
	root := scaffold(t, configYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	out, err := renderAll(p)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]RenderedFile{}
	for _, f := range out {
		if rest, ok := strings.CutPrefix(f.Path, ".awf/hooks/"); ok {
			found[strings.TrimSuffix(rest, ".sh")] = f
		}
	}
	return found
}

// Exactly five payloads always render under .awf/hooks/. The expected set is spelled out
// rather than derived from hookNames, which would make the assertion agree with
// whatever that list happens to say: the claim names these paths, so the test
// has to name them too for a wrong set to be able to fail.
// invariant: rendering/singletons-and-payloads:hook-payloads-rendered (TestHookPayloadsRendered)
func TestHookPayloadsRendered(t *testing.T) {
	want := []string{"pre-commit", "commit-msg", "pre-push", "pre-merge-commit", "reference-transaction"}
	got := hookFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	for _, name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("expected .awf/hooks/%s.sh to render when enabled", name)
		}
	}
	if len(got) != len(want) {
		t.Errorf("rendered %d payloads, want exactly %d: %v", len(got), len(want), got)
	}
}

// With optional command vars unset, each payload remains runnable through the
// always-rendered ./awf wrapper, with no inline resolution shim or unresolved token.
// invariant: rendering/companion-scripts:hook-payloads-fallback-safe (TestHookPayloadsFallbackSafe)
func TestHookPayloadsFallbackSafe(t *testing.T) {
	for _, tc := range []struct {
		name, config, awf string
	}{
		{"runner always rendered", "prefix: example\nprofile: full\nintegrationBranch: main\n", "./awf"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := hookFiles(t, tc.config)
			wantCmds := map[string][]string{
				"pre-commit":            {tc.awf + " check\n"},
				"commit-msg":            {tc.awf + ` check staged commit "$1"` + "\n"},
				"pre-push":              {"test-gate\n"},
				"pre-merge-commit":      {tc.awf + " check staged\n"},
				"reference-transaction": {"  " + tc.awf + ` check commit-policy "${targets[@]}"`},
			}
			for name, f := range got {
				lines := strings.Split(f.Content, "\n")
				if lines[0] != "#!/usr/bin/env bash" {
					t.Errorf("%s: first line = %q, want the shebang", name, lines[0])
				}
				if !strings.Contains(f.Content, "set -euo pipefail") {
					t.Errorf("%s: missing set -euo pipefail", name)
				}
				for _, want := range wantCmds[name] {
					if !strings.Contains(f.Content, "\n"+want) {
						t.Errorf("%s: missing fallback command %q:\n%s", name, want, f.Content)
					}
				}
				if strings.Contains(f.Content, "awf() {") {
					t.Errorf("%s: payloads carry no inline resolution shim:\n%s", name, f.Content)
				}
				if strings.Contains(f.Content, "<no value>") {
					t.Errorf("%s: unresolved-value token in output:\n%s", name, f.Content)
				}
			}
		})
	}
}

// With the command vars set, each payload runs them verbatim and omits the
// pin-aware shim.
func TestHookPayloadsUseConfiguredCommands(t *testing.T) {
	got := hookFiles(t, `prefix: example
integrationBranch: main
vars:
  checkCmd: ./x check
  gateCmd: ./x gate
  gateCmdFull: ./x gate full
  commitGateCmd: ./x commit-gate
`)
	want := map[string][]string{
		"pre-commit":            {"./x check\n./x gate\n"},
		"commit-msg":            {"./x commit-gate \"$1\"\n"},
		"pre-push":              {`./x gate full "${ranges[@]+"${ranges[@]}"}"`},
		"pre-merge-commit":      {"./x check staged\n"},
		"reference-transaction": {"  ./awf check commit-policy \"${targets[@]}\""},
	}
	for name, f := range got {
		for _, w := range want[name] {
			if !strings.Contains(f.Content, w) {
				t.Errorf("%s: missing %q:\n%s", name, w, f.Content)
			}
		}
		if strings.Contains(f.Content, "bootstrap.sh") {
			t.Errorf("%s: pin-aware shim should be omitted when commands are set:\n%s", name, f.Content)
		}
	}
	// pre-push falls back through the chain: gateCmd when gateCmdFull is unset.
	chain := hookFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars:\n  gateCmd: ./x gate\n")
	if f := chain["pre-push"]; !strings.Contains(f.Content, "./x gate\n") {
		t.Errorf("pre-push: want gateCmd fallback, got:\n%s", f.Content)
	}
}

func TestIntegrationBranchReflagsPolicyConsumers(t *testing.T) {
	main := hookFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	release := hookFiles(t, "prefix: example\nprofile: full\nintegrationBranch: release/next\n")
	consumers := map[string]bool{"pre-push": true, "reference-transaction": true}
	for name := range consumers {
		if main[name].ConfigHash == release[name].ConfigHash || main[name].Content == release[name].Content {
			t.Errorf("integrationBranch change did not reflag and regenerate %s", name)
		}
		if !strings.Contains(release[name].Content, "integration_branch_hex='72656c656173652f6e657874'") {
			t.Errorf("release integration branch not projected safely into %s:\n%s", name, release[name].Content)
		}
	}
	for name := range main {
		if consumers[name] {
			continue
		}
		if main[name].ConfigHash != release[name].ConfigHash || main[name].Content != release[name].Content {
			t.Errorf("integrationBranch change reflagged unrelated %s payload", name)
		}
	}
}

// The policy payloads buffer complete hook input before invoking the common
// verifier, evaluate only commit-bearing branch or tag targets, and run the
// configured pre-push gate only after policy success.
// invariant: rendering/singletons-and-payloads:commit-policy-hook-payloads (TestCommitPolicyHookPayloads)
func TestCommitPolicyHookPayloads(t *testing.T) {
	got := hookFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars:\n  gateCmdFull: ./x gate full\n")
	transaction := got["reference-transaction"].Content
	for _, want := range []string{
		`[[ "${1:-}" == "prepared" ]] || exit 0`,
		`while IFS= read -r update || [[ -n "$update" ]]; do updates+=("$update"); done`,
		`refs/heads/*`,
		`integration_branch_hex='6d61696e'`,
		`"$old_oid..$new_oid"`,
		`check commit-policy "${targets[@]}"`,
		`refs changed: false`,
		`policy verifier changed index: false`,
		`index/worktree may already have changed before this hook`,
	} {
		if !strings.Contains(transaction, want) {
			t.Errorf("reference-transaction missing %q:\n%s", want, transaction)
		}
	}
	if strings.Contains(transaction, "git ls-remote") {
		t.Errorf("reference-transaction must resolve integration evidence locally:\n%s", transaction)
	}
	push := got["pre-push"].Content
	for _, want := range []string{
		`while IFS= read -r update || [[ -n "$update" ]]; do updates+=("$update"); done`,
		`git cat-file -t "$object"`,
		`git rev-parse --verify "$object^{}"`,
		`git ls-remote`,
		`integration_branch_hex='6d61696e'`,
		`resolves to non-commit`,
		`check commit-policy "${policy_targets[@]}"`,
		"./x gate full",
	} {
		if !strings.Contains(push, want) {
			t.Errorf("pre-push missing %q:\n%s", want, push)
		}
	}
	if policy, gate := strings.Index(push, `check commit-policy "${policy_targets[@]}"`), strings.Index(push, "./x gate full"); policy < 0 || gate < policy {
		t.Errorf("pre-push must run policy before gate:\n%s", push)
	}
	for name, payload := range map[string]string{"reference-transaction": transaction, "pre-push": push} {
		for _, forbidden := range []string{"mapfile", "declare -A"} {
			if strings.Contains(payload, forbidden) {
				t.Errorf("%s requires Bash 4 builtin %q:\n%s", name, forbidden, payload)
			}
		}
	}

	root := t.TempDir()
	runHookGit(t, root, "init")
	runHookGit(t, root, "config", "user.name", "Hook Test")
	runHookGit(t, root, "config", "user.email", "hook@example.test")
	runHookGit(t, root, "commit", "--allow-empty", "-m", "base")
	base := strings.TrimSpace(runHookGit(t, root, "rev-parse", "HEAD"))
	runHookGit(t, root, "commit", "--allow-empty", "-m", "update")
	head := strings.TrimSpace(runHookGit(t, root, "rev-parse", "HEAD"))
	runHookGit(t, root, "branch", "main", base)
	runHookGit(t, root, "tag", "-a", "pushed", "-m", "pushed")
	tag := strings.TrimSpace(runHookGit(t, root, "rev-parse", "refs/tags/pushed"))
	remote := filepath.Join(t.TempDir(), "remote.git")
	if err := os.Mkdir(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	runHookGit(t, remote, "init", "--bare")
	runHookGit(t, root, "remote", "add", "origin", remote)
	runHookGit(t, root, "push", "origin", base+":refs/heads/main")

	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(root, "hook.log")
	for name, body := range map[string]string{
		"awf":  "#!/usr/bin/env bash\nprintf 'policy:%s\\n' \"$*\" >>\"$HOOK_LOG\"\n[[ ! -e \"$HOOK_FAIL\" ]]\n",
		"gate": "#!/usr/bin/env bash\nprintf 'gate:%s\\n' \"$*\" >>\"$HOOK_LOG\"\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		if name == "awf" {
			if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	writeHook := func(name, content string) string {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	transactionPath := writeHook("reference-transaction", transaction)
	pushPath := writeHook("pre-push", hookFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars:\n  gateCmdFull: gate\n")["pre-push"].Content)
	bashEnv := filepath.Join(root, "bash3-env")
	if err := os.WriteFile(bashEnv, []byte("enable -n mapfile\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(path, input string, args ...string) (string, error) {
		t.Helper()
		if path == pushPath && len(args) == 0 {
			args = []string{"origin", remote}
		}
		cmd := exec.Command("bash", append([]string{path}, args...)...)
		cmd.Dir, cmd.Stdin = root, strings.NewReader(input)
		cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "HOOK_LOG="+log, "HOOK_FAIL="+filepath.Join(root, "fail-policy"), "BASH_ENV="+bashEnv)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	readLog := func() string {
		t.Helper()
		b, err := os.ReadFile(log)
		if os.IsNotExist(err) {
			return ""
		}
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	clearLog := func() {
		t.Helper()
		if err := os.Remove(log); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}

	if _, err := run(transactionPath, "bad record\n", "committed"); err != nil || readLog() != "" {
		t.Fatalf("non-prepared transaction evaluated policy: err=%v log=%q", err, readLog())
	}
	before := strings.TrimSpace(runHookGit(t, root, "rev-parse", "refs/heads/master"))
	if output, err := run(transactionPath, "bad record\n", "prepared"); err == nil || !strings.Contains(output, "refs changed: false") {
		t.Fatalf("malformed prepared transaction: err=%v output=%q", err, output)
	}
	if output, err := run(transactionPath, "bad record", "prepared"); err == nil || !strings.Contains(output, "refs changed: false") {
		t.Fatalf("unterminated malformed prepared transaction: err=%v output=%q", err, output)
	}
	if after := strings.TrimSpace(runHookGit(t, root, "rev-parse", "refs/heads/master")); after != before {
		t.Fatalf("rejected transaction moved ref: before=%s after=%s", before, after)
	}
	if output, err := run(transactionPath, "bad "+head+" refs/tags/malformed\n", "prepared"); err == nil || !strings.Contains(output, "malformed object ID") || readLog() != "" {
		t.Fatalf("malformed non-branch record: err=%v output=%q log=%q", err, output, readLog())
	}
	if output, err := run(transactionPath, strings.Repeat("0", len(head))+" ref:refs/heads/master HEAD\n", "prepared"); err != nil || readLog() != "" {
		t.Fatalf("valid symbolic pseudoref record: err=%v output=%q log=%q", err, output, readLog())
	}
	clearLog()
	if output, err := run(transactionPath, base+" "+head+" refs/heads/master\n"+base+" "+head+" refs/heads/duplicate\n", "prepared"); err != nil {
		t.Fatalf("prepared update: %v: %s", err, output)
	}
	if got := readLog(); !strings.Contains(got, "policy:check commit-policy "+base+".."+head) || strings.Count(got, base+".."+head) != 1 {
		t.Fatalf("prepared update policy target = %q", got)
	}
	clearLog()
	if output, err := run(transactionPath, head+" "+base+" refs/heads/master\n", "prepared"); err != nil || readLog() != "" {
		t.Fatalf("backward update evaluated policy: err=%v output=%q log=%q", err, output, readLog())
	}
	zeroOID := strings.Repeat("0", len(head))
	clearLog()
	if output, err := run(transactionPath, head+" "+zeroOID+" refs/heads/deleted\n", "prepared"); err != nil || readLog() != "" {
		t.Fatalf("deletion evaluated policy: err=%v output=%q log=%q", err, output, readLog())
	}
	clearLog()
	if output, err := run(transactionPath, zeroOID+" "+head+" refs/heads/created\n", "prepared"); err != nil || readLog() != "policy:check commit-policy "+base+".."+head+"\n" {
		t.Fatalf("new branch target: err=%v output=%q log=%q", err, output, readLog())
	}
	clearLog()
	if output, err := run(transactionPath, base+" "+head+" refs/heads/main\n"+zeroOID+" "+head+" refs/heads/created\n", "prepared"); err != nil || readLog() != "policy:check commit-policy "+base+".."+head+"\n" {
		t.Fatalf("same-transaction integration update changed new-branch base: err=%v output=%q log=%q", err, output, readLog())
	}
	runHookGit(t, root, "branch", "-D", "main")
	clearLog()
	if output, err := run(transactionPath, zeroOID+" "+head+" refs/heads/created\n", "prepared"); err == nil || !strings.Contains(output, "local integration branch") || readLog() != "" {
		t.Fatalf("missing local integration branch evidence: err=%v output=%q log=%q", err, output, readLog())
	}
	clearLog()
	if output, err := run(transactionPath, base+" "+head+" refs/heads/master\n", "prepared"); err != nil || readLog() != "policy:check commit-policy "+base+".."+head+"\n" {
		t.Fatalf("existing branch unnecessarily required integration evidence: err=%v output=%q log=%q", err, output, readLog())
	}
	runHookGit(t, root, "branch", "main", base)
	tree := strings.TrimSpace(runHookGit(t, root, "rev-parse", "HEAD^{tree}"))
	side := strings.TrimSpace(runHookGit(t, root, "commit-tree", tree, "-p", base, "-m", "side"))
	clearLog()
	if output, err := run(transactionPath, head+" "+side+" refs/heads/diverged\n", "prepared"); err != nil || readLog() != "policy:check commit-policy "+head+".."+side+"\n" {
		t.Fatalf("divergent branch target: err=%v output=%q log=%q", err, output, readLog())
	}

	runHookGit(t, root, "tag", "-a", "outer-pushed", "-m", "outer", "pushed")
	outerTag := strings.TrimSpace(runHookGit(t, root, "rev-parse", "refs/tags/outer-pushed"))
	clearLog()
	if output, err := run(pushPath, "refs/tags/outer-pushed "+outerTag+" refs/tags/outer-pushed "+base+"\nrefs/tags/pushed "+tag+" refs/tags/duplicate "+base+"\n"); err != nil {
		t.Fatalf("tag push: %v: %s", err, output)
	}
	if got := readLog(); got != "policy:check commit-policy "+base+".."+head+"\ngate:--range "+base+" "+outerTag+" --range "+base+" "+tag+"\n" {
		t.Fatalf("tag push argv/order = %q", got)
	}
	clearLog()
	if output, err := run(pushPath, "(delete) "+zeroOID+" refs/heads/deleted "+head+"\n"); err != nil || readLog() != "gate:--range "+head+" "+head+"\n" {
		t.Fatalf("push deletion: err=%v output=%q log=%q", err, output, readLog())
	}
	clearLog()
	blob := strings.TrimSpace(runHookGit(t, root, "hash-object", "-w", "--stdin"))
	runHookGit(t, root, "tag", "-a", "annotated-blob", "-m", "blob", blob)
	annotatedBlob := strings.TrimSpace(runHookGit(t, root, "rev-parse", "refs/tags/annotated-blob"))
	if output, err := run(pushPath, "refs/tags/annotated-blob "+annotatedBlob+" refs/tags/annotated-blob "+base+"\n"); err != nil || !strings.Contains(output, "resolves to non-commit blob") || readLog() != "gate:--range "+base+" "+annotatedBlob+"\n" {
		t.Fatalf("non-commit tag: err=%v output=%q log=%q", err, output, readLog())
	}
	clearLog()
	missing := strings.Repeat("f", len(head))
	if output, err := run(pushPath, "refs/heads/missing "+missing+" refs/heads/missing "+base+"\n"); err == nil || !strings.Contains(output, "names missing object") || readLog() != "" {
		t.Fatalf("missing push object: err=%v output=%q log=%q", err, output, readLog())
	}
	brokenTag := gitfixture.NativeHashObject(t, gitfixture.At(root), "tag", []byte("object "+missing+"\ntype commit\ntag broken\ntagger Hook Test <hook@example.test> 1 +0000\n\nbroken\n"))
	if output, err := run(pushPath, "refs/tags/broken "+brokenTag+" refs/tags/broken "+base+"\n"); err == nil || !strings.Contains(output, "cannot be peeled") || readLog() != "" {
		t.Fatalf("broken push tag: err=%v output=%q log=%q", err, output, readLog())
	}
	clearLog()
	if output, err := run(pushPath, "refs/heads/master "+head+" refs/heads/master "+base+"\n"); err != nil {
		t.Fatalf("ordinary push: %v: %s", err, output)
	}
	if got := readLog(); got != "policy:check commit-policy "+base+".."+head+"\ngate:--range "+base+" "+head+"\n" {
		t.Fatalf("ordinary push argv/order = %q", got)
	}
	clearLog()
	if output, err := run(pushPath, "refs/heads/force "+side+" refs/heads/force "+head+"\n"); err != nil {
		t.Fatalf("force push: %v: %s", err, output)
	}
	if got := readLog(); got != "policy:check commit-policy "+head+".."+side+"\ngate:--range "+head+" "+side+"\n" {
		t.Fatalf("force push argv/order = %q", got)
	}
	clearLog()
	if output, err := run(pushPath, "refs/heads/master "+head+" refs/heads/master "+base+"\nrefs/heads/other "+head+" refs/heads/other "+base+"\n"); err != nil {
		t.Fatalf("multi-ref push: %v: %s", err, output)
	}
	if got := readLog(); got != "policy:check commit-policy "+base+".."+head+"\ngate:--range "+base+" "+head+" --range "+base+" "+head+"\n" {
		t.Fatalf("multi-ref push argv/order = %q", got)
	}
	clearLog()
	if output, err := run(pushPath, "refs/heads/new "+head+" refs/heads/new "+zeroOID+"\n"); err != nil {
		t.Fatalf("new branch push: %v: %s", err, output)
	}
	if got := readLog(); got != "policy:check commit-policy "+base+".."+head+"\ngate:--range invalid-base "+head+"\n" {
		t.Fatalf("new branch conservative argv/order = %q", got)
	}
	clearLog()
	if output, err := run(pushPath, "refs/tags/new "+outerTag+" refs/tags/new "+zeroOID+"\n"); err != nil {
		t.Fatalf("new annotated tag push: %v: %s", err, output)
	}
	if got := readLog(); got != "policy:check commit-policy "+base+".."+head+"\ngate:--range invalid-base "+outerTag+"\n" {
		t.Fatalf("new annotated tag argv/order = %q", got)
	}
	clearLog()
	runHookGit(t, root, "push", "origin", ":refs/heads/main")
	if output, err := run(pushPath, "refs/heads/new "+head+" refs/heads/new "+zeroOID+"\n"); err == nil || !strings.Contains(output, "destination integration branch") || readLog() != "" {
		t.Fatalf("missing integration tip: err=%v output=%q log=%q", err, output, readLog())
	}
	runHookGit(t, root, "push", "origin", base+":refs/heads/main")
	remoteTree := strings.TrimSpace(runHookGit(t, remote, "rev-parse", base+"^{tree}"))
	runHookGit(t, remote, "config", "user.name", "Remote Hook Test")
	runHookGit(t, remote, "config", "user.email", "remote-hook@example.test")
	remoteOnly := strings.TrimSpace(runHookGit(t, remote, "commit-tree", remoteTree, "-p", base, "-m", "remote-only"))
	runHookGit(t, remote, "update-ref", "refs/heads/main", remoteOnly)
	if _, err := gitfixture.NativeRun(gitfixture.At(root), "cat-file", "-e", remoteOnly+"^{commit}"); err == nil {
		t.Fatalf("remote-only integration tip %s unexpectedly exists locally", remoteOnly)
	}
	if output, err := run(pushPath, "refs/heads/new "+head+" refs/heads/new "+zeroOID+"\n"); err == nil || !strings.Contains(output, "names unresolvable commit "+remoteOnly) || readLog() != "" {
		t.Fatalf("locally unavailable integration tip: err=%v output=%q log=%q", err, output, readLog())
	}
	runHookGit(t, remote, "update-ref", "refs/heads/main", base)
	if output, err := run(pushPath, "refs/heads/new "+head+" refs/heads/new "+zeroOID+"\n", "origin", filepath.Join(t.TempDir(), "missing-remote")); err == nil || !strings.Contains(output, "destination integration branch") || readLog() != "" {
		t.Fatalf("unavailable integration remote: err=%v output=%q log=%q", err, output, readLog())
	}
	clearLog()
	if output, err := run(pushPath, "(delete) "+zeroOID+" refs/heads/deleted "+head+"\n"); err != nil {
		t.Fatalf("deletion-only push: %v: %s", err, output)
	}
	if got := readLog(); got != "gate:--range "+head+" "+head+"\n" {
		t.Fatalf("deletion-only exact-empty argv = %q", got)
	}

	clearLog()
	if err := os.WriteFile(filepath.Join(root, "fail-policy"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(pushPath, "refs/heads/master "+head+" refs/heads/master "+base+"\n"); err == nil || strings.Contains(readLog(), "gate") {
		t.Fatalf("failed policy ran gate: err=%v log=%q", err, readLog())
	}
	if err := os.Remove(filepath.Join(root, "fail-policy")); err != nil {
		t.Fatal(err)
	}
	clearLog()
	if _, err := run(pushPath, "malformed\n"); err == nil || readLog() != "" {
		t.Fatalf("malformed push ran policy or gate: err=%v log=%q", err, readLog())
	}
	if _, err := run(pushPath, "malformed"); err == nil || readLog() != "" {
		t.Fatalf("unterminated malformed push ran policy or gate: err=%v log=%q", err, readLog())
	}

	t.Run("native-git-enforcement", func(t *testing.T) {
		testCommitPolicyHooksNative(t)
	})
}

func testCommitPolicyHooksNative(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	binary := filepath.Join(binDir, "awf")
	build := exec.Command("go", "build", "-o", binary, "./cmd/awf")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build awf: %v: %s", err, output)
	}

	for _, hooksPath := range []string{".githooks", filepath.Join(t.TempDir(), "shared-hooks")} {
		hooksPath := hooksPath
		t.Run(map[bool]string{true: "relative", false: "absolute"}[!filepath.IsAbs(hooksPath)], func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
			fixture := gitfixture.InitNativeAt(t, root)
			run := func(wantOK bool, args ...string) string {
				t.Helper()
				output, err := gitfixture.NativeRun(fixture, args...)
				if (err == nil) != wantOK {
					t.Fatalf("git %v: wantOK=%v err=%v\n%s", args, wantOK, err, output)
				}
				return output
			}
			run(true, "config", "user.name", "Allowed")
			run(true, "config", "user.email", "allowed@example.test")
			if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
				t.Fatal(err)
			}
			wrapper := "#!/usr/bin/env bash\nexec " + shellQuote(binary) + " \"$@\"\n"
			if err := os.WriteFile(filepath.Join(root, "awf"), []byte(wrapper), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte("prefix: hook-test\nprofile: full\nintegrationBranch: master\nvars: {gateCmd: true}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".awf", "awf.lock"), []byte("{\"awfVersion\":\"0.36.0\",\"schemaVersion\":46,\"files\":{\"prior\":{}}}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			run(true, "add", ".awf/config.yaml", ".awf/awf.lock")
			run(true, "commit", "--no-gpg-sign", "-m", "baseline")
			base := strings.TrimSpace(run(true, "rev-parse", "HEAD"))
			privateKey, publicKey := nativeHookSSHKey(t)

			gateLog := filepath.Join(root, "gate.log")
			gate := filepath.Join(root, "gate")
			if err := os.WriteFile(gate, []byte("#!/usr/bin/env bash\nprintf 'gate\\n' >>"+shellQuote(gateLog)+"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			config := "prefix: hook-test\nprofile: full\nintegrationBranch: master\nvars:\n  gateCmd: true\n  gateCmdFull: " + gate + "\ncommitPolicy:\n  grandfatheredThrough: " + base + "\n  allowedIdentities:\n    - name: Allowed\n      email: allowed@example.test\n  requireSignedCommits: true\n  allowedSigners:\n    - principal: allowed@example.test\n      key: " + publicKey + "\n"
			if err := os.MkdirAll(filepath.Join(root, ".awf", "hooks"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(config), 0o644); err != nil {
				t.Fatal(err)
			}
			files := hookFiles(t, "prefix: hook-test\nprofile: full\nintegrationBranch: master\nvars:\n  gateCmdFull: "+gate+"\n")
			for _, name := range []string{"reference-transaction", "pre-push"} {
				if err := os.WriteFile(filepath.Join(root, ".awf", "hooks", name+".sh"), []byte(files[name].Content), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			stubRoot := hooksPath
			if !filepath.IsAbs(stubRoot) {
				stubRoot = filepath.Join(root, stubRoot)
			}
			if err := os.MkdirAll(stubRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"reference-transaction", "pre-push"} {
				stub := "#!/usr/bin/env bash\nrepo_root=$(git rev-parse --show-toplevel) || exit 1\npayload=\"$repo_root/.awf/hooks/" + name + ".sh\"\n"
				if name == "reference-transaction" {
					stub += "if [[ ! -f \"$payload\" ]]; then\n  primary_root=$(git worktree list --porcelain | sed -n '1s/^worktree //p') || exit 1\n  payload=\"$primary_root/.awf/hooks/reference-transaction.sh\"\nfi\n"
				}
				stub += "exec bash \"$payload\" \"$@\"\n"
				if err := os.WriteFile(filepath.Join(stubRoot, name), []byte(stub), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			run(true, "config", "core.hooksPath", hooksPath)
			run(true, "config", "gpg.format", "ssh")
			run(true, "config", "user.signingKey", privateKey)
			run(true, "config", "commit.gpgSign", "true")

			run(true, "commit", "--allow-empty", "-S", "-m", "allowed")
			allowed := strings.TrimSpace(run(true, "rev-parse", "HEAD"))

			// A new local branch at the integration tip introduces no commit even
			// when inherited history contains an accepted policy exception.
			run(true, "-c", "core.hooksPath=/dev/null", "commit", "--allow-empty", "--no-gpg-sign", "-m", "inherited exception")
			inheritedException := strings.TrimSpace(run(true, "rev-parse", "HEAD"))
			linked := filepath.Join(t.TempDir(), "linked")
			run(true, "worktree", "add", "-b", "linked-policy", linked, inheritedException)
			run(true, "-c", "core.hooksPath=/dev/null", "reset", "--hard", allowed)
			if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(config), 0o644); err != nil {
				t.Fatal(err)
			}
			linkedConfig := strings.Replace(config, "    - name: Allowed\n      email: allowed@example.test", "    - name: Linked\n      email: linked@example.test", 1)
			linkedConfig = strings.Replace(linkedConfig, "    - principal: allowed@example.test", "    - principal: linked@example.test", 1)
			if err := os.WriteFile(filepath.Join(linked, ".awf", "config.yaml"), []byte(linkedConfig), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(linked, ".awf", "hooks"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(linked, "awf"), []byte(wrapper), 0o755); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"reference-transaction", "pre-push"} {
				if err := os.WriteFile(filepath.Join(linked, ".awf", "hooks", name+".sh"), []byte(files[name].Content), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if !filepath.IsAbs(hooksPath) {
				linkedStubs := filepath.Join(linked, hooksPath)
				if err := os.MkdirAll(linkedStubs, 0o755); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"reference-transaction", "pre-push"} {
					body, err := os.ReadFile(filepath.Join(stubRoot, name))
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(linkedStubs, name), body, 0o755); err != nil {
						t.Fatal(err)
					}
				}
			}
			linkedFixture := gitfixture.At(linked)
			linkedRun := func(wantOK bool, args ...string) string {
				t.Helper()
				output, err := gitfixture.NativeRun(linkedFixture, args...)
				if (err == nil) != wantOK {
					t.Fatalf("linked git %v: wantOK=%v err=%v\n%s", args, wantOK, err, output)
				}
				return output
			}
			if output := linkedRun(false, "-c", "user.name=Allowed", "-c", "user.email=allowed@example.test", "-c", "gpg.format=ssh", "-c", "user.signingKey="+privateKey, "commit", "--allow-empty", "-S", "-m", "wrong worktree policy"); !strings.Contains(output, "author | Allowed <allowed@example.test>") {
				t.Fatalf("linked worktree used wrong policy: %q", output)
			}
			linkedRun(true, "-c", "user.name=Linked", "-c", "user.email=linked@example.test", "-c", "gpg.format=ssh", "-c", "user.signingKey="+privateKey, "commit", "--allow-empty", "-S", "-m", "linked allowed")

			if output := run(false, "commit", "--allow-empty", "--no-gpg-sign", "-m", "unsigned"); !strings.Contains(output, "signature | missing") {
				t.Fatalf("unsigned refusal = %q", output)
			}
			if got := strings.TrimSpace(run(true, "rev-parse", "HEAD")); got != allowed {
				t.Fatalf("unsigned commit moved ref: %s -> %s", allowed, got)
			}
			if output := run(false, "-c", "user.name=Wrong", "-c", "user.email=wrong@example.test", "commit", "--allow-empty", "-S", "-m", "wrong identity"); !strings.Contains(output, "author | Wrong <wrong@example.test>") {
				t.Fatalf("identity refusal = %q", output)
			}
			if got := strings.TrimSpace(run(true, "rev-parse", "HEAD")); got != allowed {
				t.Fatalf("identity refusal moved ref: %s -> %s", allowed, got)
			}

			remote := filepath.Join(t.TempDir(), "remote")
			gitfixture.InitNativeAt(t, remote)
			runHookGit(t, remote, "config", "receive.denyCurrentBranch", "ignore")
			run(true, "remote", "add", "origin", remote)
			run(true, "-c", "core.hooksPath=/dev/null", "push", "origin", allowed+":refs/heads/master")
			run(true, "-c", "core.hooksPath=/dev/null", "commit", "--allow-empty", "--no-gpg-sign", "-m", "bypass")
			bypass := strings.TrimSpace(run(true, "rev-parse", "HEAD"))
			if output := run(false, "push", "origin", "HEAD:refs/heads/main"); !strings.Contains(output, "signature | missing") {
				t.Fatalf("pre-push refusal = %q", output)
			}
			if _, err := os.Stat(gateLog); !os.IsNotExist(err) {
				t.Fatalf("policy refusal ran gate: %v", err)
			}
			run(true, "reset", "--hard", allowed)
			if got := strings.TrimSpace(run(true, "rev-parse", "HEAD")); got != allowed || bypass == allowed {
				t.Fatalf("cleanup failed: head=%s allowed=%s bypass=%s", got, allowed, bypass)
			}

			// Model a destination-authored exception that is already accepted by the
			// remote. A conforming descendant must not re-evaluate that old tip.
			run(true, "-c", "core.hooksPath=/dev/null", "commit", "--allow-empty", "--no-gpg-sign", "-m", "hosted exception")
			hosted := strings.TrimSpace(run(true, "rev-parse", "HEAD"))
			run(true, "-c", "core.hooksPath=/dev/null", "push", "origin", hosted+":refs/heads/main")
			run(true, "commit", "--allow-empty", "-S", "-m", "conforming descendant")
			conforming := strings.TrimSpace(run(true, "rev-parse", "HEAD"))
			run(true, "push", "origin", "HEAD:refs/heads/main")
			if output := run(true, "ls-remote", "origin", "refs/heads/main"); !strings.Contains(output, conforming) {
				t.Fatalf("conforming descendant was not published: %q", output)
			}
			if body, err := os.ReadFile(gateLog); err != nil || strings.TrimSpace(string(body)) != "gate" {
				t.Fatalf("conforming descendant gate log = %q, %v", body, err)
			}
			if err := os.Remove(gateLog); err != nil {
				t.Fatal(err)
			}

			run(true, "commit", "--allow-empty", "-S", "-m", "missing baseline probe")
			missingBaseline := strings.Repeat("f", len(base))
			if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(strings.Replace(config, base, missingBaseline, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			if output := run(false, "push", "origin", "HEAD:refs/heads/main"); !strings.Contains(output, "state: baseline") || strings.Contains(output, "gate") {
				t.Fatalf("baseline refusal = %q", output)
			}
			if _, err := os.Stat(gateLog); !os.IsNotExist(err) {
				t.Fatalf("baseline refusal ran gate: %v", err)
			}
			if output := run(true, "ls-remote", "origin", "refs/heads/main"); !strings.Contains(output, conforming) {
				t.Fatalf("refused push changed remote main: %q", output)
			}
			if output := run(true, "ls-remote", "origin", "refs/heads/master"); !strings.Contains(output, allowed) {
				t.Fatalf("integration branch evidence changed: %q", output)
			}
		})
	}
}

func nativeHookSSHKey(t *testing.T) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "key")
	cmd := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v: %s", err, output)
	}
	body, err := os.ReadFile(path + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(body))
	return path, strings.Join(fields[:2], " ")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func runHookGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := gitfixture.NativeRun(gitfixture.At(root), args...)
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return out
}

// Hook payload template ids label as their script name in advisories.
func TestHookPayloadLabel(t *testing.T) {
	if got, want := artifactLabel("hooks/pre-commit.sh.tmpl"), "hooks pre-commit"; got != want {
		t.Errorf("artifactLabel = %q, want %q", got, want)
	}
}
