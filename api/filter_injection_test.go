package api

import (
	"strings"
	"testing"
)

func TestFilterBackslashQuoteStaysInsideLiteral(t *testing.T) {
	value := `\' OR (SELECT count(*) FROM pg_user)>0 OR name=\'`
	filter := `name = "` + value + `"`

	p := &IndexParam{FilterMap: map[string]string{}}
	got, err := p.parseFilter(filter)
	if err != nil {
		t.Fatalf("parseFilter() error = %v", err)
	}

	literal, ok := strings.CutPrefix(got, "name=")
	if !ok {
		t.Fatalf("unexpected filter %s", got)
	}
	decoded, rest, ok := readStandardLiteral(literal)
	if !ok || rest != "" {
		t.Fatalf("value escaped the literal: %s", got)
	}
	if decoded != value {
		t.Fatalf("literal = %q, want %q", decoded, value)
	}
}

func TestFilterRejectsFunctionOperands(t *testing.T) {
	for _, filter := range []string{
		`password_hash()~'a%'`,
		`result.password_hash(1)~'a%'`,
		`name='x' && password_hash(0)>'a'`,
		`'a'<password_hash()`,
		`(id=1 || lower(password_hash)~'a%')`,
	} {
		p := &IndexParam{FilterMap: map[string]string{}}
		got, err := p.parseFilter(filter)
		if err == nil {
			t.Errorf("parseFilter(%q) = %q, want an error", filter, got)
		}
	}

	p := &IndexParam{FilterMap: map[string]string{}}
	if _, err := p.parseFilter(`id=1 && name~'a%'`); err != nil {
		t.Fatalf("plain column filter rejected: %v", err)
	}
	if err := p.ValidateColumns([]string{"id", "name"}, nil); err != nil {
		t.Fatalf("plain column filter failed the allow-list: %v", err)
	}
}

func readStandardLiteral(s string) (value, rest string, ok bool) {
	if !strings.HasPrefix(s, "'") {
		return "", s, false
	}
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		if s[i] != '\'' {
			b.WriteByte(s[i])
			continue
		}
		if i+1 < len(s) && s[i+1] == '\'' {
			b.WriteByte('\'')
			i++
			continue
		}
		return b.String(), s[i+1:], true
	}
	return "", "", false
}
