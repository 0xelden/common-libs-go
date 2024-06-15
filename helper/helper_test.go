package helper

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

func TestReduceSum(t *testing.T) {
	testSource := []int{1, 2, 3, 4, 5}
	expectedValue := 15

	total := Reduce(testSource, 0, func(previous int, current int, index int) int {
		return previous + current
	})
	if total != expectedValue {
		t.Errorf("Expected = %v, got = %v", expectedValue, total)
	}
}

func TestReduceCollect(t *testing.T) {
	type foo struct {
		id int
	}
	testSource := []foo{
		{1},
		{2},
		{3},
		{4},
		{5},
	}

	expectedValue := []int{1, 2, 3, 4, 5}

	collected := Reduce(testSource, []int{}, func(previous []int, current foo, index int) []int {
		return append(previous, current.id)
	})
	if !reflect.DeepEqual(collected, expectedValue) {
		t.Errorf("Expected = %v, got = %v", expectedValue, collected)
	}
}

func TestCoalesce(t *testing.T) {
	var v1 *int
	p1, p2 := Ptr(1), Ptr(2)

	cases := []struct {
		name     string
		exp, got any
	}{
		{
			"strings",
			"1", Coalesce("", "1", "2"),
		},
		{
			"strings first",
			"1", Coalesce("1", "2", "3"),
		},
		{
			"strings last",
			"1", Coalesce("", "", "1"),
		},
		{
			"strings all zero",
			"", Coalesce("", "", ""),
		},
		{
			"strings no args",
			"", Coalesce[string](),
		},
		{
			"ints",
			1, Coalesce(0, 1, 2, 3),
		},
		{
			"ints first",
			1, Coalesce(1, 2, 3),
		},
		{
			"ints last",
			1, Coalesce(0, 0, 0, 0, 1),
		},
		{
			"ints all zero",
			0, Coalesce(0, 0, 0, 0),
		},
		{
			"ints no args",
			0, Coalesce[int](),
		},
		{
			"pointers",
			p1, Coalesce(v1, p1, p2), //nolint
		},
		{
			"pointers first",
			p1, Coalesce(p1, p2),
		},
		{
			"pointers last",
			p1, Coalesce(v1, nil, p1), //noinspection ALL
		},
		{
			"pointers all zero",
			(*int)(nil), Coalesce[*int](nil, nil, nil),
		},
		{
			"pointers no args",
			(*int)(nil), Coalesce[*int](),
		},
	}

	for _, c := range cases {
		if c.exp != c.got {
			t.Errorf("[%s] Expected: %v, got: %v", c.name, c.exp, c.got)
		}
	}

}

func TestIf(t *testing.T) {
	{
		i1, i2 := 1, 2
		exp, got := i1, If(true, i1, i2)
		if got != exp {
			t.Errorf("[int] Expected %d, got: %d", exp, got)
		}
		exp, got = i2, If(false, i1, i2)
		if got != exp {
			t.Errorf("[int] Expected %d, got: %d", exp, got)
		}
	}

	{
		s1, s2 := "first", "second"
		exp, got := s1, If(true, s1, s2)
		if got != exp {
			t.Errorf("[string] Expected %s, got: %s", exp, got)
		}
		exp, got = s2, If(false, s1, s2)
		if got != exp {
			t.Errorf("[string] Expected %s, got: %s", exp, got)
		}
	}
}

func TestOr(t *testing.T) {
	stringCases := []struct {
		name       string
		value      string
		candidates []string
		expected   bool
	}{
		{
			name:       "match first",
			value:      "a",
			candidates: []string{"a", "b", "c"},
			expected:   true,
		},
		{
			name:       "match later",
			value:      "c",
			candidates: []string{"a", "b", "c"},
			expected:   true,
		},
		{
			name:       "no match",
			value:      "d",
			candidates: []string{"a", "b", "c"},
			expected:   false,
		},
		{
			name:       "no candidates",
			value:      "a",
			candidates: nil,
			expected:   false,
		},
	}

	for _, c := range stringCases {
		if got := Or(c.value, c.candidates...); got != c.expected {
			t.Errorf("[string %s] expected %v, got %v", c.name, c.expected, got)
		}
	}

	intCases := []struct {
		name       string
		value      int
		candidates []int
		expected   bool
	}{
		{
			name:       "match first",
			value:      1,
			candidates: []int{1, 2, 3},
			expected:   true,
		},
		{
			name:       "match later",
			value:      3,
			candidates: []int{1, 2, 3},
			expected:   true,
		},
		{
			name:       "no match",
			value:      4,
			candidates: []int{1, 2, 3},
			expected:   false,
		},
		{
			name:       "no candidates",
			value:      1,
			candidates: nil,
			expected:   false,
		},
	}

	for _, c := range intCases {
		if got := Or(c.value, c.candidates...); got != c.expected {
			t.Errorf("[int %s] expected %v, got %v", c.name, c.expected, got)
		}
	}

	type status string

	statusCases := []struct {
		name       string
		value      status
		candidates []status
		expected   bool
	}{
		{
			name:       "match named type",
			value:      status("new"),
			candidates: []status{status("old"), status("new")},
			expected:   true,
		},
		{
			name:       "no match named type",
			value:      status("archived"),
			candidates: []status{status("new"), status("old")},
			expected:   false,
		},
	}

	for _, c := range statusCases {
		if got := Or(c.value, c.candidates...); got != c.expected {
			t.Errorf("[status %s] expected %v, got %v", c.name, c.expected, got)
		}
	}
}

func TestDeterministicUUID(t *testing.T) {
	tests := []struct {
		args    string
		wantErr bool
	}{
		{
			args:    "Ante dolor diam tellus arcu congue dictum nec.",
			wantErr: false,
		},
		{
			args:    "",
			wantErr: false,
		},
		{
			args:    " ",
			wantErr: false,
		},
		{
			args:    "Sit",
			wantErr: false,
		},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			got, err := DeterministicUUID(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeterministicUUID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got2, err := DeterministicUUID(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeterministicUUID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != got2 {
				t.Errorf("DeterministicUUID() got = %v, want %v", got, got2)
			}
			if !IsValidUUID(got) {
				t.Errorf("DeterministicUUID() got = %v, want a valid uuid", got)
			}
		})
	}
}

func TestContextGetString(t *testing.T) {
	ctx := context.WithValue(context.Background(), "foo", "0")
	ctx = ContextSetString(ctx, "foo", "1")
	ctx = ContextSetString(ctx, "foo", "2")

	foo := ContextGetString(ctx, "foo")
	if foo != "2" {
		t.Fatal("should be 2, got:", foo)
	}
}

func TestMap(t *testing.T) {
	t.Run("int → int (square)", func(t *testing.T) {
		in := []int{1, 2, 3, 4, 5}
		got := Map(in, func(v int, _ int) int { return v * v })
		want := []int{1, 4, 9, 16, 25}

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Map(%v) = %v, want %v", in, got, want)
		}
	})

	t.Run("string → int (length)", func(t *testing.T) {
		in := []string{"go", "gopher", "!"}
		got := Map(in, func(s string, _ int) int { return len(s) })
		want := []int{2, 6, 1}

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Map(%v) = %v, want %v", in, got, want)
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		in := []float64{}
		got := Map(in, func(v float64, _ int) float64 { return v * 2 })
		if len(got) != 0 {
			t.Fatalf("expected empty slice, got %v", got)
		}
	})
}

func TestJoinPath(t *testing.T) {
	tests := []struct {
		name  string
		base  string
		comps []string
		want  string
	}{
		{
			name:  "simple join with base path",
			base:  "https://example.com/api",
			comps: []string{"v1", "users"},
			want:  "https://example.com/api/v1/users",
		},
		{
			name:  "base without trailing slash (root path empty ➜ should insert /)",
			base:  "https://example.com",
			comps: []string{"v1", "users"},
			want:  "https://example.com/v1/users",
		},
		{
			name:  "cleanup duplicate leading & trailing slashes",
			base:  "https://example.com/api/",
			comps: []string{"v1/", "/users/"},
			want:  "https://example.com/api/v1/users",
		},
		{
			name:  "preserve query and fragment",
			base:  "https://example.com/api?foo=bar#sec",
			comps: []string{"v1"},
			want:  "https://example.com/api/v1?foo=bar#sec",
		},
		{
			name:  "dot and dot-dot elements are cleaned",
			base:  "https://example.com/",
			comps: []string{".", "a", "..", "b", "c/"},
			want:  "https://example.com/b/c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JoinPath(tt.base, tt.comps...)
			if got != tt.want {
				t.Errorf("JoinPath(%q, %s) = %q, want %q",
					tt.base, fmt.Sprintf("%q", tt.comps), got, tt.want)
			}
		})
	}
}

// tenantCtx returns a context with the `"tenant"` key populated so that
func tenantCtx(tenant string) context.Context {
	return context.WithValue(context.Background(), "tenant", tenant)
}

func TestNormalizedEnvLower(t *testing.T) {
	type args struct {
		baseKey string
		suffix  string
		value   string
	}
	tests := []struct {
		args args
		want string
	}{
		{
			args: args{
				baseKey: "foo:bar",
				suffix:  "baz",
				value:   "1",
			},
			want: "1",
		},
		{
			args: args{
				baseKey: "1:2",
				suffix:  "3",
				value:   "true",
			},
			want: "true",
		},
	}
	for i, tt := range tests {
		t.Run(ToString(i), func(t *testing.T) {
			t.Setenv(ToEnvKey(tt.args.baseKey)+"_"+ToEnvKey(tt.args.suffix), tt.args.value)
			if got := NormalizedEnvLower(tt.args.baseKey, tt.args.suffix); got != tt.want {
				t.Errorf("NormalizedEnvLower() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGreetingByTime(t *testing.T) {
	// Each case picks a representative hour for the target greeting
	tests := []struct {
		name     string
		hour     int // 0–23
		expected string
	}{
		{"pagi-lower-bound", 1, "Selamat pagi"},
		{"pagi-upper-bound", 9, "Selamat pagi"},
		{"siang-lower-bound", 10, "Selamat siang"},
		{"siang-upper-bound", 13, "Selamat siang"},
		{"sore-lower-bound", 14, "Selamat sore"},
		{"sore-upper-bound", 17, "Selamat sore"},
		{"malam-midnight", 0, "Selamat malam"},
		{"malam-evening", 23, "Selamat malam"},
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create a time.Time with the desired hour; date is irrelevant
			got := GreetingByTime(time.Date(2025, time.July, 15, tc.hour, 0, 0, 0, time.UTC))

			if got != tc.expected {
				t.Fatalf("hour %d: expected %q, got %q", tc.hour, tc.expected, got)
			}
		})
	}
}

func TestStructToMap(t *testing.T) {
	type Embed struct {
		Secret string `json:"secret"`
	}

	//goland:noinspection GoVetStructTag
	type Sample struct {
		ID    int     `json:"id"`
		Name  string  `json:"name,omitempty"`
		Score *int    `json:"score"` // pointer field
		skip  string  `json:"-"`     // unexported + "-"
		Extra float64 // no tag → field name key
		Embed         // anonymous embedded struct
	}

	type Unexported struct { // nothing is exported
		private int
	}
	n := 99
	tests := []struct {
		name      string
		in        any
		tag       string
		fields    []string // selective subset
		want      map[string]any
		wantEmpty bool // expecting an empty map
	}{
		// ☑ Positive cases ----------------------------------------------------
		{
			name: "basic struct, all fields",
			in: Sample{
				ID:    1,
				Name:  "foo",
				Score: &n,
				Extra: 3.14,
				Embed: Embed{Secret: "s3cr3t"},
			},
			tag: "json",
			want: map[string]any{
				"id":     1,
				"name":   "foo",
				"score":  &n,
				"Extra":  3.14,
				"secret": "s3cr3t",
			},
		},
		{
			name:   "selective fields by tag",
			in:     Sample{ID: 2, Name: "bar"},
			tag:    "json",
			fields: []string{"id"}, // only ID
			want:   map[string]any{"id": 2},
		},
		{
			name:   "selective fields by struct name (no tag)",
			in:     Sample{ID: 3, Extra: 6.28},
			tag:    "json",
			fields: []string{"Extra"},
			want:   map[string]any{"Extra": 6.28},
		},

		// ☠ Negative cases ----------------------------------------------------
		{
			name:      "non-struct input",
			in:        42,
			tag:       "json",
			wantEmpty: true,
		},
		{
			name:      "nil interface",
			in:        nil,
			tag:       "json",
			wantEmpty: true,
		},
		{
			name:      "struct with only unexported fields",
			in:        Unexported{private: 7},
			tag:       "json",
			wantEmpty: true,
		},

		// ⚠ Edge cases --------------------------------------------------------
		{
			name:      "pointer to nil struct",
			in:        (*Sample)(nil),
			tag:       "json",
			wantEmpty: true,
		},
		{
			name: "embedded struct with omitempty tag stripped",
			in: struct {
				Embed
				Age int `json:"age,omitempty"`
			}{Embed: Embed{Secret: "x"}, Age: 0},
			tag: "json",
			want: map[string]any{
				"secret": "x",
				"age":    0,
			},
		},
	}

	for _, tc := range tests {
		tc := tc // capture
		t.Run(tc.name, func(t *testing.T) {
			got := StructToMap(tc.in, tc.tag, tc.fields...)
			if tc.wantEmpty && len(got) != 0 {
				t.Fatalf("expected empty map, got %v", got)
			}
			if !tc.wantEmpty && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("mismatch:\nwant: %#v\ngot : %#v", tc.want, got)
			}
		})
	}
}

func TestStructToMapDB(t *testing.T) {
	type (
		EmbA struct {
			A string `db:"a"`
		}

		EmbB struct {
			B int `db:"b"`
		}

		Mixed struct {
			EmbA        // first anonymous (must be kept)
			EmbB        // second anonymous (must be ignored)
			Name string `db:"name"` // regular field
		}

		OnlyAnon struct {
			EmbA       // only anonymous, acts like a flat struct
			hidden int // unexported, ignored
		}

		Unexported struct {
			private int
		}
	)

	var p42 = 42

	tests := []struct {
		name      string
		in        any
		fields    []string // selective subset
		want      map[string]any
		wantEmpty bool
	}{
		// ✅ positive --------------------------------------------------------
		{
			name: "basic struct, all fields",
			in: Mixed{
				EmbA: EmbA{A: "foo"},
				EmbB: EmbB{B: 7},
				Name: "bar",
			},
			want: map[string]any{
				"a": "foo", // from first anonymous
			},
		},
		{
			name:   "selective by tag",
			in:     Mixed{EmbA: EmbA{A: "x"}, Name: "y"},
			fields: []string{"name"},
			want:   map[string]any{},
		},
		{
			name: "only anonymous struct",
			in:   OnlyAnon{EmbA: EmbA{A: "z"}},
			want: map[string]any{"a": "z"},
		},

		// ☠ negative --------------------------------------------------------
		{
			name:      "non-struct input",
			in:        99,
			wantEmpty: true,
		},
		{
			name:      "nil interface",
			in:        nil,
			wantEmpty: true,
		},
		{
			name:      "all-unexported struct",
			in:        Unexported{private: 1},
			wantEmpty: true,
		},

		// ⚠ edge ------------------------------------------------------------
		{
			name:      "pointer to nil struct",
			in:        (*Mixed)(nil),
			wantEmpty: true,
		},
		{
			name: "pointer fields preserved",
			in: &Mixed{
				EmbA: EmbA{A: "ptr"},
				Name: "baz",
			},
			want: map[string]any{
				"a": "ptr",
			},
		},
		{
			name: "selective by struct field name (no tag)",
			in: struct {
				Age    int `db:"age"`
				Height int `db:"height"`
			}{Age: 10, Height: 150},
			fields: []string{"Age", "height"},
			want:   map[string]any{"height": 150},
		},
		{
			name: "embedded pointer ignored after first",
			in: struct {
				*EmbA      // first, must be used
				*EmbB      // second, ignored
				Score *int `db:"score"`
			}{EmbA: &EmbA{A: "alpha"}, EmbB: &EmbB{B: 2}, Score: &p42},
			want: map[string]any{
				"a": "alpha",
			},
		},
	}

	for _, tc := range tests {
		tc := tc // capture loop var
		t.Run(tc.name, func(t *testing.T) {
			got := StructToMapDB(tc.in, tc.fields...)
			if tc.wantEmpty && len(got) != 0 {
				t.Fatalf("expected empty map, got %v", got)
			}
			if !tc.wantEmpty && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("mismatch\nwant: %#v\ngot : %#v", tc.want, got)
			}
		})
	}
}

func TestOrigin(t *testing.T) {
	tests := []string{
		"https://example.com/path/to/resource?query=param#fragment",
		"http://localhost:8080/api/v1/users",
		"https://subdomain.example.co.uk/path/to/resource",
	}
	for _, s := range tests {
		base, err := UrlOrigin(s)
		if err != nil {
			fmt.Println("ERR:", err)
			t.Fatal(err)
		}
		fmt.Println(base)
	}
}

func TestStructToMapDB1(t *testing.T) {
	type PortalDto struct {
		Id          string     `form:"id" json:"id" validate:"omitempty,uuid"`
		Code        *string    `form:"code" db:"code" json:"code"`
		Name        *string    `form:"name" db:"name" json:"name"`
		Domain      *string    `form:"domain" db:"domain" json:"domain"`
		Description *string    `form:"description" db:"description" json:"description"`
		Status      *int       `form:"status" db:"status" json:"status"`
		CreatedBy   *string    `json:"created_by,omitempty" form:"created_by" db:"created_by"`
		CreatedAt   *time.Time `json:"created_at,omitempty" form:"created_at" db:"created_at"`
		UpdatedBy   *string    `json:"updated_by,omitempty" form:"updated_by" db:"updated_by"`
		UpdatedAt   *time.Time `json:"updated_at,omitempty" form:"updated_at" db:"updated_at"`
	}

	type args struct {
		data   any
		fields []string
	}
	code := "portal-code"
	name := "Portal Name"
	tests := []struct {
		name string
		args args
		want map[string]any
	}{
		{
			name: "prefer tagged key for id and do not emit field name",
			args: args{
				data: PortalDto{
					Id:   "portal-1",
					Code: &code,
					Name: &name,
				},
			},
			want: map[string]any{
				"id":          "portal-1",
				"code":        &code,
				"name":        &name,
				"domain":      (*string)(nil),
				"description": (*string)(nil),
				"status":      (*int)(nil),
				"created_by":  (*string)(nil),
				"created_at":  (*time.Time)(nil),
				"updated_by":  (*string)(nil),
				"updated_at":  (*time.Time)(nil),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StructToMapDB(tt.args.data, tt.args.fields...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StructToMapDB() = %v, want %v", got, tt.want)
			}
		})
	}
}
