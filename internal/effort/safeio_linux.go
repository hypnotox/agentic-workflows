//go:build linux

package effort

import (
	"os"
	"syscall"
)

func linkCount(info os.FileInfo) uint64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Nlink
	}
	return 1 // coverage-ignore: Linux os.FileInfo values carry syscall.Stat_t
}
