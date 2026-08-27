package snapshot

import "sort"

// Selection is an immutable, path-sorted explicit set of files. It remains the
// historical sparse content representation; LiveContext is the distinct live
// inventory-plus-content representation used by ordinary context.
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

// Entry is a live path inventory entry. Unlike File it makes no claim that
// bytes were captured, so an unread present path cannot be confused with an
// absent path.
type Entry struct {
	Path string
	Mode Mode
}

// Inventory is an immutable complete live path and mode inventory.
type Inventory struct{ entries []Entry }

// NewInventory validates and sorts a complete inventory independently from
// selected content.
func NewInventory(entries []Entry) (*Inventory, error) {
	seen := map[string]bool{}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.Mode != Regular && e.Mode != Executable && e.Mode != Symlink {
			return nil, ErrUnsupportedMode
		}
		if !safePath(e.Path) {
			return nil, ErrUnsafePath
		}
		if seen[e.Path] {
			return nil, ErrDuplicatePath
		}
		seen[e.Path] = true
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return &Inventory{entries: out}, nil
}

// Lookup returns the inventory entry at p, if present.
func (i *Inventory) Lookup(p string) (Entry, bool) {
	at := sort.Search(len(i.entries), func(n int) bool { return i.entries[n].Path >= p })
	if at < len(i.entries) && i.entries[at].Path == p {
		return i.entries[at], true
	}
	return Entry{}, false
}

// List returns inventory entries in path order.
func (i *Inventory) List() []Entry { return append([]Entry(nil), i.entries...) }

// LiveContext joins complete inventory to separately selected immutable bytes.
type LiveContext struct {
	inventory *Inventory
	selected  *Selection
}

func NewLiveContext(inventory *Inventory, selected *Selection) (*LiveContext, error) {
	if inventory == nil || selected == nil {
		return nil, ErrUnsafePath
	}
	for _, f := range selected.List() {
		e, ok := inventory.Lookup(f.Path)
		if !ok || e.Mode != f.Mode {
			return nil, ErrUnsafePath
		}
	}
	return &LiveContext{inventory: inventory, selected: selected}, nil
}

// Inventory returns the complete live path and mode inventory.
func (v *LiveContext) Inventory() *Inventory { return v.inventory }

// Selection returns the selected immutable file content.
func (v *LiveContext) Selection() *Selection { return v.selected }
