//go:build !linux && !darwin && !windows

package filesystem

import (
	"errors"
	"io/fs"
	"os"
)

func exchangeExpectedAnchored(_ *os.Root, _ *os.File, _, _ string, _ fs.FileInfo, _ bool, _ bool) (bool, error) {
	return false, errors.New("atomic expected-identity mutation is unsupported on this platform")
}
