package effort

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestActivityCrossProcessTakeoverRefusesStaleMutations(t *testing.T) {
	if os.Getenv("AWF_ACTIVITY_HELPER") != "" {
		root, owner := os.Getenv("AWF_ACTIVITY_ROOT"), os.Getenv("AWF_ACTIVITY_OWNER")
		service := openTestService(t, root, func(d *Dependencies) { noTopology(d) })
		if got := service.AttachActivity(testContext(t), "cross-process", activityFor(root, owner)); got.Condition != ActivityTakenOver {
			t.Fatalf("helper takeover = %#v", got)
		}
		return
	}
	for _, test := range []struct {
		name   string
		mutate func(*Service, string) ActivityReply
	}{
		{"takeover", func(s *Service, owner string) ActivityReply {
			return s.AttachActivity(testContext(t), "cross-process", activityFor(s.paths.roots.InvokingRoot, owner))
		}},
		{"heartbeat", func(s *Service, owner string) ActivityReply { return s.HeartbeatActivity("cross-process", owner) }},

		{"checkout", func(s *Service, owner string) ActivityReply {
			return s.CheckoutActivity(testContext(t), "cross-process", owner, s.paths.roots.InvokingRoot, CheckoutReceiving)
		}},
		{"detach", func(s *Service, owner string) ActivityReply { return s.DetachActivity("cross-process", owner) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := initEffortRepo(t)
			service := openTestService(t, root, func(d *Dependencies) { noTopology(d) })
			if _, err := service.New(testContext(t), "Cross process"); err != nil {
				t.Fatal(err)
			}
			if got := service.AttachActivity(testContext(t), "cross-process", activityFor(root, testIDA)); got.Condition != ActivityAttached {
				t.Fatalf("initial attach = %#v", got)
			}
			activityBeforePublish = func() {
				cmd := exec.Command(os.Args[0], "-test.run=^TestActivityCrossProcessTakeoverRefusesStaleMutations$")
				cmd.Env = append(os.Environ(), "AWF_ACTIVITY_HELPER=1", "AWF_ACTIVITY_ROOT="+root, "AWF_ACTIVITY_OWNER="+testIDB)
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("helper takeover: %v: %s", err, output)
				}
			}
			t.Cleanup(func() { activityBeforePublish = nil })
			got := test.mutate(service, testIDA)
			if got.Condition != ActivityNotOwner || got.Activity == nil || got.Activity.Owner != testIDB {
				t.Fatalf("stale %s = %#v", test.name, got)
			}
			current, err := service.activityCurrent("cross-process")
			if err != nil || current == nil || current.Owner != testIDB {
				t.Fatalf("successor changed or deleted: %#v, %v", current, err)
			}
		})
	}
}

func TestActivityCrossProcessFirstAttachRefusesWinner(t *testing.T) {
	if os.Getenv("AWF_ACTIVITY_FIRST_ATTACH_HELPER") != "" {
		root := os.Getenv("AWF_ACTIVITY_ROOT")
		service := openTestService(t, root, func(d *Dependencies) { noTopology(d) })
		if got := service.AttachActivity(testContext(t), "first-attach", activityFor(root, testIDB)); got.Condition != ActivityAttached {
			t.Fatalf("helper first attach = %#v", got)
		}
		return
	}
	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) { noTopology(d) })
	if _, err := service.New(testContext(t), "First attach"); err != nil {
		t.Fatal(err)
	}
	activityBeforePublish = func() {
		cmd := exec.Command(os.Args[0], "-test.run=^TestActivityCrossProcessFirstAttachRefusesWinner$")
		cmd.Env = append(os.Environ(), "AWF_ACTIVITY_FIRST_ATTACH_HELPER=1", "AWF_ACTIVITY_ROOT="+root)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("helper first attach: %v: %s", err, output)
		}
	}
	t.Cleanup(func() { activityBeforePublish = nil })
	got := service.AttachActivity(testContext(t), "first-attach", activityFor(root, testIDA))
	if got.Condition != ActivityNotOwner || got.Activity == nil || got.Activity.Owner != testIDB || got.Outcome == nil || got.Outcome.ChangedActivity {
		t.Fatalf("first attach refusal = %#v", got)
	}
}

func TestActivityStorageFailuresIdentifyOperationAndStage(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) { noTopology(d) })
	if _, err := service.New(testContext(t), "Storage context"); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		operation string
		stage     string
		call      func() ActivityReply
	}{
		{"attach", "attach", "activity.write", func() ActivityReply {
			return service.AttachActivity(testContext(t), "storage-context", activityFor(root, testIDA))
		}},
		{"takeover", "takeover", "activity.rename", func() ActivityReply {
			return service.AttachActivity(testContext(t), "storage-context", activityFor(root, testIDB))
		}},
		{"heartbeat", "heartbeat", "activity.fsync", func() ActivityReply { return service.HeartbeatActivity("storage-context", testIDA) }},
		{"checkout-after-replace", "checkout", "activity.directory-fsync", func() ActivityReply {
			return service.CheckoutActivity(testContext(t), "storage-context", testIDA, root, CheckoutReceiving)
		}},
		{"detach-before-remove", "detach", "activity.remove", func() ActivityReply { return service.DetachActivity("storage-context", testIDA) }},
		{"detach-after-remove", "detach", "activity.directory-fsync", func() ActivityReply { return service.DetachActivity("storage-context", testIDA) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.name != "attach" {
				service.store.fault = nil
				if got := service.AttachActivity(testContext(t), "storage-context", activityFor(root, testIDA)); got.Condition != ActivityAttached && got.Condition != ActivityTakenOver {
					t.Fatalf("seed = %#v", got)
				}
			}
			service.store.fault = func(stage string) error {
				if stage == test.stage {
					return errors.New("injected")
				}
				return nil
			}
			got := test.call()
			if got.Outcome == nil || !strings.Contains(got.Outcome.Cause, "activity "+test.operation) || got.Outcome.ChangedActivity != (test.stage == "activity.directory-fsync") {
				t.Fatalf("%s outcome = %#v", test.name, got)
			}
			if test.stage == "activity.directory-fsync" {
				current, err := service.activityCurrent("storage-context")
				if test.operation == "detach" && (err != nil || current != nil) {
					t.Fatalf("detach published before directory fsync: activity=%#v err=%v", current, err)
				}
				if test.operation != "detach" && (err != nil || current == nil || current.Owner != testIDA) {
					t.Fatalf("replace published before directory fsync: activity=%#v err=%v", current, err)
				}
			}
			service.store.fault = nil
		})
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
	publication := publicationIdentityRefusal(cause)
	wrapped := activityStorageFailure(activityHeartbeat, "replace", publication)
	var refusal *activityPublicationRefusal
	if !errors.As(wrapped, &refusal) || !errors.Is(wrapped, cause) || !errors.Is(refusal, cause) {
		t.Fatalf("identity refusal wrapping = %v", wrapped)
	}
	if !errors.Is((&publicationIdentityError{err: cause}), cause) {
		t.Fatal("publication identity unwrap lost cause")
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
		r := refusalFor(activityResolve, condition, errors.New("cause"), nil)
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
	if _, err := readRegularNoFollowBounded(service.paths.activityFile(slug), maxMemoryBytes); !errors.Is(err, os.ErrNotExist) {
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
	if got := checkoutRefusalFor(activityResolve, errors.New("mechanism")); got.Condition != ActivityRepositoryMismatch {
		t.Fatalf("mechanism = %#v", got)
	}
	if got := checkoutRefusalFor(activityResolve, NewCheckoutResolutionError(CheckoutUnsafe, errors.New("unsafe"))); got.Condition != ActivityUnsafeResident {
		t.Fatalf("unsafe = %#v", got)
	}
	service.checkoutResolver = nil
	if got := service.ResolveActivity(testContext(t), slug, CheckoutReceiving, ""); got.Condition != ActivityRepositoryMismatch {
		t.Fatalf("nil resolver = %#v", got)
	}
}

func TestActivityReviewFixReceivingResolutionNeverUsesDotAndValidatesRequestedCheckout(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	if _, err := service.New(testContext(t), "Receiving review fix"); err != nil {
		t.Fatal(err)
	}
	slug := "receiving-review-fix"
	managed := service.paths.managedWorktree(slug)
	var resolved []string
	service.checkoutResolver = func(_ context.Context, path string) (CheckoutFacts, error) {
		resolved = append(resolved, path)
		if path == "." {
			t.Fatal("empty receiving checkout was resolved as dot")
		}
		return CheckoutFacts{InvokingRoot: path, PrimaryRoot: root}, nil
	}
	managedFacts := CheckoutFacts{InvokingRoot: managed, PrimaryRoot: root}
	if _, err := service.destination(testContext(t), slug, CheckoutReceiving, "", nil, managedFacts); err == nil {
		t.Fatal("managed-first receiving resolution succeeded without a recorded or explicit checkout")
	}
	if len(resolved) != 0 {
		t.Fatalf("managed-first receiving resolved unexpected paths: %q", resolved)
	}
	prior := &Activity{ReceivingCheckout: filepath.Join(root, "recorded")}
	if got, err := service.destination(testContext(t), slug, CheckoutReceiving, filepath.Join(root, "explicit"), prior, managedFacts); err != nil || got.CWD != prior.ReceivingCheckout {
		t.Fatalf("recorded receiving checkout did not win: destination=%#v err=%v", got, err)
	}
	if got, err := service.destination(testContext(t), slug, CheckoutReceiving, filepath.Join(root, "explicit"), nil, managedFacts); err != nil || got.CWD != filepath.Join(root, "explicit") {
		t.Fatalf("explicit receiving checkout was not resolved: destination=%#v err=%v", got, err)
	}
	for _, receiving := range []string{".", "relative", root + "/child/..", managed} {
		if _, err := service.destination(testContext(t), slug, CheckoutReceiving, receiving, nil, managedFacts); err == nil {
			t.Fatalf("unsafe explicit receiving checkout accepted: %q", receiving)
		}
	}
	if _, err := service.destination(testContext(t), slug, CheckoutReceiving, "", &Activity{ReceivingCheckout: managed}, managedFacts); err == nil {
		t.Fatal("managed selected receiving checkout accepted")
	}

	other := filepath.Join(root, "same-repository-different-checkout")
	service.checkoutResolver = func(_ context.Context, path string) (CheckoutFacts, error) {
		if path == other {
			return CheckoutFacts{InvokingRoot: root, PrimaryRoot: root}, nil
		}
		return CheckoutFacts{InvokingRoot: path, PrimaryRoot: root}, nil
	}
	claim := activityFor(other, testIDA)
	claim.ReceivingCheckout = other
	if err := service.verifyActivityDestination(testContext(t), slug, claim); err == nil {
		t.Fatal("checkout validation accepted a same-repository different checkout")
	}
	claim = activityFor(root, testIDA)
	claim.ReceivingCheckout = other
	if err := service.verifyActivityDestination(testContext(t), slug, claim); err == nil {
		t.Fatal("receiving activity accepted a same-repository different checkout")
	}
	service.checkoutResolver = func(_ context.Context, path string) (CheckoutFacts, error) {
		return CheckoutFacts{InvokingRoot: path, PrimaryRoot: root}, nil
	}
	if err := service.verifyActivityDestination(testContext(t), slug, claim); err == nil {
		t.Fatal("receiving activity accepted different requested checkout paths")
	}
}

func TestActivityReviewFixRefusalsReportOperationAxesAndMemoryRemedies(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) { noTopology(d) })
	if _, err := service.New(testContext(t), "Outcome review fix"); err != nil {
		t.Fatal(err)
	}
	slug := "outcome-review-fix"
	if err := os.WriteFile(service.paths.memoryFile(slug), []byte("---\neffort: outcome-review-fix\nphase: \"\"\nnext: continue\nupdated: 2026-08-02T12:00:00Z\n---\nbody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolve := service.ResolveActivity(testContext(t), slug, CheckoutReceiving, root)
	attach := service.AttachActivity(testContext(t), slug, activityFor(root, testIDA))
	for name, reply := range map[string]ActivityReply{"resolve": resolve, "attach": attach} {
		if reply.Condition != ActivityInvalidMemory || reply.Outcome == nil || reply.Outcome.ChangedActivity || reply.Outcome.ChangedMemory || reply.Outcome.ChangedCWD != (name == "attach") || len(reply.Outcome.NextActions) != 1 || reply.Outcome.Cause != "" {
			t.Fatalf("%s invalid-memory outcome = %#v", name, reply)
		}
		if name == "attach" && reply.Outcome.NextActions[0] != "./awf effort memory update outcome-review-fix --phase <replacement-phase>" {
			t.Fatalf("attach repair action = %q", reply.Outcome.NextActions[0])
		}
	}
	if got := service.HeartbeatActivity(slug, testIDA); got.Outcome == nil || got.Outcome.ChangedActivity || got.Outcome.ChangedMemory || got.Outcome.ChangedCWD {
		t.Fatalf("heartbeat refusal axes = %#v", got)
	}
	for operation, changedCWD := range map[activityOperation]bool{
		activityResolve: false, activityAttach: true, activityHeartbeat: false, activityCheckout: true, activityDetach: false,
	} {
		if got := refusalFor(operation, ActivityMissing, nil, nil).Outcome; got.ChangedActivity || got.ChangedMemory || got.ChangedCWD != changedCWD {
			t.Fatalf("%s refusal axes = %#v", operation, got)
		}
	}
	for name, raw := range map[string]string{
		"canonical-missing": "---\neffort: outcome-review-fix\nphase: \"retained ' phase\"\nnext: retained next\n---\nbody\n",
		"legacy-invalid":    "Effort: outcome-review-fix\nPhase: retained ' phase\nNext: retained next\nUpdated: invalid\n\nbody\n",
	} {
		if err := os.WriteFile(service.paths.memoryFile(slug), []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		got := service.ResolveActivity(testContext(t), slug, CheckoutReceiving, root)
		const want = "./awf effort memory update outcome-review-fix --phase 'retained '\"'\"' phase'"
		if got.Condition != ActivityInvalidMemory || got.Outcome == nil || len(got.Outcome.NextActions) != 1 || got.Outcome.NextActions[0] != want {
			t.Fatalf("%s updated-only repair = %#v", name, got)
		}
	}
	if err := os.WriteFile(service.paths.memoryFile(slug), []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	manual := service.ResolveActivity(testContext(t), slug, CheckoutReceiving, root)
	if manual.Outcome == nil || manual.Outcome.Cause != "" || manual.Outcome.NextActions[0] != "repair .awf/efforts/outcome-review-fix/memory.md manually: preserve its body and restore a matching effort identity with a recognized canonical or legacy metadata boundary" {
		t.Fatalf("manual memory remedy = %#v", manual)
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
	if got, err := service.destination(testContext(t), slug, CheckoutManaged, root, nil, facts); err != nil || got.CWD != managed || got.ReceivingCheckout != root {
		t.Fatalf("managed = %#v %v", got, err)
	}
	prior := &Activity{ReceivingCheckout: root}
	if _, err := service.destination(testContext(t), slug, CheckoutReceiving, ".", prior, facts); err == nil {
		t.Fatal("relative explicit receiving checkout accepted over recorded checkout")
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
	if err := service.store.removeActivityExpected(path, nil); err == nil {
		t.Fatal("remove fault accepted")
	}
	service.store.fault = nil
	if err := service.store.removeActivityExpected(path, nil); err != nil {
		t.Fatal(err)
	}
	if err := service.store.removeActivityExpected(path, nil); err != nil {
		t.Fatal(err)
	}
	if err := service.store.replaceResidentExpected(filepath.Join(root, "missing", "activity.json"), []byte("x"), "activity", nil, activityAttach); err == nil {
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
	if err := service.store.replaceResidentExpected(bad, []byte("data"), "activity", nil, activityAttach); err == nil {
		t.Fatal("directory rename target accepted")
	}
	if err := os.WriteFile(filepath.Join(bad, "child"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.store.removeActivityExpected(bad, nil); err == nil {
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
	if got := service.mutateActivity(slug, testIDB, activityHeartbeat, ActivityHeartbeat, func(a *Activity) { a.Role = "bad" }); got.Condition != ActivityRepositoryMismatch {
		t.Fatalf("invalid mutation = %#v", got)
	}
	service.store.fault = func(stage string) error {
		if stage == "activity.write" {
			return errors.New("persist fault")
		}
		return nil
	}
	if got := service.mutateActivity(slug, testIDB, activityHeartbeat, ActivityHeartbeat, func(*Activity) {}); got.Condition != ActivityUnsafeResident {
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
	if err := service.store.removeActivityExpected(path, nil); err == nil {
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
		if _, err := service.activityCurrent(slug); err == nil {
			t.Fatalf("accepted malformed activity %q", raw)
		}
	}
	if err := os.WriteFile(path, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.activityCurrent(slug); err != nil {
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
