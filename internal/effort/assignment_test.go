package effort

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// invariant: tooling/effort-management:session-effort-assignment
func TestSessionEffortAssignmentAuthority(t *testing.T) {
	root := initEffortRepo(t)
	linked := filepath.Join(filepath.Dir(root), "linked assignment checkout")
	runEffortGit(t, "-C", root, "worktree", "add", "--detach", linked, "HEAD")
	ids := []string{idA, idB}
	service, err := Open(t.Context(), root, Options{
		Clock: func() time.Time { return time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC) },
		UUID:  func() (string, error) { id := ids[0]; ids = ids[1:]; return id, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.New("First", false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.New("Second", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Complete(second.ID); err != nil {
		t.Fatal(err)
	}

	assigned, err := service.Assign(first.ID, "z-session")
	if err != nil || assigned != (Assignment{SessionID: "z-session", EffortID: first.ID}) {
		t.Fatalf("initial assignment = %#v, %v", assigned, err)
	}
	path := filepath.Join(root, ".awf", "assignments", "sessions.json")
	want := `{"schemaVersion":1,"sessions":{"z-session":"` + first.ID + `"}}`
	if raw, err := os.ReadFile(path); err != nil || string(raw) != want {
		t.Fatalf("assignment schema = %q, %v; want %q", raw, err, want)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Assign(first.ID, "z-session"); err != nil {
		t.Fatalf("idempotent assignment: %v", err)
	}
	if after, err := os.ReadFile(path); err != nil || string(after) != string(before) {
		t.Fatalf("idempotent assignment changed authority: %q, %v", after, err)
	}
	if _, err := service.Assign(second.ID, "z-session"); err != nil {
		t.Fatalf("terminal reassignment: %v", err)
	}
	if record, err := service.Show(second.ID); err != nil || record.State != StateCompleted || !reflect.DeepEqual(record.AssignedSessionIDs, []string{"z-session"}) {
		t.Fatalf("terminal assignment changed state or join: %#v, %v", record, err)
	}
	if _, err := service.Assign(second.ID, "a-session"); err != nil {
		t.Fatal(err)
	}
	assignments, err := service.Assignments(second.ID)
	if err != nil || !reflect.DeepEqual(assignments, []Assignment{{SessionID: "a-session", EffortID: second.ID}, {SessionID: "z-session", EffortID: second.ID}}) {
		t.Fatalf("sorted assignments = %#v, %v", assignments, err)
	}
	if _, err := service.Abandon(first.ID); err != nil {
		t.Fatal(err)
	}
	abandonedBefore, err := service.Show(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	abandonedBytesBefore, err := os.ReadFile(filepath.Join(root, ".awf", "efforts", first.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Assign(first.ID, "y-session"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Assign(first.ID, "b-session"); err != nil {
		t.Fatal(err)
	}
	abandonedAfter, err := service.Show(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if abandonedAfter.State != StateAbandoned || abandonedAfter.CreatedAt != abandonedBefore.CreatedAt || abandonedAfter.UpdatedAt != abandonedBefore.UpdatedAt || !reflect.DeepEqual(abandonedAfter.AssignedSessionIDs, []string{"b-session", "y-session"}) {
		t.Fatalf("abandoned retrospective assignment = %#v", abandonedAfter)
	}
	if after, err := os.ReadFile(filepath.Join(root, ".awf", "efforts", first.ID+".json")); err != nil || string(after) != string(abandonedBytesBefore) {
		t.Fatalf("abandoned assignment mutated effort = %q, %v", after, err)
	}
	assignments, err = service.Assignments("")
	if err != nil {
		t.Fatal(err)
	}
	linkedService, err := Open(t.Context(), linked, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := linkedService.Assignments(""); err != nil || !reflect.DeepEqual(got, assignments) {
		t.Fatalf("linked assignment parity = %#v, %v", got, err)
	}
	if removed, err := linkedService.Unassign("a-session"); err != nil || removed.EffortID != second.ID {
		t.Fatalf("unassign = %#v, %v", removed, err)
	}
	if _, err := service.Unassign("a-session"); err == nil || !strings.Contains(err.Error(), "unknown session") {
		t.Fatalf("unknown session = %v", err)
	}
	for _, session := range []string{"", " ../bad", "../bad", strings.Repeat("x", 161)} {
		if _, err := service.Assign(first.ID, session); err == nil {
			t.Errorf("unsafe session %q accepted", session)
		}
	}
	if _, err := service.Assign("bad", "new-session"); err == nil {
		t.Fatal("invalid effort accepted")
	}
	if _, err := service.Assign("33333333-3333-4333-8333-333333333333", "new-session"); err == nil {
		t.Fatal("unknown effort accepted")
	}
	if _, err := service.Assignments("33333333-3333-4333-8333-333333333333"); err == nil {
		t.Fatal("unknown assignment effort accepted")
	}

	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeEffortFile(t, path, `{`)
	if _, err := service.Assign(first.ID, "new-session"); err == nil {
		t.Fatal("corrupt authority accepted")
	} else {
		var corrupt *CorruptError
		if !errors.As(err, &corrupt) {
			t.Fatalf("corrupt assignment error = %T %v", err, err)
		}
	}
	if _, err := service.Unassign("z-session"); err == nil {
		t.Fatal("corrupt authority accepted by unassign")
	}
	if _, err := service.Assignments(""); err == nil {
		t.Fatal("corrupt authority accepted by assignments")
	}
	if _, err := service.Unassign("../unsafe"); err == nil {
		t.Fatal("invalid unassignment session accepted")
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != `{` {
		t.Fatalf("corrupt authority changed = %q, %v", raw, err)
	}
	writeEffortFile(t, path, string(original))
	faulty, err := Open(t.Context(), root, Options{Filesystem: &faultFileSystem{fail: "publish"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := faulty.Assign(first.ID, "fault-session"); !errors.Is(err, errInjectedDurability) {
		t.Fatalf("assignment durability fault = %v", err)
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != string(original) {
		t.Fatalf("failed assignment changed authority = %q, %v", raw, err)
	}
	if raw, err := os.ReadFile(filepath.Join(root, ".awf", "efforts", second.ID+".json")); err != nil || strings.Contains(string(raw), "assignedSessionIds") {
		t.Fatalf("effort record duplicates assignments = %q, %v", raw, err)
	}
}

func TestAssignmentTargetValidationPreservesAuthority(t *testing.T) {
	cases := []struct {
		name       string
		targetFile string
		assignment string
	}{
		{name: "missing target", assignment: `{"schemaVersion":1,"sessions":{"missing-session":"` + idB + `"}}`},
		{name: "corrupt target", targetFile: `{`, assignment: `{"schemaVersion":1,"sessions":{"corrupt-session":"` + idB + `"}}`},
		{name: "schema-invalid target", targetFile: `{"schemaVersion":2}`, assignment: `{"schemaVersion":1,"sessions":{"schema-session":"` + idB + `"}}`},
		{name: "mixed map", targetFile: `{"schemaVersion":2}`, assignment: `{"schemaVersion":1,"sessions":{"valid-session":"` + idA + `","invalid-session":"` + idB + `"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := initEffortRepo(t)
			service := openEffortService(t, root, time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC))
			if _, err := service.New("Valid target", false); err != nil {
				t.Fatal(err)
			}
			if tc.targetFile != "" {
				writeEffortFile(t, service.paths.record(idB), tc.targetFile)
			}
			path := service.paths.assignments()
			writeEffortFile(t, path, tc.assignment)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.Assignments(""); err == nil {
				t.Fatal("invalid assignment target accepted")
			} else {
				var corrupt *CorruptError
				if !errors.As(err, &corrupt) {
					t.Fatalf("target validation error = %T %v", err, err)
				}
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) {
				t.Fatalf("assignment authority changed = %q, %v; want %q", after, err, before)
			}
		})
	}
}
