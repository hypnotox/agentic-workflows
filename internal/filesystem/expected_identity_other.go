//go:build !linux

package filesystem

import "fmt"

// ExpectedIdentity acquires one leaf for an expected mutation. The caller must
// Release an identity it abandons; expected-mutation methods consume it.
// Supported non-Linux platforms currently use their native metadata identity.
func (h *Handle) ExpectedIdentity(name string) (*ExpectedIdentity, error) {
	info, err := h.LinkInfo(name)
	if err != nil {
		return nil, fmt.Errorf("filesystem: expected identity %q: %w", name, err)
	}
	return &ExpectedIdentity{info: info}, nil
}
