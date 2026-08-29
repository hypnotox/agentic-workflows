package main

import (
	"context"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func testContext(t *testing.T) context.Context { return testsupport.Context(t) }
