package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0xelden/common-libs-go/shared"
	"github.com/gin-gonic/gin"
)

func TestErrorIgnoresTraceQueryOutsideStaging(t *testing.T) {
	gin.SetMode(gin.TestMode)
	prev := shared.Staging
	t.Cleanup(func() { shared.Staging = prev })

	for _, staging := range []bool{false, true} {
		shared.Staging = staging
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/x?"+shared.X_Trace+"=1", nil)
		c.Set(shared.X_UserId, "user-1")

		Error(c, errors.New("boom"), http.StatusBadRequest)

		body := w.Body.String()
		leaked := strings.Contains(body, "user-1") || strings.Contains(body, `"trace"`)
		if leaked != staging {
			t.Fatalf("staging=%v: verbose fields leaked=%v\n%s", staging, leaked, body)
		}
	}
}
