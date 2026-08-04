package presentation

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

type badNode struct{}

func (badNode) presentationNode() {}

// invariant: code-design/presentation-ownership:closed-presentation-tree (TestPresentationTreeContract)
func TestPresentationTreeContract(t *testing.T) {
	assertPresentationSourceContract(t)
	v := func(s string) value {
		got, err := Prose(s)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	f, _ := NewField("field", v("value"))
	f.presentationNode()
	mustReject := func(name string, construct func() error) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			if err := construct(); err == nil {
				t.Fatal("invalid grammar transition accepted")
			}
		})
	}
	r1, _ := NewRecord(v(`left|right`), v(`slash\\`))
	r2, _ := NewRecord(v("two"), v("three"))
	group, _ := NewRecordGroup("records", []string{"first", "second"}, r1, r2)
	group.presentationNode()
	list, _ := NewList("items", v("one"), v("two"))
	list.presentationNode()
	steps, _ := NewSteps("steps", v("first"), v("second"))
	steps.presentationNode()
	inner, _ := NewSection("inner", f, list, group, steps)
	inner.presentationNode()
	r1.presentationNode()
	middle, _ := NewSection("middle", inner)
	outer, _ := NewSection("outer", middle)
	doc, err := NewDocument(f, outer)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Render(&out, doc); err != nil {
		t.Fatal(err)
	}
	const wantGolden = "field: value\n\nouter:\n  middle:\n    inner:\n      field: value\n      items:\n        one\n        two\n      records:\n        left\\|right | slash\\\\\\\\\n        two | three\n      steps:\n        step 1: first\n        step 2: second\n"
	if out.String() != wantGolden {
		t.Fatalf("presentation grammar:\n--- got ---\n%s--- want ---\n%s", out.String(), wantGolden)
	}

	// Document admits only ordered root Fields then Sections. Section admits
	// every non-Record node, including another Section, and rejects the lone
	// leaf node that has no label. These are the complete node-child matrix;
	// the remaining constructors below prove each leaf's scalar arity.
	t.Run("admitted node matrix", func(t *testing.T) {
		if _, err := NewDocument(f, outer); err != nil {
			t.Fatalf("document Field/Section matrix: %v", err)
		}
		if _, err := NewSection("matrix", f, inner, list, group, steps); err != nil {
			t.Fatalf("section Field/Section/List/RecordGroup/Steps matrix: %v", err)
		}
	})
	mustReject("document rejects list", func() error { _, err := NewDocument(list); return err })
	mustReject("document rejects record group", func() error { _, err := NewDocument(group); return err })
	mustReject("document rejects record", func() error { _, err := NewDocument(r1); return err })
	mustReject("document rejects steps", func() error { _, err := NewDocument(steps); return err })
	mustReject("section rejects record", func() error { _, err := NewSection("matrix", r1); return err })
	mustReject("section rejects unknown node", func() error { _, err := NewSection("matrix", badNode{}); return err })
	mustReject("list requires scalar leaf", func() error { _, err := NewList("items", value{}); return err })
	mustReject("steps requires scalar leaf", func() error { _, err := NewSteps("steps", value{}); return err })
	mustReject("record requires scalar fields", func() error { _, err := NewRecord(value{}); return err })
	mustReject("record group requires matching record arity", func() error { _, err := NewRecordGroup("records", []string{"one"}, r1); return err })
	for _, want := range []string{"field: value\n\nouter:", "items:\n        one", `left\|right | slash\\\\`} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Errorf("output missing %q: %s", want, out.String())
		}
	}
	for _, err := range []error{
		func() error { _, e := NewRecord(); return e }(),
		func() error { _, e := NewSection("Bad", f); return e }(),
		func() error { _, e := NewList("Bad", v("one")); return e }(),
		func() error { _, e := NewRecordGroup("Bad", []string{"one", "two"}, r1); return e }(),
		func() error { _, e := NewRecord(value{}); return e }(),
		func() error { _, e := NewDocument(badNode{}); return e }(),
		func() error { _, e := NewDocument(nil); return e }(),
		func() error { _, e := NewSection("section", nil); return e }(),
		func() error { _, e := NewSection("section", badNode{}); return e }(),
		func() error { _, e := NewDocument(Section{label: "section", nodes: []Node{badNode{}}}); return e }(),
		func() error { _, e := NewDocument(Field{label: "field", value: value{}}, outer); return e }(),
		func() error { _, e := NewSection("section", Field{label: "field", value: value{}}); return e }(),
		func() error { _, e := NewList("items", value{}); return e }(),
		func() error { _, e := NewRecordGroup("records", []string{"bad label!"}, r1); return e }(),
		func() error {
			_, e := NewRecordGroup("records", []string{"one"}, Record{values: []value{{}}})
			return e
		}(),
		func() error { _, e := NewSection("section", List{label: "items", values: nil}); return e }(),
		func() error {
			_, e := NewSection("section", RecordGroup{label: "records", schema: nil, records: nil})
			return e
		}(),
		func() error { _, e := NewSection("section", List{label: "items", values: []value{{}}}); return e }(),
		func() error {
			_, e := NewSection("section", RecordGroup{label: "records", schema: []string{"one"}, records: []Record{{values: []value{{}}}}})
			return e
		}(),
		func() error { _, e := NewRecordGroup("records", nil, r1); return e }(),
		func() error { _, e := NewRecordGroup("records", []string{"one"}, r1); return e }(),
		func() error { _, e := NewList("items"); return e }(),
		func() error { _, e := NewSection("section"); return e }(),
		func() error { _, e := NewDocument(outer, f); return e }(),
	} {
		if err == nil {
			t.Error("invalid shape accepted")
		}
	}
	tooDeep, _ := NewSection("one", f)
	for _, label := range []string{"two", "three"} {
		tooDeep, _ = NewSection(label, tooDeep)
	}
	if _, err := NewSection("four", tooDeep); err == nil {
		t.Error("fourth section level accepted")
	}
}

func assertPresentationSourceContract(t *testing.T) {
	t.Helper()
	files, fileSet := presentationSourceFiles(t, "")
	nodeTypes, boundary, err := presentationContract(fileSet, files)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Field", "List", "Record", "RecordGroup", "Section", "Steps", "nodeMarker"}; !equalStrings(nodeTypes, want) {
		t.Fatalf("presentation Node implementations = %v, want %v", nodeTypes, want)
	}
	if want := []string{"NewDocument", "NewSection", "Prompt", "Render", "validateDocument", "writeDocument", "writeNode"}; !equalStrings(boundary, want) {
		t.Fatalf("presentation boundary functions = %v, want %v", boundary, want)
	}

	// This fixture is deliberately name-opaque. Semantic type checking must find
	// both the promoted marker method from pointer embedding and a new Document
	// consumer without relying on a declaration or function name convention.
	files, fileSet = presentationSourceFiles(t, `
package presentation

type escapedNode struct{ *nodeMarker }

func FormatPresentation(document Document) {}
`)
	nodeTypes, boundary, err = presentationContract(fileSet, files)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(nodeTypes, "escapedNode") || !containsString(boundary, "FormatPresentation") {
		t.Fatalf("semantic fixture escaped detection: nodes=%v boundary=%v", nodeTypes, boundary)
	}
}

func presentationSourceFiles(t *testing.T, fixture string) ([]*ast.File, *token.FileSet) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate presentation source")
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(file), "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	var files []*ast.File
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, parsed)
	}
	if fixture != "" {
		parsed, err := parser.ParseFile(fileSet, "contract_fixture.go", fixture, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, parsed)
	}
	return files, fileSet
}

func presentationContract(fileSet *token.FileSet, files []*ast.File) ([]string, []string, error) {
	info := &types.Info{Defs: make(map[*ast.Ident]types.Object)}
	config := types.Config{Importer: importer.Default()}
	pkg, err := config.Check("github.com/hypnotox/agentic-workflows/internal/presentation", fileSet, files, info)
	if err != nil {
		return nil, nil, err
	}
	document := pkg.Scope().Lookup("Document").Type()
	nodeType := pkg.Scope().Lookup("Node").Type()
	node, ok := nodeType.Underlying().(*types.Interface)
	if !ok {
		return nil, nil, errors.New("Node is not an interface")
	}
	node.Complete()
	var nodeTypes, boundary []string
	for _, name := range pkg.Scope().Names() {
		object, ok := pkg.Scope().Lookup(name).(*types.TypeName)
		if !ok || object.IsAlias() {
			continue
		}
		named, ok := object.Type().(*types.Named)
		if !ok {
			continue
		}
		if _, isInterface := named.Underlying().(*types.Interface); !isInterface && (types.Implements(named, node) || types.Implements(types.NewPointer(named), node)) {
			nodeTypes = append(nodeTypes, object.Name())
		}
	}
	for _, source := range files {
		for _, declaration := range source.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Name.Name == "presentationNode" {
				continue
			}
			object, ok := info.Defs[function.Name].(*types.Func)
			if !ok {
				return nil, nil, fmt.Errorf("resolve function %s", function.Name.Name)
			}
			signature := object.Type().(*types.Signature)
			if signatureConsumesPresentation(signature, document, nodeType, node) {
				boundary = append(boundary, object.Name())
			}
		}
	}
	sort.Strings(nodeTypes)
	sort.Strings(boundary)
	return nodeTypes, boundary, nil
}

func signatureConsumesPresentation(signature *types.Signature, document, nodeType types.Type, node *types.Interface) bool {
	if receiver := signature.Recv(); receiver != nil && (presentationOperand(receiver.Type(), document, nodeType) || types.Implements(receiver.Type(), node)) {
		return true
	}
	for i := range signature.Params().Len() {
		if presentationOperand(signature.Params().At(i).Type(), document, nodeType) {
			return true
		}
	}
	return false
}

// presentationOperand recognizes the boundary's generic Document and Node
// operands, not concrete node internals such as []Field used by validators.
func presentationOperand(typ, document, nodeType types.Type) bool {
	if types.Identical(typ, document) || types.Identical(typ, nodeType) {
		return true
	}
	switch typ := typ.(type) {
	case *types.Pointer:
		return presentationOperand(typ.Elem(), document, nodeType)
	case *types.Slice:
		return presentationOperand(typ.Elem(), document, nodeType)
	case *types.Array:
		return presentationOperand(typ.Elem(), document, nodeType)
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
