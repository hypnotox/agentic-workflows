package effort

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

func activityFor(root, owner string) Activity {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	return Activity{SchemaVersion: 1, Owner: owner, AttachedAt: now, HeartbeatAt: now, CWD: root, ReceivingCheckout: root, Role: CheckoutReceiving}
}

// invariant: tooling/effort-management:effort-record-authority (TestAdvisoryActivityDoesNotGateUnrelatedEffortCommands)
func TestAdvisoryActivityDoesNotGateUnrelatedEffortCommands(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) {
		noTopology(d)
		d.UUID = func() (string, error) { return testIDA, nil }
		d.Clock = func() time.Time { return time.Date(2026, 8, 2, 13, 0, 0, 0, time.UTC) }
	})
	if _, err := service.New(testContext(t), "Activity advisory"); err != nil {
		t.Fatal(err)
	}
	first := activityFor(root, testIDA)
	if got := service.AttachActivity(testContext(t), "activity-advisory", first); got.Condition != ActivityAttached || got.Activity == nil {
		t.Fatalf("attach = %#v", got)
	}
	second := activityFor(root, testIDB)
	if got := service.AttachActivity(testContext(t), "activity-advisory", second); got.Condition != ActivityTakenOver || got.PriorClaim == nil || got.PriorClaim.Owner != testIDA {
		t.Fatalf("takeover = %#v", got)
	}
	if got := service.HeartbeatActivity("activity-advisory", testIDA); got.Condition != ActivityNotOwner || got.Activity == nil || got.Activity.Owner != testIDB {
		t.Fatalf("old heartbeat = %#v", got)
	}
	if got := service.DetachActivity("activity-advisory", testIDA); got.Condition != ActivityNotOwner || got.Activity == nil || got.Activity.Owner != testIDB {
		t.Fatalf("old detach = %#v", got)
	}
	if got := service.DetachActivity("activity-advisory", testIDB); got.Condition != ActivityDetached {
		t.Fatalf("detach = %#v", got)
	}
	// A malformed optional resident is admitted, but never parsed by Show.
	if err := os.WriteFile(filepath.Join(root, ".awf", "efforts", "activity-advisory", "activity.json"), []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Show("activity-advisory"); err != nil {
		t.Fatalf("activity gated show: %v", err)
	}
	if _, err := service.List(); err != nil {
		t.Fatalf("activity gated list: %v", err)
	}
	if _, err := service.Finish(testContext(t), "activity-advisory"); err != nil {
		t.Fatalf("activity gated finish: %v", err)
	}
}

func TestActivityConcurrentTakeoverCannotBeOverwrittenOrDeletedByOldOwner(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) { d.UUID = func() (string, error) { return testIDA, nil } })
	if _, err := service.New(testContext(t), "Concurrent activity"); err != nil {
		t.Fatal(err)
	}
	if got := service.AttachActivity(testContext(t), "concurrent-activity", activityFor(root, testIDA)); got.Condition != ActivityAttached {
		t.Fatalf("initial attach = %#v", got)
	}
	var wait sync.WaitGroup
	wait.Add(3)
	go func() {
		defer wait.Done()
		service.AttachActivity(testContext(t), "concurrent-activity", activityFor(root, testIDB))
	}()
	go func() { defer wait.Done(); service.HeartbeatActivity("concurrent-activity", testIDA) }()
	go func() { defer wait.Done(); service.DetachActivity("concurrent-activity", testIDA) }()
	wait.Wait()
	activity, err := service.activityCurrent("concurrent-activity")
	if err != nil || activity == nil || activity.Owner != testIDB {
		t.Fatalf("successor was changed or deleted: activity=%#v err=%v", activity, err)
	}
}

func TestActivityProtocolValidationAndCheckoutResolutionError(t *testing.T) {
	cause := errors.New("retained cause")
	for _, kind := range []CheckoutResolutionKind{CheckoutUnsafe, CheckoutRepositoryMismatch} {
		err := NewCheckoutResolutionError(kind, cause)
		if err.Kind() != kind || err.Error() != "checkout resolution "+string(kind)+": retained cause" || !errors.Is(err.Unwrap(), cause) || !errors.Is(err, cause) {
			t.Fatalf("error contract = %#v", err)
		}
	}
	for _, test := range []struct {
		name string
		call func()
	}{
		{"kind", func() { _ = NewCheckoutResolutionError("other", cause) }},
		{"cause", func() { _ = NewCheckoutResolutionError(CheckoutUnsafe, nil) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("constructor did not panic")
				}
			}()
			test.call()
		})
	}
	for _, bad := range []Activity{
		{SchemaVersion: 2},
		{SchemaVersion: 1, Owner: testIDA, AttachedAt: time.Now(), HeartbeatAt: time.Now(), CWD: "relative", ReceivingCheckout: "/receiving", Role: CheckoutReceiving},
	} {
		if validActivity(bad) == nil {
			t.Fatalf("invalid activity accepted: %#v", bad)
		}
	}
}

func TestActivityFaultAndRefusalMatrix(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) {
		d.Clock = func() time.Time { return time.Date(2026, 8, 2, 13, 0, 0, 0, time.UTC) }
	})
	if _, err := service.New(testContext(t), "Activity matrix"); err != nil {
		t.Fatal(err)
	}
	slug := "activity-matrix"
	for _, condition := range []ActivityCondition{ActivityNotOwner, ActivityMissing, ActivityInvalidMemory, ActivityUnsafeResident, ActivityRepositoryMismatch} {
		r := refusal(condition, errors.New("cause"))
		if r.Outcome == nil || r.Outcome.Condition != string(condition) || r.Outcome.Cause != "cause" || r.Effort != nil || r.Memory != nil || r.Activity != nil {
			t.Fatalf("refusal %s = %#v", condition, r)
		}
	}
	if validActivity(activityFor(root, testIDA)) != nil {
		t.Fatal("valid activity rejected")
	}
	if validActivityTime(time.Time{}) || validActivityPath("relative") || validActivityPath("") {
		t.Fatal("activity validators accepted invalid values")
	}
	if got := service.HeartbeatActivity("absent", testIDA); got.Condition != ActivityMissing || got.Outcome == nil {
		t.Fatalf("missing = %#v", got)
	}
	path := service.paths.activityFile(slug)
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := service.HeartbeatActivity(slug, testIDA); got.Condition != ActivityUnsafeResident || got.Effort != nil {
		t.Fatalf("malformed resident = %#v", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	ready := service.ResolveActivity(testContext(t), slug, CheckoutReceiving, "")
	if ready.Condition != ActivityReady || ready.Effort == nil || ready.Memory == nil || ready.Destination == nil || ready.PriorClaim != nil {
		t.Fatalf("ready = %#v", ready)
	}
	attached := service.AttachActivity(testContext(t), slug, activityFor(root, testIDA))
	if attached.Condition != ActivityAttached || attached.Activity == nil || attached.PriorClaim != nil {
		t.Fatalf("attach = %#v", attached)
	}
	if got := service.HeartbeatActivity(slug, testIDA); got.Condition != ActivityHeartbeat || got.Activity == nil {
		t.Fatalf("heartbeat = %#v", got)
	}
	if got := service.HeartbeatActivity(slug, testIDB); got.Condition != ActivityNotOwner || got.Activity == nil || got.Memory != nil {
		t.Fatalf("not owner = %#v", got)
	}
	if got := service.CheckoutActivity(testContext(t), slug, testIDA, root, CheckoutReceiving); got.Condition != ActivityCheckoutUpdated || got.Activity == nil {
		t.Fatalf("checkout = %#v", got)
	}
	if got := service.CheckoutActivity(testContext(t), slug, testIDA, root, CheckoutManaged); got.Condition != ActivityRepositoryMismatch {
		t.Fatalf("invalid managed checkout = %#v", got)
	}
	if got := service.DetachActivity(slug, testIDA); got.Condition != ActivityDetached || got.Effort == nil || got.Memory != nil || got.Activity != nil {
		t.Fatalf("detach = %#v", got)
	}
	if got := service.DetachActivity(slug, testIDA); got.Condition != ActivityDetached {
		t.Fatalf("idempotent detach = %#v", got)
	}
	for _, stage := range []string{"activity.write", "activity.fsync", "activity.rename", "activity.directory-fsync"} {
		t.Run(stage, func(t *testing.T) {
			faulty := openTestService(t, root, func(d *Dependencies) {
				d.Fault = func(s string) error {
					if s == stage {
						return errors.New("fault")
					}
					return nil
				}
			})
			got := faulty.AttachActivity(testContext(t), slug, activityFor(root, testIDA))
			if got.Condition != ActivityUnsafeResident || got.Outcome == nil {
				t.Fatalf("%s = %#v", stage, got)
			}
		})
	}
}

func TestActivityResolutionDefensesAndResidentCodec(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	if _, err := service.New(testContext(t), "Resolution defenses"); err != nil {
		t.Fatal(err)
	}
	slug := "resolution-defenses"
	if _, err := service.store.readActivity(service.paths.activityFile(slug)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing read = %v", err)
	}
	for _, raw := range [][]byte{[]byte(`{"schemaVersion":1,"owner":"bad"}`), []byte(`{} {}`), []byte(`{"schemaVersion":1,"owner":"00000000-0000-4000-8000-000000000000","attachedAt":"2026-08-02T12:00:00Z","heartbeatAt":"2026-08-02T12:00:00Z","cwd":"/x","receivingCheckout":"/x","role":"other","extra":true}`)} {
		if err := os.WriteFile(service.paths.activityFile(slug), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := service.activityCurrent(slug); err == nil {
			t.Fatalf("accepted activity %s", raw)
		}
	}
	if err := os.Remove(service.paths.activityFile(slug)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.destination(testContext(t), slug, "bad", "", nil, CheckoutFacts{InvokingRoot: root, PrimaryRoot: root}); err == nil {
		t.Fatal("invalid role accepted")
	}
	if _, err := service.destination(testContext(t), slug, CheckoutReceiving, "", nil, CheckoutFacts{InvokingRoot: service.paths.managedWorktree(slug), PrimaryRoot: root}); err == nil {
		t.Fatal("managed first receiving guessed")
	}
	if got := checkoutRefusal(errors.New("mechanism")); got.Condition != ActivityRepositoryMismatch {
		t.Fatalf("mechanism = %#v", got)
	}
	if got := checkoutRefusal(NewCheckoutResolutionError(CheckoutUnsafe, errors.New("unsafe"))); got.Condition != ActivityUnsafeResident {
		t.Fatalf("unsafe = %#v", got)
	}
	service.checkoutResolver = nil
	if got := service.ResolveActivity(testContext(t), slug, CheckoutReceiving, ""); got.Condition != ActivityRepositoryMismatch {
		t.Fatalf("nil resolver = %#v", got)
	}
}

func TestActivityDestinationAndPersistenceFaultBoundaries(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	if _, err := service.New(testContext(t), "Destination branches"); err != nil {
		t.Fatal(err)
	}
	slug := "destination-branches"
	managed := service.paths.managedWorktree(slug)
	facts := CheckoutFacts{InvokingRoot: root, PrimaryRoot: root}
	service.worktrees = func(context.Context) ([]awfgit.WorktreeRegistration, error) { return nil, errors.New("list failed") }
	if _, err := service.destination(testContext(t), slug, CheckoutManaged, "", nil, facts); err == nil {
		t.Fatal("worktree error accepted")
	}
	service.worktrees = func(context.Context) ([]awfgit.WorktreeRegistration, error) {
		return []awfgit.WorktreeRegistration{{Path: managed, Branch: "refs/heads/awf/" + slug}}, nil
	}
	if got, err := service.destination(testContext(t), slug, CheckoutManaged, "/receiving", nil, facts); err != nil || got.CWD != managed || got.ReceivingCheckout != "/receiving" {
		t.Fatalf("managed = %#v %v", got, err)
	}
	prior := &Activity{ReceivingCheckout: root}
	if got, err := service.destination(testContext(t), slug, CheckoutReceiving, ".", prior, facts); err != nil || got.ReceivingCheckout != root {
		t.Fatalf("prior receiving = %#v %v", got, err)
	}
	service.checkoutResolver = func(context.Context, string) (CheckoutFacts, error) {
		return CheckoutFacts{InvokingRoot: root, PrimaryRoot: "/other"}, nil
	}
	if _, err := service.destination(testContext(t), slug, CheckoutReceiving, root, nil, facts); err == nil {
		t.Fatal("foreign receiver accepted")
	}
	service.checkoutResolver = func(context.Context, string) (CheckoutFacts, error) {
		return CheckoutFacts{InvokingRoot: "relative", PrimaryRoot: root}, nil
	}
	if _, err := service.resolveCheckout(testContext(t), root); err == nil {
		t.Fatal("noncanonical facts accepted")
	}

	path := service.paths.activityFile(slug)
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	service.store.fault = func(stage string) error {
		if stage == "activity.remove" {
			return errors.New("stop")
		}
		return nil
	}
	if err := service.store.removeActivity(path); err == nil {
		t.Fatal("remove fault accepted")
	}
	service.store.fault = nil
	if err := service.store.removeActivity(path); err != nil {
		t.Fatal(err)
	}
	if err := service.store.removeActivity(path); err != nil {
		t.Fatal(err)
	}
	if err := service.store.replaceResident(filepath.Join(root, "missing", "activity.json"), []byte("x"), "activity"); err == nil {
		t.Fatal("missing parent accepted")
	}
}

func TestActivityRefusalsAndDestinationVerification(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) { noTopology(d) })
	if _, err := service.New(testContext(t), "Activity defenses"); err != nil {
		t.Fatal(err)
	}
	slug := "activity-defenses"
	// A state-observing invalid memory returns the effort but never publishes
	// damaged metadata.
	if err := os.WriteFile(service.paths.memoryFile(slug), []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := service.ResolveActivity(testContext(t), slug, CheckoutReceiving, root); got.Condition != ActivityInvalidMemory || got.Effort == nil || got.Memory != nil {
		t.Fatalf("invalid memory = %#v", got)
	}
	if err := os.WriteFile(service.paths.memoryFile(slug), memorySkeleton(slug, time.Now()), 0o600); err != nil {
		t.Fatal(err)
	}
	// No registration may make a managed destination, even when its spelling is
	// the conventional path.
	managed := service.paths.managedWorktree(slug)
	managedActivity := activityFor(managed, testIDA)
	managedActivity.Role = CheckoutManaged
	managedActivity.ReceivingCheckout = root
	if got := service.AttachActivity(testContext(t), slug, managedActivity); got.Condition != ActivityRepositoryMismatch {
		t.Fatalf("unregistered managed attach = %#v", got)
	}
	// Receiving claims require cwd to be exactly the independently resolved
	// receiving checkout.
	badReceiving := activityFor(root, testIDA)
	badReceiving.ReceivingCheckout = filepath.Join(root, "other")
	if got := service.AttachActivity(testContext(t), slug, badReceiving); got.Condition != ActivityRepositoryMismatch {
		t.Fatalf("mismatched receiving attach = %#v", got)
	}
	if got := service.AttachActivity(testContext(t), slug, activityFor(root, testIDA)); got.Condition != ActivityAttached {
		t.Fatalf("receiving attach = %#v", got)
	}
	service.store.fault = func(stage string) error {
		if stage == "activity.remove" {
			return errors.New("remove failed")
		}
		return nil
	}
	if got := service.DetachActivity(slug, testIDA); got.Condition != ActivityUnsafeResident || got.Outcome == nil {
		t.Fatalf("remove failure = %#v", got)
	}
}

func TestActivityPersistenceFaultStagesAndOwnerBoundaries(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) { noTopology(d) })
	if _, err := service.New(testContext(t), "Activity persistence"); err != nil {
		t.Fatal(err)
	}
	slug := "activity-persistence"
	for _, stage := range []string{"activity.write", "activity.fsync", "activity.rename", "activity.directory-fsync"} {
		t.Run(stage, func(t *testing.T) {
			service.store.fault = func(got string) error {
				if got == stage {
					return errors.New("durability fault")
				}
				return nil
			}
			if got := service.AttachActivity(testContext(t), slug, activityFor(root, testIDA)); got.Condition != ActivityUnsafeResident {
				t.Fatalf("attach %s = %#v", stage, got)
			}
			_ = os.Remove(service.paths.activityFile(slug))
		})
	}
	service.store.fault = nil
	if got := service.HeartbeatActivity(slug, testIDA); got.Condition != ActivityMissing {
		t.Fatalf("missing heartbeat = %#v", got)
	}
	if got := service.CheckoutActivity(testContext(t), slug, testIDA, root, CheckoutReceiving); got.Condition != ActivityMissing {
		t.Fatalf("missing checkout = %#v", got)
	}
	// Rename refuses to replace a directory, preserving the durable resident.
	bad := filepath.Join(root, "rename-target")
	if err := os.Mkdir(bad, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := service.store.replaceResident(bad, []byte("data"), "activity"); err == nil {
		t.Fatal("directory rename target accepted")
	}
	if err := os.WriteFile(filepath.Join(bad, "child"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.store.removeActivity(bad); err == nil {
		t.Fatal("nonempty directory removed as activity")
	}
}

func TestActivityUncoveredPolicyBranches(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) { noTopology(d) })
	if _, err := service.New(testContext(t), "Uncovered policy"); err != nil {
		t.Fatal(err)
	}
	slug := "uncovered-policy"
	path := service.paths.activityFile(slug)

	// Exercise every refusal boundary with a real resident or resolver result.
	if err := os.WriteFile(service.paths.memoryFile(slug), []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := service.AttachActivity(testContext(t), slug, activityFor(root, testIDA)); got.Condition != ActivityInvalidMemory {
		t.Fatalf("bad memory = %#v", got)
	}
	if err := os.WriteFile(service.paths.memoryFile(slug), memorySkeleton(slug, time.Now()), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := service.AttachActivity(testContext(t), slug, Activity{Role: CheckoutReceiving}); got.Condition != ActivityRepositoryMismatch {
		t.Fatalf("bad claim = %#v", got)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := service.ResolveActivity(testContext(t), slug, CheckoutReceiving, root); got.Condition != ActivityUnsafeResident {
		t.Fatalf("bad prior = %#v", got)
	}
	if got := service.CheckoutActivity(testContext(t), slug, testIDA, root, CheckoutReceiving); got.Condition != ActivityUnsafeResident {
		t.Fatalf("bad checkout prior = %#v", got)
	}
	if got := service.DetachActivity(slug, testIDA); got.Condition != ActivityUnsafeResident {
		t.Fatalf("bad detach prior = %#v", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(service.paths.memoryFile(slug)); err != nil {
		t.Fatal(err)
	}
	if got := service.ResolveActivity(testContext(t), slug, CheckoutReceiving, root); got.Condition != ActivityInvalidMemory {
		t.Fatalf("missing memory = %#v", got)
	}
	if err := os.WriteFile(service.paths.memoryFile(slug), memorySkeleton(slug, time.Now()), 0o600); err != nil {
		t.Fatal(err)
	}
	// A structurally valid but unsafe effort resident has a state error, not a
	// missing-effort interpretation.
	if err := os.WriteFile(filepath.Join(service.paths.effort(slug), "extra"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := service.ResolveActivity(testContext(t), slug, CheckoutReceiving, root); got.Condition != ActivityUnsafeResident {
		t.Fatalf("unsafe effort = %#v", got)
	}
	if err := os.Remove(filepath.Join(service.paths.effort(slug), "extra")); err != nil {
		t.Fatal(err)
	}

	service.checkoutResolver = func(_ context.Context, p string) (CheckoutFacts, error) {
		if p == root {
			return CheckoutFacts{InvokingRoot: root, PrimaryRoot: "/other"}, nil
		}
		return CheckoutFacts{}, errors.New("resolver fault")
	}
	if got := service.ResolveActivity(testContext(t), slug, CheckoutReceiving, root); got.Condition != ActivityRepositoryMismatch {
		t.Fatalf("foreign invoking = %#v", got)
	}
	service.checkoutResolver = func(_ context.Context, p string) (CheckoutFacts, error) {
		if p == filepath.Join(root, "receiver") {
			return CheckoutFacts{}, errors.New("receiver fault")
		}
		return CheckoutFacts{InvokingRoot: root, PrimaryRoot: root}, nil
	}
	if _, err := service.destination(testContext(t), slug, CheckoutReceiving, filepath.Join(root, "receiver"), nil, CheckoutFacts{InvokingRoot: root, PrimaryRoot: root}); err == nil {
		t.Fatal("receiver fault accepted")
	}

	service.checkoutResolver = func(context.Context, string) (CheckoutFacts, error) {
		return CheckoutFacts{InvokingRoot: root, PrimaryRoot: root}, nil
	}
	if got := service.ResolveActivity(testContext(t), slug, "bad", root); got.Condition != ActivityRepositoryMismatch {
		t.Fatalf("bad resolution destination = %#v", got)
	}
	if _, err := service.destination(testContext(t), slug, CheckoutManaged, "", nil, CheckoutFacts{InvokingRoot: root, PrimaryRoot: root}); err == nil {
		t.Fatal("unregistered managed destination accepted")
	}
	if got := chooseReceiving(&Activity{ReceivingCheckout: root}, ""); got != root {
		t.Fatalf("prior receiver = %q", got)
	}
	if got := chooseReceiving(nil, ""); got != "" {
		t.Fatalf("empty receiver = %q", got)
	}
	managed := activityFor(service.paths.managedWorktree(slug), testIDA)
	managed.Role, managed.ReceivingCheckout = CheckoutManaged, root
	if err := service.verifyActivityDestination(testContext(t), slug, Activity{Role: "bad"}); err == nil {
		t.Fatal("bad role accepted")
	}
	service.checkoutResolver = func(context.Context, string) (CheckoutFacts, error) {
		return CheckoutFacts{}, errors.New("invoking resolver fault")
	}
	if err := service.verifyActivityDestination(testContext(t), slug, activityFor(root, testIDA)); err == nil {
		t.Fatal("invoking resolver fault accepted")
	}
	foreign := activityFor(root, testIDA)
	service.checkoutResolver = func(_ context.Context, p string) (CheckoutFacts, error) {
		return CheckoutFacts{InvokingRoot: p, PrimaryRoot: "/other"}, nil
	}
	if err := service.verifyActivityDestination(testContext(t), slug, foreign); err == nil {
		t.Fatal("foreign destination accepted")
	}
	service.checkoutResolver = func(_ context.Context, p string) (CheckoutFacts, error) {
		return CheckoutFacts{InvokingRoot: p, PrimaryRoot: root}, nil
	}
	foreign.ReceivingCheckout = filepath.Join(root, "different")
	if err := service.verifyActivityDestination(testContext(t), slug, foreign); err == nil {
		t.Fatal("mismatched receiver accepted")
	}
	service.worktrees = func(context.Context) ([]awfgit.WorktreeRegistration, error) { return nil, errors.New("worktree fault") }
	if err := service.verifyActivityDestination(testContext(t), slug, managed); err == nil {
		t.Fatal("worktree fault accepted")
	}
	service.worktrees = func(context.Context) ([]awfgit.WorktreeRegistration, error) {
		return []awfgit.WorktreeRegistration{{Path: managed.CWD, Branch: "refs/heads/awf/" + slug}}, nil
	}
	if err := service.verifyActivityDestination(testContext(t), slug, managed); err != nil {
		t.Fatalf("registered managed = %v", err)
	}
	service.worktrees = func(context.Context) ([]awfgit.WorktreeRegistration, error) { return nil, nil }
	if err := service.verifyActivityDestination(testContext(t), slug, managed); err == nil {
		t.Fatal("unregistered managed accepted")
	}

	service.checkoutResolver = func(context.Context, string) (CheckoutFacts, error) {
		return CheckoutFacts{InvokingRoot: root, PrimaryRoot: root}, nil
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := service.AttachActivity(testContext(t), slug, activityFor(root, testIDA)); got.Condition != ActivityUnsafeResident {
		t.Fatalf("bad attach prior = %#v", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := service.AttachActivity(testContext(t), slug, activityFor(root, testIDA)); got.Condition != ActivityAttached {
		t.Fatalf("attach = %#v", got)
	}
	if got := service.AttachActivity(testContext(t), slug, activityFor(root, testIDB)); got.Condition != ActivityTakenOver || got.PriorClaim == nil {
		t.Fatalf("takeover = %#v", got)
	}
	if got := service.mutateActivity(slug, testIDB, ActivityHeartbeat, func(a *Activity) { a.Role = "bad" }); got.Condition != ActivityRepositoryMismatch {
		t.Fatalf("invalid mutation = %#v", got)
	}
	service.store.fault = func(stage string) error {
		if stage == "activity.write" {
			return errors.New("persist fault")
		}
		return nil
	}
	if got := service.mutateActivity(slug, testIDB, ActivityHeartbeat, func(*Activity) {}); got.Condition != ActivityUnsafeResident {
		t.Fatalf("persist fault = %#v", got)
	}
	service.store.fault = func(stage string) error {
		if stage == "activity.directory-fsync" {
			return errors.New("sync fault")
		}
		return nil
	}
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.store.removeActivity(path); err == nil {
		t.Fatal("directory fsync fault accepted")
	}
	missing := openTestService(t, root, nil)
	if got := missing.DetachActivity("absent", testIDA); got.Condition != ActivityMissing {
		t.Fatalf("missing detach = %#v", got)
	}
	if err := os.WriteFile(filepath.Join(service.paths.effort(slug), "extra"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := service.DetachActivity(slug, testIDB); got.Condition != ActivityUnsafeResident {
		t.Fatalf("unsafe detach resident = %#v", got)
	}
}

func TestActivityCodecAndResolutionEdgeCases(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	if _, err := service.New(testContext(t), "Codec edges"); err != nil {
		t.Fatal(err)
	}
	slug, path := "codec-edges", service.paths.activityFile("codec-edges")
	valid := `{"schemaVersion":1,"owner":"018f47a0-7b3d-4c52-8f1a-123456789abc","attachedAt":"2026-08-02T12:00:00Z","heartbeatAt":"2026-08-02T12:00:00Z","cwd":"` + root + `","receivingCheckout":"` + root + `","role":"receiving"}`
	for _, raw := range []string{valid + " garbage", `{"schemaVersion":1,"owner":"018f47a0-7b3d-4c52-8f1a-123456789abc","attachedAt":"2026-08-02T12:00:00+00:00","heartbeatAt":"2026-08-02T12:00:00Z","cwd":"` + root + `","receivingCheckout":"` + root + `","role":"receiving"}`} {
		if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := service.store.readActivity(path); err == nil {
			t.Fatalf("accepted malformed activity %q", raw)
		}
	}
	if err := os.WriteFile(path, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.store.readActivity(path); err != nil {
		t.Fatal(err)
	}
	if got := service.CheckoutActivity(testContext(t), slug, testIDA, root, CheckoutReceiving); got.Condition != ActivityCheckoutUpdated {
		t.Fatalf("checkout = %#v", got)
	}
	service.checkoutResolver = func(context.Context, string) (CheckoutFacts, error) {
		return CheckoutFacts{}, NewCheckoutResolutionError(CheckoutUnsafe, errors.New("unsafe"))
	}
	if got := service.ResolveActivity(testContext(t), slug, CheckoutReceiving, root); got.Condition != ActivityUnsafeResident || got.Outcome.Cause != "unsafe" {
		t.Fatalf("unsafe resolve = %#v", got)
	}
}
