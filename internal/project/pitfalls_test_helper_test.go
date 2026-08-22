package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/pitfall"
)

func loadPitfallCorpus(p renderInputs) (pitfall.Corpus, error) {
	return loadPitfallCorpusFrom(projectTreeReader(p))
}

func loadPitfallCorpusFrom(reader ProjectTreeReader) (pitfall.Corpus, error) {
	var paths []string
	switch current := reader.(type) {
	case snapshotTreeReader:
		for _, file := range current.tree.List() {
			if strings.HasPrefix(file.Path, pitfallsSourceDir+"/") {
				paths = append(paths, file.Path)
			}
		}
	default:
		var err error
		paths, err = reader.Paths(pitfallsSourceDir + "/")
		if err != nil {
			return pitfall.Corpus{}, err
		}
	}
	files := make([]pitfall.SourceFile, 0, len(paths))
	for _, source := range paths {
		regular := true
		if current, ok := reader.(filesystemProjectReader); ok {
			info, err := os.Lstat(filepath.Join(current.root, filepath.FromSlash(source)))
			if err != nil { // coverage-ignore: Paths just enumerated this same filesystem entry; reaching Lstat failure requires a concurrent removal or IO race
				return pitfall.Corpus{}, fmt.Errorf("inspect pitfall source %s: %w", source, err)
			}
			regular = info.Mode().IsRegular()
		}
		b, ok, err := reader.ReadFile(source)
		if err != nil {
			return pitfall.Corpus{}, fmt.Errorf("read pitfall source %s: %w", source, err)
		}
		if !ok { // coverage-ignore: filesystem Lstat proved presence and snapshot enumeration includes only the same immutable tree
			regular = false
		}
		files = append(files, pitfall.SourceFile{Path: filepath.ToSlash(source), Bytes: b, Regular: regular})
	}
	return pitfall.Load(files)
}

func projectTreeReader(p renderInputs) ProjectTreeReader { return p.read }
