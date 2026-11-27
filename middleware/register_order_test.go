package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterServiceRejectsEarlierRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))

	early := gin.New()
	early.Group("/example").GET("/index", func(c *gin.Context) { c.Status(http.StatusOK) })
	if err := RegisterService("cms:/cms", early, WithoutTelemetry()); err == nil || !strings.Contains(err.Error(), "/example/index") {
		t.Fatalf("RegisterService after a route: err = %v, want it to name /example/index", err)
	}

	engine := gin.New()
	if err := RegisterService("cms:/cms", engine, WithoutTelemetry()); err != nil {
		t.Fatalf("RegisterService: %v", err)
	}
	engine.Group("/example").GET("/index", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/example/index", nil)
	req.Header.Set("X-Portal", "cms.example.com")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("route after RegisterService without a token: status %d, want 401", w.Code)
	}
}
