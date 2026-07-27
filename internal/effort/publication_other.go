//go:build darwin

package effort

import (
	"errors"
	"os"
)

// unexpectedPublicationIdentity verifies the destination displaced by a
// platform's atomic replacement primitive. A mismatch must be restored before
// the caller returns this refusal.
func unexpectedPublicationIdentity(path string, expected *fileIdentity, displaced fileIdentity, inspectErr error) error {
	if inspectErr == nil && os.SameFile(expected.info, displaced.info) {
		return nil
	}
	if inspectErr != nil {
		return inspectErr
	}
	return safety("identity", path, errors.New("destination changed before atomic publication"))
}
