package api

import (
	"net/http/httptest"
	"testing"

	authModels "github.com/0xelden/common-libs-go/models/auth"
	"github.com/0xelden/common-libs-go/shared"
	"github.com/gin-gonic/gin"
)

func newTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	return c
}

// The role list is set on the gin context by GuardRoute and has to survive the
// hop into the native context, which is what repositories and anything
// downstream actually read from.
func TestSetContextPropagatesRoleIds(t *testing.T) {
	c := newTestContext(t)
	ids := []string{"c0c5ecb7-330b-4e96-8a92-5a99dd5529e3", "1b2c3d4e-0000-4000-8000-000000000001"}
	c.Set(shared.X_RoleIds, authModels.JoinRoleIds(ids))

	ctx := SetContext(c)

	got := authModels.RoleIdsFromContext(ctx)
	if len(got) != len(ids) {
		t.Fatalf("RoleIdsFromContext() = %v, want %v", got, ids)
	}
	for i := range got {
		if got[i] != ids[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], ids[i])
		}
	}
}

func TestSetContextWithNoRoles(t *testing.T) {
	c := newTestContext(t)

	if got := authModels.RoleIdsFromContext(SetContext(c)); got != nil {
		t.Errorf("RoleIdsFromContext() = %v, want nil when no roles are set", got)
	}
}

// defaultContextValue is a fixed-size array, so a pair added to it without
// resizing would silently leave empty slots that SetContext writes to the
// context as an empty key. Guards the sizing rather than the values.
func TestDefaultContextValueHasNoEmptySlots(t *testing.T) {
	c := newTestContext(t)
	c.Set(shared.X_Portal, "p")

	def := defaultContextValue(c)
	if len(def)%2 != 0 {
		t.Fatalf("defaultContextValue length = %d, want an even number of key/value entries", len(def))
	}
	for i := 0; i < len(def); i += 2 {
		if def[i] == "" {
			t.Errorf("entry %d has an empty key: array is larger than the pairs it holds", i)
		}
	}
}
