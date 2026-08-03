package effort

import (
	"errors"
	"strings"
	"testing"
)

func TestEffortFocusedFailureBranches(t *testing.T) {
	for range 8 {
		id, err := RandomUUIDv4()
		if err != nil || !uuidV4Pattern.MatchString(id) {
			t.Fatalf("random UUID = %q, err=%v", id, err)
		}
	}
	root := initEffortRepo(t)
	service := openTestService(t, root, func(deps *Dependencies) {
		deps.UUID = func() (string, error) { return "bad", nil }
	})
	if _, err := service.New(testContext(t), NewInput{Slug: "invalid-allocator", Title: "Invalid allocator"}); err == nil || !strings.Contains(err.Error(), "invalid UUIDv4") {
		t.Fatalf("allocator error = %v", err)
	}
	service = openTestService(t, root, func(deps *Dependencies) {
		deps.UUID = func() (string, error) { return "", errors.New("entropy") }
	})
	if _, err := service.New(testContext(t), NewInput{Slug: "allocator-failure", Title: "Allocator failure"}); err == nil || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("allocation error = %v", err)
	}
	if _, err := service.Finish(testContext(t), "no-such-effort"); err == nil || !strings.Contains(err.Error(), "no active resident") {
		t.Fatalf("missing finish error = %v", err)
	}
	if _, err := service.Show("Bad_Slug"); err == nil || !strings.Contains(err.Error(), "exact slug") {
		t.Fatalf("invalid show error = %v", err)
	}
}
