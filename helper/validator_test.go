package helper

import (
	"testing"

	"github.com/0xelden/common-libs-go/shared"
)

func TestIsValidUUID(t *testing.T) {
	type args struct {
		u string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "00",
			args: args{"c5201070-df1d-4d45-ab5a-09f6e6749d66"},
			want: true,
		},
		{
			name: "01",
			args: args{shared.EmptyUuid},
			want: true,
		},
		{
			name: "02",
			args: args{""},
			want: false,
		},
		{
			name: "03",
			args: args{"   "},
			want: false,
		},
		{
			name: "04",
			args: args{"bd4d6b78-b69f-42b7-8123-1fa138cd26dx"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidUUID(tt.args.u); got != tt.want {
				t.Errorf("IsValidUUID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		in string
		ok bool
	}{
		// accepted by net/mail when presented as a bare addr-spec
		{"a@b.co", true},
		{"foo.bar@baz-qux.com", true},
		{"user+tag@sub.example.co.uk", true},
		{"foo.+.bar@baz-qux.com", true},
		{"\"john.doe\"@example.com", false}, // quoted local  is invalid
		{"a@b", true},                       // single-label domain is allowed by net/mail
		{"用户@例子.公司", true},                  // non-ASCII

		// rejected by IsValidEmail due to extra syntax around the addr-spec
		{"John Doe <john@example.com>", false}, // display name
		{"<john@example.com>", false},          // angle-addr only
		{"john@example.com (comment)", false},  // trailing comment
		{" john@example.com ", false},          // surrounding WS makes string != addr.Address

		// parse errors from net/mail
		{"john..doe@example.com", false}, // unquoted dot-dot
		{".john@example.com", false},     // leading dot
		{"john.@example.com", false},     // trailing dot
		{"a@@b.com", false},              // multiple @
		{"", false},                      // empty
		{"user example.com", false},      // missing @
	}

	for _, tc := range tests {
		got := IsValidEmail(tc.in)
		if got != tc.ok {
			t.Errorf("IsValidEmail(%q) = %v, want %v", tc.in, got, tc.ok)
		}
	}
}
