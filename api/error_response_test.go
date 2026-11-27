package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0xelden/common-libs-go/shared"
	"github.com/gin-gonic/gin"
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
