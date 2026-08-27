package filesystem

import (
	"errors"
	"io/fs"
	"testing"
	"time"
)

func TestSupportedTreeEntryResolvesAmbiguousType(t *testing.T) {
	tests := []struct {
		name      string
		entry     fs.DirEntry
		want      bool
		wantError bool
	}{
		{
			name:  "regular",
			entry: treeEntryFixture{info: treeEntryInfo{mode: 0o644}},
			want:  true,
		},
		{
			name:  "socket reported through info",
			entry: treeEntryFixture{info: treeEntryInfo{mode: fs.ModeSocket}},
			want:  false,
		},
		{
			name:  "directory reported through info",
			entry: treeEntryFixture{info: treeEntryInfo{mode: fs.ModeDir}},
			want:  true,
		},
		{
			name:      "ambiguous info failure",
			entry:     treeEntryFixture{infoErr: errors.New("info failed")},
			wantError: true,
		},
		{
			name:  "known socket needs no info",
			entry: treeEntryFixture{typ: fs.ModeSocket, infoErr: errors.New("must not inspect")},
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SupportedTreeEntry(tt.entry)
			if (err != nil) != tt.wantError {
				t.Fatalf("SupportedTreeEntry error = %v, wantError %v", err, tt.wantError)
			}
			if got != tt.want {
				t.Fatalf("SupportedTreeEntry = %v, want %v", got, tt.want)
			}
		})
	}
}

type treeEntryFixture struct {
	typ     fs.FileMode
	info    fs.FileInfo
	infoErr error
}

func (e treeEntryFixture) Name() string               { return "entry" }
func (e treeEntryFixture) IsDir() bool                { return e.typ.IsDir() }
func (e treeEntryFixture) Type() fs.FileMode          { return e.typ }
func (e treeEntryFixture) Info() (fs.FileInfo, error) { return e.info, e.infoErr }

type treeEntryInfo struct{ mode fs.FileMode }

func (i treeEntryInfo) Name() string       { return "entry" }
func (i treeEntryInfo) Size() int64        { return 0 }
func (i treeEntryInfo) Mode() fs.FileMode  { return i.mode }
func (i treeEntryInfo) ModTime() time.Time { return time.Time{} }
func (i treeEntryInfo) IsDir() bool        { return i.mode.IsDir() }
func (i treeEntryInfo) Sys() any           { return nil }
