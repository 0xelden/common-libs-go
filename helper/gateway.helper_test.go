package helper

import (
	"fmt"
	"testing"

	sq "github.com/Masterminds/squirrel"
)

type testStringer struct {
	value string
}

func (s testStringer) String() string {
	return "stringer:" + s.value
}

func TestIsNil(t *testing.T) {
	type args struct {
		value interface{}
	}
	var emptySqlizer sq.Sqlizer
	tests := []struct {
		name    string
		args    args
		wantRes bool
	}{
		{
			name:    "test for empty sq.Sqlizer",
			args:    args{emptySqlizer},
			wantRes: true,
		},
		{
			name:    "test for non empty sq.Sqlizer",
			args:    args{sq.Eq{"foo": "bar"}},
			wantRes: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotRes := IsNil(tt.args.value); gotRes != tt.wantRes {
				t.Errorf("IsNil() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}

func TestStringToInt(t *testing.T) {
	type args struct {
		value string
		def   int64
	}
	tests := []struct {
		args args
		want int64
	}{
		{
			args: args{
				value: "001",
				def:   0,
			},
			want: 1,
		},
		{
			args: args{
				value: "010",
				def:   0,
			},
			want: 10,
		},
		{
			args: args{
				value: "-",
				def:   10,
			},
			want: 10,
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			if got := StringToInt(tt.args.value, tt.args.def); got != tt.want {
				t.Errorf("StringToInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToString(t *testing.T) {
	type args struct {
		value interface{}
	}
	rawString := "hello"
	type sample struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	tests := []struct {
		name    string
		args    args
		wantRes string
	}{
		{
			name:    "test for nil value",
			args:    args{value: nil},
			wantRes: "",
		},
		{
			name:    "test for string value",
			args:    args{value: "plain string"},
			wantRes: "plain string",
		},
		{
			name:    "test for string pointer value",
			args:    args{value: &rawString},
			wantRes: "hello",
		},
		{
			name:    "test for fmt.Stringer value",
			args:    args{value: testStringer{value: "case"}},
			wantRes: "stringer:case",
		},
		{
			name:    "test for struct value",
			args:    args{value: sample{Name: "john", Age: 30}},
			wantRes: `{"name":"john","age":30}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotRes := ToString(tt.args.value); gotRes != tt.wantRes {
				t.Errorf("ToString() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}
