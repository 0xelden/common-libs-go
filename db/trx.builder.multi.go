package db

import (
	"context"
	"database/sql"
)

var (
	_ MultiTrxBuilder = TrxBuilderMulti{}
)

type TrxBuilderMulti struct {
	db map[string]*sql.DB
}

func NewTrxBuilderMulti(DB map[string]*sql.DB) TrxBuilderMulti {
	return TrxBuilderMulti{db: DB}
}

func (t TrxBuilderMulti) SupportMultiDb() {
}

func (t TrxBuilderMulti) NewTransaction(ctx context.Context) (trx TrxDriver, err error) {
	return NewSqlTransactionMultiDB(ctx, t.db)
}
