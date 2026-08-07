package api

import (
	"strings"
	"testing"
)

func TestAddFilterEscapesArguments(t *testing.T) {
	tests := []struct {
		name    string
		stmt    string
		args    []any
		want    string
		wantErr bool
	}{
		{
			name: "plain value",
			stmt: "result.company_id = ?",
			args: []any{"c1"},
			want: "result.company_id = 'c1'",
		},
		{
			// The payload is neutralised by quoting, not rejected: it becomes a
			// value that simply matches nothing.
			name: "injection payload becomes a literal",
			stmt: "result.name = ?",
			args: []any{"x'; DROP TABLE users; --"},
			want: "result.name = 'x''; DROP TABLE users; --'",
		},
		{
			name: "numeric arg",
			stmt: "result.qty > ?",
			args: []any{10},
			want: "result.qty > 10",
		},
		{
			name: "no args is passed through",
			stmt: "result.deleted_at IS NULL",
			want: "result.deleted_at IS NULL",
		},
		{
			// A hand-written template is trusted, but a broken one is still
			// caught rather than spliced into the query.
			name:    "unsafe template is rejected",
			stmt:    "result.id = 1; DROP TABLE users",
			wantErr: true,
		},
		{
			name:    "empty statement is rejected",
			stmt:    "   ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &IndexParam{}
			err := p.AddFilter(tt.stmt, tt.args...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("AddFilter() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if len(p.Filters) != 0 {
					t.Errorf("rejected filter was still appended: %v", p.Filters)
				}
				return
			}
			if len(p.Filters) != 1 || p.Filters[0] != tt.want {
				t.Errorf("AddFilter() = %v, want [%s]", p.Filters, tt.want)
			}
		})
	}
}

// TestAddFilterOutputSurvivesGuard ties AddFilter to the RowIndex.List guard:
// anything AddFilter accepts must also pass the fragment check downstream,
// otherwise a "safe" call would fail at query time instead of at build time.
func TestAddFilterOutputSurvivesGuard(t *testing.T) {
	p := &IndexParam{}
	for _, v := range []string{
		"x'; DROP TABLE users; --",
		"a--b",
		"it's fine",
		strings.Repeat("'", 5),
	} {
		if err := p.AddFilter("result.name = ?", v); err != nil {
			t.Fatalf("AddFilter(%q) unexpected error: %v", v, err)
		}
	}
	if got := p.GenerateFilterStmt(); !strings.HasPrefix(got, "WHERE ") {
		t.Fatalf("GenerateFilterStmt() = %q", got)
	}
}

func TestViewParamAddFilter(t *testing.T) {
	p := &ViewParam{}
	if err := p.AddFilter("result.company_id = ?", "c1"); err != nil {
		t.Fatalf("AddFilter() error = %v", err)
	}
	if len(p.Filters) != 1 || p.Filters[0] != "result.company_id = 'c1'" {
		t.Errorf("Filters = %v", p.Filters)
	}
	if err := p.AddFilter("result.id = 1; DROP TABLE t"); err == nil {
		t.Error("AddFilter() accepted a statement terminator")
	}
}
