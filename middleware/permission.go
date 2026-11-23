package middleware

import (
	"net/http"
	"reflect"
	"runtime"

	"github.com/0xelden/common-libs-go/api"
	"github.com/gin-gonic/gin"
	"github.com/go-errors/errors"
)

const grantedPermissionsKey = "x_granted_permissions"

type permissionRule struct {
	slugs []string
}

func (p permissionRule) handle(c *gin.Context) {
	granted, _ := c.Get(grantedPermissionsKey)
	set, _ := granted.(map[string]struct{})
	for _, slug := range p.slugs {
		if _, ok := set[slug]; ok {
			return
		}
	}
	api.Error(c, errors.New("don't have permission to access this resource"), http.StatusForbidden)
	c.Abort()
}

func requireLogin(*gin.Context) {}

// RequirePermission allows the route when the session holds at least one of
// slugs. GuardRoute rejects any non-public route that declares neither this
// nor RequireLogin.
func RequirePermission(slugs ...string) gin.HandlerFunc {
	return permissionRule{slugs: slugs}.handle
}

// RequireLogin marks a route that only needs an authenticated session.
func RequireLogin() gin.HandlerFunc {
	return requireLogin
}

var accessRuleNames = map[string]struct{}{
	handlerName(RequirePermission()): {},
	handlerName(RequireLogin()):      {},
}

func handlerName(h gin.HandlerFunc) string {
	return runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
}

func hasAccessRule(c *gin.Context) bool {
	for _, name := range c.HandlerNames() {
		if _, ok := accessRuleNames[name]; ok {
			return true
		}
	}
	return false
}

func grantedPermissions(permissions []string) map[string]struct{} {
	set := make(map[string]struct{}, len(permissions))
	for _, p := range permissions {
		if p != "" {
			set[p] = struct{}{}
		}
	}
	return set
}
