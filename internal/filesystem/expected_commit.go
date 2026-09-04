package filesystem

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
)

type expectedRegularFile struct {
	contents []byte
	mode     fs.FileMode
}

type afterExpectedExchange func(*os.Root, string) error

func exchangeExpected(root *os.Root, temporary, destination string, expected *ExpectedIdentity, exact *expectedRegularFile, remove, retain bool) (bool, error) {
	return exchangeExpectedWithHook(root, temporary, destination, expected, exact, remove, retain, nil)
}

// exchangeExpectedWithHook exists so platform tests can deterministically place
// an in-place write or chmod at the post-exchange validation boundary. Runtime
// callers always use exchangeExpected without a hook.
func exchangeExpectedWithHook(root *os.Root, temporary, destination string, expected *ExpectedIdentity, exact *expectedRegularFile, remove, retain bool, afterExchange afterExpectedExchange) (bool, error) {
	parent := path.Dir(destination)
	if path.Dir(temporary) != parent {
		return false, fmt.Errorf("filesystem: expected mutation paths have different parents")
	}
	parentRoot, err := root.OpenRoot(parent)
	if err != nil {
		return false, fmt.Errorf("filesystem: open atomic parent %q: %w", parent, err)
	}
	defer parentRoot.Close()
	anchor, err := parentRoot.Open(".")
	if err != nil {
		return false, fmt.Errorf("filesystem: open atomic parent anchor %q: %w", parent, err)
	}
	defer anchor.Close()
	consumed, err := exchangeExpectedAnchored(parentRoot, anchor, path.Base(temporary), path.Base(destination), expected, exact, remove, retain, afterExchange)
	var cleanup *filepublication.CommittedCleanupError
	if errors.As(err, &cleanup) {
		cleanup.DestinationPath = destination
		cleanup.ResiduePath = path.Join(parent, cleanup.ResiduePath)
	}
	return consumed, err
}

// finishExpectedExchange validates the displaced entry before deleting it. The
// platform owner supplies one atomic exchange operation, used first to publish
// and again to undo a mismatch.
func finishExpectedExchange(root *os.Root, temporary, destination string, expected *ExpectedIdentity, exact *expectedRegularFile, remove, retain bool, afterExchange afterExpectedExchange, exchange func() error) (bool, error) {
	if err := exchange(); err != nil {
		return false, fmt.Errorf("filesystem: exchange expected entry %q: %w", destination, err)
	}
	var inspectErr error
	if afterExchange != nil {
		inspectErr = afterExchange(root, temporary)
	}
	if inspectErr == nil {
		inspectErr = verifyDisplacedExpected(root, temporary, expected, exact)
	}
	if inspectErr != nil {
		mismatch := errors.Join(ErrIdentityChanged, inspectErr)
		if rollbackErr := exchange(); rollbackErr != nil {
			cause := errors.Join(mismatch, fmt.Errorf("restore unexpected entry at %q: %w", destination, rollbackErr))
			return true, &filepublication.CommittedCleanupError{DestinationPath: destination, ResiduePath: temporary, Cause: cause}
		}
		return false, mismatch
	}
	if retain {
		if err := root.Remove(destination); err != nil {
			if rollbackErr := exchange(); rollbackErr != nil {
				cause := errors.Join(err, fmt.Errorf("restore expected entry at %q: %w", destination, rollbackErr))
				return true, &filepublication.CommittedCleanupError{DestinationPath: destination, ResiduePath: temporary, Cause: cause}
			}
			cause := fmt.Errorf("remove retirement reservation before restoring expected entry: %w", err)
			return true, &filepublication.CommittedCleanupError{DestinationPath: destination, ResiduePath: temporary, Cause: cause}
		}
		return true, nil
	}
	if err := root.Remove(temporary); err != nil {
		if rollbackErr := exchange(); rollbackErr != nil {
			cause := errors.Join(err, fmt.Errorf("restore expected entry at %q: %w", destination, rollbackErr))
			return true, &filepublication.CommittedCleanupError{DestinationPath: destination, ResiduePath: temporary, Cause: cause}
		}
		if expected.IsDir() {
			return false, errors.Join(ErrDirectoryNotEmpty, err)
		}
		return false, err
	}
	if remove {
		if err := root.Remove(destination); err != nil {
			return true, &filepublication.CommittedCleanupError{DestinationPath: destination, ResiduePath: destination, Cause: err}
		}
	}
	return true, nil
}

func verifyDisplacedExpected(root *os.Root, temporary string, expected *ExpectedIdentity, exact *expectedRegularFile) error {
	if exact == nil {
		info, err := root.Lstat(temporary)
		if err != nil {
			return err
		}
		if !expected.same(info) {
			return errors.New("displaced entry identity differs")
		}
		return nil
	}
	if !expected.Mode().IsRegular() {
		return errors.New("displaced entry is not the expected regular file")
	}
	file, err := openExpectedEntry(root, temporary)
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck // read-only validation owns no mutation
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !expected.same(info) || !info.Mode().IsRegular() {
		return errors.New("displaced entry is not the expected regular file")
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	if info.Mode().Perm() != exact.mode.Perm() || !bytes.Equal(contents, exact.contents) {
		return errors.New("displaced regular-file image differs")
	}
	return nil
}
