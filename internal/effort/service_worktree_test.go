package effort

import (
	"strings"
	"testing"
	"time"
)

func TestWorktreeMetadataMutations(t *testing.T) {
	s := openEffortService(t, initEffortRepo(t), time.Now())
	r, err := s.New("metadata", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetIntegration(r.ID, IntegrationManual); err == nil {
		t.Fatal("set integration without worktree")
	}
	if _, err = s.RemoveWorktreeMetadata(r.ID, false); err == nil {
		t.Fatal("remove without worktree")
	}
	r, err = s.AttachWorktree(r.ID, strings.Repeat("a", 40))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AttachWorktree(r.ID, strings.Repeat("a", 40)); err == nil {
		t.Fatal("duplicate attach")
	}
	if _, err = s.RemoveWorktreeMetadata(r.ID, false); err == nil {
		t.Fatal("pending remove")
	}
	if _, err = s.SetIntegration(r.ID, IntegrationFastForward); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RemoveWorktreeMetadata(r.ID, false); err != nil {
		t.Fatal(err)
	}
}
