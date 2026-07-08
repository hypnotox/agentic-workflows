# 2026-07-08 — Lock integrity and corrupt-lock failure policy (ADR-0076)

**Goal:** implement [ADR-0076](../decisions/0076-lock-integrity-and-corrupt-lock-failure-policy.md):
atomic writes for trust-bearing files, a hard error with one recovery hint for every reader of a
present-but-unreadable `.awf/awf.lock`, truthful failure messages, upgrade/no-project UX, and the
in-process failure-path e2e suite. Design rationale lives in the ADR — this plan is execution only.

**Hard ordering constraint (user, verbatim):** "I want to do this TDD though, so test should exist
and fail first before the fix comes in." Every behavior-changing task below is a failing-test task
followed by a fix task; run the test command in between and confirm the exact RED result before
implementing. Mechanical swaps with no observable behavior change (the atomic-write call-site swaps
in Phase 2) are covered by new helper-level tests plus the existing migration suites, per the ADR's
pre-sorting of fault branches.

**Architecture summary:** new `manifest.WriteFileAtomic` + `manifest.LoadOptional`;
`migrate.Generation` gains an error return (ripples `GateState`, `Upgrade`, `gate()`,
`runUpgrade`); `project.SyncReport`/`Audit`/`CollisionsAt`/`Check`/`Uninstall` convert per ADR-0076
Decision 2; `stampLockSchema` deliberately stays on bare `Load` (corrected coverage-ignore). See
ADR-0076 for the reader inventory and supersedence mechanics.

**Tech stack:** Go 1.26; packages touched: `internal/manifest`, `internal/migrate`,
`internal/project`, `internal/config`, `cmd/awf`; docs/config: `.awf/docs/parts/pitfalls/entries.md`,
`.awf/docs/parts/domain-state/` narratives via `.awf/` domain parts, `.awf/agents-doc.yaml`,
`changelog/CHANGELOG.md`.

**File structure:**
- Created: `cmd/awf/failure_paths_test.go`
- Modified: `internal/manifest/manifest.go`, `internal/manifest/manifest_test.go`,
  `internal/migrate/{migrate.go,configedit.go,singletonstandarddocs.go,migrate_test.go}`,
  `cmd/awf/{gate.go,upgrade.go,gate_test.go}` (+ any test naming `migrate.Generation`/`GateState`),
  `internal/project/{project.go,check.go,install.go}` + their tests,
  `internal/config/config.go` + test, `.awf/agents-doc.yaml`,
  `.awf/docs/parts/pitfalls/entries.md`,
  `.awf/domains/parts/config/current-state.md`, `.awf/domains/parts/tooling/current-state.md`,
  `changelog/CHANGELOG.md`, `docs/decisions/0076-*.md` (status flip, final commit)
- Deleted: none

The recovery-hint string, used everywhere via the choke point (Decision 2):

```
unreadable .awf/awf.lock (%v) — restore it from version control, or delete it deliberately to re-adopt
```

---

## Phase 1 — manifest: `WriteFileAtomic`

(`LoadOptional` lands in Phase 3, where its first production caller lands — the deadcode gate
(ADR-0063) fails any phase that defines a production function used only later.)

- [ ] **1.1 Failing tests.** Append to `internal/manifest/manifest_test.go`:

```go
func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "awf.lock")
	if err := os.WriteFile(p, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(p, []byte("new content\n")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil || string(b) != "new content\n" {
		t.Fatalf("content = %q, err = %v", b, err)
	}
	info, err := os.Stat(p)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("perm = %v, err = %v (want 0644 regardless of prior mode)", info.Mode().Perm(), err)
	}
	ents, err := os.ReadDir(dir)
	if err != nil || len(ents) != 1 {
		t.Fatalf("temp residue left behind: %v (err %v)", ents, err)
	}
}

func TestWriteFileAtomicFailureLeavesTargetUntouched(t *testing.T) {
	// Destination path is a directory: CreateTemp succeeds, the rename fails.
	// The original path must be untouched and no temp file may remain.
	dir := t.TempDir()
	p := filepath.Join(dir, "asdir")
	if err := os.Mkdir(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(p, []byte("x")); err == nil {
		t.Fatal("want error renaming onto a directory")
	}
	ents, err := os.ReadDir(dir)
	if err != nil || len(ents) != 1 {
		t.Fatalf("temp residue after failure: %v (err %v)", ents, err)
	}
}

```

  Run `go test ./internal/manifest/` — expect `FAIL` with `undefined: WriteFileAtomic`.

- [ ] **1.2 Implement.** In `internal/manifest/manifest.go`, add after `Load`:

```go
// WriteFileAtomic writes data to path via a same-directory temp file renamed
// into place, so a crash can never leave a truncated file at path. Mode is
// 0o644 (CreateTemp's 0o600 is widened before the rename). On error the temp
// file is best-effort removed. Rename-only durability — no fsync — per
// ADR-0076 Decision 1; Go's os.Rename replaces an existing destination on
// every supported OS including Windows.
// invariant: lock-atomic-save
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".awf-atomic-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(name, 0o644)
	}
	if werr == nil {
		werr = os.Rename(name, path)
	}
	if werr != nil {
		_ = os.Remove(name)
		return werr
	}
	return nil
}
```

  Change `Save`'s last line from `return os.WriteFile(path, b, 0o644)` to
  `return WriteFileAtomic(path, b)` and add the `path/filepath` import.
  Coverage pre-sort (ADR-0076 Decision 6): the `CreateTemp` error branch and the
  `Write`/`Close`/`Chmod` sub-branches are untriggerable as root (permission faults
  root bypasses) — mark whichever `./x gate` reports uncovered with
  `// coverage-ignore: OS-level fault a root-run test cannot trigger (ADR-0076 Decision 6 pre-sort)`;
  the rename-failure branch IS covered by 1.1's directory-destination test.

  Run `go test ./internal/manifest/` → `ok`. Run `./x gate` → green.

- [ ] **1.3 Commit:** `feat(config): atomic lock save via temp-file rename`
  (body: cites ADR-0076 Decision 1; marker `lock-atomic-save` lands here).

---

## Phase 2 — migrate: atomic config rewrites (mechanical swap)

- [ ] **2.1** In `internal/migrate/configedit.go`, replace the final
  `return os.WriteFile(cfgPath, out, 0o644)` with:

```go
	// invariant: lock-atomic-save
	return manifest.WriteFileAtomic(cfgPath, out)
```

  Add the `manifest` import. In `internal/migrate/singletonstandarddocs.go`, replace
  `return os.WriteFile(path, updated, 0o644)` with the same two lines (it already
  imports nothing conflicting; add the `manifest` import).

  These rewrite an *existing* config the same bytes as before — behavior is locked by the
  existing migration suites (`TestUpgradeAppliesInOrderIdempotent`,
  `internal/migrate/singletonstandarddocs_test.go`), which must stay green; atomicity itself
  is proven at the helper level by 1.1. Fresh-file writes in `treelayout.go` stay plain
  (ADR-0076 Decision 1 exemption).

  Run `go test ./internal/migrate/` → `ok`. `./x gate` → green.

- [ ] **2.2 Commit:** `fix(config): route existing-config migration rewrites atomically`

---

## Phase 3 — corrupt lock errors in `Generation` and blocks every gated command

- [ ] **3.1 Failing tests.** First, append the choke-point test to
  `internal/manifest/manifest_test.go` (LoadOptional lands this phase with its first
  production caller, keeping the deadcode gate green):

```go
func TestLoadOptional(t *testing.T) {
	dir := t.TempDir()
	// Missing → found=false, no error.
	l, found, err := LoadOptional(filepath.Join(dir, "absent.lock"))
	if l != nil || found || err != nil {
		t.Fatalf("missing: lock=%v found=%v err=%v, want nil/false/nil", l, found, err)
	}
	// Corrupt → error carrying the recovery hint; never a lock.
	p := filepath.Join(dir, "awf.lock")
	if err := os.WriteFile(p, []byte("{truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	l, found, err = LoadOptional(p)
	if l != nil || found || err == nil {
		t.Fatalf("corrupt: lock=%v found=%v err=%v, want nil lock + error", l, found, err)
	}
	for _, want := range []string{"unreadable .awf/awf.lock", "restore it from version control", "delete it deliberately"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("hint missing %q in %q", want, err)
		}
	}
	// Valid → the lock.
	good := &Lock{AWFVersion: "0.1.0", SchemaVersion: 6, Files: map[string]Entry{}}
	if err := good.Save(p); err != nil {
		t.Fatal(err)
	}
	l, found, err = LoadOptional(p)
	if err != nil || !found || l == nil || l.SchemaVersion != 6 {
		t.Fatalf("valid: lock=%v found=%v err=%v", l, found, err)
	}
}
```

  Then append to `internal/migrate/migrate_test.go` (package `migrate`; helpers
  `writeMonolith` etc. already exist; **add
  `"github.com/hypnotox/agentic-workflows/internal/config"` to its imports** — the file
  does not import it today):

```go
func writeCorruptLock(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{\"awfVersion\": \"0.1"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGenerationCorruptTreeLockErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(root), []byte("prefix: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCorruptLock(t, config.LockPath(root))
	if _, err := Generation(root); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("want corrupt-lock error, got %v", err)
	}
	if _, err := Upgrade(root); err == nil {
		t.Fatal("Upgrade must refuse a corrupt lock upfront")
	}
}

func TestGenerationCorruptLegacyLockErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "awf", "config.yaml"), []byte("prefix: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCorruptLock(t, filepath.Join(root, ".claude", "awf", "awf.lock"))
	if _, err := Generation(root); err == nil {
		t.Fatal("want corrupt legacy-lock error")
	}
}

func TestGenerationMissingLockSemanticsPreserved(t *testing.T) {
	// Tree + no lock → Current(); nothing present → Current(); both err-free
	// (the documented standing ambiguity, ADR-0076 Decision 2 last sentence).
	root := t.TempDir()
	if gen, err := Generation(root); err != nil || gen != Current() {
		t.Fatalf("empty root: gen=%d err=%v", gen, err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(root), []byte("prefix: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if gen, err := Generation(root); err != nil || gen != Current() {
		t.Fatalf("lockless tree: gen=%d err=%v", gen, err)
	}
}

```

  Run `go test ./internal/manifest/ ./internal/migrate/` — expect
  `undefined: LoadOptional` and compile FAIL on two-valued `Generation(root)`. RED confirmed.

- [ ] **3.2 Failing e2e tests.** Create `cmd/awf/failure_paths_test.go`:

```go
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// corruptions is the ADR-0076 Decision 6 corrupt-lock variant matrix.
var corruptions = map[string][]byte{
	"truncated": []byte(`{"awfVersion":"0.9.0","schemaVersion":6,"files":{`),
	"garbage":   {0x00, 0xff, 0x13, 0x37},
	"conflict":  []byte("<<<<<<< HEAD\n{\"awfVersion\":\"0.9.0\"}\n=======\n{\"awfVersion\":\"0.8.0\"}\n>>>>>>> theirs\n"),
}

// corruptLock scaffolds a synced project, overwrites its lock with the named
// corruption, and returns root plus the corrupt bytes for the untouched check.
func corruptLock(t *testing.T, variant string) (string, []byte) {
	t.Helper()
	root := scaffoldProject(t)
	body := corruptions[variant]
	if err := os.WriteFile(config.LockPath(root), body, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, body
}

// assertRefused asserts the run refused with the recovery hint, created no
// .awf-bak backup anywhere under root, and left the corrupt lock byte-identical.
func assertRefused(t *testing.T, root string, wantBytes []byte, code int, out string) {
	t.Helper()
	if code != 1 {
		t.Fatalf("exit = %d, want 1; output:\n%s", code, out)
	}
	if !strings.Contains(out, "unreadable .awf/awf.lock") || !strings.Contains(out, "restore it from version control") {
		t.Fatalf("missing recovery hint:\n%s", out)
	}
	var baks []string
	err := filepath.WalkDir(root, func(p string, _ os.DirEntry, err error) error {
		if err == nil && strings.Contains(filepath.Base(p), ".awf-bak") {
			baks = append(baks, p)
		}
		return nil
	})
	if err != nil || len(baks) != 0 {
		t.Fatalf("backup storm: %v (err %v)", baks, err)
	}
	got, err := os.ReadFile(config.LockPath(root))
	if err != nil || !bytes.Equal(got, wantBytes) {
		t.Fatalf("corrupt lock was modified (err %v)", err)
	}
}

func TestGatedCommandsRefuseCorruptLock(t *testing.T) {
	for variant := range corruptions {
		for _, cmd := range []string{"sync", "check", "invariants", "audit", "list"} {
			t.Run(variant+"/"+cmd, func(t *testing.T) {
				root, want := corruptLock(t, variant)
				var out, errb bytes.Buffer
				code := runAt(t, root, []string{"awf", cmd}, &out, &errb)
				assertRefused(t, root, want, code, out.String()+errb.String())
			})
		}
	}
}
```

  If no `runAt` helper exists in the package (check `grep -n "func runAt" cmd/awf/*_test.go`),
  add one to this file — the package's existing tests drive command funcs directly
  (`runSync(root, …)`), but the e2e matrix needs the dispatch path; use the same seam
  `run()` uses with the working directory swapped:

```go
// runAt drives the full CLI dispatch with the process cwd at root.
func runAt(t *testing.T, root string, args []string, stdout, stderr *bytes.Buffer) int {
	t.Helper()
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	return run(args, stdout, stderr)
}
```

  (add imports `testsupport`; `getwd` is the existing seam `TestRunGetwdError` swaps).

  Run `go test ./cmd/awf/ -run TestGatedCommandsRefuseCorruptLock` — expect FAIL:
  today the gate treats a corrupt lock as current (`Generation` → `Current()`), so `sync`
  exits 0 with a backup storm. RED confirmed (the failure will report `exit = 0` and/or
  the `.awf-bak` list).

- [ ] **3.3 Implement.** In `internal/manifest/manifest.go`, add after `Load` (imports:
  add `errors`):

```go
// LoadOptional is the corrupt-lock policy choke point (ADR-0076 Decision 2): a
// missing lock reports found=false with no error so callers keep their no-lock
// semantics; a present-but-unreadable lock is a hard error carrying the one
// recovery hint.
// invariant: corrupt-lock-refuses
func LoadOptional(path string) (*Lock, bool, error) {
	l, err := Load(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("unreadable .awf/awf.lock (%w) — restore it from version control, or delete it deliberately to re-adopt", err)
	}
	return l, true, nil
}
```

  In `internal/migrate/migrate.go`:
  - `Generation(root string) (int, error)`; both lock branches convert to
    `manifest.LoadOptional`: tree branch — `l, found, err := manifest.LoadOptional(config.LockPath(root))`,
    `err != nil → return 0, err`, `!found → return Current(), nil`, else
    `return l.SchemaVersion, nil`; legacy branch — same shape with `!found → return 1, nil`.
    All other returns gain `, nil`. Update the doc comment's sentinel sentences to name the
    corrupt-lock error. Add `// invariant: corrupt-lock-refuses` above the tree-branch
    `LoadOptional` call.
  - `GateState(root string) (string, int, error)` — unnamed results (named ones would make
    the `:=` below a redeclaration error) — returning the generation so `gate()` stops
    calling `Generation` three times:
    `gen, err := Generation(root); if err != nil { return "", 0, err }; return gateStateFor(gen, Current(), registryTos()), gen, nil`.
  - `Upgrade`: `from, err := Generation(root); if err != nil { return nil, err }`.
  - `stampLockSchema`: rewrite the coverage-ignore reason to the ADR-0076 Decision 2 text:
    `// coverage-ignore: reached only via Upgrade, whose upfront Generation now hard-errors on a corrupt lock (ADR-0076), so when this runs the lock loads cleanly`.
  (`ProjectPresent` lands in Phase 5 with its first production caller — deadcode gate.)

  In `cmd/awf/gate.go`:
  - `gate()`: `state, gen, err := migrate.GateState(root); if err != nil { return err }`;
    the two `case` messages use `gen` instead of re-calling `migrate.Generation(root)`.
  - `lockVsBinary` converts to `LoadOptional` and gains an error result
    (`(lockV, binV string, ok bool, err error)`): corrupt → `("", "", false, err)`; the
    surviving skip set → `("", "", false, nil)` as today. Update both doc comments to
    mirror ADR-0076 Decision 3's enumeration verbatim: "an absent lock; an absent or empty
    `awfVersion` field; an `awfVersion` failing semver normalization — all still skip; a
    present-but-unparseable lock now errors upstream (ADR-0076 partially supersedes
    ADR-0039 Decision 5)". `gate()` and `runCheck`'s ahead-note propagate the error.
  - `cmd/awf/upgrade.go`: `runUpgrade` compiles against the new `Upgrade` unchanged
    (its own UX lands in Phase 5).

  Update every remaining compile-affected call site — enumerate with
  `grep -rn '\bGeneration(\|\bGateState(\|lockVsBinary(' cmd/ internal/` (the word-boundary
  form also catches `internal/migrate/migrate_test.go`'s ~10 in-package unqualified
  `Generation(...)`/`GateState(...)` calls around lines 94–133, 152, 388, 628–643, which the
  package-qualified grep would miss) and adjust each mechanically (two-value `Generation`,
  three-value `GateState`, four-value `lockVsBinary`), asserting `err == nil` in
  previously-passing cases.

  Run `go test ./internal/migrate/ ./cmd/awf/` → `ok` (all Phase-3 RED tests now green).
  `./x gate` → green.

- [ ] **3.4 Commit:** `fix(config): corrupt lock hard-errors in Generation and the gate`
  (body: ADR-0076 Decisions 2–3; partial supersedence of ADR-0039 D5 exercised at the gate;
  e2e matrix for gated commands lands here).

---

## Phase 4 — project package: SyncReport, Audit, CollisionsAt, Check, Uninstall

- [ ] **4.1 Failing tests.** Append to `internal/project/drift_test.go` (package `project`,
  helpers `scaffold`/`scaffoldFiles`/`syncClean`/`lockFile` exist; **add `"bytes"` to its
  imports** — the file does not import it today):

```go
func corruptProjectLock(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(lockFile(root), []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// invariant: corrupt-lock-refuses
func TestSyncReportRefusesCorruptLockBeforeWriting(t *testing.T) {
	root := scaffold(t, sampleYAML)
	syncClean(t, root)
	agents := filepath.Join(root, "AGENTS.md")
	before, err := os.ReadFile(agents)
	if err != nil {
		t.Fatal(err)
	}
	corruptProjectLock(t, root)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.SyncReport(); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("want refusal with hint, got %v", err)
	}
	after, err := os.ReadFile(agents)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("SyncReport wrote despite refusing (err %v)", err)
	}
	if fileExists(filepath.Join(root, "AGENTS.md.awf-bak")) {
		t.Fatal("backup created despite refusal")
	}
}

func TestCheckSplitsMissingVsCorrupt(t *testing.T) {
	root := scaffold(t, sampleYAML)
	syncClean(t, root)
	corruptProjectLock(t, root)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Check(); err == nil || strings.Contains(err.Error(), "no lock") || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("corrupt lock misreported: %v", err)
	}
	if err := os.Remove(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Check(); err == nil || !strings.Contains(err.Error(), "no lock (run awf sync)") {
		t.Fatalf("missing lock lost its message: %v", err)
	}
}

func TestUninstallSplitsMissingVsCorrupt(t *testing.T) {
	root := scaffold(t, sampleYAML)
	syncClean(t, root)
	corruptProjectLock(t, root)
	if _, err := Uninstall(root); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("corrupt lock must refuse uninstall with the hint, got %v", err)
	}
	if err := os.Remove(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(root); err == nil || !strings.Contains(err.Error(), "nothing to uninstall") {
		t.Fatalf("missing lock lost its message: %v", err)
	}
}

func TestAuditAndCollisionsRefuseCorruptLock(t *testing.T) {
	root := scaffold(t, sampleYAML)
	syncClean(t, root)
	corruptProjectLock(t, root)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Audit(""); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("Audit: %v", err)
	}
	if _, err := CollisionsAt(root, []string{"AGENTS.md"}); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("CollisionsAt: %v", err)
	}
}
```

  Run `go test ./internal/project/ -run 'CorruptLock|SplitsMissing'` — expect FAIL/compile
  error (`CollisionsAt` single-valued today; `SyncReport` succeeds with a backup). RED confirmed.

- [ ] **4.2 Implement.**
  - `project.go` `SyncReport`: first statement becomes the refusal —

```go
	// Refuse before rendering or writing anything: a corrupt lock must never
	// produce a backup, skip a prune, or be overwritten (ADR-0076 Decision 2).
	// invariant: corrupt-lock-refuses
	old, _, err := manifest.LoadOptional(p.lockPath())
	if err != nil {
		return nil, err
	}
```

    and the later `old, _ := manifest.Load(p.lockPath())` line is deleted (the `prior` map
    keeps its `if old != nil` guard).
  - `project.go` `Audit`: `if lock, err := manifest.Load(...); err == nil` becomes
    `lock, _, err := manifest.LoadOptional(p.lockPath()); if err != nil { return nil, err };
    if lock != nil { … }`.
  - `check.go` `Check`: replace the `manifest.Load` + blanket "no lock" wrap with
    `lock, found, err := manifest.LoadOptional(p.lockPath()); if err != nil { return nil, err };
    if !found { return nil, errors.New("no lock (run awf sync)") }` —
    `errors.New`, not a zero-arg `fmt.Errorf`, or the perfsprint linter fails the gate
    (`check.go` already imports `errors`).
  - `install.go` `CollisionsAt` → `([]string, error)`: `lock, _, err := manifest.LoadOptional(...);
    if err != nil { return nil, err }; if lock != nil { … }`. Ripple: `InitCollisions`
    and the init probe call site (`grep -rn 'CollisionsAt(' cmd/ internal/`).
  - `install.go` `Uninstall`: `lock, found, err := manifest.LoadOptional(lockPath); if err != nil
    { return 0, err }; if !found { return 0, fmt.Errorf("no %s — nothing to uninstall", …) }`.

  Run `go test ./internal/project/ ./cmd/awf/` → `ok`. `./x gate` → green.

- [ ] **4.3 Commit:** `fix(rendering): corrupt lock refuses project operations pre-write`

---

## Phase 5 — upgrade states and the no-project hint

- [ ] **5.1 Failing tests.** Append to `cmd/awf/failure_paths_test.go`:

```go
func TestUpgradeCorruptLockRefuses(t *testing.T) {
	for variant := range corruptions {
		t.Run(variant, func(t *testing.T) {
			root, want := corruptLock(t, variant)
			var out, errb bytes.Buffer
			code := runAt(t, root, []string{"awf", "upgrade"}, &out, &errb)
			assertRefused(t, root, want, code, out.String()+errb.String())
		})
	}
}

func TestUpgradeReportsBinaryBehind(t *testing.T) {
	root := scaffoldProject(t)
	lockPath := config.LockPath(root)
	l, err := manifest.Load(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	l.SchemaVersion = migrate.Current() + 1
	if err := l.Save(lockPath); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	code := runAt(t, root, []string{"awf", "upgrade"}, &out, &errb)
	all := out.String() + errb.String()
	if code != 1 || !strings.Contains(all, "update your pinned awf") || strings.Contains(all, "already current") {
		t.Fatalf("code=%d output:\n%s", code, all)
	}
}

func TestUpgradeOutsideProject(t *testing.T) {
	var out, errb bytes.Buffer
	code := runAt(t, t.TempDir(), []string{"awf", "upgrade"}, &out, &errb)
	all := out.String() + errb.String()
	if code != 1 || !strings.Contains(all, "not an awf project (run `awf init`)") {
		t.Fatalf("code=%d output:\n%s", code, all)
	}
}

func TestProjectCommandsHintInit(t *testing.T) {
	var out, errb bytes.Buffer
	code := runAt(t, t.TempDir(), []string{"awf", "sync"}, &out, &errb)
	all := out.String() + errb.String()
	if code != 1 || !strings.Contains(all, "not an awf project (run `awf init`)") {
		t.Fatalf("code=%d output:\n%s", code, all)
	}
}
```

  Imports for these tests: `manifest` and `migrate` join `failure_paths_test.go`'s import
  block.

  Run `go test ./cmd/awf/ -run 'Upgrade|HintInit'` — expect FAIL on three:
  `TestUpgradeReportsBinaryBehind` and `TestUpgradeOutsideProject` (today an "ahead" or
  absent tree makes `Upgrade` apply nothing, so `runUpgrade` prints "already current" and
  exits 0) and `TestProjectCommandsHintInit` (raw ENOENT, no hint).
  `TestUpgradeCorruptLockRefuses` is expected GREEN already — that behavior was red-tested
  in 3.1 and fixed in 3.3; it lands here as e2e closure of the Decision 6 matrix, not as a
  RED test.

- [ ] **5.2 Implement.**
  - `internal/migrate/migrate.go` gains `ProjectPresent` (first production use is below,
    so it lands in this phase — deadcode gate), plus its test appended to
    `internal/migrate/migrate_test.go`:

```go
// ProjectPresent reports whether any awf config layout (current tree,
// pre-relocation tree, or legacy single file) exists under root — the
// distinction Generation cannot express, since "nothing present" reports
// Current() (ADR-0076 Decision 4).
func ProjectPresent(root string) bool {
	for _, p := range []string{
		config.ConfigPath(root),
		filepath.Join(root, ".claude", "awf", "config.yaml"),
		filepath.Join(root, ".claude", "awf.yaml"),
	} {
		if fileExists(p) {
			return true
		}
	}
	return false
}
```

```go
func TestProjectPresent(t *testing.T) {
	root := t.TempDir()
	if ProjectPresent(root) {
		t.Fatal("empty root must not be present")
	}
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(root), []byte("prefix: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !ProjectPresent(root) {
		t.Fatal("tree root must be present")
	}
}
```

  - `cmd/awf/upgrade.go` `runUpgrade` becomes (add the `errors` import — the no-project
    message is a constant string, and a zero-arg `fmt.Errorf` fails the perfsprint linter):

```go
func runUpgrade(root string, stdout io.Writer) error {
	if !migrate.ProjectPresent(root) {
		return errors.New("not an awf project (run `awf init`)")
	}
	gen, err := migrate.Generation(root)
	if err != nil {
		return err
	}
	if gen > migrate.Current() {
		return fmt.Errorf("awf %s is behind this project's config (schema generation %d > %d); update your pinned awf",
			awfVersion(), gen, migrate.Current())
	}
	applied, err := migrate.Upgrade(root)
	…(rest unchanged)…
```

  - `internal/config/config.go` `Load`: the read-error wrap splits —

```go
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not an awf project (run `awf init`): %w", err)
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
```

    `awf init` never calls `config.Load` on the target pre-scaffold (verify:
    `grep -n 'config.Load\|project.Open' cmd/awf/init.go` — its open happens post-scaffold),
    so the exemption holds structurally.
  - The existing `internal/config/config_test.go` `TestLoadMissingConfigErrors` (~line 257)
    asserts the missing-config error contains `"read config"` — update it in the same
    commit to the new split, replacing its assertion with:

```go
func TestLoadMissingConfigErrors(t *testing.T) {
	dir := t.TempDir()
	// Missing config.yaml → the no-project hint (ADR-0076 Decision 5).
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "not an awf project (run `awf init`)") {
		t.Fatalf("missing: %v", err)
	}
	// Present but unreadable (a directory at the path) → the plain read wrap.
	if err := os.Mkdir(filepath.Join(dir, "config.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "read config") || strings.Contains(err.Error(), "awf init") {
		t.Fatalf("unreadable-but-present: %v", err)
	}
}
```

    (Keep the original function's name/location; if its current body asserts other
    properties, preserve them alongside these two wraps.)

  Run `go test ./cmd/awf/ ./internal/config/` → `ok`. `./x gate` → green.

- [ ] **5.3 Commit:** `fix(tooling): truthful upgrade states and the awf init hint`

---

## Phase 6 — uninstall/init e2e closure, docs, changelog, ADR flip

- [ ] **6.1 Failing e2e closure tests.** Append to `cmd/awf/failure_paths_test.go`:

```go
func TestUninstallAndInitRefuseCorruptLock(t *testing.T) {
	for variant := range corruptions {
		for _, cmd := range []string{"uninstall", "init"} {
			t.Run(variant+"/"+cmd, func(t *testing.T) {
				root, want := corruptLock(t, variant)
				var out, errb bytes.Buffer
				code := runAt(t, root, []string{"awf", cmd}, &out, &errb)
				all := out.String() + errb.String()
				if cmd == "init" && strings.Contains(all, "collision") {
					t.Fatalf("init reported collisions instead of the lock error:\n%s", all)
				}
				assertRefused(t, root, want, code, all)
			})
		}
	}
}
```

  This closes the full ADR-0076 Decision 6 matrix: all three corruption variants now run
  against every command family (`sync`/`check`/`invariants`/`audit`/`list` in 3.2,
  `upgrade` in 5.1, `uninstall`/`init` here).

  Run — `uninstall` should already pass (Phase 4); if `init` fails on a non-hint path
  (its collision probe may exit differently), that is the RED for the `runInit` ripple:
  make `runInit` propagate the `CollisionsAt` error unchanged. GREEN both, `./x gate`.

- [ ] **6.1b Commit:** `test(config): close the corrupt-lock e2e matrix for uninstall and init`
  (own commit — the closure tests plus any `runInit` error-propagation ripple are one
  concern; the docs-and-flip commit below is another).

- [ ] **6.2 Docs travel.**
  - `.awf/docs/parts/pitfalls/entries.md`: in the "Registry-relative constants in migration
    code drift" entry, update the `Generation` sentinel description to note that since
    ADR-0076 a present-but-unreadable lock is a hard error, not a sentinel (`Current()`/`1`);
    sentinels remain only for genuinely-absent locks.
  - Domain narratives: refresh `.awf/domains/parts/config/current-state.md` (lock handling:
    atomic save, corrupt-lock hard error, LoadOptional choke point) and
    `.awf/domains/parts/tooling/current-state.md` (gate refuses corrupt locks; upgrade's
    behind/no-project states; ADR-0039 D5 narrowed).
  - `.awf/agents-doc.yaml` `data.invariants`: add two entries —
    `- ref: ADR-0076` / text: `**Atomic trust-bearing writes.** .awf/awf.lock and
    existing-config migration rewrites go through the temp-file-plus-rename helper; no
    truncate-in-place write remains.` and `- ref: ADR-0076` / text: `**Corrupt lock refuses.**
    A present-but-unreadable .awf/awf.lock is a hard error in every reader; sync refuses
    before writing anything.` (match the file's existing `- ref:`/`text:` shape).
  - `changelog/CHANGELOG.md` `[Unreleased]`: under `### Breaking changes` — the
    unparseable-lock behavior flip (previously skipped the version sub-check per ADR-0039;
    now every command refuses with a recovery hint); under `### Bug fixes` — the backup-storm
    /skipped-prune fix, truthful check/uninstall messages, upgrade behind/no-project states,
    the `awf init` hint; under `### Features` — atomic lock/config writes.
  - `./x sync && ./x check` → clean; rendered `AGENTS.md`, `docs/pitfalls.md`,
    `docs/domains/{config,tooling}.md` regenerate.

- [ ] **6.3 Final commit + flip.** Edit `docs/decisions/0076-*.md` frontmatter
  `status: Proposed` → `status: Implemented`; `./x sync` (ACTIVE.md + domain indexes regen);
  `./x gate`; commit 6.2 + 6.3 together as
  `docs(adr): mark 0076 implemented with its doc-currency bundle` — body cites the
  Doc-currency-at-the-flip Consequences bullet (pitfalls, domain narratives, agent-guide
  invariant bullets, changelog, ACTIVE.md travel with the flip). Run
  `go run ./cmd/awf invariants` → `awf invariants: clean` (both new slugs backed since
  Phases 1/3).

---

## Verification (after all phases)

- [ ] `./x gate` green; `./x check` clean; `go run ./cmd/awf invariants` clean.
- [ ] The greppable `lock-atomic-save` condition holds:
  `grep -rn 'os.WriteFile' internal/manifest/*.go internal/migrate/*.go | grep -v _test.go`
  — expect exactly one hit, the exempt fresh-file write in `internal/migrate/treelayout.go`
  (ADR-0076 Decision 1 exemption).
- [ ] `./x audit-local origin/main..HEAD` — changelog conformance clean.
- [ ] Manual smoke: in a scratch dir, `git init`, `awf init` (via `go run ./cmd/awf`), corrupt
  `.awf/awf.lock` with `echo '{broken' > .awf/awf.lock`, then `go run ./cmd/awf sync` — expect
  exit 1, the recovery hint, no `.awf-bak` files (`find . -name '*.awf-bak*'` empty).
- [ ] Terminal step: invoke `awf-reviewing-impl` over the implementation range.
