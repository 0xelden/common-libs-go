package sqlstring

import (
	"testing"
	"time"
)

func TestEscape(t *testing.T) {
	var nilInts []int
	var nilIntPtr *int

	tests := []struct {
		name string
		args any
		want string
	}{
		{
			name: "",
			args: `((product_code::text=%MANSET 5" X 4.5 MM X 400 MM))`,
			want: `'((product_code::text=%MANSET 5" X 4.5 MM X 400 MM))'`,
		},
		{
			name: "",
			args: `((product_code::text=%MANSET 5\" X 4.5 MM X 400 MM))`,
			want: `'((product_code::text=%MANSET 5\" X 4.5 MM X 400 MM))'`,
		},
		{
			name: "escape single quote",
			args: "t'est",
			want: `'t''est'`,
		},
		{
			name: "keep backslash",
			args: `a\b`,
			want: `'a\b'`,
		},
		{
			name: "nil becomes null",
			args: nil,
			want: "NULL",
		},
		{
			name: "bytes become decode",
			args: []byte{0xde, 0xad, 0xbe, 0xef},
			want: "decode('deadbeef','hex')",
		},
		{
			name: "slice becomes csv",
			args: []int{1, 2, 3},
			want: "1,2,3",
		},
		{
			name: "empty slice becomes null",
			args: []int{},
			want: "NULL",
		},
		{
			name: "nil slice becomes null",
			args: nilInts,
			want: "NULL",
		},
		{
			name: "nil pointer becomes null",
			args: nilIntPtr,
			want: "NULL",
		},
		{
			name: "float becomes fixed precision",
			args: 1.2,
			want: "1.200000",
		},
		{
			name: "backslash + quote preserved",
			args: `te\'st`,
			want: `'te\''st'`,
		},
		{
			name: "double quotes preserved",
			args: `"te\'st"`,
			want: `'"te\''st"'`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Escape(tt.args); got != tt.want {
				t.Errorf("Escape() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	got := Format("select * from users where name=? and age=? and raw=?", "t'est", 10, `a\b`)
	want := "select * from users where name='t''est' and age=10 and raw='a\\b'"
	if got != want {
		t.Fatalf("Format() = %v, want %v", got, want)
	}
}

func TestFormat_ArgsMismatch(t *testing.T) {
	got := Format("select ? and ?", 1)
	want := "select 1 and ?"
	if got != want {
		t.Fatalf("Format() = %v, want %v", got, want)
	}

	got = Format("select ?", 1, 2)
	want = "select 1"
	if got != want {
		t.Fatalf("Format() = %v, want %v", got, want)
	}
}

func TestEscapeInLocation_Time(t *testing.T) {
	tm := time.Date(2024, 1, 2, 3, 4, 5, 123_000_000, time.UTC)
	loc := time.FixedZone("UTC+7", 7*60*60)

	got := EscapeInLocation(tm, loc)
	want := `'2024-01-02 10:04:05.123'`
	if got != want {
		t.Fatalf("EscapeInLocation() = %v, want %v", got, want)
	}
}

func TestEscape_JSONSingleQuote(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	got := Escape(payload{Name: "t'est"})
	want := `'{"name":"t''est"}'`
	if got != want {
		t.Fatalf("Escape() = %v, want %v", got, want)
	}
}

func TestEscape_JSONMarshalErrorBecomesNull(t *testing.T) {
	got := Escape(func() {})
	want := "NULL"
	if got != want {
		t.Fatalf("Escape() = %v, want %v", got, want)
	}
}

func TestSetSingleQuoteEscaper(t *testing.T) {
	SetSingleQuoteEscaper("\\'")
	t.Cleanup(func() {
		SetSingleQuoteEscaper("''")
	})

	got := Escape("t'est")
	want := `'t\'est'`
	if got != want {
		t.Fatalf("Escape() = %v, want %v", got, want)
	}
}
