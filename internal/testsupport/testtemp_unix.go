//go:build linux || darwin

package testsupport

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

func testTempRoot() (string, error) {
	return filepath.Join(os.TempDir(), fmt.Sprintf("awf-test-homes-%d", os.Geteuid())), nil
}

func validateTestTempPath(_ string, info fs.FileInfo) error {
	if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("not a real directory")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("ownership representation unavailable")
	}
	if int(stat.Uid) != os.Geteuid() { // coverage-ignore: an unprivileged test cannot create a foreign-owned directory
		return fmt.Errorf("owner UID %d does not match effective UID %d", stat.Uid, os.Geteuid())
	}
	if info.Mode().Perm()&0o077 != 0 {
		return errors.New("group or world accessible")
	}
	return nil
}
