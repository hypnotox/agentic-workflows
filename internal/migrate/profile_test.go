package migrate

import "testing"

// invariant: config/migrations-and-locks:profile-full-migration (TestProfileMigrationProof)
func TestProfileMigrationProof(t *testing.T) {
	src := []byte("prefix: x\nintegrationBranch: main\n")
	got, err := ConfigForCurrentSchema(src, 45)
	if err != nil || string(got) != "prefix: x\nintegrationBranch: main\nprofile: full\n" {
		t.Fatalf("got %q err %v", got, err)
	}
	again, err := ConfigForCurrentSchema(got, 46)
	if err != nil || string(again) != string(got) {
		t.Fatalf("idempotence got %q err %v", again, err)
	}
	if _, err := ConfigForCurrentSchema([]byte("["), 45); err == nil {
		t.Fatal("malformed historical config accepted")
	}
}
