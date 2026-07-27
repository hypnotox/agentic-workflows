//go:build !linux

package effort

import "errors"

func publishAtomic(string, string, *fileIdentity) error {
	return errors.New("atomic conditional effort publication is unsupported on this platform")
}
