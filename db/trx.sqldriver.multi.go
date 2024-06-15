package db

import (
	"context"
	"database/sql"

	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/shared"
)

var (
	_ MultiTrxDriver = TrxSqlDriverMultiDB{}
)

type TrxSqlDriverMultiDB struct {
	tx map[string]*sql.Tx
	db map[string]*sql.DB
}

func NewSqlTransactionMultiDB(ctx context.Context, db map[string]*sql.DB) (*TrxSqlDriverMultiDB, error) {
	txs := map[string]*sql.Tx{}
	for k, v := range db {
		tx, err := v.BeginTx(ctx, &sql.TxOptions{ReadOnly: false})
		if err != nil {
			return nil, err
		}
		txs[k] = tx
	}
	return &TrxSqlDriverMultiDB{tx: txs, db: db}, nil
}

func (t TrxSqlDriverMultiDB) SupportMultiDb() {
}

func (t TrxSqlDriverMultiDB) getOne(ctx context.Context) (string, *sql.Tx) {
	if val := ctx.Value(shared.ExecuteIn); val != nil {
		if str, ok := val.(string); ok {
			if tx, ok2 := t.tx[str]; ok2 {
				helper.Trace(ctx, "sql.context."+str, true)
				return str, tx
			}
		}
	}
	if val := ctx.Value(shared.X_PortalCode); val != nil {
		if str, ok := val.(string); ok {
			if tx, ok2 := t.tx[str]; ok2 {
				helper.Trace(ctx, "sql.context."+str, true)
				return str, tx
			}
		}
	}
	helper.Trace(ctx, "sql.context."+shared.Default, true)
	return shared.Default, t.tx[shared.Default]
}

func (t TrxSqlDriverMultiDB) Tx(ctx context.Context) *sql.Tx {
	return helper.Second(t.getOne(ctx))
}

func (t TrxSqlDriverMultiDB) Rollback() (err error) {
	for _, tx := range t.tx {
		_ = tx.Rollback()
	}
	return nil
}

func (t TrxSqlDriverMultiDB) Commit() (err error) {
	for _, tx := range t.tx {
		err = tx.Commit()
		if err != nil {
			return err
		}
	}
	return nil
}

func (t TrxSqlDriverMultiDB) PingContext(ctx context.Context) error {
	return t.db[helper.First(t.getOne(ctx))].Ping()
}

func (t TrxSqlDriverMultiDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.Tx(ctx).ExecContext(ctx, query, args...)
}

func (t TrxSqlDriverMultiDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.Tx(ctx).QueryContext(ctx, query, args...)
}

func (t TrxSqlDriverMultiDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.Tx(ctx).QueryRowContext(ctx, query, args...)
}
