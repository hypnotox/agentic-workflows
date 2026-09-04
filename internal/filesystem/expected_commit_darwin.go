//go:build darwin

package filesystem

import (
	"os"

	"golang.org/x/sys/unix"
)

// exchangeExpectedAnchored performs the platform-native leaf exchange.
//
// The common expected-mutation owner resolves the immediate parent through
// os.Root, opens its stable anchor, and supplies only final path components
// before this platform helper runs. Keeping intermediate path resolution out
// of the native syscall prevents a replaced parent symlink from redirecting
// the exchange outside the selected root while preserving one native owner
// for each released platform's atomic replacement operation.
func exchangeExpectedAnchored(root *os.Root, anchor *os.File, temporary, destination string, expected *ExpectedIdentity, exact *expectedRegularFile, remove, retain bool, afterExchange afterExpectedExchange) (bool, error) {
	exchange := func() error {
		return unix.RenameatxNp(int(anchor.Fd()), temporary, int(anchor.Fd()), destination, unix.RENAME_SWAP)
	}
	return finishExpectedExchange(root, temporary, destination, expected, exact, remove, retain, afterExchange, exchange)
}
