//go:build !linux && !darwin && !windows

package filesystem

import (
	"errors"
	"os"
)

func exchangeExpectedAnchored(_ *os.Root, _ *os.File, _, _ string, _ *ExpectedIdentity, _ bool, _ bool) (bool, error) {
	return false, errors.New("atomic expected-identity mutation is unsupported on this platform")
}
