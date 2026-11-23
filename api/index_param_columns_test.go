package api

import (
	"context"
	"testing"
)

func TestValidateColumns(t *testing.T) {
	filterable := []string{"id", "name", "content", "created_at"}
	sortable := []string{"name", "created_at", "content"}

	tests := []struct {
		name    string
		filter  string
		sort    []string
		wantErr bool
	}{
		{name: "no filter or sort"},
		{name: "allowed filter with alias", filter: "result.name~'a%'"},
		{name: "allowed filter without alias", filter: "name='a'"},
		{name: "allowed filter with cast", filter: "result.created_at::date='2024-01-01'"},
		{name: "allowed jsonb filter", filter: "result.content.title='a'"},
		{name: "null and booleans are not columns", filter: "result.name=null && result.id!=true"},
		{name: "allowed sort", sort: []string{"name:asc", "created_at:desc"}},
		{name: "allowed jsonb sort", sort: []string{"content.title:asc"}},
		{name: "hidden column in filter", filter: "result.password_hash~'$2a%'", wantErr: true},
		{name: "hidden column on right side", filter: "result.name=result.password_hash", wantErr: true},
		{name: "hidden column in nested group", filter: "result.name='a' && (result.id='b' || password_hash='c')", wantErr: true},
		{name: "case does not bypass", filter: "result.PASSWORD_HASH='x'", wantErr: true},
		{name: "filterable but not sortable", sort: []string{"id:asc"}, wantErr: true},
		{name: "hidden column in sort", sort: []string{"password_hash:asc"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param, err := NewIndexParamWithContext(context.Background(), &IndexParam{Filter: tt.filter, Sort: tt.sort})
			if err != nil {
				t.Fatalf("NewIndexParamWithContext: %v", err)
			}
			err = param.ValidateColumns(filterable, sortable)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateColumns() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
