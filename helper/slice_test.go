package helper

import (
	"reflect"
	"testing"
)

func TestRemoveAt_Ints(t *testing.T) {
	type TC struct {
		name string
		in   []int
		idx  int
		want []int
		ok   bool
	}
	cases := []TC{
		{"middle", []int{10, 20, 30, 40, 50}, 2, []int{10, 20, 40, 50}, true},
		{"first", []int{1, 2, 3}, 0, []int{2, 3}, true},
		{"last", []int{1, 2, 3}, 2, []int{1, 2}, true},
		{"negative idx", []int{1, 2, 3}, -1, []int{1, 2, 3}, false},
		{"idx == len", []int{1, 2, 3}, 3, []int{1, 2, 3}, false},
		{"empty", []int{}, 0, []int{}, false},
		{"single ok", []int{7}, 0, []int{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := RemoveAt(tc.in, tc.idx)
			if ok != tc.ok || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("RemoveAt(%v,%d) = (%v,%v), want (%v,%v)",
					tc.in, tc.idx, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestRemoveAt_Variadic(t *testing.T) {
	type TC struct {
		name string
		in   []int
		idx  []int
		want []int
		ok   bool
	}
	cases := []TC{
		{"remove middle", []int{10, 20, 30, 40, 50}, []int{2}, []int{10, 20, 40, 50}, true},
		{"remove multiple unordered", []int{1, 2, 3, 4, 5}, []int{4, 0, 2}, []int{2, 4}, true},
		{"remove dup idx", []int{1, 2, 3}, []int{1, 1, 1}, []int{1, 3}, true},
		{"out of range ignored", []int{1, 2, 3}, []int{9}, []int{1, 2, 3}, false},
		{"negative idx ignored", []int{1, 2, 3}, []int{-1, 2}, []int{1, 2}, true},
		{"empty slice", []int{}, []int{0}, []int{}, false},
		{"remove all", []int{1, 2, 3}, []int{0, 1, 2}, []int{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := RemoveAt(tc.in, tc.idx...)
			if ok != tc.ok || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("RemoveAt(%v,%v) = (%v,%v), want (%v,%v)",
					tc.in, tc.idx, got, ok, tc.want, tc.ok)
			}
		})
	}
}
