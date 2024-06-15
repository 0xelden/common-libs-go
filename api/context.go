package api

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/0xelden/common-libs-go/db"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/shared"
)

func defaultContextValue(c *gin.Context) [28]string {
	def := [28]string{
		shared.X_Portal, c.GetString(shared.X_Portal),
		shared.X_PortalCode, c.GetString(shared.X_PortalCode),
		shared.X_RoleId, c.GetString(shared.X_RoleId),
		shared.X_UserId, c.GetString(shared.X_UserId),
		shared.X_Level, c.GetString(shared.X_Level),
		shared.X_Action, c.GetString(shared.X_Action),
		shared.X_UserName, c.GetString(shared.X_UserName),
		shared.X_UserEmail, c.GetString(shared.X_UserEmail),
		shared.X_UserPhone, c.GetString(shared.X_UserPhone),
		shared.X_Trace, c.GetString(shared.X_Trace),
		shared.Wsk, c.GetString(shared.Wsk),
	}
	return def
}

// SetGrpcContext read gin context and set grpc context value
// Deprecated: we do not do that here
//
//goland:noinspection GoUnusedExportedFunction
func SetGrpcContext(c *gin.Context, kv ...string) context.Context {
	ctx := SetContext(c, kv...)
	return helper.GrpcContextSetString(ctx, kv...)
}

// SetContext read gin context and set native context value
func SetContext(c *gin.Context, kv ...string) context.Context {
	ctx := c.Request.Context()
	def := defaultContextValue(c)
	pair := append(def[:], kv...)
	for i := 0; i < len(pair); i += 2 {
		ctx = context.WithValue(ctx, pair[i], pair[i+1])
	}
	c.Request = c.Request.WithContext(ctx) // save filled kv to gin request context
	return ctx
}

// SetCtxAndTrx read gin context to set transaction and native context value
func SetCtxAndTrx(c *gin.Context, txb db.TrxBuilder, kv ...string) (context.Context, db.TrxDriver) {
	ctx := SetContext(c, kv...)
	trx, err := txb.NewTransaction(ctx)
	if err != nil {
		helper.Trace(ctx, "SetCtxAndTrx.error", err)
	}
	c.Set(shared.TrxInstance, trx)
	return ctx, trx
}

// SetReplicaContext behaves like SetContext but also routes the read through
// the current request's portal replica, via shared.ExecuteIn. Use it for
// read-only Index/View handlers backed by a Driver built from
// connection.ConnectAllPgsqlReplica.
//
// A bare shared.ExecuteIn="replica" would ignore shared.X_PortalCode (getOne
// checks ExecuteIn first) and always hit the default portal's replica, so
// this folds the request's portal code into the key: "<portal>:replica",
// falling back to shared.Default when no portal code is set.
func SetReplicaContext(c *gin.Context, kv ...string) context.Context {
	code := c.GetString(shared.X_PortalCode)
	if code == "" {
		code = shared.Default
	}
	return SetContext(c, append(kv, shared.ExecuteIn, code+":replica")...)
}
