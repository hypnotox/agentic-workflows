package contextq

import (
	"fmt"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/contextinput"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

func benchmarkContextQuery(b *testing.B, size int) *Query {
	b.Helper()
	files := make([]snapshot.File, size)
	for i := range files {
		files[i] = snapshot.File{Path: fmt.Sprintf("internal/bench/file-%04d.go", i), Mode: snapshot.Regular}
	}
	tree, err := snapshot.NewTree(files)
	if err != nil {
		b.Fatal(err)
	}
	return New(contextinput.New(contextinput.Layout{}, currentstate.Loaded{}, contextinput.PlanContext{}, tree, nil, nil, nil, nil))
}

// BenchmarkContextProjectionExact measures real query/result assembly for one
// requested file across representative complete inventories.
func BenchmarkContextProjectionExact(b *testing.B) {
	for _, size := range []int{32, 512} {
		b.Run(fmt.Sprintf("files-%d", size), func(b *testing.B) {
			q := benchmarkContextQuery(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				_ = q.ContextForOptions([]string{"internal/bench/file-0000.go"}, ContextOptions{Selection: SelectionExplicit})
			}
		})
	}
}

// BenchmarkContextProjectionDirectory measures real query/result assembly for
// a directory request across representative complete inventories.
func BenchmarkContextProjectionDirectory(b *testing.B) {
	for _, size := range []int{32, 512} {
		b.Run(fmt.Sprintf("files-%d", size), func(b *testing.B) {
			q := benchmarkContextQuery(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				_ = q.ContextForOptions([]string{"internal/bench"}, ContextOptions{Selection: SelectionExplicit})
			}
		})
	}
}
