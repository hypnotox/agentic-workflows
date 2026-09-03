package publisher

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/render"
)

func collectVars(fsys fs.FS, path string, varSet map[string]bool) error {
	src, err := fs.ReadFile(fsys, path)
	if err != nil {
		return fmt.Errorf("scaffold: read template %s: %w", path, err)
	}
	for _, value := range render.ReferencedVars(string(src)) {
		varSet[value] = true
	}
	return nil
}

func artifactLabel(tid string) string {
	segs := strings.Split(tid, "/")
	switch segs[0] {
	case "skills", "docs":
		name := segs[1]
		if segs[0] != "skills" {
			name = strings.TrimSuffix(name, ".md.tmpl")
		}
		return strings.TrimSuffix(segs[0], "s") + " " + name
	case "hooks":
		return "hooks " + strings.TrimSuffix(segs[1], ".sh.tmpl")
	default:
		return segs[0]
	}
}
