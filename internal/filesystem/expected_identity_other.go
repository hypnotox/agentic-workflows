//go:build !linux

package filesystem

import "fmt"

// ExpectedIdentity acquires metadata for an expected mutation. Supported
// non-Linux platforms currently use their native metadata identity.
func (h *Handle) ExpectedIdentity(name string) (*ExpectedIdentity, error) {
	info, err := h.LinkInfo(name)
	if err != nil {
		return nil, fmt.Errorf("filesystem: expected identity %q: %w", name, err)
	}
	return &ExpectedIdentity{info: info}, nil
}
