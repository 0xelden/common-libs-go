package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const wsTestID = "6f1c2d3e-4a5b-4c6d-8e9f-0a1b2c3d4e5f"

func serveWsCredential(t *testing.T, env string, prepare func(*http.Request)) http.Header {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", env)
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 50 * time.Millisecond})
	t.Cleanup(func() { _ = client.Close() })

	r := gin.New()
	r.Use(SetWsCredential(client, "wsk"))
	r.GET("/ws", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	prepare(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("request without a stored token reached the handler")
	}
	return w.Header()
}

func TestWsCredentialShortProtocolHeader(t *testing.T) {
	for _, proto := range []string{"a", "1234567", "12345678"} {
		h := serveWsCredential(t, "production", func(r *http.Request) { r.Header.Set("Sec-WebSocket-Protocol", proto) })
		if got := h.Get("Sec-WebSocket-Protocol"); got != "" {
			t.Errorf("protocol %q was accepted as id %q", proto, got)
		}
	}
}

func TestWsCredentialProtocolHeader(t *testing.T) {
	h := serveWsCredential(t, "production", func(r *http.Request) { r.Header.Set("Sec-WebSocket-Protocol", "access, "+wsTestID) })
	if got := h.Get("Sec-WebSocket-Protocol"); got != wsTestID {
		t.Fatalf("id from header = %q, want %q", got, wsTestID)
	}
}

func TestWsCredentialDebugKeyOnlyLocal(t *testing.T) {
	query := func(r *http.Request) { r.URL.RawQuery = "wsk-debug-key=" + wsTestID }
	for env, want := range map[string]string{"production": "", "staging": "", "local": wsTestID} {
		if got := serveWsCredential(t, env, query).Get("Sec-WebSocket-Protocol"); got != want {
			t.Errorf("APP_ENV=%s: debug key id = %q, want %q", env, got, want)
		}
	}
}
