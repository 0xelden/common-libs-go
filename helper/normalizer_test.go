package helper_test

import (
	"testing"
	"time"

	"github.com/0xelden/common-libs-go/helper"
)

func TestNormalizeMobileNumber(t *testing.T) {
	type args struct {
		mobile      string
		countryCode *string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			args: args{
				mobile:      "08123",
				countryCode: nil,
			},
			want: "+628123",
		},
		{
			args: args{
				mobile:      "+638123",
				countryCode: nil,
			},
			want: "+638123",
		},
		{
			args: args{
				mobile:      "0123",
				countryCode: helper.Ptr("+123"),
			},
			want: "+123123",
		},
		{
			args: args{
				mobile:      "+6208123",
				countryCode: nil,
			},
			want: "+628123",
		},
		{
			args: args{
				mobile:      "+62 081-23",
				countryCode: nil,
			},
			want: "+628123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := helper.NormalizeMobileNumber(tt.args.mobile, tt.args.countryCode); got != tt.want {
				t.Errorf("NormalizeMobileNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStripCommonAuditFields_FullStruct(t *testing.T) {
	type Foo struct {
		Status    *int
		CreatedBy *string
		CreatedAt *time.Time
		UpdatedBy *string
		UpdatedAt *time.Time
		Other     string
	}

	i := 123
	cb := "creator"
	ub := "updater"
	now := time.Now()

	f := &Foo{
		Status:    &i,
		CreatedBy: &cb,
		CreatedAt: &now,
		UpdatedBy: &ub,
		UpdatedAt: &now,
		Other:     "keep me",
	}

	helper.StripCommonAuditFields(f)

	if f.Status != nil {
		t.Errorf("Status not nil, got %v", *f.Status)
	}
	if f.CreatedBy != nil {
		t.Errorf("CreatedBy not nil, got %v", *f.CreatedBy)
	}
	if f.CreatedAt != nil {
		t.Errorf("CreatedAt not nil, got %v", *f.CreatedAt)
	}
	if f.UpdatedBy != nil {
		t.Errorf("UpdatedBy not nil, got %v", *f.UpdatedBy)
	}
	if f.UpdatedAt != nil {
		t.Errorf("UpdatedAt not nil, got %v", *f.UpdatedAt)
	}

	if f.Other != "keep me" {
		t.Errorf("Other modified, want %q got %q", "keep me", f.Other)
	}
}

func TestStripCommonAuditFields_PartialFields(t *testing.T) {
	type Foo struct {
		Status *int
		Other  int
	}

	i := 10
	f := &Foo{
		Status: &i,
		Other:  42,
	}

	helper.StripCommonAuditFields(f)

	if f.Status != nil {
		t.Errorf("Status not nil, got %v", *f.Status)
	}
	if f.Other != 42 {
		t.Errorf("Other modified, want 42 got %d", f.Other)
	}
}

func TestStripCommonAuditFields_NoAuditFields(t *testing.T) {
	type Foo struct {
		Other int
	}

	f := &Foo{Other: 7}

	helper.StripCommonAuditFields(f)

	if f.Other != 7 {
		t.Errorf("Other modified, want 7 got %d", f.Other)
	}
}

func TestStripCommonAuditFields_NilPointer(t *testing.T) {
	type Foo struct {
		Status *int
	}

	var f *Foo // nil

	// should not panic
	helper.StripCommonAuditFields(f)
}

func TestStripCommonAuditFields_NonStructIgnored(t *testing.T) {
	// T is non-struct; function should just return without panic
	i := 5
	helper.StripCommonAuditFields(&i)

	if i != 5 {
		t.Errorf("value modified, want 5 got %d", i)
	}
}

func TestStripCommonAuditFieldsSlice_Basic(t *testing.T) {
	type Foo struct {
		Status    *int
		CreatedBy *string
		CreatedAt *time.Time
		UpdatedBy *string
		UpdatedAt *time.Time
		Other     string
	}

	i1, i2 := 1, 2
	cb1, cb2 := "a", "b"
	ub1, ub2 := "x", "y"
	now1 := time.Now()
	now2 := now1.Add(time.Minute)

	items := []Foo{
		{
			Status:    &i1,
			CreatedBy: &cb1,
			CreatedAt: &now1,
			UpdatedBy: &ub1,
			UpdatedAt: &now1,
			Other:     "keep-1",
		},
		{
			Status:    &i2,
			CreatedBy: &cb2,
			CreatedAt: &now2,
			UpdatedBy: &ub2,
			UpdatedAt: &now2,
			Other:     "keep-2",
		},
	}

	helper.StripCommonAuditFieldsSlice(items)

	for idx, it := range items {
		if it.Status != nil {
			t.Fatalf("items[%d].Status not nil", idx)
		}
		if it.CreatedBy != nil {
			t.Fatalf("items[%d].CreatedBy not nil", idx)
		}
		if it.CreatedAt != nil {
			t.Fatalf("items[%d].CreatedAt not nil", idx)
		}
		if it.UpdatedBy != nil {
			t.Fatalf("items[%d].UpdatedBy not nil", idx)
		}
		if it.UpdatedAt != nil {
			t.Fatalf("items[%d].UpdatedAt not nil", idx)
		}
	}

	if items[0].Other != "keep-1" {
		t.Fatalf("items[0].Other modified, got %q", items[0].Other)
	}
	if items[1].Other != "keep-2" {
		t.Fatalf("items[1].Other modified, got %q", items[1].Other)
	}
}

func TestStripCommonAuditFieldsSlice_Empty(t *testing.T) {
	type Foo struct {
		Status *int
		Other  int
	}

	var items []Foo

	// should be no panic, no-op
	helper.StripCommonAuditFieldsSlice(items)
}

func TestStripCommonAuditFieldsSlice_NonStructElement(t *testing.T) {
	// elements are int; underlying StripCommonAuditFields should no-op
	items := []int{1, 2, 3}
	helper.StripCommonAuditFieldsSlice(items)

	if len(items) != 3 || items[0] != 1 || items[1] != 2 || items[2] != 3 {
		t.Fatalf("slice modified unexpectedly: %#v", items)
	}
}

type AuditFields struct {
	Status    *int
	CreatedBy *string
	CreatedAt *time.Time
	UpdatedBy *string
	UpdatedAt *time.Time
}

func TestStripCommonAuditFields_TopLevel(t *testing.T) {
	type Foo struct {
		Status    *int
		CreatedBy *string
		CreatedAt *time.Time
		UpdatedBy *string
		UpdatedAt *time.Time
		Other     string
	}

	i := 123
	cb := "creator"
	ub := "updater"
	now := time.Now()

	f := &Foo{
		Status:    &i,
		CreatedBy: &cb,
		CreatedAt: &now,
		UpdatedBy: &ub,
		UpdatedAt: &now,
		Other:     "keep",
	}

	helper.StripCommonAuditFields(f)

	if f.Status != nil {
		t.Fatalf("Status not nil")
	}
	if f.CreatedBy != nil {
		t.Fatalf("CreatedBy not nil")
	}
	if f.CreatedAt != nil {
		t.Fatalf("CreatedAt not nil")
	}
	if f.UpdatedBy != nil {
		t.Fatalf("UpdatedBy not nil")
	}
	if f.UpdatedAt != nil {
		t.Fatalf("UpdatedAt not nil")
	}
	if f.Other != "keep" {
		t.Fatalf("Other modified, got %q", f.Other)
	}
}

// ---------- deep embedded: anonymous embedded structs ----------
func TestStripCommonAuditFields_DeepEmbedded_Anonymous(t *testing.T) {
	type Base struct {
		AuditFields
		Meta string
	}

	type Outer struct {
		Base
		Tag string
	}

	i := 10
	cb := "creator"
	ub := "updater"
	now := time.Now()

	o := &Outer{
		Base: Base{
			AuditFields: AuditFields{
				Status:    &i,
				CreatedBy: &cb,
				CreatedAt: &now,
				UpdatedBy: &ub,
				UpdatedAt: &now,
			},
			Meta: "meta",
		},
		Tag: "tag",
	}

	helper.StripCommonAuditFields(o)

	if o.Status != nil {
		t.Fatalf("Status not nil on embedded AuditFields")
	}
	if o.CreatedBy != nil {
		t.Fatalf("CreatedBy not nil on embedded AuditFields")
	}
	if o.CreatedAt != nil {
		t.Fatalf("CreatedAt not nil on embedded AuditFields")
	}
	if o.UpdatedBy != nil {
		t.Fatalf("UpdatedBy not nil on embedded AuditFields")
	}
	if o.UpdatedAt != nil {
		t.Fatalf("UpdatedAt not nil on embedded AuditFields")
	}

	if o.Meta != "meta" {
		t.Fatalf("Meta modified, got %q", o.Meta)
	}
	if o.Tag != "tag" {
		t.Fatalf("Tag modified, got %q", o.Tag)
	}
}

// ---------- deep embedded: named nested struct field ----------
func TestStripCommonAuditFields_DeepEmbedded_Named(t *testing.T) {
	type Inner struct {
		AuditFields
		Note string
	}

	type Outer struct {
		Name  string
		Inner Inner
	}

	i := 99
	cb := "x"
	ub := "y"
	now := time.Now()

	o := &Outer{
		Name: "outer",
		Inner: Inner{
			AuditFields: AuditFields{
				Status:    &i,
				CreatedBy: &cb,
				CreatedAt: &now,
				UpdatedBy: &ub,
				UpdatedAt: &now,
			},
			Note: "inner-note",
		},
	}

	helper.StripCommonAuditFields(o)

	if o.Inner.Status != nil {
		t.Fatalf("Inner.Status not nil")
	}
	if o.Inner.CreatedBy != nil {
		t.Fatalf("Inner.CreatedBy not nil")
	}
	if o.Inner.CreatedAt != nil {
		t.Fatalf("Inner.CreatedAt not nil")
	}
	if o.Inner.UpdatedBy != nil {
		t.Fatalf("Inner.UpdatedBy not nil")
	}
	if o.Inner.UpdatedAt != nil {
		t.Fatalf("Inner.UpdatedAt not nil")
	}

	if o.Name != "outer" {
		t.Fatalf("Name modified, got %q", o.Name)
	}
	if o.Inner.Note != "inner-note" {
		t.Fatalf("Inner.Note modified, got %q", o.Inner.Note)
	}
}

// ---------- pointer-to-struct nested field ----------
func TestStripCommonAuditFields_DeepEmbedded_PointerNested(t *testing.T) {
	type Inner struct {
		AuditFields
	}

	type Outer struct {
		Inner *Inner
		Flag  bool
	}

	i := 7
	cb := "cb"
	ub := "ub"
	now := time.Now()

	o := &Outer{
		Inner: &Inner{
			AuditFields: AuditFields{
				Status:    &i,
				CreatedBy: &cb,
				CreatedAt: &now,
				UpdatedBy: &ub,
				UpdatedAt: &now,
			},
		},
		Flag: true,
	}

	helper.StripCommonAuditFields(o)

	if o.Inner.Status != nil ||
		o.Inner.CreatedBy != nil ||
		o.Inner.CreatedAt != nil ||
		o.Inner.UpdatedBy != nil ||
		o.Inner.UpdatedAt != nil {
		t.Fatalf("audit fields on pointer-nested Inner not fully cleared")
	}

	if !o.Flag {
		t.Fatalf("Flag modified, expected true")
	}
}

// ---------- slice variant: deep embedded ----------
func TestStripCommonAuditFieldsSlice_DeepEmbedded(t *testing.T) {
	type Base struct {
		AuditFields
		Code string
	}
	type Outer struct {
		Base
		Idx int
	}

	i1, i2 := 1, 2
	cb1, cb2 := "a", "b"
	ub1, ub2 := "x", "y"
	now := time.Now()

	items := []Outer{
		{
			Base: Base{
				AuditFields: AuditFields{
					Status:    &i1,
					CreatedBy: &cb1,
					CreatedAt: &now,
					UpdatedBy: &ub1,
					UpdatedAt: &now,
				},
				Code: "c1",
			},
			Idx: 1,
		},
		{
			Base: Base{
				AuditFields: AuditFields{
					Status:    &i2,
					CreatedBy: &cb2,
					CreatedAt: &now,
					UpdatedBy: &ub2,
					UpdatedAt: &now,
				},
				Code: "c2",
			},
			Idx: 2,
		},
	}

	helper.StripCommonAuditFieldsSlice(items)

	for idx, it := range items {
		if it.Status != nil ||
			it.CreatedBy != nil ||
			it.CreatedAt != nil ||
			it.UpdatedBy != nil ||
			it.UpdatedAt != nil {
			t.Fatalf("items[%d] embedded audit fields not cleared", idx)
		}
	}

	if items[0].Code != "c1" || items[1].Code != "c2" {
		t.Fatalf("Code modified: %#v", items)
	}
	if items[0].Idx != 1 || items[1].Idx != 2 {
		t.Fatalf("Idx modified: %#v", items)
	}
}

func strPtr(s string) *string { return &s }

type simple struct {
	Status    *string
	CreatedBy *string
	CreatedAt *time.Time
	UpdatedBy *string
	UpdatedAt *time.Time
	Name      *string
}

type nestedChild struct {
	simple
	Extra *string
}

type nestedParent struct {
	Child nestedChild
}

type cyclicNode struct {
	Status *string
	Next   *cyclicNode
}

func TestStripCommonAuditFields_Simple(t *testing.T) {
	now := time.Now()
	s := simple{
		Status:    strPtr("ACTIVE"),
		CreatedBy: strPtr("user1"),
		CreatedAt: &now,
		UpdatedBy: strPtr("user2"),
		UpdatedAt: &now,
		Name:      strPtr("keep-me"),
	}

	helper.StripCommonAuditFields(&s)

	if s.Status != nil {
		t.Fatalf("Status not cleared, got %v", *s.Status)
	}
	if s.CreatedBy != nil {
		t.Fatalf("CreatedBy not cleared, got %v", *s.CreatedBy)
	}
	if s.CreatedAt != nil {
		t.Fatalf("CreatedAt not cleared, got %v", *s.CreatedAt)
	}
	if s.UpdatedBy != nil {
		t.Fatalf("UpdatedBy not cleared, got %v", *s.UpdatedBy)
	}
	if s.UpdatedAt != nil {
		t.Fatalf("UpdatedAt not cleared, got %v", *s.UpdatedAt)
	}
	if s.Name == nil || *s.Name != "keep-me" {
		t.Fatalf("Name should be preserved, got %v", s.Name)
	}
}

func TestStripCommonAuditFields_Nested(t *testing.T) {
	now := time.Now()
	p := nestedParent{
		Child: nestedChild{
			simple: simple{
				Status:    strPtr("ACTIVE"),
				CreatedBy: strPtr("nested-user"),
				CreatedAt: &now,
				Name:      strPtr("nested-name"),
			},
			Extra: strPtr("extra"),
		},
	}

	helper.StripCommonAuditFields(&p)

	if p.Child.Status != nil {
		t.Fatalf("nested Status not cleared")
	}
	if p.Child.CreatedBy != nil {
		t.Fatalf("nested CreatedBy not cleared")
	}
	if p.Child.CreatedAt != nil {
		t.Fatalf("nested CreatedAt not cleared")
	}
	if p.Child.Name == nil || *p.Child.Name != "nested-name" {
		t.Fatalf("nested Name should be preserved, got %v", p.Child.Name)
	}
	if p.Child.Extra == nil || *p.Child.Extra != "extra" {
		t.Fatalf("Extra should be preserved, got %v", p.Child.Extra)
	}
}

func TestStripCommonAuditFields_Slice(t *testing.T) {
	items := []simple{
		{Status: strPtr("A"), Name: strPtr("n1")},
		{Status: strPtr("B"), Name: strPtr("n2")},
	}

	helper.StripCommonAuditFieldsSlice(items)

	for i, it := range items {
		if it.Status != nil {
			t.Fatalf("item %d Status not cleared", i)
		}
		if it.Name == nil {
			t.Fatalf("item %d Name should be preserved", i)
		}
	}
}

func TestStripCommonAuditFields_NilPtr_NoPanic(t *testing.T) {
	var s *simple
	// should be a no-op
	helper.StripCommonAuditFields(s)
}

func TestStripCommonAuditFields_CyclicGraph_NoInfiniteRecursion(t *testing.T) {
	s1 := strPtr("S1")
	s2 := strPtr("S2")

	n1 := &cyclicNode{Status: s1}
	n2 := &cyclicNode{Status: s2}

	// create cycle: n1 -> n2 -> n1
	n1.Next = n2
	n2.Next = n1

	// if cycle handling is broken this will stack overflow
	helper.StripCommonAuditFields(n1)

	if n1.Status != nil {
		t.Fatalf("n1.Status not cleared")
	}
	if n2.Status != nil {
		t.Fatalf("n2.Status not cleared")
	}
}

func TestStripCommonAuditFields_PointerToNestedStruct(t *testing.T) {
	now := time.Now()

	type Wrapper struct {
		Inner *simple
	}

	w := Wrapper{
		Inner: &simple{
			Status:    strPtr("ACTIVE"),
			CreatedBy: strPtr("user"),
			CreatedAt: &now,
			Name:      strPtr("inner-name"),
		},
	}

	helper.StripCommonAuditFields(&w)

	if w.Inner == nil {
		t.Fatalf("Inner should not be nil")
	}
	if w.Inner.Status != nil || w.Inner.CreatedBy != nil || w.Inner.CreatedAt != nil {
		t.Fatalf("Inner audit fields not cleared")
	}
	if w.Inner.Name == nil || *w.Inner.Name != "inner-name" {
		t.Fatalf("Inner.Name should be preserved, got %v", w.Inner.Name)
	}
}

func TestStripCommonAuditFields_WithOptionalColumns(t *testing.T) {
	type Foo struct {
		Status    *int
		DeletedBy *string
		DeletedAt *time.Time
		Note      string
	}

	status := 1
	deletedBy := "deleter"
	now := time.Now()

	f := &Foo{
		Status:    &status,
		DeletedBy: &deletedBy,
		DeletedAt: &now,
		Note:      "keep",
	}

	helper.StripCommonAuditFields(f, "deleted_by", "DeletedAt")

	if f.Status != nil {
		t.Fatalf("Status not nil")
	}
	if f.DeletedBy != nil {
		t.Fatalf("DeletedBy not nil")
	}
	if f.DeletedAt != nil {
		t.Fatalf("DeletedAt not nil")
	}
	if f.Note != "keep" {
		t.Fatalf("Note modified, got %q", f.Note)
	}
}

func TestStripCommonAuditFieldsSlice_WithOptionalColumns(t *testing.T) {
	type Foo struct {
		DeletedBy *string
		Name      string
	}

	d1 := "a"
	d2 := "b"
	items := []Foo{
		{DeletedBy: &d1, Name: "first"},
		{DeletedBy: &d2, Name: "second"},
	}

	helper.StripCommonAuditFieldsSlice(items, "deleted_by")

	for i := range items {
		if items[i].DeletedBy != nil {
			t.Fatalf("items[%d].DeletedBy not nil", i)
		}
	}

	if items[0].Name != "first" || items[1].Name != "second" {
		t.Fatalf("Name modified: %#v", items)
	}
}

func TestToPascalCase(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty string",
			args: args{name: ""},
			want: "",
		},
		{
			name: "whitespace only",
			args: args{name: "   \t\n  "},
			want: "",
		},
		{
			name: "single word lowercase",
			args: args{name: "hello"},
			want: "Hello",
		},
		{
			name: "single word uppercase preserved except first char rule",
			args: args{name: "HELLO"},
			want: "HELLO",
		},
		{
			name: "single word mixed case without underscore preserves tail",
			args: args{name: "hELLo"},
			want: "HELLo",
		},
		{
			name: "trim surrounding spaces",
			args: args{name: "  hello  "},
			want: "Hello",
		},
		{
			name: "snake case lowercase",
			args: args{name: "hello_world"},
			want: "HelloWorld",
		},
		{
			name: "snake case uppercase converts remaining letters to lower",
			args: args{name: "HELLO_WORLD"},
			want: "HelloWorld",
		},
		{
			name: "snake case mixed case normalizes each segment",
			args: args{name: "hELLo_wORLd"},
			want: "HelloWorld",
		},
		{
			name: "snake case with surrounding spaces on input and segments",
			args: args{name: "  first_name  "},
			want: "FirstName",
		},
		{
			name: "multiple underscores skip empty segments",
			args: args{name: "first__name"},
			want: "FirstName",
		},
		{
			name: "leading and trailing underscores",
			args: args{name: "_first_name_"},
			want: "FirstName",
		},
		{
			name: "single underscore only",
			args: args{name: "_"},
			want: "",
		},
		{
			name: "preserve digits in segments",
			args: args{name: "field_2_name"},
			want: "Field2Name",
		},
		{
			name: "no underscore with internal spaces preserves tail",
			args: args{name: "hello world"},
			want: "Hello world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := helper.ToPascalCase(tt.args.name); got != tt.want {
				t.Errorf("ToPascalCase() = %v, want %v", got, tt.want)
			}
		})
	}
}
