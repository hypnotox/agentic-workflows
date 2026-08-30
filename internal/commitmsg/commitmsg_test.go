package commitmsg

import "testing"

func TestCleanCommitMessage(t *testing.T) {
	raw := []byte("\r\n  # threshold >8 is only a comment\r\nfeat: subject  \r\n\r\nbody  \r\n# ------------------------ >8 ------------------------\r\ndiff\r\n")
	got := Clean(raw)
	if got.Subject != "feat: subject" {
		t.Fatalf("Subject = %q", got.Subject)
	}
	if got.Text != "\nfeat: subject  \n\nbody  " {
		t.Fatalf("Text = %q", got.Text)
	}
	if got := Clean([]byte("# only\n\n")); got != (Message{}) {
		t.Fatalf("comment-only Clean = %#v", got)
	}
}
