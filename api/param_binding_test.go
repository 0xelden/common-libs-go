package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

func newBindingTestContext(query string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x?"+query, nil)
	return c, w
}

func assertUntouched(t *testing.T, c *gin.Context) {
	t.Helper()
	if c.IsAborted() || c.Writer.Written() || c.Writer.Status() != http.StatusOK {
		t.Fatalf("binding wrote the response: aborted=%v written=%v status=%d", c.IsAborted(), c.Writer.Written(), c.Writer.Status())
	}
}

func TestNewIndexParamReturnsBindingErrors(t *testing.T) {
	c, _ := newBindingTestContext("page=abc")
	param, err := NewIndexParam(c, binding.Query)
	if err == nil || param != nil {
		t.Fatalf("NewIndexParam(page=abc) = %+v, %v; want an error", param, err)
	}
	assertUntouched(t, c)
}

func TestNewSortParamBindingBindsThroughAPointer(t *testing.T) {
	c, _ := newBindingTestContext("sort=name:asc")
	param, err := NewSortParamBinding(c, binding.Query)
	if err != nil {
		t.Fatalf("NewSortParamBinding: %v", err)
	}
	if param.SortStmt != `ORDER BY "name" ASC` {
		t.Fatalf("SortStmt = %q", param.SortStmt)
	}
	assertUntouched(t, c)
}

type failingBinding struct{}

func (failingBinding) Name() string                  { return "failing" }
func (failingBinding) Bind(*http.Request, any) error { return errBadBinding }

var errBadBinding = errors.New("bad binding")

func TestNewViewParamReturnsBindingErrors(t *testing.T) {
	c, _ := newBindingTestContext("")
	if _, err := NewViewParam(c, failingBinding{}, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, errBadBinding) {
		t.Fatalf("NewViewParam error = %v, want %v", err, errBadBinding)
	}
	assertUntouched(t, c)
}
