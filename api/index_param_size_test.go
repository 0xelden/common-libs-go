package api

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/0xelden/common-libs-go/shared"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

func newIndexParamFromQuery(t *testing.T, query string, allData bool) *IndexParam {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?"+query, nil)
	if allData {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), shared.AllData, "1"))
	}
	param, err := NewIndexParam(c, binding.Query)
	if err != nil {
		t.Fatalf("NewIndexParam(%q) returned error: %v", query, err)
	}
	return param
}

func TestNewIndexParamSizeLimits(t *testing.T) {
	tests := []struct {
		query    string
		wantSize int
		wantPage int
	}{
		{"", 10, 1},
		{"size=25&page=3", 25, 3},
		{"size=100", 100, 1},
		{"size=1000000", maxRequestPageSize, 1},
		{"size=-5", 10, 1},
		{"page=0", 10, 1},
		{"page=-2", 10, 1},
		{"omit_total=1", maxPageSize, 1},
		{"omit_total=1&size=300", 300, 1},
		{"omit_total=1&size=1000000", maxPageSize, 1},
	}
	for _, tt := range tests {
		param := newIndexParamFromQuery(t, tt.query, false)
		if param.Size != tt.wantSize || param.Page != tt.wantPage {
			t.Errorf("%q: got size=%d page=%d, want size=%d page=%d", tt.query, param.Size, param.Page, tt.wantSize, tt.wantPage)
		}
	}
}

func TestNewIndexParamExportKeepsExportSize(t *testing.T) {
	param := newIndexParamFromQuery(t, "size=5&page=4", true)
	if param.Size != maxPageSizeExport || param.Page != 1 {
		t.Fatalf("got size=%d page=%d, want size=%d page=1", param.Size, param.Page, maxPageSizeExport)
	}
}
