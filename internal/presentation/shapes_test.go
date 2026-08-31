package presentation

import (
	"bytes"
	"testing"
)

type badNode struct{}

func (badNode) presentationNode() {}

func TestCollectionDocumentValidationAndEmptyResult(t *testing.T) {
	value, err := Prose("value")
	if err != nil {
		t.Fatal(err)
	}
	record, err := NewRecord(value)
	if err != nil {
		t.Fatal(err)
	}
	for _, collection := range []Collection{
		{Status: " \n\t"},
		{Status: "ready", Categories: []CollectionCategory{{Label: "Bad", Schema: []string{"value"}, Records: []Record{record}}}},
		{Status: "ready", Categories: []CollectionCategory{{Label: "values", Values: []Value{{}}}}},
		{Status: "ready", Categories: []CollectionCategory{{Label: "values", Schema: []string{"value"}, Values: []Value{value}}}},
	} {
		if _, err := collection.Document(); err == nil {
			t.Fatal("invalid collection accepted")
		}
	}
	document, err := (Collection{Status: "none"}).Document()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := Render(&output, document); err != nil {
		t.Fatal(err)
	}
	if output.String() != "status: none\n" {
		t.Fatalf("empty collection = %q", output.String())
	}
}

func TestReportDocumentRejectsConstructibleZeroFields(t *testing.T) {
	for _, report := range []Report{{Context: []Field{{}}}, {Summary: []Field{{}}}} {
		if _, err := report.Document(); err == nil {
			t.Fatal("report accepted a constructible zero Field")
		}
	}
}

// invariant: code-design/presentation-ownership:closed-presentation-tree (TestPresentationTreeContract)
func TestPresentationTreeContract(t *testing.T) {
	v := func(s string) value {
		got, err := Prose(s)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	f, _ := NewField("field", v("value"))
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
	list, _ := NewList("items", v("one"), v("two"))
	steps, _ := NewSteps("steps", v("first"), v("second"))
	inner, _ := NewSection("inner", f, list, group, steps)
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
		func() error { _, e := NewSteps("Bad", v("one")); return e }(),
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
