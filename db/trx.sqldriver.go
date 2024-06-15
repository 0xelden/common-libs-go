package db

import (
	"context"
	"database/sql"
	"errors"
)

var (
	_ TrxDriver = TrxSqlDriver{}
)

type TrxSqlDriver struct {
	tx *sql.Tx
	db *sql.DB
}

func NewSqlTransaction(ctx context.Context, DB *sql.DB) (*TrxSqlDriver, error) {
	if DB == nil {
		return nil, errors.New("invalid *sql.DB param")
	}
	tx, err := DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: false})
	if err != nil {
		return nil, err
	}
	return &TrxSqlDriver{tx: tx, db: DB}, nil
}

func (t TrxSqlDriver) Tx(ctx context.Context) *sql.Tx {
	return t.tx
}

func (t TrxSqlDriver) Rollback() error {
	return t.tx.Rollback()
}

func (t TrxSqlDriver) Commit() error {
	return t.tx.Commit()
}

func (t TrxSqlDriver) PingContext(ctx context.Context) error {
	return t.db.Ping()
}

func (t TrxSqlDriver) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (t TrxSqlDriver) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

func (t TrxSqlDriver) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}
