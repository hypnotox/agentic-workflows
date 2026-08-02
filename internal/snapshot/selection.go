package snapshot

import "sort"

// Selection is an immutable, path-sorted explicit set of files. Build one with
// NewSelection; its Lookup and List methods hand out byte copies so callers
// cannot reach the captured content.
type Selection struct {
	files []File
}

// NewSelection validates files, copies each one's bytes, and returns a
// path-sorted Selection. It rejects an unsupported mode, an unsafe path, or a
// duplicate path so every selected file is addressable by exactly one canonical
// relative path.
func NewSelection(files []File) (*Selection, error) {
	fileSet, err := newFileSet(files)
	if err != nil {
		return nil, err
	}
	return &Selection{files: fileSet}, nil
}

// Lookup returns the selected file at the exact path and whether it exists. The
// returned File carries a byte copy.
func (s *Selection) Lookup(p string) (File, bool) {
	i := sort.Search(len(s.files), func(i int) bool { return s.files[i].Path >= p })
	if i < len(s.files) && s.files[i].Path == p {
		return s.files[i].clone(), true
	}
	return File{}, false
}

// List returns every selected file in path order, each with a byte copy.
func (s *Selection) List() []File {
	out := make([]File, len(s.files))
	for i := range s.files {
		out[i] = s.files[i].clone()
	}
	return out
}
