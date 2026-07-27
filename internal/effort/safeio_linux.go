//go:build linux

package effort

import (
	"os"
	"syscall"
)

func platformWidthUint64[T ~uint32 | ~uint64](value T) uint64 { return uint64(value) }

func linkCount(info os.FileInfo) uint64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return platformWidthUint64(stat.Nlink)
	}
	return 1 // coverage-ignore: Linux os.FileInfo values carry syscall.Stat_t
}
