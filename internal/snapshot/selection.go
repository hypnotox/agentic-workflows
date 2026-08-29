package snapshot

import "sort"

// Selection is an immutable, path-sorted explicit set of files used by
// historical sparse reads.
type Selection struct{ files []File }

// NewSelection validates files, copies their bytes, and sorts them by path.
func NewSelection(files []File) (*Selection, error) {
	fileSet, err := newFileSet(files)
	if err != nil {
		return nil, err
	}
	return &Selection{files: fileSet}, nil
}

// Lookup returns a copied selected file at p, if present.
func (s *Selection) Lookup(p string) (File, bool) {
	i := sort.Search(len(s.files), func(i int) bool { return s.files[i].Path >= p })
	if i < len(s.files) && s.files[i].Path == p {
		return s.files[i].clone(), true
	}
	return File{}, false
}

// List returns copied selected files in path order.
func (s *Selection) List() []File {
	out := make([]File, len(s.files))
	for i := range s.files {
		out[i] = s.files[i].clone()
	}
	return out
}
