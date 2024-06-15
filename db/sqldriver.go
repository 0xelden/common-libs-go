package db

import (
	"context"
	"database/sql"
)

var (
	_ Driver = SqlDriver{}
)

type SqlDriver struct {
	db *sql.DB
}

func NewSqlDriver(DB *sql.DB) (*SqlDriver, error) {
	return &SqlDriver{db: DB}, nil
}

func (t SqlDriver) PingContext(ctx context.Context) error {
	return t.db.Ping()
}

func (t SqlDriver) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.db.ExecContext(ctx, query, args...)
}

func (t SqlDriver) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.db.QueryContext(ctx, query, args...)
}

func (t SqlDriver) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.db.QueryRowContext(ctx, query, args...)
}
