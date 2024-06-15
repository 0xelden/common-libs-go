package db

import (
	"context"
	"database/sql"

	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/shared"
)

var (
	_ MultiDriver = SqlDriverMulti{}
)

type SqlDriverMulti struct {
	db map[string]*sql.DB
}

func NewSqlDriverMulti(DB map[string]*sql.DB) (*SqlDriverMulti, error) {
	return &SqlDriverMulti{db: DB}, nil
}

func (t SqlDriverMulti) SupportMultiDb() {
}

func (t SqlDriverMulti) getOne(ctx context.Context) *sql.DB {
	if val := ctx.Value(shared.ExecuteIn); val != nil {
		if str, ok := val.(string); ok {
			if tx, ok2 := t.db[str]; ok2 {
				helper.Trace(ctx, "sql.context."+str, true)
				return tx
			}
		}
	}
	if val := ctx.Value(shared.X_PortalCode); val != nil {
		if str, ok := val.(string); ok {
			if tx, ok2 := t.db[str]; ok2 {
				helper.Trace(ctx, "sql.context."+str, true)
				return tx
			}
		}
	}
	helper.Trace(ctx, "sql.context."+shared.Default, true)
	return t.db[shared.Default]
}

func (t SqlDriverMulti) PingContext(ctx context.Context) error {
	return t.getOne(ctx).Ping()
}

func (t SqlDriverMulti) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.getOne(ctx).ExecContext(ctx, query, args...)
}

func (t SqlDriverMulti) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.getOne(ctx).QueryContext(ctx, query, args...)
}

func (t SqlDriverMulti) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.getOne(ctx).QueryRowContext(ctx, query, args...)
}
