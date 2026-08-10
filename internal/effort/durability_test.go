package effort

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
)

func TestProtocol2DirectorySyncAndExclusivePublication(t *testing.T) {
	dir := t.TempDir()
	if err := syncDirectory(dir); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "destination")
	if err := filepublication.Publish(destination, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(destination); err != nil || string(raw) != "new" {
		t.Fatalf("published bytes=%q err=%v", raw, err)
	}
	identity, err := lstatRegular(destination)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishAtomic(filepath.Join(dir, "missing-temporary"), destination, &identity); err == nil {
		t.Fatal("missing replacement temporary accepted")
	}
}

func TestProtocol2SafeLeafRefusals(t *testing.T) {
	root := t.TempDir()
	directoryInfo, err := os.Lstat(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateLeaf(root, directoryInfo); err == nil {
		t.Fatal("directory accepted as regular leaf")
	}
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	linkInfo, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateLeaf(link, linkInfo); err == nil {
		t.Fatal("symlink accepted as regular leaf")
	}
	if _, err := platformLstatRegular(filepath.Join(root, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing lstat error = %v", err)
	}
	if _, err := platformLstatRegular(link); err == nil {
		t.Fatal("symlink lstat accepted")
	}
	created := filepath.Join(root, "created")
	file, err := platformOpenRegularNoFollow(created, true, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if file, err := platformOpenRegularNoFollow(filepath.Join(root, "missing"), false, 0); err == nil {
		_ = file.Close()
		t.Fatal("missing open accepted")
	}
	if file, err := platformOpenRegularNoFollow(link, false, 0); err == nil {
		_ = file.Close()
		t.Fatal("symlink open accepted")
	}
}
