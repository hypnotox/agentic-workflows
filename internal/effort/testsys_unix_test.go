//go:build linux || darwin

package effort

import (
	"os"
	"syscall"
)

func testCurrentEUID() int { return os.Geteuid() }

func testChown(path string, uid int) error { return os.Chown(path, uid, -1) }

func testMkfifo(path string, mode uint32) error { return syscall.Mkfifo(path, mode) }
