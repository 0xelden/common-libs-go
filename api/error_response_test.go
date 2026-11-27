package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0xelden/common-libs-go/serror"
	"github.com/0xelden/common-libs-go/shared"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func TestErrorHidesInfrastructureDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	leak := errors.New("dial tcp 10.0.3.15:5432: connect: connection refused")
	tests := []struct {
		name    string
		code    int
		err     error
		verbose bool
		want    string
	}{
		{name: "500 in production", code: http.StatusInternalServerError, err: leak, want: "Internal Server Error"},
		{name: "503 in production", code: http.StatusServiceUnavailable, err: leak, want: "Service Unavailable"},
		{name: "500 in verbose mode", code: http.StatusInternalServerError, err: leak, verbose: true, want: leak.Error()},
		{name: "400 keeps the user message", code: http.StatusBadRequest, err: errors.New("invalid id, expected to be a uuid"), want: "invalid id, expected to be a uuid"},
		{name: "403 keeps the user message", code: http.StatusForbidden, err: errors.New("don't have permission to access this resource"), want: "don't have permission to access this resource"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := shared.Staging
			shared.Staging = tt.verbose
			t.Cleanup(func() { shared.Staging = old })

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
			Error(c, tt.err, tt.code)

			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v\n%s", err, w.Body.String())
			}
			if w.Code != tt.code || body.Error != tt.want {
				t.Fatalf("status = %d error = %q, want %d %q", w.Code, body.Error, tt.code, tt.want)
			}
			if !tt.verbose && tt.code >= 500 && strings.Contains(w.Body.String(), "10.0.3.15") {
				t.Fatalf("response leaks the database address: %s", w.Body.String())
			}
		})
	}
}

func TestErrorTreatsInfrastructureFailuresAsInternal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dial := &net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("10.1.2.3"), Port: 5432}, Err: errors.New("connect: connection refused")}
	auth := &pq.Error{Code: "28P01", Message: `password authentication failed for user "cms_app"`}
	tests := []struct {
		name     string
		err      error
		verbose  bool
		wantCode int
		wantMsg  string
	}{
		{name: "dial error", err: dial, wantCode: http.StatusInternalServerError, wantMsg: "Internal Server Error"},
		{name: "postgres auth error", err: auth, wantCode: http.StatusInternalServerError, wantMsg: "Internal Server Error"},
		{name: "wrapped in serror", err: serror.NewFromError(fmt.Errorf("index example: %w", auth)), wantCode: http.StatusInternalServerError, wantMsg: "Internal Server Error"},
		{name: "deadline", err: context.DeadlineExceeded, wantCode: http.StatusInternalServerError, wantMsg: "Internal Server Error"},
		{name: "redis down", err: redis.ErrClosed, wantCode: http.StatusBadRequest, wantMsg: redis.ErrClosed.Error()},
		{name: "verbose keeps the message", err: auth, verbose: true, wantCode: http.StatusInternalServerError, wantMsg: auth.Error()},
		{name: "user error stays 400", err: serror.New("invalid id, expected to be a uuid"), wantCode: http.StatusBadRequest, wantMsg: "invalid id, expected to be a uuid"},
		{name: "missing redis key stays 400", err: redis.Nil, wantCode: http.StatusBadRequest, wantMsg: redis.Nil.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := shared.Staging
			shared.Staging = tt.verbose
			t.Cleanup(func() { shared.Staging = old })

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
			Error(c, tt.err)

			var body struct {
				Code  int    `json:"code"`
				Error string `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v\n%s", err, w.Body.String())
			}
			if w.Code != tt.wantCode || body.Code != tt.wantCode || body.Error != tt.wantMsg {
				t.Fatalf("status = %d code = %d error = %q, want %d %q", w.Code, body.Code, body.Error, tt.wantCode, tt.wantMsg)
			}
			if !tt.verbose && (strings.Contains(w.Body.String(), "10.1.2.3") || strings.Contains(w.Body.String(), "cms_app")) {
				t.Fatalf("response leaks infrastructure details: %s", w.Body.String())
			}
		})
	}
}
