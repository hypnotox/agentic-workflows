package commitmsg

import (
	"errors"
	"reflect"
	"testing"
)

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

func TestParseAuthorizations(t *testing.T) {
	if got, err := ParseAuthorizations(Message{}, func(string) bool { return false }); err != nil || got != nil {
		t.Fatalf("empty message = %#v, %v", got, err)
	}
	valid := func(value string) bool { return value == "legacy" || value == "current-state-v2" }
	msg := Clean([]byte("Merge branch 'old'\n\nbody\n\nSigned-off-by: A <a@example.test>\nAWF-Allow-Version: current-state-v2 \nAWF-Allow-Reason:  stale branch \nAWF-Allow-Version: legacy\nAWF-Allow-Reason: preserved history\nReviewed-by: B\n"))
	got, err := ParseAuthorizations(msg, valid)
	if err != nil {
		t.Fatal(err)
	}
	want := []Authorization{{Version: "current-state-v2", Reason: "stale branch"}, {Version: "legacy", Reason: "preserved history"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("authorizations = %#v, want %#v", got, want)
	}
	if got, err := ParseAuthorizations(Clean([]byte("feat: ordinary\n")), valid); err != nil || got != nil {
		t.Fatalf("ordinary message = %#v, %v", got, err)
	}
}

func TestParseAuthorizationsRefusesMalformedReservedSyntax(t *testing.T) {
	valid := func(value string) bool { return value == "legacy" }
	cases := []string{
		"feat: x\nAWF-Allow-Version: legacy\n",
		"feat: x\n\nAWF-Allow-Nope: legacy\n",
		"feat: x\n\nAWF-Allow-Version: future\nAWF-Allow-Reason: why\n",
		"feat: x\n\nAWF-Allow-Reason: why\nAWF-Allow-Version: legacy\n",
		"feat: x\n\nAWF-Allow-Version: legacy\nOther: x\nAWF-Allow-Reason: why\n",
		"feat: x\n\nAWF-Allow-Version: legacy\nAWF-Allow-Reason:   \n",
		"feat: x\n\nAWF-Allow-Version: legacy\nAWF-Allow-Reason: \v\f\n",
		"feat: x\n\nAWF-Allow-Version: legacy\n continuation\nAWF-Allow-Reason: why\n",
		"feat: x\n\nAWF-Allow-Version: legacy\n",
	}
	for _, raw := range cases {
		_, err := ParseAuthorizations(Clean([]byte(raw)), valid)
		var syntax *SyntaxError
		if !errors.As(err, &syntax) || syntax.Line == 0 || syntax.Reason == "" {
			t.Errorf("ParseAuthorizations(%q) error = %#v", raw, err)
		}
	}
}
