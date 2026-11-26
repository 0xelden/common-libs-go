package middleware

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xelden/common-libs-go/api"
	"github.com/0xelden/common-libs-go/db"
	"github.com/0xelden/common-libs-go/shared"
	"github.com/gin-gonic/gin"
)

type fakeTrx struct {
	db.Driver
	committed, rolledBack bool
}

func (f *fakeTrx) Commit() error              { f.committed = true; return nil }
func (f *fakeTrx) Rollback() error            { f.rolledBack = true; return nil }
func (f *fakeTrx) Tx(context.Context) *sql.Tx { return nil }

type fakeTrxBuilder struct {
	trx *fakeTrx
	err error
}

func (b fakeTrxBuilder) NewTransaction(context.Context) (db.TrxDriver, error) {
	if b.err != nil {
		return nil, b.err
	}
	return b.trx, nil
}

func TestRollbackOnPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	trx := &fakeTrx{}
	r := gin.New()
	r.Use(gin.Recovery(), RollbackOnPanic())
	r.POST("/x", func(c *gin.Context) {
		if _, _, err := api.BeginTrx(c, fakeTrxBuilder{trx: trx}); err != nil {
			t.Fatalf("BeginTrx: %v", err)
		}
		panic("boom")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if !trx.rolledBack || trx.committed {
		t.Fatalf("rolledBack=%v committed=%v, want rollback only", trx.rolledBack, trx.committed)
	}
}

func TestBeginTrxError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/x", nil)

	_, trx, err := api.BeginTrx(c, fakeTrxBuilder{err: errors.New("pool exhausted")})
	if err == nil || trx != nil {
		t.Fatalf("trx=%v err=%v, want nil trx and an error", trx, err)
	}
	if _, ok := c.Get(shared.TrxInstance); ok {
		t.Fatal("a failed transaction must not be stored on the context")
	}
}
