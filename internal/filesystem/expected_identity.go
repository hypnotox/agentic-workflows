package filesystem

import (
	"io/fs"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// ExpectedIdentity is an opaque, releasable identity capability for one
// observed directory entry. It is acquired only for an expected mutation.
// On platforms that need it to resist inode reuse it retains a native handle.
type ExpectedIdentity struct {
	info     fs.FileInfo
	release  func() error
	once     sync.Once
	released atomic.Bool
	err      error
}

func (e *ExpectedIdentity) Name() string       { return e.info.Name() }
func (e *ExpectedIdentity) Size() int64        { return e.info.Size() }
func (e *ExpectedIdentity) Mode() fs.FileMode  { return e.info.Mode() }
func (e *ExpectedIdentity) ModTime() time.Time { return e.info.ModTime() }
func (e *ExpectedIdentity) IsDir() bool        { return e.info.IsDir() }
func (e *ExpectedIdentity) Sys() any           { return e.info.Sys() }

func (e *ExpectedIdentity) valid() bool {
	return e != nil && e.info != nil && !e.released.Load()
}

// Release releases resources retained by this identity. It is idempotent.
func (e *ExpectedIdentity) Release() error {
	if e == nil {
		return nil
	}
	e.once.Do(func() {
		e.released.Store(true)
		if e.release != nil {
			e.err = e.release()
		}
	})
	return e.err
}

// SameFile reports whether info names the entry retained by this identity.
func (e *ExpectedIdentity) SameFile(info fs.FileInfo) bool {
	return e.valid() && os.SameFile(e.info, info)
}

func (e *ExpectedIdentity) same(info fs.FileInfo) bool { return e.SameFile(info) }
