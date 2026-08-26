//go:build !linux && !darwin && !windows

package filesystem

import (
	"errors"
	"io/fs"
	"os"
)

// exchangeExpected refuses on platforms without a native exchange or
// replacement primitive rather than opening a check-to-commit race.
func exchangeExpected(_ *os.Root, _, _ string, _ fs.FileInfo, _ bool) (bool, error) {
	return false, errors.New("atomic expected-identity mutation is unsupported on this platform")
}
