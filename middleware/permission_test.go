package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAccessRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(grantedPermissionsKey, grantedPermissions([]string{"create_x", "list_x"}))
		if !hasAccessRule(c) {
			c.AbortWithStatus(http.StatusForbidden)
		}
	})
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	otherGuard := func(c *gin.Context) {}

	r.POST("/granted", RequirePermission("create_x"), ok)
	r.POST("/any-of", RequirePermission("delete_x", "list_x"), ok)
	r.POST("/missing", RequirePermission("delete_x"), ok)
	r.GET("/login-only", RequireLogin(), ok)
	r.GET("/no-rule", ok)
	r.GET("/custom-guard", otherGuard, ok)

	cases := map[string]int{
		"POST /granted":     http.StatusOK,
		"POST /any-of":      http.StatusOK,
		"POST /missing":     http.StatusForbidden,
		"GET /login-only":   http.StatusOK,
		"GET /no-rule":      http.StatusForbidden,
		"GET /custom-guard": http.StatusForbidden,
	}
	for route, want := range cases {
		method, path, _ := strings.Cut(route, " ")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		if w.Code != want {
			t.Errorf("%s = %d, want %d", route, w.Code, want)
		}
	}
}

func TestRequirePermissionWithoutSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", RequirePermission("list_x"), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403", w.Code)
	}
}
