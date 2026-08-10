//go:build !linux && !darwin && !windows

package filepublication

import "errors"

func publishNoReplace(temporary, path string) error {
	return errors.New("exclusive file publication is unsupported on this platform")
}
