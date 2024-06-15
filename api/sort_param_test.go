package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func newSortParamTestContext(query string) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	target := "/"
	if query != "" {
		target += "?" + query
	}
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
	return ctx
}

func TestSortParamGenerateSortStmt(t *testing.T) {
	cases := []struct {
		name    string
		sort    []string
		want    string
		wantErr bool
	}{
		{
			name: "single asc",
			sort: []string{"name:asc"},
			want: `ORDER BY "name" ASC`,
		},
		{
			name: "single desc",
			sort: []string{"name:desc"},
			want: `ORDER BY "name" DESC`,
		},
		{
			name: "trim spaces",
			sort: []string{"  name  :  DESC  "},
			want: `ORDER BY "name" DESC`,
		},
		{
			name: "unknown order defaults asc",
			sort: []string{"name:foo"},
			want: `ORDER BY "name" ASC`,
		},
		{
			name: "multiple columns",
			sort: []string{"name:desc", "created_at:asc"},
			want: `ORDER BY "name" DESC, "created_at" ASC`,
		},
		{
			name:    "invalid missing colon",
			sort:    []string{"name"},
			wantErr: true,
		},
		{
			name:    "invalid empty column",
			sort:    []string{":desc"},
			wantErr: true,
		},
		{
			name:    "invalid extra colon",
			sort:    []string{"a:b:c"},
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			param := &SortParam{Sort: c.sort}
			got, err := param.generateSortStmt()
			if (err != nil) != c.wantErr {
				t.Fatalf("generateSortStmt() error = %v, wantErr %v", err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			if got != c.want {
				t.Fatalf("generateSortStmt() = %v, want %v", got, c.want)
			}
			param.SortStmt = got
			if param.GenerateSortStmt() != got {
				t.Fatalf("GenerateSortStmt() = %v, want %v", param.GenerateSortStmt(), got)
			}
		})
	}
}

func TestNewSortParam(t *testing.T) {
	cases := []struct {
		name      string
		queryKey  string
		query     string
		wantSort  []string
		wantStmt  string
		expectErr bool
	}{
		{
			name:     "empty sort",
			queryKey: "sort",
			query:    "",
			wantStmt: "",
		},
		{
			name:     "single sort",
			queryKey: "sort",
			query:    "sort=name:desc",
			wantSort: []string{"name:desc"},
			wantStmt: `ORDER BY "name" DESC`,
		},
		{
			name:     "multiple sorts",
			queryKey: "sort",
			query:    "sort=name:asc&sort=created_at:desc",
			wantSort: []string{"name:asc", "created_at:desc"},
			wantStmt: `ORDER BY "name" ASC, "created_at" DESC`,
		},
		{
			name:      "invalid sort",
			queryKey:  "sort",
			query:     "sort=name",
			expectErr: true,
		},
		{
			name:     "custom query key",
			queryKey: "order",
			query:    "order=updated_at:asc",
			wantSort: []string{"updated_at:asc"},
			wantStmt: `ORDER BY "updated_at" ASC`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := newSortParamTestContext(c.query)
			got, err := NewSortParam(ctx, c.queryKey)
			if (err != nil) != c.expectErr {
				t.Fatalf("NewSortParam() error = %v, expectErr %v", err, c.expectErr)
			}
			if c.expectErr {
				if got.Sort != nil {
					t.Fatalf("NewSortParam() = %v, want nil", got)
				}
				return
			}
			if len(c.wantSort) > 0 && !reflect.DeepEqual(got.Sort, c.wantSort) {
				t.Fatalf("NewSortParam() Sort = %v, want %v", got.Sort, c.wantSort)
			}
			if len(c.wantSort) == 0 && len(got.Sort) != 0 {
				t.Fatalf("NewSortParam() Sort = %v, want empty", got.Sort)
			}
			if got.SortStmt != c.wantStmt {
				t.Fatalf("NewSortParam() SortStmt = %v, want %v", got.SortStmt, c.wantStmt)
			}
		})
	}
}
