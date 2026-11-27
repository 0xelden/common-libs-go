package api

import (
	"context"
	"crypto/x509"
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

func TestErrorMapsInputPostgresErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	unique := &pq.Error{Code: "23505", Message: `duplicate key value violates unique constraint "contents_slug_key"`, Constraint: "contents_slug_key", Table: "contents"}
	badUUID := &pq.Error{Code: "22P02", Message: `invalid input syntax for type uuid: "abc"`}
	tests := []struct {
		name     string
		err      error
		code     []int
		verbose  bool
		wantCode int
		wantMsg  string
	}{
		{name: "unique violation", err: unique, wantCode: http.StatusConflict, wantMsg: "Conflict"},
		{name: "foreign key violation", err: &pq.Error{Code: "23503", Message: `violates foreign key constraint "contents_category_fkey"`}, wantCode: http.StatusConflict, wantMsg: "Conflict"},
		{name: "invalid uuid", err: badUUID, wantCode: http.StatusBadRequest, wantMsg: "Bad Request"},
		{name: "value too long", err: &pq.Error{Code: "22001", Message: "value too long for type character varying(255)"}, wantCode: http.StatusBadRequest, wantMsg: "Bad Request"},
		{name: "handler passed 500", err: serror.NewFromError(fmt.Errorf("create content: %w", unique)), code: []int{http.StatusInternalServerError}, wantCode: http.StatusConflict, wantMsg: "Conflict"},
		{name: "connection failure stays 500", err: &pq.Error{Code: "08006", Message: "connection failure"}, wantCode: http.StatusInternalServerError, wantMsg: "Internal Server Error"},
		{name: "deadlock stays 500", err: &pq.Error{Code: "40P01", Message: "deadlock detected"}, wantCode: http.StatusInternalServerError, wantMsg: "Internal Server Error"},
		{name: "verbose keeps the message", err: unique, verbose: true, wantCode: http.StatusConflict, wantMsg: unique.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := shared.Staging
			shared.Staging = tt.verbose
			t.Cleanup(func() { shared.Staging = old })

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/x", nil)
			Error(c, tt.err, tt.code...)

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
			if !tt.verbose && (strings.Contains(w.Body.String(), "contents") || strings.Contains(w.Body.String(), "abc")) {
				t.Fatalf("response leaks the constraint or value: %s", w.Body.String())
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
		{name: "redis client closed", err: redis.ErrClosed, wantCode: http.StatusInternalServerError, wantMsg: "Internal Server Error"},
		{name: "redis pool timeout", err: fmt.Errorf("read session: %w", redis.ErrPoolTimeout), wantCode: http.StatusInternalServerError, wantMsg: "Internal Server Error"},
		{name: "tls unknown authority", err: fmt.Errorf("dial 10.1.2.3:6380: %w", x509.UnknownAuthorityError{}), wantCode: http.StatusInternalServerError, wantMsg: "Internal Server Error"},
		{name: "tls hostname mismatch", err: x509.HostnameError{Certificate: &x509.Certificate{}, Host: "10.1.2.3"}, wantCode: http.StatusInternalServerError, wantMsg: "Internal Server Error"},
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
