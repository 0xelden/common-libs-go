package db

import (
	"context"
	"database/sql"
)

var (
	_ TrxBuilder = TxBuilder{}
)

type TxBuilder struct {
	db *sql.DB
}

func NewTrxBuilder(db *sql.DB) TxBuilder {
	return TxBuilder{db: db}
}

func (t TxBuilder) NewTransaction(ctx context.Context) (trx TrxDriver, err error) {
	return NewSqlTransaction(ctx, t.db)
}
