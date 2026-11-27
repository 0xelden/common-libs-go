package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func TestAbortSessionLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	unreachable := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 200 * time.Millisecond, MaxRetries: -1})
	t.Cleanup(func() { _ = unreachable.Close() })
	_, dialErr := unreachable.Get(context.Background(), "session-x").Result()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "missing key", err: redis.Nil, want: http.StatusUnauthorized},
		{name: "wrapped missing key", err: fmt.Errorf("get: %w", redis.Nil), want: http.StatusUnauthorized},
		{name: "redis unreachable", err: dialErr, want: http.StatusServiceUnavailable},
		{name: "other error", err: errors.New("READONLY"), want: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("expected a Redis error")
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)

			abortSessionLookup(c, "session marker", tt.err)

			if w.Code != tt.want || !c.IsAborted() {
				t.Fatalf("status = %d aborted = %v, want %d and aborted", w.Code, c.IsAborted(), tt.want)
			}
		})
	}
}
