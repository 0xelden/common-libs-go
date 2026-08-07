package api

import (
	"testing"

	"github.com/0xelden/common-libs-go/helper"
)

// TestSortStmtRejectsIdentifierEscape pins the sort-parameter regression: the
// column name used to be quoted with strconv.Quote, whose \" escaping is not
// honoured by PostgreSQL inside a quoted identifier, so the payload below
// closed the identifier and the rest was parsed as SQL.
func TestSortStmtRejectsIdentifierEscape(t *testing.T) {
	payload := `a"; DROP TABLE users; --`

	t.Run("IndexParam", func(t *testing.T) {
		p := &IndexParam{Sort: []string{payload + ":asc"}}
		got, err := p.generateSortStmt()
		if err != nil {
			t.Fatalf("generateSortStmt() error = %v", err)
		}
		want := `ORDER BY "a""; DROP TABLE users; --" ASC`
		if got != want {
			t.Errorf("generateSortStmt() = %s, want %s", got, want)
		}
	})

	t.Run("SortParam", func(t *testing.T) {
		p := &SortParam{Sort: []string{payload + ":asc"}}
		got, err := p.generateSortStmt()
		if err != nil {
			t.Fatalf("generateSortStmt() error = %v", err)
		}
		want := `ORDER BY "a""; DROP TABLE users; --" ASC`
		if got != want {
			t.Errorf("generateSortStmt() = %s, want %s", got, want)
		}
	})
}

// TestParseFilterRejectsStatementBreak documents that the fexpr-backed filter
// DSL already refuses to tokenise a statement terminator, so the `filter=`
// query parameter is not an injection vector on its own.
func TestParseFilterRejectsStatementBreak(t *testing.T) {
	for _, in := range []string{
		"id='x'; DROP TABLE users; --'",
		"id=1;DROP TABLE t",
	} {
		p := &IndexParam{FilterMap: map[string]string{}}
		if _, err := p.parseFilter(in); err == nil {
			t.Errorf("parseFilter(%q) succeeded, want error", in)
		}
	}
}

// TestGenerateFilterStmtGuard covers the raw path: IndexParam.Filters is a
// plain []string that service code appends to directly, so a fragment can
// reach the query template without going through parseFilter.
func TestGenerateFilterStmtGuard(t *testing.T) {
	tests := []struct {
		name    string
		filters []string
		want    bool
	}{
		{"safe", []string{"result.id = 'abc'"}, false},
		{"safe with dashes in literal", []string{"result.name ILIKE '%a--b%'"}, false},
		{"injected drop", []string{"1=1); DROP TABLE users; --"}, true},
		{"injected comment", []string{"1=1 --"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &IndexParam{Filters: tt.filters}
			if got := helper.SQLFragmentHasStatementBreak(p.GenerateFilterStmt()); got != tt.want {
				t.Errorf("blocked = %v, want %v", got, tt.want)
			}
		})
	}
}
