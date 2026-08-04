package presentation

import (
	"bytes"
	"testing"
)

type badNode struct{}

func (badNode) presentationNode() {}

func TestPresentationTreeShapes(t *testing.T) {
	v := func(s string) value {
		got, err := Prose(s)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	f, _ := NewField("field", v("value"))
	f.presentationNode()
	r1, _ := NewRecord(v(`left|right`), v(`slash\\`))
	r2, _ := NewRecord(v("two"), v("three"))
	group, _ := NewRecordGroup("records", []string{"first", "second"}, r1, r2)
	group.presentationNode()
	list, _ := NewList("items", v("one"), v("two"))
	list.presentationNode()
	inner, _ := NewSection("inner", f, list, group)
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
	const wantGolden = "field: value\n\nouter:\n  middle:\n    inner:\n      field: value\n      items:\n        one\n        two\n      records:\n        left\\|right | slash\\\\\\\\\n        two | three\n"
	if out.String() != wantGolden {
		t.Fatalf("presentation grammar:\n--- got ---\n%s--- want ---\n%s", out.String(), wantGolden)
	}
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
