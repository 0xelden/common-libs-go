package helper

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"unicode/utf8"
)

func TestSecond(t *testing.T) {
	type args[T any] struct {
		in0    any
		second T
		in2    []any
	}
	type testCase[T any] struct {
		name string
		args args[T]
		want T
	}
	tests := []testCase[int]{
		{
			args: args[int]{
				in0:    errors.New("foo"),
				second: 42,
				in2:    []any{1, 23, 4},
			},
			want: 42,
		},
		{
			args: args[int]{
				second: -16,
			},
			want: -16,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Second(tt.args.in0, tt.args.second, tt.args.in2...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Second() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDistinct(t *testing.T) {
	if //goland:noinspection GoBoolExpressions
	1 == 1 {
		return
	}
	type args[T comparable] struct {
		slice []T
	}
	type testCase[T comparable] struct {
		name string
		args args[T]
		want []T
	}
	tests := []testCase[string]{
		{
			args: args[string]{
				[]string{
					"aa",
					"aa",
					"aa",
					"ab",
				},
			},
			want: []string{
				"aa",
				"ab",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Distinct(tt.args.slice); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Distinct() = %v, want %v", got, tt.want)
			}
		})
	}
	tests2 := []testCase[int]{
		{
			args: args[int]{
				[]int{
					1e6,
					1,
					0,
					1e6,
				},
			},
			want: []int{
				1e6,
				1,
				0,
			},
		},
	}
	for _, tt := range tests2 {
		t.Run(tt.name, func(t *testing.T) {
			if got := Distinct(tt.args.slice); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Distinct() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSliceChunk(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		chunkSize int
		expected  [][]int
	}{
		{
			name:      "Even split",
			input:     []int{1, 2, 3, 4, 5, 6},
			chunkSize: 2,
			expected:  [][]int{{1, 2}, {3, 4}, {5, 6}},
		},
		{
			name:      "Uneven split",
			input:     []int{1, 2, 3, 4, 5},
			chunkSize: 2,
			expected:  [][]int{{1, 2}, {3, 4}, {5}},
		},
		{
			name:      "Chunk size larger than slice",
			input:     []int{1, 2, 3},
			chunkSize: 5,
			expected:  [][]int{{1, 2, 3}},
		},
		{
			name:      "Chunk size of 1",
			input:     []int{1, 2, 3},
			chunkSize: 1,
			expected:  [][]int{{1}, {2}, {3}},
		},
		{
			name:      "Empty slice",
			input:     []int{},
			chunkSize: 3,
			expected:  nil,
		},
		{
			name:      "Zero chunk size",
			input:     []int{1, 2, 3},
			chunkSize: 0,
			expected:  [][]int{{1, 2, 3}},
		},
		{
			name:      "Negative chunk size",
			input:     []int{1, 2, 3},
			chunkSize: -1,
			expected:  [][]int{{1, 2, 3}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := SliceChunk(test.input, test.chunkSize)
			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("Expected %v, got %v", test.expected, result)
			}
		})
	}
}

func TestKeyByStrings(t *testing.T) {
	input := []string{"one", "two", "three", "four", "five"}
	result := KeyBy(input, func(s string) int {
		return len(s)
	})

	expected := map[int]string{
		3: "two",
		4: "five",
		5: "three",
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("KeyBy(strings) = %v, expected %v", result, expected)
	}
}

func TestKeyByStruct(t *testing.T) {
	type Person struct {
		ID   int
		Name string
	}

	persons := []Person{
		{ID: 1, Name: "Alice"},
		{ID: 2, Name: "Bob"},
		{ID: 3, Name: "Charlie"},
	}

	result := KeyBy(persons, func(p Person) int {
		return p.ID
	})

	expected := map[int]Person{
		1: {ID: 1, Name: "Alice"},
		2: {ID: 2, Name: "Bob"},
		3: {ID: 3, Name: "Charlie"},
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("KeyBy(persons) = %v, expected %v", result, expected)
	}
}

func TestKeyByEmptySlice(t *testing.T) {
	var empty []int
	result := KeyBy(empty, func(i int) int {
		return i
	})
	expected := map[int]int{}
	if len(result) != 0 {
		t.Errorf("Expected empty map, got %v", result)
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("KeyBy(empty) = %v, expected %v", result, expected)
	}
}

func TestKeyByDuplicates(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	result := KeyBy(input, func(i int) int {
		return i % 2
	})
	expected := map[int]int{
		0: 4,
		1: 5,
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("KeyBy(duplicates) = %v, expected %v", result, expected)
	}
}

func TestFirstN(t *testing.T) {
	type testCase[T any] struct {
		name string
		src  []T
		n    int
		want []T
	}

	intCases := []testCase[int]{
		{
			name: "n smaller than len(src)",
			src:  []int{1, 2, 3, 4},
			n:    2,
			want: []int{1, 2},
		},
		{
			name: "n equal len(src)",
			src:  []int{1, 2},
			n:    2,
			want: []int{1, 2},
		},
		{
			name: "n greater than len(src)",
			src:  []int{1, 2},
			n:    5,
			want: []int{1, 2},
		},
		{
			name: "empty src",
			src:  []int{},
			n:    3,
			want: []int{},
		},
		{
			name: "negative n",
			src:  []int{1, 2, 3},
			n:    -1,
			want: []int{},
		},
	}

	for _, tc := range intCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			got := FirstNSlice(tc.src, tc.n)

			// Verify length and contents
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("FirstNSlice(%v, %d) = %v, want %v", tc.src, tc.n, got, tc.want)
			}

			// Ensure reslice shares the same backing array when len(src) >= n and n >= 0
			if tc.n >= 0 && len(tc.src) >= tc.n {
				if len(tc.want) > 0 && &got[0] != &tc.src[0] {
					t.Fatalf("FirstNSlice should return a reslice sharing backing array")
				}
			}
		})
	}
}

func TestFirstNRunes(t *testing.T) {
	cases := []struct {
		name, in string
		n        int
		want     string
	}{
		{"ASCII truncate", "abcdef", 3, "abc"},
		{"ASCII no-truncate", "go", 5, "go"},
		{"UTF-8 safe", "céféé", 4, "céfé"}, // é stays intact
		{"emoji", "🍕🍔🍟", 2, "🍕🍔"},
		{"zero n", "data", 0, ""},
		{"negative n", "data", -1, ""},
		{"negative n", "", 2, ""},
	}

	for _, c := range cases {
		got := FirstNRunes(c.in, c.n)
		if got != c.want {
			t.Fatalf("%s: got %q, want %q", c.name, got, c.want)
		}
		// Extra correctness check: result must be valid UTF-8.
		if !utf8.ValidString(got) {
			t.Fatalf("%s: result is not valid UTF-8", c.name)
		}
	}
}

func TestTakeDistinct(t *testing.T) {
	type mockRow struct{ Project string }
	rows := []mockRow{
		{Project: "A"},
		{Project: "A"},
		{Project: "B"},
		{Project: "A"},
		{Project: "B"},
		{Project: "I"},
	}

	got := DistinctBy(rows, func(r mockRow) string { return r.Project })
	want := []string{"A", "B", "I"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DistinctBy() = %v, want %v", got, want)
	}
}

func TestUniqueBy(t *testing.T) {
	type mockRow struct{ Project string }
	rows := []mockRow{
		{Project: "A"},
		{Project: "A"},
		{Project: "B"},
		{Project: "A"},
		{Project: "B"},
		{Project: "I"},
	}

	got := UniqueBy(rows, func(r mockRow) string { return r.Project })
	want := []mockRow{
		{Project: "A"},
		{Project: "B"},
		{Project: "I"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DistinctBy() = %v, want %v", got, want)
	}
}

// Example demonstrates typical use; note keys are unordered,
// so sort before comparing/printing in examples or tests that
// require deterministic output.
func ExampleMapKeys() {
	m := map[string]float64{"pi": 3.14159, "e": 2.71828}
	keys := MapKeys(m)
	sort.Strings(keys)
	fmt.Println(keys)
	// Output: [e pi]
}

func TestMapKeys_Basic(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}

	got := MapKeys(m)
	sort.Strings(got)

	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MapKeys()=%v, want %v", got, want)
	}
}

func TestMapKeys_EmptyAndNil(t *testing.T) {
	empty := map[string]struct{}{}
	if got := MapKeys(empty); len(got) != 0 {
		t.Fatalf("MapKeys(empty) len=%d, want 0", len(got))
	}

	var nilMap map[string]int
	if got := MapKeys(nilMap); len(got) != 0 {
		t.Fatalf("MapKeys(nil) len=%d, want 0", len(got))
	}
}

func TestUniqueByEmptySlice(t *testing.T) {
	var items []int
	got := UniqueBy(items, func(v int) int { return v })

	if got != nil {
		t.Fatalf("expected nil for empty input, got %#v", got)
	}
}

func TestUniqueBySingleElement(t *testing.T) {
	items := []int{42}
	got := UniqueBy(items, func(v int) int { return v })

	want := []int{42}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected result.\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestUniqueByIntsIdentity(t *testing.T) {
	items := []int{1, 1, 2, 3, 3, 3, 2, 4}
	got := UniqueBy(items, func(v int) int { return v })

	want := []int{1, 2, 3, 4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected result.\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestUniqueByPreservesOrder(t *testing.T) {
	items := []string{"b", "a", "b", "c", "a", "d"}
	got := UniqueBy(items, func(s string) string { return s })

	// Order must follow the first occurrence of each key.
	want := []string{"b", "a", "c", "d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order not preserved.\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestUniqueByStructField(t *testing.T) {
	type user struct {
		ID   int
		Name string
		Age  int
	}

	items := []user{
		{ID: 1, Name: "alice", Age: 30},
		{ID: 2, Name: "bob", Age: 25},
		{ID: 1, Name: "alice-dup", Age: 31}, // same ID, different fields
		{ID: 3, Name: "charlie", Age: 40},
		{ID: 2, Name: "bob-dup", Age: 26},
	}

	got := UniqueBy(items, func(u user) int { return u.ID })

	// Expect first occurrence for each ID.
	want := []user{
		{ID: 1, Name: "alice", Age: 30},
		{ID: 2, Name: "bob", Age: 25},
		{ID: 3, Name: "charlie", Age: 40},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected result.\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestUniqueByCustomProjection(t *testing.T) {
	// Project by string length to ensure projection, not raw equality, drives uniqueness.
	items := []string{"aa", "b", "ccc", "dd", "eee", "f"}
	got := UniqueBy(items, func(s string) int { return len(s) })

	// First seen of length 2,1,3 -> "aa","b","ccc"
	want := []string{"aa", "b", "ccc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected result.\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestUniqueByMergeEmptySlice(t *testing.T) {
	var items []int

	got := UniqueByMerge(items,
		func(v int) int { return v },
		func(existing, dup int) int { return existing + dup },
	)

	if got != nil {
		t.Fatalf("expected nil for empty input, got %#v", got)
	}
}

func TestUniqueByMergeNoDuplicates(t *testing.T) {
	items := []int{1, 2, 3}

	got := UniqueByMerge(items,
		func(v int) int { return v },
		func(existing, dup int) int { return existing + dup },
	)

	if !reflect.DeepEqual(got, items) {
		t.Fatalf("expected passthrough, want %#v, got %#v", items, got)
	}
}

func TestUniqueByMergeSumInts(t *testing.T) {
	// Duplicate keys: 1, 2
	items := []int{1, 1, 2, 3, 2}

	got := UniqueByMerge(items,
		func(v int) int { return v },                          // key = value
		func(existing, dup int) int { return existing + dup }, // sum duplicates
	)

	// For key=1: 1+1 = 2
	// For key=2: 2+2 = 4
	// Key=3: single
	want := []int{2, 4, 3}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected result.\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestUniqueByMergeStructSumField(t *testing.T) {
	type item struct {
		ID    string
		Name  string
		Count int
	}

	items := []item{
		{ID: "a", Name: "first-a", Count: 1},
		{ID: "b", Name: "first-b", Count: 5},
		{ID: "a", Name: "second-a", Count: 3},
		{ID: "c", Name: "first-c", Count: 2},
		{ID: "b", Name: "second-b", Count: 7},
		{ID: "a", Name: "third-a", Count: 6},
	}

	got := UniqueByMerge(items,
		func(it item) string { return it.ID }, // dedupe by ID
		func(existing, dup item) item {
			// Preserve first Name, sum Count.
			existing.Count += dup.Count
			return existing
		},
	)

	want := []item{
		{ID: "a", Name: "first-a", Count: 1 + 3 + 6},
		{ID: "b", Name: "first-b", Count: 5 + 7},
		{ID: "c", Name: "first-c", Count: 2},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected result.\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestUniqueByMergePreservesFirstOrder(t *testing.T) {
	type item struct {
		Key   string
		Value int
	}

	items := []item{
		{Key: "x", Value: 1},
		{Key: "y", Value: 2},
		{Key: "x", Value: 3},
		{Key: "z", Value: 4},
		{Key: "y", Value: 5},
	}

	got := UniqueByMerge(items,
		func(it item) string { return it.Key },
		func(existing, dup item) item {
			existing.Value += dup.Value
			return existing
		},
	)

	// First appearances: x, y, z => order must be [x, y, z]
	want := []item{
		{Key: "x", Value: 1 + 3},
		{Key: "y", Value: 2 + 5},
		{Key: "z", Value: 4},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order or merge incorrect.\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestUniqueByMergeCustomProjection(t *testing.T) {
	// Unique by string length, merge by choosing lexicographically smaller.
	items := []string{"aaa", "bb", "c", "dddd", "ee", "f"}

	got := UniqueByMerge(items,
		func(s string) int { return len(s) }, // key = length
		func(existing, dup string) string {
			if dup < existing {
				return dup
			}
			return existing
		},
	)

	// Lengths: 3,2,1,4,2,1
	// For len=3: "aaa"
	// For len=2: min("bb","ee") = "bb"
	// For len=1: min("c","f") = "c"
	// For len=4: "dddd"
	want := []string{"aaa", "bb", "c", "dddd"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected result.\nwant: %#v\ngot:  %#v", want, got)
	}
}
