package worktree

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

const worktreeTestID = "018f47a0-7b3d-4c52-8f1a-123456789abc"

func renderedTopologyDiagnostic(t *testing.T, err interface {
	Diagnostic() (presentation.Diagnostic, error)
}) string {
	t.Helper()
	diagnostic, diagnosticErr := err.Diagnostic()
	if diagnosticErr != nil {
		t.Fatal(diagnosticErr)
	}
	document, documentErr := diagnostic.Document()
	if documentErr != nil {
		t.Fatal(documentErr)
	}
	var out bytes.Buffer
	if renderErr := presentation.Render(&out, document); renderErr != nil {
		t.Fatal(renderErr)
	}
	return out.String()
}

func TestWorktreeMutationPreservesRepeatedPathSpacesThroughEffortComposition(t *testing.T) {
	repositoryRoot := filepath.Join(t.TempDir(), "repository  root")
	gitfixture.InitNativeObjectFormat(t, repositoryRoot, "sha1")
	worktreePath := filepath.Join(repositoryRoot, ".awf", "worktrees", "worktree  root")
	result := Result{
		Condition:       "managed worktree added",
		ChangedTopology: true,
		Path:            worktreePath,
		Branch:          "awf/worktree-root",
		NextAction:      "continue the effort in " + worktreePath,
	}
	mutation, err := result.Mutation()
	if err != nil {
		t.Fatal(err)
	}
	mutation, err = (effort.Record{Slug: "worktree-root", Title: "Worktree root", MemoryPath: ".awf/efforts/worktree-root/memory.md"}).NewEffortMutation(mutation)
	if err != nil {
		t.Fatal(err)
	}
	document, err := mutation.Document()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	want := "status: managed worktree added\n\nmutation:\n  identity:\n    effort: worktree-root\n    title: Worktree root\n    memory: .awf/efforts/worktree-root/memory.md\n    worktree: " + worktreePath + "\n    branch: awf/worktree-root\n  changes:\n    completed:\n      managed topology\n  next actions:\n    step 1: continue the effort in " + worktreePath + "\n"
	if got := out.String(); got != want {
		t.Fatalf("composed worktree mutation = %q, want %q", got, want)
	}
}

func TestWorktreeMutationRejectsLineBreakLiterals(t *testing.T) {
	for _, result := range []Result{
		{
			Condition: "managed worktree added", Path: filepath.Join(t.TempDir(), "worktree\nroot"),
			NextAction: "continue the effort",
		},
		{
			Condition: "managed worktree added", Path: filepath.Join(t.TempDir(), "worktree-root"),
			NextAction: "continue the effort in worktree\nroot",
		},
	} {
		if _, err := result.Mutation(); err == nil || !strings.Contains(err.Error(), "line break") {
			t.Fatalf("mutation result %#v error = %v, want literal line-break rejection", result, err)
		}
	}
}

func TestManagedWorktreeAddIntegrateAndRestartableRemove(t *testing.T) {
	// invariant: tooling/effort-management:managed-worktree-lifecycle (TestManagedWorktreeAddIntegrateAndRestartableRemove)
	root := initWorktreeRepo(t, "sha1")
	createEffort(t, root, "managed-result", "Managed result")
	manager := freshWorktreeManager(t, root)
	added, err := manager.Add(testContext(t), "managed-result", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !added.ChangedTopology || !strings.Contains(added.Condition, "managed worktree added") {
		t.Fatalf("add result = %#v", added)
	}
	managed := filepath.Join(root, ".awf", "worktrees", "managed-result")
	if added.Path != managed || added.Branch != "awf/managed-result" || !strings.Contains(added.NextAction, added.Path) {
		t.Fatalf("add facts = %#v, want path %q and branch awf/managed-result named by the next action", added, managed)
	}
	writeWorktreeFile(t, filepath.Join(managed, "effort.txt"), "effort\n")
	commitWorktree(t, managed, "effort")

	integrated, err := manager.Integrate(testContext(t), "managed-result", "")
	if err != nil {
		t.Fatal(err)
	}
	if !integrated.ChangedTopology || !strings.Contains(integrated.Condition, "fast-forwarded") {
		t.Fatalf("integrate result = %#v", integrated)
	}
	already, err := manager.Integrate(testContext(t), "managed-result", "")
	if err != nil {
		t.Fatal(err)
	}
	if already.ChangedTopology || !strings.Contains(already.Condition, "already integrated") {
		t.Fatalf("already result = %#v", already)
	}

	removed, err := manager.Remove(testContext(t), "managed-result")
	if err != nil {
		t.Fatal(err)
	}
	if !removed.ChangedTopology {
		t.Fatalf("remove result = %#v", removed)
	}
	if _, err := os.Lstat(managed); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("managed path remains: %v", err)
	}
	if gitfixture.NativeRevisionExists(t, gitfixture.At(root), "refs/heads/awf/managed-result") {
		t.Fatal("managed branch remains")
	}
	again, err := manager.Remove(testContext(t), "managed-result")
	if err != nil {
		t.Fatal(err)
	}
	if again.ChangedTopology {
		t.Fatalf("idempotent remove changed topology: %#v", again)
	}
}

func TestDivergentIntegrationStopsBeforeCommit(t *testing.T) {
	root := initWorktreeRepo(t, "sha1")
	createEffort(t, root, "divergent-result", "Divergent result")
	manager := freshWorktreeManager(t, root)
	if _, err := manager.Add(testContext(t), "divergent-result", "HEAD"); err != nil {
		t.Fatal(err)
	}
	managed := filepath.Join(root, ".awf", "worktrees", "divergent-result")
	writeWorktreeFile(t, filepath.Join(managed, "effort.txt"), "effort\n")
	commitWorktree(t, managed, "effort")
	writeWorktreeFile(t, filepath.Join(root, "target.txt"), "target\n")
	commitWorktree(t, root, "target")

	result, err := manager.Integrate(testContext(t), "divergent-result", "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.ChangedTopology || !strings.Contains(result.Condition, "staged without a commit") ||
		!strings.Contains(result.NextAction, "check staged") || !strings.Contains(result.NextAction, "project gate") ||
		strings.Contains(result.NextAction, "./x gate") {
		t.Fatalf("divergent result = %#v", result)
	}
	mergeHead := gitfixture.NativeGitPath(t, gitfixture.At(root), "MERGE_HEAD")
	if !filepath.IsAbs(mergeHead) {
		mergeHead = filepath.Join(root, mergeHead)
	}
	if _, err := os.Stat(mergeHead); err != nil {
		t.Fatalf("MERGE_HEAD absent: %v", err)
	}
	gitfixture.NativeMergeAbort(t, gitfixture.At(root))
}

func TestIntegrationConflictAndUnrelatedHistoryStayVisibleAndActionable(t *testing.T) {
	t.Run("conflict", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		createEffort(t, root, "conflict-result", "Conflict result")
		manager := freshWorktreeManager(t, root)
		if _, err := manager.Add(testContext(t), "conflict-result", "HEAD"); err != nil {
			t.Fatal(err)
		}
		managed := filepath.Join(root, ".awf", "worktrees", "conflict-result")
		writeWorktreeFile(t, filepath.Join(managed, "tracked.txt"), "effort\n")
		commitWorktree(t, managed, "effort conflict")
		writeWorktreeFile(t, filepath.Join(root, "tracked.txt"), "target\n")
		commitWorktree(t, root, "target conflict")
		_, err := manager.Integrate(testContext(t), "conflict-result", "make gate")
		if err == nil || !strings.Contains(err.Error(), "changed topology: yes") || !strings.Contains(err.Error(), "resolve or abort") ||
			!strings.Contains(err.Error(), "`make gate`") || strings.Contains(err.Error(), "./x gate") {
			t.Fatalf("conflict error = %v", err)
		}
		if !gitfixture.NativeRevisionExists(t, gitfixture.At(root), "MERGE_HEAD") {
			t.Fatal("conflict merge state was hidden")
		}
		gitfixture.NativeMergeAbort(t, gitfixture.At(root))
	})

	t.Run("unrelated", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		createEffort(t, root, "unrelated-result", "Unrelated result")
		manager := freshWorktreeManager(t, root)
		if _, err := manager.Add(testContext(t), "unrelated-result", "HEAD"); err != nil {
			t.Fatal(err)
		}
		managed := filepath.Join(root, ".awf", "worktrees", "unrelated-result")
		gitfixture.NativeCheckoutOrphan(t, gitfixture.At(managed), "unrelated-temp")
		gitfixture.NativeRemoveAll(t, gitfixture.At(managed))
		writeWorktreeFile(t, filepath.Join(managed, "orphan.txt"), "orphan\n")
		commitWorktree(t, managed, "orphan")
		gitfixture.NativeBranchForce(t, gitfixture.At(root), "awf/unrelated-result", "unrelated-temp")
		gitfixture.NativeCheckout(t, gitfixture.At(managed), "awf/unrelated-result")
		before := gitfixture.NativeRevParse(t, gitfixture.At(root), "HEAD")
		_, err := manager.Integrate(testContext(t), "unrelated-result", "")
		if err == nil || !strings.Contains(err.Error(), "no proven common ancestor") || !strings.Contains(err.Error(), "changed topology: no") || !strings.Contains(err.Error(), "do not use --allow-unrelated-histories") {
			t.Fatalf("unrelated error = %v", err)
		}
		if after := gitfixture.NativeRevParse(t, gitfixture.At(root), "HEAD"); after != before {
			t.Fatalf("target changed from %s to %s", before, after)
		}
	})
}

func TestRemovalRefusesDirtyAndUnmergedWithoutForce(t *testing.T) {
	root := initWorktreeRepo(t, "sha1")
	createEffort(t, root, "guard-removal", "Guard removal")
	manager := freshWorktreeManager(t, root)
	if _, err := manager.Add(testContext(t), "guard-removal", "HEAD"); err != nil {
		t.Fatal(err)
	}
	managed := filepath.Join(root, ".awf", "worktrees", "guard-removal")
	writeWorktreeFile(t, filepath.Join(managed, "dirty.txt"), "dirty\n")
	_, err := manager.Remove(testContext(t), "guard-removal")
	if err == nil || !strings.Contains(err.Error(), "not merged") {
		// With no effort commit the branch is merged, so cleanliness is the first
		// destructive precondition after identity.
		if err == nil || !strings.Contains(err.Error(), "dirty") {
			t.Fatalf("dirty removal error = %v", err)
		}
	}
	if err := os.Remove(filepath.Join(managed, "dirty.txt")); err != nil {
		t.Fatal(err)
	}
	writeWorktreeFile(t, filepath.Join(managed, "effort.txt"), "unmerged\n")
	commitWorktree(t, managed, "unmerged")
	_, err = manager.Remove(testContext(t), "guard-removal")
	if err == nil || !strings.Contains(err.Error(), "not merged") || !strings.Contains(err.Error(), "native Git") {
		t.Fatalf("unmerged removal error = %v", err)
	}
	if _, statErr := os.Stat(managed); statErr != nil {
		t.Fatalf("managed worktree was discarded: %v", statErr)
	}
}

// TestInvokingCheckoutCleanlinessGuardsDestructiveOperations pins the refusal
// that guards the INVOKING checkout, in both directions. The oracle behind it
// changed in this phase: the implementation this replaced carried a regex
// allowance for untracked .awf/efforts and .awf/worktrees entries, while
// ChangeCounts carries none, so owned resident state now stays invisible only
// because awf renders the .gitignore that covers it. Neither direction was
// pinned at this layer, so removing the refusal entirely left the suite green.
func TestInvokingCheckoutCleanlinessGuardsDestructiveOperations(t *testing.T) {
	root := initWorktreeRepo(t, "sha1")
	createEffort(t, root, "invoking-cleanliness", "Invoking cleanliness")
	manager := freshWorktreeManager(t, root)
	if _, err := manager.Add(testContext(t), "invoking-cleanliness", "HEAD"); err != nil {
		t.Fatal(err)
	}

	foreign := filepath.Join(root, "foreign.txt")
	writeWorktreeFile(t, foreign, "foreign\n")
	_, err := manager.Remove(testContext(t), "invoking-cleanliness")
	if err == nil || !strings.Contains(err.Error(), "cleanliness") {
		t.Fatalf("remove with a foreign untracked file in the invoking checkout = %v, want a cleanliness refusal", err)
	}

	// Owned resident state alone must not refuse: the rendered .gitignore is
	// the entire mechanism keeping it out of the counts.
	if err := os.Remove(foreign); err != nil {
		t.Fatal(err)
	}
	writeWorktreeFile(t, filepath.Join(root, ".awf", "efforts", "invoking-cleanliness", "memory.md"), "---\neffort: invoking-cleanliness\nphase: active\nnext: finish\nupdated: \"2026-08-25T00:00:00Z\"\n---\n")
	if _, err := manager.Remove(testContext(t), "invoking-cleanliness"); err != nil {
		t.Fatalf("remove with only owned resident state present = %v, want success", err)
	}
}

func TestAddFailureReportsActualTopologyAndPreservesEffort(t *testing.T) {
	root := initWorktreeRepo(t, "sha1")
	createEffort(t, root, "partial-add", "Partial add")
	manager := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
		stub.worktreeAdd = func(ctx context.Context, path, branch, base string) error {
			if err := stub.Runner.WorktreeAdd(ctx, path, branch, base); err != nil {
				return err
			}
			return failing("post-add failure")
		}
	}))
	_, err := manager.Add(testContext(t), "partial-add", "HEAD")
	if err == nil || !strings.Contains(err.Error(), "changed topology: yes") || !strings.Contains(err.Error(), "actual Git topology") {
		t.Fatalf("add failure = %v", err)
	}
	var refusal *RefusalError
	if !errors.As(err, &refusal) || !errors.Is(err, refusal.Err) {
		t.Fatalf("add failure lost typed refusal identity: %v", err)
	}
	const want = "condition: git worktree add failed\nstate: operation\ncause: injected post-add failure\n\ndiagnostic:\n  changed:\n    managed topology: yes\n  steps:\n    step 1: inspect actual Git topology\n    step 2: clean only the named managed path, registration, and branch with native Git\n    step 3: retry add\n"
	if got := renderedTopologyDiagnostic(t, refusal); got != want {
		t.Fatalf("worktree refusal diagnostic = %q, want %q", got, want)
	}
	if _, err := freshWorktreeManager(t, root).efforts.Show("partial-add"); err != nil {
		t.Fatalf("complete effort changed by add failure: %v", err)
	}
}

func TestWorktreeAddFailureWithoutResidueAddressesTheFailedCallBeforeRetry(t *testing.T) {
	root := initWorktreeRepo(t, "sha1")
	createEffort(t, root, "failed-add", "Failed add")
	manager := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
		stub.worktreeAdd = func(context.Context, string, string, string) error { return failing("worktree add") }
	}))
	_, err := manager.Add(testContext(t), "failed-add", "HEAD")
	var refusal *RefusalError
	if !errors.As(err, &refusal) || refusal.ChangedTopology {
		t.Fatalf("add failure = %#v, want unchanged typed refusal", err)
	}
	const want = "condition: git worktree add failed\nstate: operation\ncause: injected worktree add\n\ndiagnostic:\n  changed:\n    managed topology: no\n  steps:\n    step 1: address or resolve the reported failed Git call\n    step 2: retry add\n"
	if got := renderedTopologyDiagnostic(t, refusal); got != want {
		t.Fatalf("worktree refusal diagnostic = %q, want %q", got, want)
	}
}

// invariant: tooling/effort-management:default-worktree-creation (TestNewEffortCreatesTheManagedWorktreeByDefault)
func TestNewEffortCreatesTheManagedWorktreeByDefault(t *testing.T) {
	root := initWorktreeRepo(t, "sha1")
	manager := freshWorktreeManager(t, root)
	record, result, err := manager.NewEffort(testContext(t), effort.NewInput{Slug: "default-creation", Title: "Default creation"}, "")
	if err != nil {
		t.Fatal(err)
	}
	managed := filepath.Join(root, ".awf", "worktrees", "default-creation")
	if record.Slug != "default-creation" || record.Title != "Default creation" {
		t.Fatalf("record = %#v", record)
	}
	if !result.ChangedTopology || result.Path != managed || result.Branch != "awf/default-creation" {
		t.Fatalf("result = %#v, want the managed path %q on awf/default-creation", result, managed)
	}
	if info, statErr := os.Lstat(managed); statErr != nil || !info.IsDir() {
		t.Fatalf("managed checkout absent: %v", statErr)
	}
	if !worktreeBranchExists(t, root, "default-creation") {
		t.Fatal("managed branch absent")
	}
	if _, showErr := manager.efforts.Show("default-creation"); showErr != nil {
		t.Fatalf("effort resident absent: %v", showErr)
	}
}

// invariant: tooling/effort-management:default-worktree-creation (TestNewEffortRollsBackOnlyWhenTopologyIsProvenAbsent)
func TestNewEffortRollsBackOnlyWhenTopologyIsProvenAbsent(t *testing.T) {
	t.Run("rolled back", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		manager := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.worktreeAdd = func(context.Context, string, string, string) error { return failing("worktree add") }
		}))
		_, _, err := manager.NewEffort(testContext(t), effort.NewInput{Slug: "rolled-back", Title: "Rolled back"}, "")
		if err == nil || !strings.Contains(err.Error(), "effort rolled-back rolled back") || !strings.Contains(err.Error(), "retry `awf effort new --slug \"rolled-back\" \"Rolled back\"`") {
			t.Fatalf("rollback error = %v", err)
		}
		var creation *CreationError
		if !errors.As(err, &creation) || !errors.Is(err, creation.Cause) || creation.RollbackCause != nil {
			t.Fatalf("rolled-back creation lost typed mechanism identity: %v", err)
		}
		const want = "condition: managed worktree creation failed and the effort was rolled back\nstate: operation\ncause: injected worktree add\n\ndiagnostic:\n  changed:\n    effort resident: no\n    managed topology: no\n  steps:\n    step 1: fix the reported cause\n    step 2: retry `awf effort new --slug \"rolled-back\" \"Rolled back\"`\n"
		if got := renderedTopologyDiagnostic(t, creation); got != want {
			t.Fatalf("rollback before rename diagnostic = %q, want %q", got, want)
		}
		if _, statErr := os.Lstat(filepath.Join(root, ".awf", "efforts", "rolled-back")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("rolled-back effort resident remains: %v", statErr)
		}
	})

	t.Run("retained with topology", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		manager := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.worktreeAdd = func(ctx context.Context, path, branch, base string) error {
				if err := stub.Runner.WorktreeAdd(ctx, path, branch, base); err != nil {
					return err
				}
				return failing("post-add failure")
			}
		}))
		_, _, err := manager.NewEffort(testContext(t), effort.NewInput{Slug: "retained-topology", Title: "Retained topology"}, "")
		if err == nil || !strings.Contains(err.Error(), "retained: managed topology remains") || !strings.Contains(err.Error(), "git worktree list --porcelain") {
			t.Fatalf("retained error = %v", err)
		}
		var creation *CreationError
		if !errors.As(err, &creation) || !errors.Is(err, creation.Cause) || !errors.Is(err, creation.RollbackCause) || !errors.Is(err, effort.ErrManagedTopologyPresent) {
			t.Fatalf("retained creation lost typed mechanism identity: %v", err)
		}
		want := "condition: managed worktree creation failed and topology remains\nstate: operation\ncause: injected post-add failure\n\ndiagnostic:\n  changed:\n    effort resident: yes\n    managed topology: yes\n  steps:\n    step 1: inspect `git worktree list --porcelain`\n    step 2: clean up with native Git or `awf effort worktree remove retained-topology`\n    step 3: retry `awf effort worktree add retained-topology` or finish the effort\n"
		if got := renderedTopologyDiagnostic(t, creation); got != want {
			t.Fatalf("retained topology diagnostic = %q, want %q", got, want)
		}
		if _, showErr := manager.efforts.Show("retained-topology"); showErr != nil {
			t.Fatalf("retained effort was removed: %v", showErr)
		}
	})
}

// invariant: tooling/effort-management:default-worktree-creation (TestNewEffortReportsInterruptedAndFailedRollbacksDistinctly)
func TestNewEffortReportsInterruptedAndFailedRollbacksDistinctly(t *testing.T) {
	for _, test := range []struct {
		name     string
		stage    string
		wants    []string
		removed  bool
		reserved bool
	}{
		{
			name:  "rollback failed before reservation",
			stage: "rollback.rename",
			wants: []string{"retained: rollback failed", "retry `awf effort worktree add retained-rollback`"},
		},
		{
			name:     "interrupted after reservation",
			stage:    "rollback.root-fsync",
			wants:    []string{"deletion rollback was interrupted", "inspect the finishing reservation"},
			removed:  true,
			reserved: true,
		},
		{
			name:    "durability uncertain after deletion",
			stage:   "rollback.delete-fsync",
			wants:   []string{"deletion completed with parent durability uncertainty", "verify the active resident"},
			removed: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := initWorktreeRepo(t, "sha1")
			manager := managerWithFaultingEfforts(t, root, test.stage)
			_, _, err := manager.NewEffort(testContext(t), effort.NewInput{Slug: "retained-rollback", Title: "Retained rollback"}, "")
			if err == nil {
				t.Fatal("faulted rollback reported success")
			}
			for _, want := range test.wants {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %v does not report %q", err, want)
				}
			}
			var creation *CreationError
			if !errors.As(err, &creation) || !errors.Is(err, creation.Cause) || !errors.Is(err, creation.RollbackCause) {
				t.Fatalf("faulted rollback lost typed mechanism identity: %v", err)
			}
			want := "condition: managed worktree creation failed and effort rollback failed\nstate: operation\ncause: injected worktree add | injected failure at rollback.rename: injected rollback.rename\n\ndiagnostic:\n  changed:\n    effort resident: yes\n    managed topology: no\n  steps:\n    step 1: resolve the rollback failure\n    step 2: retry `awf effort worktree add retained-rollback` or `awf effort finish retained-rollback`\n"
			if test.stage == "rollback.root-fsync" {
				want = "condition: managed worktree creation failed and effort deletion rollback was interrupted\nstate: operation\ncause: injected worktree add | sync efforts parent after rollback reservation: injected failure at rollback.root-fsync: injected rollback.root-fsync\n\ndiagnostic:\n  changed:\n    effort resident: yes\n    managed topology: no\n  steps:\n    step 1: inspect the identity-bound finishing reservation\n    step 2: complete safe manual cleanup only after verifying its immutable identity\n"
			}
			if test.stage == "rollback.delete-fsync" {
				want = "condition: managed worktree creation failed after effort deletion with durability uncertainty\nstate: operation\ncause: injected worktree add | sync efforts parent after rollback deletion: injected failure at rollback.delete-fsync: injected rollback.delete-fsync\n\ndiagnostic:\n  changed:\n    effort resident: no\n    managed topology: no\n  steps:\n    step 1: verify `.awf/efforts/retained-rollback` is absent\n    step 2: verify `.awf/efforts/.finishing-018f47a0-7b3d-4c52-8f1a-123456789abc-retained-rollback` is absent\n    step 3: retry effort creation only after both paths are absent\n"
			}
			if got := renderedTopologyDiagnostic(t, creation); got != want {
				t.Fatalf("%s diagnostic = %q, want %q", test.name, got, want)
			}
			_, statErr := os.Lstat(filepath.Join(root, ".awf", "efforts", "retained-rollback"))
			if test.removed != errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("active resident presence = %v, want removed=%v", statErr, test.removed)
			}
			reservation := filepath.Join(root, ".awf", "efforts", ".finishing-"+worktreeTestID+"-retained-rollback")
			_, reservationErr := os.Lstat(reservation)
			if test.reserved == errors.Is(reservationErr, os.ErrNotExist) {
				t.Fatalf("reservation presence = %v, want reserved=%v", reservationErr, test.reserved)
			}
		})
	}
}

func TestNewEffortReturnsResidentFailuresBeforeAnyTopology(t *testing.T) {
	root := initWorktreeRepo(t, "sha1")
	added := false
	manager := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
		stub.worktreeAdd = func(ctx context.Context, path, branch, base string) error {
			added = true
			return stub.Runner.WorktreeAdd(ctx, path, branch, base)
		}
	}))
	for _, test := range []struct {
		name  string
		input effort.NewInput
		want  string
	}{
		{name: "blank title", input: effort.NewInput{Slug: "", Title: "   "}, want: "invalid outcome title"},
		{name: "33-byte new slug", input: effort.NewInput{Slug: strings.Repeat("s", 33), Title: "Overlong slug"}, want: "1-32 bytes"},
	} {
		t.Run(test.name, func(t *testing.T) {
			record, result, err := manager.NewEffort(testContext(t), test.input, "")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("resident error = %v, want %q", err, test.want)
			}
			if record != (effort.Record{}) || result != (Result{}) {
				t.Fatalf("record=%#v result=%#v, want zero values", record, result)
			}
			if added {
				t.Fatal("add was attempted after a resident failure")
			}
		})
	}
}

func managerWithFaultingEfforts(t *testing.T, root, stage string) *Manager {
	t.Helper()
	roots := worktreeControlRoots(t, root)
	service := newEffortService(t, roots, func() (string, error) { return worktreeTestID, nil }, func(got string) error {
		if got == stage {
			return failing(stage)
		}
		return nil
	})
	open := invokingStub(root, func(stub *checkoutStub) {
		stub.worktreeAdd = func(context.Context, string, string, string) error { return failing("worktree add") }
	})
	manager, err := Open(roots, open, service)
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func TestResolveCommitAcceptsSHA1AndSHA256ObjectIDs(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			root := initWorktreeRepo(t, format)
			repo, err := awfgit.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			id, err := repo.ResolveCommit(testContext(t), "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			want := 40
			if format == "sha256" {
				want = 64
			}
			if len(id) != want {
				t.Fatalf("object ID length = %d, want %d", len(id), want)
			}
		})
	}
}

func TestManagerRefusesAMissingDependency(t *testing.T) {
	root := initWorktreeRepo(t, "sha1")
	roots := worktreeControlRoots(t, root)
	service := newEffortService(t, roots, nil, nil)
	for name, open := range map[string]OpenCheckout{"checkout opener": nil, "effort service": openCheckout} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				message, ok := recover().(string)
				if !ok || !strings.Contains(message, name) {
					t.Fatalf("panic = %v, want one naming the %s dependency", message, name)
				}
			}()
			if open == nil {
				_, _ = Open(roots, nil, service)
			} else {
				_, _ = Open(roots, open, nil)
			}
			t.Fatal("missing dependency accepted")
		})
	}
	if _, err := Open(roots, func(string) (Runner, error) { return nil, failing("open") }, service); err == nil {
		t.Fatal("unopenable invoking checkout accepted")
	}
}

func TestManagerValidationAndOperationRefusals(t *testing.T) {
	root := initWorktreeRepo(t, "sha1")
	createEffort(t, root, "validation-result", "Validation result")
	manager := freshWorktreeManager(t, root)
	foreign := initWorktreeRepo(t, "sha1")
	if err := manager.validateManagedTarget(testContext(t), foreign); err == nil {
		t.Fatal("foreign managed target accepted")
	}
	for name, invoking := range map[string]string{
		"missing": filepath.Join(root, "missing"),
		"foreign": foreign,
	} {
		t.Run(name, func(t *testing.T) {
			drifted := managerRooted(t, root, func(roots *awfgit.ControlRoots) { roots.InvokingRoot = invoking }, nil)
			if err := drifted.validateLiveInvokingCheckout(testContext(t)); err == nil {
				t.Fatalf("%s invoking checkout accepted", name)
			}
		})
	}

	for _, operation := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply"} {
		t.Run(operation, func(t *testing.T) {
			present := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
				stub.gitPath = markerAt(t, operation)
			}))
			err := operationFree(testContext(t), present.git)
			var refused *RefusalError
			if !errors.As(err, &refused) || refused.Category != "operation" {
				t.Fatalf("operation error = %v, want an operation refusal", err)
			}
			// Only a merge conditions resolution on ownership; the other four
			// operations are unambiguously the caller's own to finish or abort.
			merge := operation == "MERGE_HEAD"
			if strings.Contains(refused.NextAction, "only if you started it") != merge {
				t.Fatalf("%s refusal = %v, want merge advice only for MERGE_HEAD", operation, refused)
			}
		})
	}
	faulted := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
		stub.gitPath = func(context.Context, string) (string, error) { return "", failing("git path") }
	}))
	if err := operationFree(testContext(t), faulted.git); err == nil {
		t.Fatal("operation probe error hidden")
	}
}

// A merge is the one in-progress operation whose resolution destroys work the
// caller may not own, so its refusal conditions finishing or aborting on having
// started it, named or not, and names the effort when one is provable.
func TestMergeRefusalConditionsResolutionOnOwnership(t *testing.T) {
	for name, expect := range map[string]struct {
		list func(context.Context) ([]awfgit.WorktreeRegistration, error)
		slug string
	}{
		"attributed to the effort whose tip is being merged": {
			list: registrations(
				awfgit.WorktreeRegistration{Path: "/primary", Branch: "refs/heads/main", HEAD: mergedTip},
				awfgit.WorktreeRegistration{Path: "/managed/peer", Branch: "refs/heads/awf/peer", HEAD: mergedTip},
			),
			slug: "peer",
		},
		"unattributed when no effort branch is at the merged tip": {
			list: registrations(awfgit.WorktreeRegistration{Path: "/managed/peer", Branch: "refs/heads/awf/peer", HEAD: otherTip}),
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := operationFree(testContext(t), &checkoutStub{
				gitPath:       markerAt(t, "MERGE_HEAD"),
				resolveCommit: mergeHeadAt(mergedTip),
				worktreeList:  expect.list,
			})
			if err == nil {
				t.Fatal("merge in progress accepted")
			}
			if !strings.Contains(err.Error(), "finish or abort this merge only if you started it") {
				t.Fatalf("refusal = %v, want ownership-conditioned advice", err)
			}
			named := strings.Contains(err.Error(), "effort "+expect.slug)
			if (expect.slug != "") != named {
				t.Fatalf("refusal = %v, want attribution to %q", err, expect.slug)
			}
		})
	}
}

// Attribution is restricted to effort branches, and to a slug that exists. A
// candidate that fails either guard must not end the scan, or a registration
// listed after it would never be reached.
func TestIntegrationHolderSkipsNonEffortRegistrations(t *testing.T) {
	for name, listed := range map[string][]awfgit.WorktreeRegistration{
		"ordinary branch at the merged tip": {
			{Path: "/primary", Branch: "refs/heads/main", HEAD: mergedTip},
			{Path: "/managed/peer", Branch: "refs/heads/awf/peer", HEAD: mergedTip},
		},
		"effort prefix with an empty slug at the merged tip": {
			{Path: "/managed", Branch: "refs/heads/awf/", HEAD: mergedTip},
			{Path: "/managed/peer", Branch: "refs/heads/awf/peer", HEAD: mergedTip},
		},
	} {
		t.Run(name, func(t *testing.T) {
			holder := integrationHolder(testContext(t), &checkoutStub{
				resolveCommit: mergeHeadAt(mergedTip),
				worktreeList:  registrations(listed...),
			})
			if holder != "peer" {
				t.Fatalf("holder = %q, want peer", holder)
			}
		})
	}
}

// A probe that cannot answer leaves the merge unattributed rather than
// propagating: the refusal it decorates is already correct without a name.
func TestIntegrationHolderAnswersUnattributed(t *testing.T) {
	for name, checkout := range map[string]*checkoutStub{
		"merge head unresolvable": {
			resolveCommit: func(context.Context, string) (string, error) { return "", failing("resolve") },
		},
		// The listing carries a registration that would attribute the merge, so
		// swallowing its error names a holder read from a failed probe.
		"registrations unreadable": {
			resolveCommit: mergeHeadAt(mergedTip),
			worktreeList: func(context.Context) ([]awfgit.WorktreeRegistration, error) {
				return []awfgit.WorktreeRegistration{
					{Path: "/managed/peer", Branch: "refs/heads/awf/peer", HEAD: mergedTip},
				}, failing("worktree list")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if slug := integrationHolder(testContext(t), checkout); slug != "" {
				t.Fatalf("holder = %q, want unattributed", slug)
			}
		})
	}
}

// The cleanliness refusal cannot attribute unstaged work, so it warns rather
// than directing an agent to discard changes that may not be its own.
func TestCleanlinessRefusalWarnsBeforeDiscarding(t *testing.T) {
	dirty := &checkoutStub{changeCounts: func(context.Context) (int, int, error) { return 1, 0, nil }}
	err := requireClean(testContext(t), dirty)
	if err == nil || !strings.Contains(err.Error(), "concurrent") {
		t.Fatalf("refusal = %v, want one warning about concurrent work", err)
	}
}

func TestManagerAuthorityErrorBranches(t *testing.T) {
	m, root := newManagerWithEffort(t, "authority-errors", "Authority errors")
	unrooted := managerRooted(t, root, func(roots *awfgit.ControlRoots) { roots.PrimaryRoot = "relative" }, nil)
	if _, err := unrooted.managed("authority-errors"); err == nil {
		t.Fatal("invalid managed root accepted")
	}
	if err := unrooted.validateManagedTarget(testContext(t), t.TempDir()); err == nil {
		t.Fatal("invalid resident authority accepted")
	}
	plain := t.TempDir()
	if err := m.validateManagedTarget(testContext(t), plain); err == nil || !strings.Contains(err.Error(), "repository-identity") {
		t.Fatalf("plain managed target error = %v", err)
	}
	plainInvoking := managerRooted(t, root, func(roots *awfgit.ControlRoots) { roots.InvokingRoot = plain }, nil)
	if err := plainInvoking.validateLiveInvokingCheckout(testContext(t)); err == nil || !strings.Contains(err.Error(), "repository-identity") {
		t.Fatalf("plain invoking error = %v", err)
	}
}

func TestAddPreconditionAndRunnerFailureBranches(t *testing.T) {
	t.Run("existing path", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "add-path", "Add path")
		path := filepath.Join(root, ".awf", "worktrees", "add-path")
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := m.Add(testContext(t), "add-path", "HEAD"); err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("existing branch", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "add-branch", "Add branch")
		gitfixture.NativeBranch(t, gitfixture.At(root), "awf/add-branch")
		if _, err := m.Add(testContext(t), "add-branch", "HEAD"); err == nil || !strings.Contains(err.Error(), "branch already exists") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("registered branch", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		createEffort(t, root, "add-registered", "Add registered")
		m := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.worktreeList = func(context.Context) ([]awfgit.WorktreeRegistration, error) {
				return []awfgit.WorktreeRegistration{
					{Path: root, HEAD: "abc", Branch: "refs/heads/main"},
					{Path: "/foreign", HEAD: "def", Branch: "refs/heads/awf/add-registered"},
				}, nil
			}
		}))
		if _, err := m.Add(testContext(t), "add-registered", "HEAD"); err == nil || !strings.Contains(err.Error(), "registration") {
			t.Fatalf("error = %v", err)
		}
	})
	for _, tc := range []struct {
		name      string
		breakStub func(*checkoutStub)
	}{
		{"registrations", func(s *checkoutStub) {
			s.worktreeList = func(context.Context) ([]awfgit.WorktreeRegistration, error) { return nil, failing("worktree list") }
		}},
		{"branch probe", func(s *checkoutStub) {
			s.branchProbe = func(context.Context, string) (bool, error) { return false, failing("show-ref") }
		}},
		{"operation", func(s *checkoutStub) {
			s.gitPath = func(context.Context, string) (string, error) { return "", failing("git path") }
		}},
		{"base", func(s *checkoutStub) {
			s.resolveCommit = func(context.Context, string) (string, error) { return "", failing("rev-parse") }
		}},
		{"add", func(s *checkoutStub) {
			s.worktreeAdd = func(context.Context, string, string, string) error { return failing("worktree add") }
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := initWorktreeRepo(t, "sha1")
			createEffort(t, root, "add-"+strings.ReplaceAll(tc.name, " ", "-"), "Add "+tc.name)
			m := managerWith(t, root, invokingStub(root, tc.breakStub))
			if _, err := m.Add(testContext(t), "add-"+strings.ReplaceAll(tc.name, " ", "-"), "HEAD"); err == nil {
				t.Fatal("runner fault hidden")
			}
		})
	}
	t.Run("invalid effort and stat fault", func(t *testing.T) {
		m, _ := newManagerWithEffort(t, "add-stat-fault", "Add stat fault")
		if _, err := m.Add(testContext(t), "missing-effort", "HEAD"); err == nil {
			t.Fatal("missing effort accepted")
		}
		old := managedLstat
		managedLstat = func(string) (os.FileInfo, error) { return nil, errors.New("stat fault") }
		defer func() { managedLstat = old }()
		if _, err := m.Add(testContext(t), "add-stat-fault", "HEAD"); err == nil || !strings.Contains(err.Error(), "stat fault") {
			t.Fatalf("stat error = %v", err)
		}
	})
	t.Run("topology registration probe", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		createEffort(t, root, "topology-probe", "Topology probe")
		var path string
		m := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.worktreeList = func(context.Context) ([]awfgit.WorktreeRegistration, error) {
				return []awfgit.WorktreeRegistration{{Path: path, HEAD: "abc", Branch: "refs/heads/awf/topology-probe"}}, nil
			}
		}))
		resolved, err := m.managed("topology-probe")
		if err != nil {
			t.Fatal(err)
		}
		path = resolved
		old := managedLstat
		managedLstat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		defer func() { managedLstat = old }()
		if !m.topologyPresent(testContext(t), "topology-probe", path) {
			t.Fatal("registration topology was not observed")
		}
	})
	t.Run("post-add registration", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		createEffort(t, root, "add-post-registration", "Add post registration")
		added := false
		m := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.worktreeAdd = func(ctx context.Context, path, branch, base string) error {
				err := stub.Runner.WorktreeAdd(ctx, path, branch, base)
				added = err == nil
				return err
			}
			stub.worktreeList = func(ctx context.Context) ([]awfgit.WorktreeRegistration, error) {
				if added {
					return []awfgit.WorktreeRegistration{{Path: root, HEAD: "abc", Branch: "refs/heads/main"}}, nil
				}
				return stub.Runner.WorktreeList(ctx)
			}
		}))
		if _, err := m.Add(testContext(t), "add-post-registration", "HEAD"); err == nil || !strings.Contains(err.Error(), "exact managed registration") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestIntegratePreconditionAndMutationFailureBranches(t *testing.T) {
	t.Run("managed caller", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "managed-caller", "Managed caller")
		if _, err := m.Add(testContext(t), "managed-caller", "HEAD"); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ".awf", "worktrees", "managed-caller")
		fromManaged := managerRooted(t, root, func(roots *awfgit.ControlRoots) { roots.InvokingRoot = path }, nil)
		if _, err := fromManaged.Integrate(testContext(t), "managed-caller", ""); err == nil || !strings.Contains(err.Error(), "receiving checkout") {
			t.Fatalf("error = %v", err)
		}
	})
	for _, tc := range []struct {
		name      string
		breakStub func(*checkoutStub)
	}{
		{"registration", func(s *checkoutStub) {
			s.worktreeList = func(context.Context) ([]awfgit.WorktreeRegistration, error) { return nil, failing("worktree list") }
		}},
		{"operation", func(s *checkoutStub) {
			s.gitPath = func(context.Context, string) (string, error) { return "", failing("git path") }
		}},
		{"cleanliness", func(s *checkoutStub) {
			s.changeCounts = func(context.Context) (int, int, error) { return 0, 0, failing("status") }
		}},
		{"detached", func(s *checkoutStub) {
			s.currentBranch = func(context.Context) (string, error) { return "", failing("symbolic-ref") }
		}},
		{"tip", func(s *checkoutStub) {
			s.resolveCommit = func(ctx context.Context, revision string) (string, error) {
				if strings.HasPrefix(revision, "awf/") {
					return "", failing("rev-parse tip")
				}
				return s.Runner.ResolveCommit(ctx, revision)
			}
		}},
		{"target", func(s *checkoutStub) {
			s.resolveCommit = func(ctx context.Context, revision string) (string, error) {
				if revision == "HEAD" {
					return "", failing("rev-parse HEAD")
				}
				return s.Runner.ResolveCommit(ctx, revision)
			}
		}},
		{"ancestor", func(s *checkoutStub) {
			s.ancestor = func(context.Context, string, string) (bool, error) { return false, failing("merge-base") }
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := initWorktreeRepo(t, "sha1")
			slug := "integrate-" + strings.ReplaceAll(tc.name, " ", "-")
			createEffort(t, root, slug, "Integrate "+tc.name)
			if _, err := freshWorktreeManager(t, root).Add(testContext(t), slug, "HEAD"); err != nil {
				t.Fatal(err)
			}
			managed := filepath.Join(root, ".awf", "worktrees", slug)
			writeWorktreeFile(t, filepath.Join(managed, "change"), "x")
			commitWorktree(t, managed, "change")
			m := managerWith(t, root, invokingStub(root, tc.breakStub))
			if _, err := m.Integrate(testContext(t), slug, ""); err == nil {
				t.Fatal("runner fault hidden")
			}
		})
	}
	t.Run("detached target", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		createEffort(t, root, "integrate-detached", "Integrate detached")
		if _, err := freshWorktreeManager(t, root).Add(testContext(t), "integrate-detached", "HEAD"); err != nil {
			t.Fatal(err)
		}
		m := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.currentBranch = func(context.Context) (string, error) { return "", nil }
		}))
		if _, err := m.Integrate(testContext(t), "integrate-detached", ""); err == nil || !strings.Contains(err.Error(), "detached") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("effort target branch", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		createEffort(t, root, "integrate-own", "Integrate own")
		if _, err := freshWorktreeManager(t, root).Add(testContext(t), "integrate-own", "HEAD"); err != nil {
			t.Fatal(err)
		}
		m := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.currentBranch = func(context.Context) (string, error) { return "awf/integrate-own", nil }
		}))
		if _, err := m.Integrate(testContext(t), "integrate-own", ""); err == nil || !strings.Contains(err.Error(), "effort branch") {
			t.Fatalf("error = %v root=%s", err, root)
		}
	})
	t.Run("fast-forward failure", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		createEffort(t, root, "integrate-ff-failure", "Integrate ff failure")
		if _, err := freshWorktreeManager(t, root).Add(testContext(t), "integrate-ff-failure", "HEAD"); err != nil {
			t.Fatal(err)
		}
		managed := filepath.Join(root, ".awf", "worktrees", "integrate-ff-failure")
		writeWorktreeFile(t, filepath.Join(managed, "change"), "x")
		commitWorktree(t, managed, "change")
		m := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.mergeFastForward = func(context.Context, string) error { return failing("merge --ff-only") }
		}))
		if _, err := m.Integrate(testContext(t), "integrate-ff-failure", ""); err == nil || !strings.Contains(err.Error(), "fast-forward failed") {
			t.Fatalf("error = %v", err)
		}
		before := gitfixture.NativeRevParse(t, gitfixture.At(root), "HEAD")
		if m.targetChanged(testContext(t), before) {
			t.Fatal("unchanged target reported changed")
		}
		if !m.targetChanged(testContext(t), "0000000000000000000000000000000000000000") {
			t.Fatal("changed target reported unchanged")
		}
	})
}

func TestManagerMutationPropagationBranches(t *testing.T) {
	t.Run("add authority", func(t *testing.T) {
		_, root := newManagerWithEffort(t, "add-authority", "Add authority")
		unrooted := managerRooted(t, root, func(roots *awfgit.ControlRoots) { roots.PrimaryRoot = "relative" }, nil)
		if _, err := unrooted.Add(testContext(t), "add-authority", "HEAD"); err == nil {
			t.Fatal("invalid add authority accepted")
		}
	})
	t.Run("add live identity", func(t *testing.T) {
		_, root := newManagerWithEffort(t, "add-live-identity", "Add live identity")
		plain := t.TempDir()
		m := managerRooted(t, root, func(roots *awfgit.ControlRoots) { roots.InvokingRoot = plain },
			func(stub *checkoutStub) {
				stub.worktreeList = func(context.Context) ([]awfgit.WorktreeRegistration, error) {
					return []awfgit.WorktreeRegistration{{Path: root, HEAD: "abc", Branch: "refs/heads/main"}}, nil
				}
				stub.gitPath = func(_ context.Context, name string) (string, error) { return filepath.Join(plain, name), nil }
				stub.resolveCommit = func(context.Context, string) (string, error) {
					return "0000000000000000000000000000000000000000", nil
				}
			})
		if _, err := m.Add(testContext(t), "add-live-identity", "HEAD"); err == nil || !strings.Contains(err.Error(), "repository-identity") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("integrate authority and fact propagation", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		createEffort(t, root, "integrate-propagation", "Integrate propagation")
		if _, err := freshWorktreeManager(t, root).Add(testContext(t), "integrate-propagation", "HEAD"); err != nil {
			t.Fatal(err)
		}
		managed := filepath.Join(root, ".awf", "worktrees", "integrate-propagation")
		writeWorktreeFile(t, filepath.Join(managed, "change"), "x")
		commitWorktree(t, managed, "change")
		unrooted := managerRooted(t, root, func(roots *awfgit.ControlRoots) { roots.PrimaryRoot = "relative" }, nil)
		if _, err := unrooted.Integrate(testContext(t), "integrate-propagation", ""); err == nil {
			t.Fatal("invalid integrate authority accepted")
		}
		calls := 0
		drifting := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.resolveCommit = func(ctx context.Context, revision string) (string, error) {
				if revision == "HEAD" {
					calls++
					if calls == 2 {
						return "0000000000000000000000000000000000000000", nil
					}
				}
				return stub.Runner.ResolveCommit(ctx, revision)
			}
		}))
		if _, err := drifting.Integrate(testContext(t), "integrate-propagation", ""); err == nil || !strings.Contains(err.Error(), "target HEAD changed") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("validate fact prerequisites", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		createEffort(t, root, "validate-prerequisites", "Validate prerequisites")
		m := freshWorktreeManager(t, root)
		if _, err := m.Add(testContext(t), "validate-prerequisites", "HEAD"); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ".awf", "worktrees", "validate-prerequisites")
		target := gitfixture.NativeRevParse(t, gitfixture.At(root), "HEAD")
		tip := gitfixture.NativeRevParse(t, gitfixture.At(root), "awf/validate-prerequisites")
		drifted := managerRooted(t, root, func(roots *awfgit.ControlRoots) { roots.InvokingRoot = t.TempDir() }, nil)
		if err := drifted.validateIntegrationFacts(testContext(t), path, "validate-prerequisites", target, tip); err == nil {
			t.Fatal("invalid invoking checkout accepted")
		}
		if err := m.validateIntegrationFacts(testContext(t), filepath.Join(root, "missing"), "validate-prerequisites", target, tip); err == nil {
			t.Fatal("missing managed target accepted")
		}
		probeFault := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.worktreeList = func(context.Context) ([]awfgit.WorktreeRegistration, error) { return nil, failing("worktree list") }
		}))
		if err := probeFault.validateIntegrationFacts(testContext(t), path, "validate-prerequisites", target, tip); err == nil {
			t.Fatal("registration probe fault hidden")
		}
	})
	t.Run("remove authority and target", func(t *testing.T) {
		_, root := newManagerWithEffort(t, "remove-propagation", "Remove propagation")
		if _, err := freshWorktreeManager(t, root).Remove(testContext(t), "missing-effort"); err == nil {
			t.Fatal("missing effort accepted")
		}
		unrooted := managerRooted(t, root, func(roots *awfgit.ControlRoots) { roots.PrimaryRoot = "relative" }, nil)
		if _, err := unrooted.Remove(testContext(t), "remove-propagation"); err == nil {
			t.Fatal("invalid remove authority accepted")
		}
		m, root := newManagerWithEffort(t, "remove-target-error", "Remove target error")
		if _, err := m.Add(testContext(t), "remove-target-error", "HEAD"); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ".awf", "worktrees", "remove-target-error")
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := m.Remove(testContext(t), "remove-target-error"); err == nil || !strings.Contains(err.Error(), "repository-identity") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestIntegrationAdditionalRefusalBranches(t *testing.T) {
	t.Run("missing effort and target", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "integration-missing-target", "Integration missing target")
		if _, err := m.Integrate(testContext(t), "missing-effort", ""); err == nil {
			t.Fatal("missing effort accepted")
		}
		if _, err := m.Add(testContext(t), "integration-missing-target", "HEAD"); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ".awf", "worktrees", "integration-missing-target")
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
		if _, err := m.Integrate(testContext(t), "integration-missing-target", ""); err == nil {
			t.Fatal("missing target accepted")
		}
	})
	t.Run("second ancestry probe", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		createEffort(t, root, "integration-second-ancestry", "Integration second ancestry")
		if _, err := freshWorktreeManager(t, root).Add(testContext(t), "integration-second-ancestry", "HEAD"); err != nil {
			t.Fatal(err)
		}
		managed := filepath.Join(root, ".awf", "worktrees", "integration-second-ancestry")
		writeWorktreeFile(t, filepath.Join(managed, "change"), "x")
		commitWorktree(t, managed, "change")
		calls := 0
		m := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.ancestor = func(ctx context.Context, older, newer string) (bool, error) {
				calls++
				if calls == 2 {
					return false, errors.New("second ancestry")
				}
				return stub.Runner.Ancestor(ctx, older, newer)
			}
		}))
		if _, err := m.Integrate(testContext(t), "integration-second-ancestry", ""); err == nil || !strings.Contains(err.Error(), "second ancestry") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestIntegrationFactDriftBranches(t *testing.T) {
	root := initWorktreeRepo(t, "sha1")
	createEffort(t, root, "fact-drift", "Fact drift")
	m := freshWorktreeManager(t, root)
	if _, err := m.Add(testContext(t), "fact-drift", "HEAD"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".awf", "worktrees", "fact-drift")
	target := gitfixture.NativeRevParse(t, gitfixture.At(root), "HEAD")
	tip := gitfixture.NativeRevParse(t, gitfixture.At(root), "awf/fact-drift")
	if err := m.validateIntegrationFacts(testContext(t), path, "fact-drift", "0000000000000000000000000000000000000000", tip); err == nil || !strings.Contains(err.Error(), "target HEAD changed") {
		t.Fatalf("target drift error = %v", err)
	}
	if err := m.validateIntegrationFacts(testContext(t), path, "fact-drift", target, "0000000000000000000000000000000000000000"); err == nil || !strings.Contains(err.Error(), "effort branch changed") {
		t.Fatalf("tip drift error = %v", err)
	}
	targetFault := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
		stub.resolveCommit = func(ctx context.Context, revision string) (string, error) {
			if revision == "HEAD" {
				return "", failing("target resolve")
			}
			return stub.Runner.ResolveCommit(ctx, revision)
		}
	}))
	if err := targetFault.validateIntegrationFacts(testContext(t), path, "fact-drift", target, tip); err == nil || !strings.Contains(err.Error(), "target HEAD changed") {
		t.Fatalf("target resolve error = %v", err)
	}
	tipFault := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
		stub.resolveCommit = func(ctx context.Context, revision string) (string, error) {
			if revision == "awf/fact-drift" {
				return "", failing("tip resolve")
			}
			return stub.Runner.ResolveCommit(ctx, revision)
		}
	}))
	if err := tipFault.validateIntegrationFacts(testContext(t), path, "fact-drift", target, tip); err == nil || !strings.Contains(err.Error(), "effort branch changed") {
		t.Fatalf("tip resolve error = %v", err)
	}
}

func TestRemovalPostMutationProbeFailureIsActionable(t *testing.T) {
	cause := failing("worktree list after removal")
	if got := removalProbeFailure(false, "managed topology probe failed during removal", cause); !errors.Is(got, cause) {
		t.Fatalf("pre-mutation probe error = %v, want original cause", got)
	}
	prior := refusal("ancestry", "managed branch is not merged", false, "integrate first")
	wrapped := removalProbeFailure(true, "managed checkout operation probe failed during removal", prior)
	var semantic *RefusalError
	if !errors.As(wrapped, &semantic) || semantic.Category != "ancestry" || !semantic.ChangedTopology || !errors.Is(wrapped, prior) {
		t.Fatalf("semantic post-mutation probe error = %#v", wrapped)
	}
	for _, category := range []string{"repository-identity", "symlink", "file-type"} {
		safety := &awfgit.HardSafetyError{Category: category, Path: "/managed", Err: failing(category)}
		wrapped = removalProbeFailure(true, "managed target validation failed during removal", safety)
		if !errors.As(wrapped, &semantic) || semantic.Category != "repository-identity" || !semantic.ChangedTopology || !errors.Is(wrapped, safety) {
			t.Fatalf("%s safety post-mutation probe error = %#v", category, wrapped)
		}
	}

	root := initWorktreeRepo(t, "sha1")
	const slug = "post-mutation-probe"
	createEffort(t, root, slug, "Post mutation probe")
	if _, err := freshWorktreeManager(t, root).Add(testContext(t), slug, "HEAD"); err != nil {
		t.Fatal(err)
	}
	calls := 0
	m := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
		stub.worktreeList = func(ctx context.Context) ([]awfgit.WorktreeRegistration, error) {
			calls++
			if calls == 2 {
				return nil, cause
			}
			return stub.Runner.WorktreeList(ctx)
		}
	}))
	_, err := m.Remove(testContext(t), slug)
	var refused *RefusalError
	if !errors.As(err, &refused) || refused.Category != "operation" || !refused.ChangedTopology || !errors.Is(err, cause) {
		t.Fatalf("post-mutation worktree-list error = %#v", err)
	}
	const want = "condition: managed topology probe failed during removal\nstate: operation\ncause: injected worktree list after removal\n\ndiagnostic:\n  changed:\n    managed topology: yes\n  steps:\n    step 1: run `git worktree list --porcelain`\n    step 2: inspect the managed path and branch\n    step 3: resolve the reported probe failure\n    step 4: retry ordinary removal\n"
	if got := renderedTopologyDiagnostic(t, refused); got != want {
		t.Fatalf("post-mutation worktree-list diagnostic = %q, want %q", got, want)
	}
	if calls != 2 {
		t.Fatalf("worktree-list calls = %d, want failure after first topology mutation", calls)
	}
	if _, statErr := os.Lstat(filepath.Join(root, ".awf", "worktrees", slug)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("first topology mutation did not remove managed path: %v", statErr)
	}
}

func TestRemovalPartialTopologyAndFailureBranches(t *testing.T) {
	// Removal probes both checkouts, and a merge in either refuses. Both say the
	// same thing, because the caller can resolve only a merge it started: the
	// managed case is exactly where that merge provably is the caller's own.
	for name, locate := range map[string]func(root string) string{
		"target operation":  func(root string) string { return root },
		"managed operation": func(root string) string { return filepath.Join(root, ".awf", "worktrees", "remove-operation") },
	} {
		t.Run(name, func(t *testing.T) {
			m, root := newManagerWithEffort(t, "remove-operation", "Remove operation")
			if _, err := m.Add(testContext(t), "remove-operation", "HEAD"); err != nil {
				t.Fatal(err)
			}
			checkout := locate(root)
			mergeHead := gitfixture.NativeGitPath(t, gitfixture.At(checkout), "MERGE_HEAD")
			if !filepath.IsAbs(mergeHead) {
				mergeHead = filepath.Join(checkout, mergeHead)
			}
			if err := os.WriteFile(mergeHead, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := m.Remove(testContext(t), "remove-operation")
			var refused *RefusalError
			if !errors.As(err, &refused) || refused.Category != "operation" {
				t.Fatalf("error = %v, want an operation refusal", err)
			}
			if !strings.Contains(refused.NextAction, "finish or abort this merge only if you started it") {
				t.Fatalf("refusal = %v, want ownership-conditioned advice", refused)
			}
		})
	}
	t.Run("remove and branch failures", func(t *testing.T) {
		for name, broken := range map[string]func(*checkoutStub){
			"worktree remove": func(s *checkoutStub) {
				s.worktreeRemove = func(context.Context, string) error { return failing("worktree remove") }
			},
			"branch delete": func(s *checkoutStub) {
				s.branchDelete = func(context.Context, string) error { return failing("branch -d") }
			},
		} {
			t.Run(name, func(t *testing.T) {
				root := initWorktreeRepo(t, "sha1")
				slug := "remove-" + strings.ReplaceAll(name, " ", "-")
				createEffort(t, root, slug, "Remove "+name)
				if _, err := freshWorktreeManager(t, root).Add(testContext(t), slug, "HEAD"); err != nil {
					t.Fatal(err)
				}
				if name == "branch delete" {
					gitfixture.NativeWorktreeRemove(t, gitfixture.At(root), filepath.Join(root, ".awf", "worktrees", slug))
				}
				m := managerWith(t, root, invokingStub(root, broken))
				if _, err := m.Remove(testContext(t), slug); err == nil {
					t.Fatal("mutation fault hidden")
				}
			})
		}
	})
	t.Run("runner probes", func(t *testing.T) {
		for name, broken := range map[string]func(*checkoutStub){
			"cleanliness": func(s *checkoutStub) {
				s.changeCounts = func(context.Context) (int, int, error) { return 0, 0, failing("status") }
			},
			"worktree list": func(s *checkoutStub) {
				s.worktreeList = func(context.Context) ([]awfgit.WorktreeRegistration, error) { return nil, failing("worktree list") }
			},
			"branch probe": func(s *checkoutStub) {
				s.branchProbe = func(context.Context, string) (bool, error) { return false, failing("show-ref") }
			},
			"ancestry": func(s *checkoutStub) {
				s.ancestor = func(context.Context, string, string) (bool, error) { return false, failing("merge-base") }
			},
		} {
			t.Run(name, func(t *testing.T) {
				root := initWorktreeRepo(t, "sha1")
				slug := "remove-probe-" + strings.ReplaceAll(name, " ", "-")
				createEffort(t, root, slug, "Remove probe "+name)
				if _, err := freshWorktreeManager(t, root).Add(testContext(t), slug, "HEAD"); err != nil {
					t.Fatal(err)
				}
				if name == "ancestry" {
					managed := filepath.Join(root, ".awf", "worktrees", slug)
					writeWorktreeFile(t, filepath.Join(managed, "change"), "x")
					commitWorktree(t, managed, "change")
				}
				m := managerWith(t, root, invokingStub(root, broken))
				if _, err := m.Remove(testContext(t), slug); err == nil {
					t.Fatal("probe fault hidden")
				}
			})
		}
	})
	t.Run("managed operation and dirt", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		createEffort(t, root, "remove-managed-checks", "Remove managed checks")
		m := freshWorktreeManager(t, root)
		if _, err := m.Add(testContext(t), "remove-managed-checks", "HEAD"); err != nil {
			t.Fatal(err)
		}
		managed := filepath.Join(root, ".awf", "worktrees", "remove-managed-checks")
		probeFault := managerWith(t, root, stubOpener(func(opened string, stub *checkoutStub) {
			if sameWorktreePath(opened, managed) {
				stub.gitPath = func(context.Context, string) (string, error) { return "", failing("managed operation probe") }
			}
		}))
		if _, err := probeFault.Remove(testContext(t), "remove-managed-checks"); err == nil {
			t.Fatal("managed operation probe hidden")
		}
		unopenable := managerWith(t, root, func(opened string) (Runner, error) {
			if sameWorktreePath(opened, managed) {
				return nil, failing("managed checkout open")
			}
			return awfgit.Open(opened)
		})
		if _, err := unopenable.Remove(testContext(t), "remove-managed-checks"); err == nil || !strings.Contains(err.Error(), "managed checkout open") {
			t.Fatalf("managed open error = %v", err)
		}
		writeWorktreeFile(t, filepath.Join(managed, "dirty"), "x")
		if _, err := m.Remove(testContext(t), "remove-managed-checks"); err == nil || !strings.Contains(err.Error(), "dirty") {
			t.Fatalf("dirty error = %v", err)
		}
	})
	t.Run("foreign registration", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		createEffort(t, root, "remove-foreign-registration", "Remove foreign registration")
		m := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.worktreeList = func(context.Context) ([]awfgit.WorktreeRegistration, error) {
				return []awfgit.WorktreeRegistration{
					{Path: root, HEAD: "abc", Branch: "refs/heads/main"},
					{Path: "/foreign", HEAD: "def", Branch: "refs/heads/awf/remove-foreign-registration"},
				}, nil
			}
		}))
		if _, err := m.Remove(testContext(t), "remove-foreign-registration"); err == nil || !strings.Contains(err.Error(), "foreign path") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("managed caller and malformed registration", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "remove-caller", "Remove caller")
		if _, err := m.Add(testContext(t), "remove-caller", "HEAD"); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ".awf", "worktrees", "remove-caller")
		fromManaged := managerRooted(t, root, func(roots *awfgit.ControlRoots) { roots.InvokingRoot = path }, nil)
		if _, err := fromManaged.Remove(testContext(t), "remove-caller"); err == nil || !strings.Contains(err.Error(), "target checkout") {
			t.Fatalf("caller error = %v", err)
		}

		root = initWorktreeRepo(t, "sha1")
		createEffort(t, root, "remove-malformed", "Remove malformed")
		malformedPath := filepath.Join(root, ".awf", "worktrees", "remove-malformed")
		malformed := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.worktreeList = func(context.Context) ([]awfgit.WorktreeRegistration, error) {
				return []awfgit.WorktreeRegistration{
					{Path: root, HEAD: "abc", Branch: "refs/heads/main"},
					{Path: malformedPath, HEAD: "def", Detached: true},
				}, nil
			}
		}))
		if _, err := malformed.Remove(testContext(t), "remove-malformed"); err == nil || !strings.Contains(err.Error(), "not exact") {
			t.Fatalf("registration error = %v", err)
		}
	})
	t.Run("unregistered path cleanup", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		createEffort(t, root, "remove-unregistered", "Remove unregistered")
		if _, err := freshWorktreeManager(t, root).Add(testContext(t), "remove-unregistered", "HEAD"); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ".awf", "worktrees", "remove-unregistered")
		m := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.worktreeList = func(context.Context) ([]awfgit.WorktreeRegistration, error) {
				return []awfgit.WorktreeRegistration{{Path: root, HEAD: "abc", Branch: "refs/heads/main"}}, nil
			}
		}))
		if _, err := m.Remove(testContext(t), "remove-unregistered"); err == nil || !strings.Contains(err.Error(), "branch deletion failed") {
			t.Fatalf("error=%v", err)
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("path remains: %v", err)
		}
	})
	t.Run("prune success and failure", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "remove-prune-success", "Remove prune success")
		if _, err := m.Add(testContext(t), "remove-prune-success", "HEAD"); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ".awf", "worktrees", "remove-prune-success")
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
		if result, err := m.Remove(testContext(t), "remove-prune-success"); err != nil || !result.ChangedTopology {
			t.Fatalf("prune result=%#v err=%v", result, err)
		}

		root = initWorktreeRepo(t, "sha1")
		createEffort(t, root, "remove-prune", "Remove prune")
		if _, err := freshWorktreeManager(t, root).Add(testContext(t), "remove-prune", "HEAD"); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(filepath.Join(root, ".awf", "worktrees", "remove-prune")); err != nil {
			t.Fatal(err)
		}
		pruneFault := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.worktreePrune = func(context.Context) error { return failing("worktree prune") }
		}))
		if _, err := pruneFault.Remove(testContext(t), "remove-prune"); err == nil || !strings.Contains(err.Error(), "prunable") {
			t.Fatalf("error = %v", err)
		}
	})
}

// TestRestartCompletesFromPartialTopology proves the restartable property that
// makes the stateless design safe: after a fault between Remove's two
// mutations, a fresh manager reads the real topology and finishes from wherever
// the previous attempt stopped, with no stored evidence of that attempt.
func TestRestartCompletesFromPartialTopology(t *testing.T) {
	// invariant: tooling/effort-management:managed-worktree-lifecycle (TestRestartCompletesFromPartialTopology)
	t.Run("branch delete faulted", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		slug := "restart-branch-delete"
		createEffort(t, root, "restart-branch-delete", "Restart branch delete")
		if _, err := freshWorktreeManager(t, root).Add(testContext(t), slug, "HEAD"); err != nil {
			t.Fatal(err)
		}
		faulted := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.branchDelete = func(context.Context, string) error { return failing("branch -d") }
		}))
		if _, err := faulted.Remove(testContext(t), slug); err == nil {
			t.Fatal("branch delete fault hidden")
		}
		// Genuinely partial: the checkout is gone, the branch is not.
		if _, err := os.Stat(filepath.Join(root, ".awf", "worktrees", slug)); !os.IsNotExist(err) {
			t.Fatalf("managed path survived the faulted attempt: %v", err)
		}
		if !worktreeBranchExists(t, root, slug) {
			t.Fatal("branch already deleted, so the restart fixture proves nothing")
		}
		if _, err := freshWorktreeManager(t, root).Remove(testContext(t), slug); err != nil {
			t.Fatalf("restart did not complete from partial topology: %v", err)
		}
		if worktreeBranchExists(t, root, slug) {
			t.Fatal("restart left the branch behind")
		}
	})
	t.Run("worktree remove faulted", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		slug := "restart-worktree-remove"
		createEffort(t, root, "restart-worktree-remove", "Restart worktree remove")
		if _, err := freshWorktreeManager(t, root).Add(testContext(t), slug, "HEAD"); err != nil {
			t.Fatal(err)
		}
		faulted := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.worktreeRemove = func(context.Context, string) error { return failing("worktree remove") }
		}))
		if _, err := faulted.Remove(testContext(t), slug); err == nil {
			t.Fatal("worktree remove fault hidden")
		}
		managed := filepath.Join(root, ".awf", "worktrees", slug)
		if _, err := os.Stat(managed); err != nil {
			t.Fatalf("faulted removal discarded the checkout anyway: %v", err)
		}
		if _, err := freshWorktreeManager(t, root).Remove(testContext(t), slug); err != nil {
			t.Fatalf("restart did not complete after a failed first mutation: %v", err)
		}
		if _, err := os.Stat(managed); !os.IsNotExist(err) {
			t.Fatalf("restart left the managed path: %v", err)
		}
		if worktreeBranchExists(t, root, slug) {
			t.Fatal("restart left the branch behind")
		}
	})
	t.Run("add faulted before mutating", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		slug := "restart-add"
		createEffort(t, root, "restart-add", "Restart add")
		faulted := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
			stub.worktreeAdd = func(context.Context, string, string, string) error { return failing("worktree add") }
		}))
		if _, err := faulted.Add(testContext(t), slug, "HEAD"); err == nil {
			t.Fatal("add fault hidden")
		}
		if _, err := os.Stat(filepath.Join(root, ".awf", "worktrees", slug)); !os.IsNotExist(err) {
			t.Fatalf("failed add left a path: %v", err)
		}
		if _, err := freshWorktreeManager(t, root).Add(testContext(t), slug, "HEAD"); err != nil {
			t.Fatalf("restart could not add after a failed attempt: %v", err)
		}
	})
}

// TestPreMutationRefusalInvokesNoDestructiveCommand pins the negative half of
// the refusal contract: a refusal that reports no topology change must not have
// run a destructive command at all.
func TestPreMutationRefusalInvokesNoDestructiveCommand(t *testing.T) {
	root := initWorktreeRepo(t, "sha1")
	slug := "refusal-invokes-nothing"
	createEffort(t, root, "refusal-invokes-nothing", "Refusal invokes nothing")
	if _, err := freshWorktreeManager(t, root).Add(testContext(t), slug, "HEAD"); err != nil {
		t.Fatal(err)
	}
	managed := filepath.Join(root, ".awf", "worktrees", slug)
	writeWorktreeFile(t, filepath.Join(managed, "effort.txt"), "unmerged\n")
	commitWorktree(t, managed, "unmerged")

	var invoked []string
	m := managerWith(t, root, invokingStub(root, func(stub *checkoutStub) {
		stub.worktreeRemove = func(ctx context.Context, path string) error {
			invoked = append(invoked, "worktree remove")
			return stub.Runner.WorktreeRemove(ctx, path)
		}
		stub.worktreePrune = func(ctx context.Context) error {
			invoked = append(invoked, "worktree prune")
			return stub.Runner.WorktreePrune(ctx)
		}
		stub.branchDelete = func(ctx context.Context, name string) error {
			invoked = append(invoked, "branch delete")
			return stub.Runner.BranchDelete(ctx, name)
		}
		stub.ancestor = func(ctx context.Context, older, newer string) (bool, error) {
			invoked = append(invoked, "ancestor")
			return stub.Runner.Ancestor(ctx, older, newer)
		}
	}))
	_, err := m.Remove(testContext(t), slug)
	if err == nil || !strings.Contains(err.Error(), "not merged") {
		t.Fatalf("unmerged removal error = %v", err)
	}
	if !strings.Contains(err.Error(), "changed topology: no") {
		t.Fatalf("refusal must report an unchanged topology: %v", err)
	}
	// Without this the negative loop below would pass on an empty log.
	if len(invoked) == 0 {
		t.Fatal("no git operation was recorded, so the negative assertion proves nothing")
	}
	for _, operation := range invoked {
		for _, destructive := range []string{"worktree remove", "worktree prune", "branch delete"} {
			if operation == destructive {
				t.Errorf("refusal invoked %q", operation)
			}
		}
	}
	if _, statErr := os.Stat(managed); statErr != nil {
		t.Fatalf("refusal discarded the managed worktree: %v", statErr)
	}
	if !worktreeBranchExists(t, root, slug) {
		t.Fatal("refusal deleted the branch")
	}
}

func freshWorktreeManager(t *testing.T, root string) *Manager {
	t.Helper()
	return managerWith(t, root, openCheckout)
}

func worktreeBranchExists(t *testing.T, root, slug string) bool {
	t.Helper()
	return gitfixture.NativeRevisionExists(t, gitfixture.At(root), "refs/heads/awf/"+slug)
}

func newManagerWithEffort(t *testing.T, slug, title string) (*Manager, string) {
	t.Helper()
	root := initWorktreeRepo(t, "sha1")
	createEffort(t, root, slug, title)
	return freshWorktreeManager(t, root), root
}

func createEffort(t *testing.T, root, slug, title string) {
	t.Helper()
	roots := worktreeControlRoots(t, root)
	service := newEffortService(t, roots, func() (string, error) { return worktreeTestID, nil }, nil)
	if _, err := service.New(testContext(t), effort.NewInput{Slug: slug, Title: title}); err != nil {
		t.Fatal(err)
	}
}

// initWorktreeRepo builds the checkout an adopted project has: a base commit
// carrying the resident .gitignore files awf renders as TRACKED files, which is
// the entire reason owned effort and worktree state stays invisible to the
// cleanliness oracle exactly as it does in a real project.
func initWorktreeRepo(t *testing.T, format string) string {
	t.Helper()
	repo := gitfixture.InitNativeObjectFormat(t, t.TempDir(), format)
	root := repo.Root()
	writeWorktreeFile(t, filepath.Join(root, "tracked.txt"), "base\n")
	for _, resident := range []string{"efforts", "worktrees"} {
		ignore := filepath.Join(root, ".awf", resident, ".gitignore")
		writeWorktreeFile(t, ignore, "*\n!.gitignore\n")
		gitfixture.NativeAdd(t, repo, filepath.ToSlash(filepath.Join(".awf", resident, ".gitignore")))
	}
	commitWorktree(t, root, "base")
	return root
}

func writeWorktreeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// commitWorktree commits everything outside .awf, so the managed-worktree roots
// under .awf/worktrees never enter the parent checkout's index.
func commitWorktree(t *testing.T, root, message string) {
	t.Helper()
	repo := gitfixture.At(root)
	gitfixture.NativeAddAllExcept(t, repo, ".awf")
	gitfixture.NativeCommit(t, repo, message)
}
