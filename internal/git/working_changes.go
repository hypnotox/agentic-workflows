package git

import (
	"bytes"
	"context"
	"fmt"
)

// WorktreeChangedPaths returns the sorted, unique paths changed from HEAD to
// the current working tree, including staged, unstaged, deleted, renamed, and
// nonignored untracked paths. Renames contribute both the previous and current
// names so consumers can conservatively account for deleted package ownership.
// Native porcelain v2 is used because it is the repository's normalized
// working-tree evidence and transports pathnames as terminal-NUL records.
func (r *Repo) WorktreeChangedPaths(ctx context.Context) ([]string, error) {
	argv := append(r.runner.excludesFileArgs(ctx), "--no-optional-locks", "status", "--porcelain=v2", "-z", "--untracked-files=all")
	out, err := r.runner.run(ctx, argv...)
	if err != nil {
		return nil, fmt.Errorf("read native Git worktree changes: %w", err)
	}
	paths, err := parseWorktreeChangedPaths(out)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, path := range paths {
		if path, ok := rerootPath(path, r.prefix); ok && path != "" {
			set[path] = true
		}
	}
	return sortedPaths(set), nil
}

func parseWorktreeChangedPaths(out []byte) ([]string, error) {
	paths := []string{}
	for len(out) > 0 {
		end := bytes.IndexByte(out, 0)
		if end < 0 {
			return nil, fmt.Errorf("parse native Git worktree changes: unterminated record")
		}
		record := out[:end]
		out = out[end+1:]
		if len(record) < 3 || record[1] != ' ' {
			return nil, fmt.Errorf("parse native Git worktree changes: malformed record")
		}
		var skip int
		rename := false
		switch record[0] {
		case '1':
			skip = 8
		case '2':
			skip, rename = 9, true
		case 'u':
			skip = 10
		case '?':
			skip = 1
		default:
			return nil, fmt.Errorf("parse native Git worktree changes: unknown record type %q", record[0])
		}
		path, ok := afterFields(record, skip)
		if !ok || len(path) == 0 {
			return nil, fmt.Errorf("parse native Git worktree changes: malformed path record")
		}
		paths = append(paths, string(path))
		if rename {
			end := bytes.IndexByte(out, 0)
			if end <= 0 {
				return nil, fmt.Errorf("parse native Git worktree changes: rename missing original path")
			}
			paths = append(paths, string(out[:end]))
			out = out[end+1:]
		}
	}
	return paths, nil
}

func afterFields(record []byte, fields int) ([]byte, bool) {
	for range fields {
		space := bytes.IndexByte(record, ' ')
		if space < 0 {
			return nil, false
		}
		record = record[space+1:]
	}
	return record, true
}
