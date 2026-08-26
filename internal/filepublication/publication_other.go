//go:build !linux && !darwin && !windows

package filepublication

import (
	"errors"
	"os"
)

func publishNoReplace(temporary, path string) error {
	return errors.New("exclusive file publication is unsupported on this platform")
}

func publishNoReplaceAt(_ *os.File, _, _ string) error {
	return errors.New("exclusive anchored publication is unsupported on this platform")
}
