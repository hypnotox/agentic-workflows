// Package snapshot captures immutable file trees from a Git repository. A Tree
// owns a private copy of every file's bytes, so a consumer can read the
// captured content and mode without being able to mutate the snapshot or the
// caller's original data. It captures four universes: the working tree, the
// stage-0 index, an arbitrary commit, and a first-parent before/after range
// pair. Each is the complete selected file set; consumers apply their own
// eligibility filters.
package snapshot

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
)

// Mode is the file mode a Tree preserves. Symlink bytes are inert targets.
type Mode uint8

const (
	// Regular is an ordinary, non-executable file.
	Regular Mode = iota
	// Executable is a file with the executable bit set.
	Executable
	// Symlink is an inert symbolic link whose bytes are its target.
	Symlink
)

// Construction faults. NewTree returns one of these when a File would make the
// snapshot ambiguous or unsafe to address by path.
var (
	// ErrUnsupportedMode reports a File whose Mode is not representable.
	ErrUnsupportedMode = errors.New("snapshot: unsupported file mode")
	// ErrUnsafePath reports a File whose path is empty, absolute, or escapes
	// the tree root through traversal or a non-canonical form.
	ErrUnsafePath = errors.New("snapshot: unsafe path")
	// ErrDuplicatePath reports two Files sharing one path.
	ErrDuplicatePath = errors.New("snapshot: duplicate path")
)

// File is one file in a Tree: a repo-relative slash path, its Mode, and a
// private copy of its bytes.
type File struct {
	Path  string
	Mode  Mode
	Bytes []byte
}

// Scannable reports whether authority parsers may inspect this file's bytes.
func (f File) Scannable() bool { return f.Mode == Regular || f.Mode == Executable }

// clone returns a copy of f whose Bytes cannot alias the receiver's.
func (f File) clone() File {
	return File{Path: f.Path, Mode: f.Mode, Bytes: cloneBytes(f.Bytes)}
}

// Tree is an immutable, path-sorted set of files. Build one with NewTree; its
// Lookup and List methods hand out byte copies so callers cannot reach the
// captured content.
type Tree struct {
	files []File
}

// NewTree validates files, copies each one's bytes, and returns a path-sorted
// Tree. It rejects an unsupported mode, an unsafe path, or a duplicate path so
// every file is addressable by exactly one canonical relative path.
func NewTree(files []File) (*Tree, error) {
	out := make([]File, 0, len(files))
	seen := make(map[string]bool, len(files))
	for _, f := range files {
		if f.Mode != Regular && f.Mode != Executable && f.Mode != Symlink {
			return nil, fmt.Errorf("%w: %q has mode %d", ErrUnsupportedMode, f.Path, f.Mode)
		}
		if !safePath(f.Path) {
			return nil, fmt.Errorf("%w: %q", ErrUnsafePath, f.Path)
		}
		if seen[f.Path] {
			return nil, fmt.Errorf("%w: %q", ErrDuplicatePath, f.Path)
		}
		seen[f.Path] = true
		out = append(out, f.clone())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return &Tree{files: out}, nil
}

// Lookup returns the file at the exact path and whether it exists. The returned
// File carries a byte copy.
func (t *Tree) Lookup(p string) (File, bool) {
	i := sort.Search(len(t.files), func(i int) bool { return t.files[i].Path >= p })
	if i < len(t.files) && t.files[i].Path == p {
		return t.files[i].clone(), true
	}
	return File{}, false
}

// List returns every file in path order, each with a byte copy.
func (t *Tree) List() []File {
	out := make([]File, len(t.files))
	for i := range t.files {
		out[i] = t.files[i].clone()
	}
	return out
}

// safePath reports whether p is a canonical, relative, forward-slash path that
// stays within the tree root. It rejects the empty path, backslashes, absolute
// paths, non-canonical forms (redundant or trailing slashes, "." segments), and
// any parent-directory traversal.
func safePath(p string) bool {
	if p == "" || strings.ContainsRune(p, '\\') || path.IsAbs(p) || p != path.Clean(p) {
		return false
	}
	return p != "." && p != ".." && !strings.HasPrefix(p, "../")
}

// cloneBytes returns an independent copy of b. A nil slice clones to an empty,
// non-nil slice, which reads identically for content scans.
func cloneBytes(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}
