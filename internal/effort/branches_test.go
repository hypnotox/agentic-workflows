package effort

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestEffortFocusedFailureBranches(t *testing.T) {
	for range 8 {
		id, err := randomUUIDv4()
		if err != nil || !uuidV4Pattern.MatchString(id) {
			t.Fatalf("random UUID = %q, err=%v", id, err)
		}
	}
	root := initEffortRepo(t)
	service, err := Open(context.Background(), root, Options{UUID: func() (string, error) { return "bad", nil }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.New("Invalid allocator"); err == nil || !strings.Contains(err.Error(), "invalid UUIDv4") {
		t.Fatalf("allocator error = %v", err)
	}
	service, err = Open(context.Background(), root, Options{UUID: func() (string, error) { return "", errors.New("entropy") }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.New("Allocator failure"); err == nil || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("allocation error = %v", err)
	}
	if _, err := service.Finish("no-such-effort"); err == nil || !strings.Contains(err.Error(), "no active resident") {
		t.Fatalf("missing finish error = %v", err)
	}
	if _, err := service.Show("Bad_Slug"); err == nil || !strings.Contains(err.Error(), "exact slug") {
		t.Fatalf("invalid show error = %v", err)
	}
}
