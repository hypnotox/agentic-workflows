package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func snapshotTree(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		b.WriteString(rel + "@" + info.Mode().String() + ":")
		if !d.IsDir() {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			b.Write(content)
		}
		b.WriteByte(';')
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}
