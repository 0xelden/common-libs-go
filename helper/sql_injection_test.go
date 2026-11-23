package helper

import "testing"

func TestQuoteIdentUsesSQLEscaping(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "name", `"name"`},
		{"reserved word", "user", `"user"`},
		// The regression: strconv.Quote produced "a\"; DROP TABLE users; --",
		// and PostgreSQL ends the identifier at the \" because a backslash is
		// an ordinary character inside quoted identifiers.
		{"embedded quote is doubled", `a"; DROP TABLE users; --`, `"a""; DROP TABLE users; --"`},
		{"only a quote", `"`, `""""`},
		{"nul is stripped", "a\x00b", `"ab"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := QuoteIdent(tt.in); got != tt.want {
				t.Errorf("QuoteIdent(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestSQLFragmentHasStatementBreak(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"plain filter", `WHERE (result.id = 'abc')`, false},
		{"quoted identifier", `WHERE ("user"."id" = 1)`, false},
		{"order by", `ORDER BY "name" ASC`, false},

		// Dashes and semicolons inside literals are legitimate data.
		{"dashes inside literal", `WHERE (name ILIKE '%a--b%')`, false},
		{"semicolon inside literal", `WHERE (note = 'x; y')`, false},
		{"doubled quote then dashes", `WHERE (note = 'it''s--fine')`, false},

		// The actual attacks.
		{"statement terminator", `WHERE (1=1); DROP TABLE users`, true},
		{"line comment", `WHERE (1=1) --`, true},
		{"block comment", `WHERE (1=1) /*x*/`, true},
		{"unterminated literal", `WHERE (name = 'abc`, true},
		{"break after closing literal", `WHERE (name = 'a'); DROP TABLE t`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SQLFragmentHasStatementBreak(tt.in); got != tt.want {
				t.Errorf("SQLFragmentHasStatementBreak(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
