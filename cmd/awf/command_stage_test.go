package main

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"
	"time"
)

func TestCommandStagesUseIndependentDeadlines(t *testing.T) {
	first, cancelFirst := newGitCommandContext()
	second, cancelSecond := newGitCommandContext()
	defer cancelSecond()
	firstDeadline, firstOK := first.Deadline()
	secondDeadline, secondOK := second.Deadline()
	if !firstOK || !secondOK {
		t.Fatalf("stage deadlines present = %v, %v", firstOK, secondOK)
	}
	for name, deadline := range map[string]time.Time{"first": firstDeadline, "second": secondDeadline} {
		remaining := time.Until(deadline)
		if remaining < gitCommandTimeout-time.Second || remaining > gitCommandTimeout {
			t.Fatalf("%s stage deadline remaining = %v, want approximately %v", name, remaining, gitCommandTimeout)
		}
	}
	cancelFirst()
	if !errors.Is(first.Err(), context.Canceled) || second.Err() != nil {
		t.Fatalf("stage cancellation leaked: first=%v second=%v", first.Err(), second.Err())
	}

	fset := token.NewFileSet()
	src, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	for _, decl := range src.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "run" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "newGitCommandContext" {
				calls++
			}
			return true
		})
	}
	if calls != 3 {
		t.Fatalf("run creates %d stage contexts, want guard, gate, and handler", calls)
	}
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	for stage, wiring := range map[string]string{
		"guard":   "guardProjectState(guardCtx,",
		"gate":    "gateFn(gateCtx,",
		"handler": "ctx: handlerCtx,",
	} {
		if !bytes.Contains(raw, []byte(wiring)) {
			t.Errorf("%s stage is not wired to its own context variable %q", stage, wiring)
		}
	}
}
