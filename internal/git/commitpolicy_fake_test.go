package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/commitpolicy"
)

func TestVerifySSHNativeVerdicts(t *testing.T) {
	dir := t.TempDir()
	git := filepath.Join(dir, "git")
	body := `#!/bin/sh
case "$*" in
  *"cat-file -p"*) printf 'tree x\ngpgsig ssh-ed25519 AAAA\n\nmessage\n' ;;
  *"verify-commit good"*) exit 0 ;;
  *"verify-commit malformed"*) echo 'invalid format' >&2; exit 1 ;;
  *"verify-commit cancelled"*) sleep 1 ;;
  *) echo 'unknown signer' >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(git, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	repo := &Repo{runner: newRunner(dir)}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, tc := range []struct {
		id   string
		want commitpolicy.SignatureVerdict
	}{{"good", commitpolicy.SignatureValid}, {"wrong", commitpolicy.SignatureWrongKey}, {"malformed", commitpolicy.SignatureMalformed}} {
		got, err := repo.VerifySSH(ctx, tc.id, []commitpolicy.Signer{{Principal: "a", Key: "key"}})
		if err != nil || got != tc.want {
			t.Fatalf("%s = %v, %v", tc.id, got, err)
		}
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if _, err := repo.VerifySSH(cancelled, "cancelled", nil); err == nil {
		t.Fatal("cancelled process succeeded")
	}
}
