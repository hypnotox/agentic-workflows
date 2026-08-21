package main

import (
	"context"
	"testing"
)

func TestStagedContextStatePropagatesPreparationFailure(t *testing.T) {
	if _, err := stagedContextState(context.Background(), t.TempDir()); err == nil {
		t.Fatal("stagedContextState accepted a directory outside a repository")
	}
}
