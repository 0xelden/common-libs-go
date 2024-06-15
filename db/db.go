package db

import (
	"context"
	"database/sql"
)

// Driver adalah generic sql driver yang bersifat long lived
type Driver interface {
	PingContext(ctx context.Context) error
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Rollbacker interface {
	Rollback() error
}

type Committer interface {
	Commit() error
}

// TrxDriver bentuk transactional dari Driver interface yang lifecycle-nya jauh lebih cepat, yaitu per request
// BeginTx diharuskan secara implisit sudah terpanggil ketika konstruktor dipanggil
type TrxDriver interface {
	Driver
	Committer
	Rollbacker
	Tx(ctx context.Context) *sql.Tx
}

// TrxBuilder adalah abstract factory atau interface builder untuk TrxDriver interface
type TrxBuilder interface {
	// NewTransaction membuat transaksi baru dan memanggil BeginTx
	NewTransaction(ctx context.Context) (TrxDriver, error)
}

type SupportMultiDb interface {
	SupportMultiDb()
}

type MultiDriver interface {
	Driver
	SupportMultiDb
}

type MultiTrxDriver interface {
	TrxDriver
	SupportMultiDb
}

type MultiTrxBuilder interface {
	SupportMultiDb
	NewTransaction(ctx context.Context) (TrxDriver, error)
}
