package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// Test cases for ViewParam validation
func TestViewParamValidation(t *testing.T) {
	validate := validator.New()

	testCases := []struct {
		name      string
		input     ViewParam
		expectErr bool
	}{
		{
			name: "Valid - Id provided, empty filters",
			input: ViewParam{
				Id:      "550e8400-e29b-41d4-a716-446655440000", // Valid UUID
				Filters: []string{},
			},
			expectErr: false,
		},
		{
			name: "Valid - Filters provided, empty Id",
			input: ViewParam{
				Id:      "",
				Filters: []string{"filter1", "filter2"},
			},
			expectErr: false,
		},
		{
			name: "Invalid - Empty Id and Filters",
			input: ViewParam{
				Id:      "",
				Filters: nil,
			},
			expectErr: true,
		},
		{
			name: "Invalid - Id provided but invalid UUID",
			input: ViewParam{
				Id:      "invalid-uuid",
				Filters: []string{},
			},
			expectErr: true,
		},
		{
			name: "Invalid - Empty filter value",
			input: ViewParam{
				Id:      "",
				Filters: []string{"filter1", ""},
			},
			expectErr: true,
		},
		{
			name: "Invalid - Empty filter with no Id",
			input: ViewParam{
				Id:      "",
				Filters: []string{""},
			},
			expectErr: true,
		},
		{
			name: "Valid - Id with filters (extra validation)",
			input: ViewParam{
				Id:      "550e8400-e29b-41d4-a716-446655440000",
				Filters: []string{"filter1", "filter2"},
			},
			expectErr: false,
		},
	}

	// Run each test case
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validate.Struct(tc.input)
			if tc.expectErr && err == nil {
				t.Errorf("Expected error but got none for case: %s", tc.name)
			}
			if !tc.expectErr && err != nil {
				t.Errorf("Unexpected error for case %s: %v", tc.name, err)
			}
		})
	}
}

func newViewParamTestContext(query string) *gin.Context {
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

func TestNewViewParam(t *testing.T) {
	type args struct {
		ctx     *gin.Context
		binding binding.Binding
		id      string
		filter  []string
	}
	tests := []struct {
		name    string
		args    args
		want    *ViewParam
		wantErr bool
	}{
		{
			name: "valid id only",
			args: args{
				ctx:     newViewParamTestContext(""),
				binding: binding.Query,
				id:      "550e8400-e29b-41d4-a716-446655440000",
			},
			want: &ViewParam{
				Id: "550e8400-e29b-41d4-a716-446655440000",
			},
		},
		{
			name: "valid variadic filters without id",
			args: args{
				ctx:     newViewParamTestContext(""),
				binding: binding.Query,
				filter:  []string{"status='active'", "type='customer'"},
			},
			want: &ViewParam{
				Filters: []string{"status='active'", "type='customer'"},
			},
		},
		{
			// Security regression: Filters is raw SQL and must not be bindable
			// from the request. This case used to bind ?filters= into the WHERE
			// clause; now it is ignored, leaving the request unconstrained (no
			// id, no filters), which validation correctly rejects.
			name: "query filters are ignored",
			args: args{
				ctx:     newViewParamTestContext("filters=status%3Dactive&filters=type%3Dcustomer"),
				binding: binding.Query,
			},
			wantErr: true,
		},
		{
			// The auth-bypass this closes: gin assigns a bound field whenever
			// the key is present, so ?filters= previously REPLACED the scoping
			// fragment the caller passed in, widening the query to any row.
			name: "query filters cannot override caller filters",
			args: args{
				ctx:     newViewParamTestContext("filters=1%3D1"),
				binding: binding.Query,
				id:      "550e8400-e29b-41d4-a716-446655440000",
				filter:  []string{"result.company_id = 'c1'"},
			},
			want: &ViewParam{
				Id:      "550e8400-e29b-41d4-a716-446655440000",
				Filters: []string{"result.company_id = 'c1'"},
			},
		},
		{
			name: "valid query columns with id",
			args: args{
				ctx:     newViewParamTestContext("column=name&column=email&item_column=item_name"),
				binding: binding.Query,
				id:      "550e8400-e29b-41d4-a716-446655440000",
			},
			want: &ViewParam{
				Id:         "550e8400-e29b-41d4-a716-446655440000",
				Column:     []string{"name", "email"},
				ItemColumn: []string{"item_name"},
			},
		},
		{
			name: "valid id with variadic filters",
			args: args{
				ctx:     newViewParamTestContext("column=name"),
				binding: binding.Query,
				id:      "550e8400-e29b-41d4-a716-446655440000",
				filter:  []string{"status='active'"},
			},
			want: &ViewParam{
				Id:      "550e8400-e29b-41d4-a716-446655440000",
				Column:  []string{"name"},
				Filters: []string{"status='active'"},
			},
		},
		{
			name: "invalid empty id and filters",
			args: args{
				ctx:     newViewParamTestContext(""),
				binding: binding.Query,
			},
			wantErr: true,
		},
		{
			name: "invalid id",
			args: args{
				ctx:     newViewParamTestContext(""),
				binding: binding.Query,
				id:      "invalid-uuid",
			},
			wantErr: true,
		},
		{
			name: "invalid empty variadic filter without id",
			args: args{
				ctx:     newViewParamTestContext(""),
				binding: binding.Query,
				filter:  []string{""},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewViewParam(tt.args.ctx, tt.args.binding, tt.args.id, tt.args.filter...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewViewParam() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewViewParam() got = %v, want %v", got, tt.want)
			}
		})
	}
}
