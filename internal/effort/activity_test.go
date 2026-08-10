package effort

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// invariant: tooling/effort-management:effort-record-authority (TestAdvisoryActivityDoesNotGateUnrelatedEffortCommands)
func TestAdvisoryActivityDoesNotGateUnrelatedEffortCommands(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) {
		noTopology(d)
		d.UUID = func() (string, error) { return testIDA, nil }
		d.Clock = func() time.Time { return time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC) }
	})
	if _, err := service.New(testContext(t), NewInput{Slug: "activity-advisory", Title: "Activity advisory"}); err != nil {
		t.Fatal(err)
	}
	if got := service.AttachActivity("activity-advisory", testIDA); got.Condition != ActivityAttached || got.Activity == nil || got.Activity.SchemaVersion != 2 {
		t.Fatalf("attach = %#v", got)
	}
	if got := service.AttachActivity("activity-advisory", testIDB); got.Condition != ActivityTakenOver || got.Activity.Owner != testIDB {
		t.Fatalf("takeover = %#v", got)
	}
	if got := service.HeartbeatActivity("activity-advisory", testIDA); got.Condition != ActivityNotOwner || got.Outcome == nil {
		t.Fatalf("old heartbeat = %#v", got)
	}
	if got := service.DetachActivity("activity-advisory", testIDA); got.Condition != ActivityNotOwner {
		t.Fatalf("old detach = %#v", got)
	}
	if got := service.DetachActivity("activity-advisory", testIDB); got.Condition != ActivityDetached || got.Effort != nil || got.Memory != nil || got.Activity != nil || got.Outcome != nil {
		t.Fatalf("detach = %#v", got)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "efforts", "activity-advisory", "activity.json"), []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Show("activity-advisory"); err != nil {
		t.Fatalf("activity gated show: %v", err)
	}
	if _, err := service.List(); err != nil {
		t.Fatalf("activity gated list: %v", err)
	}
}

func TestActivityV2StorageAndResidentBoundaries(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) { noTopology(d) })
	if _, err := service.New(testContext(t), NewInput{Slug: "storage", Title: "Storage"}); err != nil {
		t.Fatal(err)
	}
	slug, path := "storage", service.paths.activityFile("storage")
	cause := errors.New("injected")
	storage := activityStorageFailure(activityAttach, "replace", cause)
	var storageError *activityStorageError
	if !errors.As(storage, &storageError) || !errors.Is(storage, cause) || !strings.Contains(storage.Error(), "activity attach replace") {
		t.Fatalf("storage error = %v", storage)
	}
	identityFailure := activityStorageFailure(activityAttach, "replace", publicationIdentityRefusal(cause))
	var publication *activityPublicationRefusal
	if !errors.As(identityFailure, &publication) || !errors.Is(identityFailure, cause) || !strings.Contains(publication.Error(), "conditional-publication") {
		t.Fatalf("publication error = %v", identityFailure)
	}
	valid := Activity{SchemaVersion: 2, Owner: testIDA, AttachedAt: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), HeartbeatAt: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)}
	if err := service.store.replaceActivity(path, valid, nil, activityAttach); err != nil {
		t.Fatal(err)
	}
	_, identity, err := readRegularNoFollowBoundedIdentity(path, maxMemoryBytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.store.replaceActivity(path, valid, &identity, activityHeartbeat); err != nil {
		t.Fatal(err)
	}
	if err := service.store.removeActivityExpected(path, &identity); err == nil {
		t.Fatal("stale identity removed successor")
	}
	_, identity, err = readRegularNoFollowBoundedIdentity(path, maxMemoryBytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.store.removeActivityExpected(path, &identity); err != nil {
		t.Fatal(err)
	}
	if err := service.store.removeActivityExpected(path, nil); err != nil {
		t.Fatal(err)
	}
	if err := service.store.replaceResidentExpected(filepath.Join(root, "missing", "activity.json"), []byte("x"), "activity", nil, activityAttach); err == nil {
		t.Fatal("missing parent accepted")
	}
	for _, stage := range []string{"activity.write", "activity.fsync", "activity.rename", "activity.directory-fsync"} {
		t.Run(stage, func(t *testing.T) {
			service.store.fault = func(got string) error {
				if got == stage {
					return cause
				}
				return nil
			}
			got := service.AttachActivity(slug, testIDA)
			if got.Condition != ActivityUnsafeResident || got.Outcome == nil || got.Outcome.Cause == "" || got.Outcome.ChangedActivity != (stage == "activity.directory-fsync") {
				t.Fatalf("%s = %#v", stage, got)
			}
			service.store.fault = nil
			_ = os.Remove(path)
		})
	}
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	service.store.fault = func(stage string) error {
		if stage == "activity.remove" {
			return cause
		}
		return nil
	}
	if err := service.store.removeActivityExpected(path, nil); err == nil {
		t.Fatal("remove fault accepted")
	}
	service.store.fault = nil
}

// invariant: tooling/file-publication:exclusive-file-publication-single-home (TestActivityExpectedReplacementFaultMatrix)
func TestActivityExpectedReplacementFaultMatrix(t *testing.T) {
	cause := errors.New("injected replacement fault")
	for _, stage := range []string{"activity.write", "activity.fsync", "activity.rename", "activity.directory-fsync"} {
		t.Run(stage, func(t *testing.T) {
			root := initEffortRepo(t)
			service := openTestService(t, root, func(d *Dependencies) { noTopology(d) })
			if _, err := service.New(testContext(t), NewInput{Slug: "replacement", Title: "Replacement"}); err != nil {
				t.Fatal(err)
			}
			path := service.paths.activityFile("replacement")
			activity := Activity{SchemaVersion: 2, Owner: testIDA, AttachedAt: time.Now().UTC(), HeartbeatAt: time.Now().UTC()}
			if err := service.store.replaceActivity(path, activity, nil, activityAttach); err != nil {
				t.Fatal(err)
			}
			_, identity, err := readRegularNoFollowBoundedIdentity(path, maxMemoryBytes)
			if err != nil {
				t.Fatal(err)
			}
			service.store.fault = func(got string) error {
				if got == stage {
					return cause
				}
				return nil
			}
			err = service.store.replaceActivity(path, activity, &identity, activityHeartbeat)
			if !errors.Is(err, cause) {
				t.Fatalf("%s error = %v", stage, err)
			}
			var storage *activityStorageError
			if !errors.As(err, &storage) {
				t.Fatalf("%s did not preserve storage error identity: %v", stage, err)
			}
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".activity-") {
					t.Fatalf("%s left temporary %s", stage, entry.Name())
				}
			}
		})
	}

	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) { noTopology(d) })
	identity := fileIdentity{}
	err := service.store.replaceResidentExpected(filepath.Join(root, "missing", "activity.json"), []byte("x"), "activity", &identity, activityHeartbeat)
	var storage *activityStorageError
	if !errors.As(err, &storage) {
		t.Fatalf("missing replacement parent error = %v", err)
	}
}

// invariant: tooling/effort-management:effort-record-authority (TestActivityV2SafeRecoveryAndRefusals)
func TestActivityV2SafeRecoveryAndRefusals(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) { noTopology(d) })
	if _, err := service.New(testContext(t), NewInput{Slug: "recovery", Title: "Recovery"}); err != nil {
		t.Fatal(err)
	}
	slug, path := "recovery", service.paths.activityFile("recovery")
	for _, raw := range [][]byte{[]byte("not-json"), []byte(`{"schemaVersion":1}`)} {
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if got := service.AttachActivity(slug, testIDA); got.Condition != ActivityTakenOver || got.Activity.Owner != testIDA {
			t.Fatalf("takeover = %#v", got)
		}
	}
	for _, raw := range [][]byte{[]byte(`{"schemaVersion":2,"owner":"bad"}`), []byte(`{} {}`)} {
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := service.activityCurrentIdentity(slug); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := service.HeartbeatActivity(slug, testIDA); got.Condition != ActivityMissing || got.Outcome == nil {
		t.Fatalf("missing = %#v", got)
	}
	if got := service.DetachActivity(slug, testIDA); got.Condition != ActivityDetached {
		t.Fatalf("idempotent detach = %#v", got)
	}
	if err := os.WriteFile(service.paths.memoryFile(slug), []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := service.AttachActivity(slug, testIDA); got.Condition != ActivityInvalidMemory || got.Outcome == nil || got.Outcome.Cause != "" {
		t.Fatalf("invalid memory = %#v", got)
	}
	if err := os.Remove(service.paths.memoryFile(slug)); err != nil {
		t.Fatal(err)
	}
	if got := service.AttachActivity(slug, testIDA); got.Condition != ActivityInvalidMemory || got.Outcome == nil || got.Outcome.Cause == "" {
		t.Fatalf("missing memory = %#v", got)
	}
	if err := os.WriteFile(service.paths.memoryFile(slug), memorySkeleton(slug, time.Now()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("elsewhere", path); err != nil {
		t.Fatal(err)
	}
	if got := service.AttachActivity(slug, testIDA); got.Condition != ActivityUnsafeResident {
		t.Fatalf("symlink = %#v", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := service.AttachActivity(slug, testIDA); got.Condition != ActivityUnsafeResident {
		t.Fatalf("directory = %#v", got)
	}
}

// invariant: tooling/effort-management:effort-record-authority (TestActivityV2MutationAndPublicationBoundaries)
func TestActivityV2MutationAndPublicationBoundaries(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) { noTopology(d) })
	if _, err := service.New(testContext(t), NewInput{Slug: "mutation", Title: "Mutation"}); err != nil {
		t.Fatal(err)
	}
	slug, path := "mutation", service.paths.activityFile("mutation")
	if a, identity, err := service.activityCurrentIdentity(slug); err != nil || a != nil || identity != nil {
		t.Fatalf("missing current = %#v %#v %v", a, identity, err)
	}
	if err := os.Symlink("target", path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.activityCurrentIdentity(slug); err == nil {
		t.Fatal("symlink decoded")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := service.AttachActivity(slug, testIDA); got.Condition != ActivityAttached {
		t.Fatalf("attach = %#v", got)
	}
	if got := service.HeartbeatActivity(slug, testIDB); got.Condition != ActivityNotOwner {
		t.Fatalf("owner = %#v", got)
	}
	if got := service.HeartbeatActivity(slug, testIDA); got.Condition != ActivityHeartbeat {
		t.Fatalf("heartbeat = %#v", got)
	}
	if got := service.DetachActivity(slug, testIDB); got.Condition != ActivityNotOwner {
		t.Fatalf("detach owner = %#v", got)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := service.DetachActivity(slug, testIDA); got.Condition != ActivityUnsafeResident {
		t.Fatalf("unsafe detach = %#v", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := service.DetachActivity("absent", testIDA); got.Condition != ActivityMissing {
		t.Fatalf("missing effort detach = %#v", got)
	}
	if got := service.publicationRefusal(activityAttach, errors.New("plain")); got.Outcome == nil || got.Outcome.Cause != "" {
		t.Fatalf("plain refusal = %#v", got)
	}
	activityBeforePublish = func() { _ = os.WriteFile(path, []byte("race"), 0o600) }
	t.Cleanup(func() { activityBeforePublish = nil })
	if got := service.AttachActivity(slug, testIDB); got.Condition != ActivityUnsafeResident || got.Outcome == nil {
		t.Fatalf("race refusal = %#v", got)
	}
}

// invariant: tooling/effort-management:effort-record-authority (TestActivityV2AdditionalSafetyBranches)
func TestActivityV2AdditionalSafetyBranches(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) { noTopology(d) })
	if _, err := service.New(testContext(t), NewInput{Slug: "additional", Title: "Additional"}); err != nil {
		t.Fatal(err)
	}
	slug, path := "additional", service.paths.activityFile("additional")
	if err := os.WriteFile(path, make([]byte, maxMemoryBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := service.AttachActivity(slug, testIDA); got.Condition != ActivityUnsafeResident {
		t.Fatalf("oversized = %#v", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(service.paths.effort(slug), "unexpected"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := service.AttachActivity(slug, testIDA); got.Condition != ActivityUnsafeResident || got.Outcome == nil || got.Outcome.Condition != "the effort resident cannot be safely used" {
		t.Fatalf("unsafe effort = %#v", got)
	}
	if err := os.Remove(filepath.Join(service.paths.effort(slug), "unexpected")); err != nil {
		t.Fatal(err)
	}
	if got := service.AttachActivity(slug, testIDA); got.Condition != ActivityAttached {
		t.Fatalf("attach = %#v", got)
	}
	service.store.fault = func(stage string) error {
		if stage == "activity.directory-fsync" {
			return errors.New("sync")
		}
		return nil
	}
	if got := service.HeartbeatActivity(slug, testIDA); got.Condition != ActivityUnsafeResident || got.Outcome == nil || !got.Outcome.ChangedActivity {
		t.Fatalf("heartbeat fault = %#v", got)
	}
	service.store.fault = nil
	if got := service.AttachActivity(slug, testIDB); got.Condition != ActivityTakenOver {
		t.Fatalf("takeover = %#v", got)
	}
	service.store.fault = func(stage string) error {
		if stage == "activity.directory-fsync" {
			return errors.New("sync")
		}
		return nil
	}
	if got := service.DetachActivity(slug, testIDB); got.Condition != ActivityUnsafeResident || got.Outcome == nil || !got.Outcome.ChangedActivity {
		t.Fatalf("detach fault = %#v", got)
	}
	service.store.fault = nil
	if got := service.AttachActivity(slug, testIDB); got.Condition != ActivityAttached {
		t.Fatalf("re-attach = %#v", got)
	}
	activityBeforePublish = func() {}
	if got := service.DetachActivity(slug, testIDB); got.Condition != ActivityDetached {
		t.Fatalf("detach = %#v", got)
	}
	activityBeforePublish = nil
	if err := os.WriteFile(filepath.Join(service.paths.effort(slug), "unexpected"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := service.DetachActivity(slug, testIDB); got.Condition != ActivityUnsafeResident {
		t.Fatalf("unsafe effort detach = %#v", got)
	}
}

func TestActivityV2LowLevelFailureAndMutationRefusals(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) { noTopology(d) })
	if _, err := service.New(testContext(t), NewInput{Slug: "low-level", Title: "Low level"}); err != nil {
		t.Fatal(err)
	}
	path := service.paths.activityFile("low-level")
	if err := os.Mkdir(filepath.Join(root, "nonempty"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nonempty", "child"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.store.removeActivityExpected(filepath.Join(root, "nonempty"), nil); err == nil {
		t.Fatal("nonempty directory removed")
	}
	if err := service.store.replaceActivity(filepath.Join(root, "missing", "activity.json"), Activity{SchemaVersion: 2, Owner: testIDA, AttachedAt: time.Now().UTC(), HeartbeatAt: time.Now().UTC()}, nil, activityAttach); err == nil {
		t.Fatal("missing activity parent accepted")
	}
	activity := Activity{SchemaVersion: 2, Owner: testIDA, AttachedAt: time.Now().UTC(), HeartbeatAt: time.Now().UTC()}
	if err := service.store.replaceActivity(path, activity, nil, activityAttach); err != nil {
		t.Fatal(err)
	}
	_, identity, err := readRegularNoFollowBoundedIdentity(path, maxMemoryBytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := service.store.replaceActivity(path, activity, &identity, activityHeartbeat); err == nil {
		t.Fatal("stale replacement identity accepted")
	}
	if got := service.HeartbeatActivity("low-level", testIDA); got.Condition != ActivityMissing || got.Outcome == nil {
		t.Fatalf("missing activity = %#v", got)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := service.HeartbeatActivity("low-level", testIDA); got.Condition != ActivityUnsafeResident || got.Outcome == nil || got.Outcome.Condition != "the activity resident cannot be safely used" {
		t.Fatalf("unsafe activity = %#v", got)
	}
	if err := os.WriteFile(service.paths.memoryFile("low-level"), []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := service.HeartbeatActivity("low-level", testIDA); got.Condition != ActivityInvalidMemory || got.Outcome == nil {
		t.Fatalf("invalid memory = %#v", got)
	}
}

func TestActivityV2RefusalEnvelopesAreConditionSpecific(t *testing.T) {
	for _, condition := range []ActivityCondition{ActivityNotOwner, ActivityMissing, ActivityInvalidMemory, ActivityUnsafeResident} {
		got := refusalFor(activityAttach, condition, errors.New("safety observation"))
		if got.SchemaVersion != 2 || got.Condition != condition || got.Outcome == nil || got.Outcome.Category != "operation" || got.Outcome.Condition == string(condition) || got.Outcome.Cause != "" || len(got.Outcome.NextActions) < 2 || got.Outcome.ChangedActivity {
			t.Fatalf("%s envelope = %#v", condition, got)
		}
	}
	if activityObservedCondition("other") != "the activity operation cannot complete" || len(activityRecoveryActions("other")) != 2 {
		t.Fatal("default refusal constructor is not bounded")
	}
	storage := activityStorageFailure(activityAttach, "directory-fsync", errors.New("disk full"))
	got := refusalFor(activityAttach, ActivityUnsafeResident, storage)
	if got.Outcome == nil || got.Outcome.Condition != "activity storage cannot complete the operation" || got.Outcome.Cause == "" || !got.Outcome.ChangedActivity {
		t.Fatalf("storage envelope = %#v", got)
	}
	for _, action := range got.Outcome.NextActions {
		if strings.Contains(action, "retry") || strings.Contains(action, " and ") {
			t.Fatalf("storage action is not independently executable: %q", action)
		}
	}
}

func TestActivityV2Refusals(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) { noTopology(d); d.UUID = func() (string, error) { return testIDA, nil } })
	if got := service.AttachActivity("missing", testIDA); got.Condition != ActivityMissing || got.Outcome == nil || got.Outcome.Condition != "the requested effort is absent" {
		t.Fatalf("missing effort = %#v", got)
	}
	if _, err := service.New(testContext(t), NewInput{Slug: "v2", Title: "V2"}); err != nil {
		t.Fatal(err)
	}
	if got := service.HeartbeatActivity("v2", testIDA); got.Condition != ActivityMissing || got.Outcome == nil || got.Outcome.Condition != "the requested activity is absent" {
		t.Fatalf("missing activity = %#v", got)
	}
	if got := service.AttachActivity("v2", "bad"); got.Condition != ActivityUnsafeResident || got.Outcome == nil || got.Outcome.Condition != "the supplied activity identity is invalid" {
		t.Fatalf("bad owner = %#v", got)
	}
}

func TestActivityV2RefusalActionsAndMemoryCausesArePrecise(t *testing.T) {
	if got, want := activityRecoveryActions(ActivityNotOwner), []string{"confirm the active session owner", "attach this session owner to take over"}; !slices.Equal(got, want) {
		t.Fatalf("not-owner recovery = %q, want %q", got, want)
	}
	mechanismErr := &os.PathError{Op: "open", Path: "memory.md", Err: os.ErrPermission}
	if !activityReadMechanismFailure(&residentReadError{errors.New("state read failed")}) {
		t.Fatal("resident read mechanism was not classified")
	}
	if got := refusalFor(activityHeartbeat, ActivityUnsafeResident, mechanismErr); got.Outcome == nil || got.Outcome.Cause != mechanismErr.Error() {
		t.Fatalf("activity read mechanism cause = %#v", got)
	}
	mechanism := invalidMemoryRefusal(activityAttach, "demo", mechanismErr, true)
	if mechanism.Outcome == nil || mechanism.Outcome.Condition != "the effort memory cannot be read" || mechanism.Outcome.Cause != mechanismErr.Error() {
		t.Fatalf("memory read failure = %#v", mechanism)
	}
	semantic := invalidMemoryRefusal(activityAttach, "demo", errors.New("invalid header"), false)
	if semantic.Outcome == nil || semantic.Outcome.Condition != "the effort memory metadata is invalid" || semantic.Outcome.Cause != "" {
		t.Fatalf("invalid memory = %#v", semantic)
	}
	for _, err := range []error{
		errors.New("invalid JSON"),
		safety("symlink", "activity.json", nil),
		&CorruptError{Path: "state.json", Err: &residentReadError{safety("symlink", "state.json", nil)}},
	} {
		got := refusalFor(activityHeartbeat, ActivityUnsafeResident, err)
		if got.Outcome == nil || got.Outcome.Cause != "" || activityReadMechanismFailure(err) {
			t.Fatalf("semantic/safety read cause = %#v for %T", got, err)
		}
	}
	identity := &activityPublicationRefusal{Operation: activityAttach, Err: errors.New("changed")}
	if got := serviceRefusalForTest(identity); got.Outcome == nil || got.Outcome.Condition != "the activity publication identity changed" {
		t.Fatalf("identity refusal = %#v", got)
	}
}

func serviceRefusalForTest(err error) ActivityReply {
	return (&Service{}).publicationRefusal(activityAttach, err)
}
