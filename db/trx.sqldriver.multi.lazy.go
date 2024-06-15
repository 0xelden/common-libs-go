package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/shared"
)

var (
	_ MultiTrxDriver = TrxSqlDriverMultiDBLazy{}

	ErrSqlTransactionClosed = errors.New("sql transaction already closed")
)

type trxSqlDriverMultiDBState struct {
	mu     sync.Mutex
	closed bool
}

type TrxSqlDriverMultiDBLazy struct {
	tx    map[string]*sql.Tx
	db    map[string]*sql.DB
	state *trxSqlDriverMultiDBState
}

func NewSqlTransactionMultiDBLazy(ctx context.Context, db map[string]*sql.DB) (*TrxSqlDriverMultiDBLazy, error) {
	if len(db) == 0 {
		return nil, errors.New("empty sql db map")
	}

	dbs := make(map[string]*sql.DB, len(db))
	for k, v := range db {
		if v == nil {
			return nil, fmt.Errorf("sql db %q is nil", k)
		}
		dbs[k] = v
	}

	trx := &TrxSqlDriverMultiDBLazy{
		tx:    make(map[string]*sql.Tx),
		db:    dbs,
		state: &trxSqlDriverMultiDBState{},
	}

	if err := trx.BeginContext(ctx); err != nil {
		return nil, err
	}

	return trx, nil
}

func (t TrxSqlDriverMultiDBLazy) SupportMultiDb() {
}

func (t TrxSqlDriverMultiDBLazy) getDBKey(ctx context.Context) string {
	if val := ctx.Value(shared.ExecuteIn); val != nil {
		if str, ok := val.(string); ok {
			if _, ok2 := t.db[str]; ok2 {
				helper.Trace(ctx, "sql.context."+str, true)
				return str
			}
		}
	}

	if val := ctx.Value(shared.X_PortalCode); val != nil {
		if str, ok := val.(string); ok {
			if _, ok2 := t.db[str]; ok2 {
				helper.Trace(ctx, "sql.context."+str, true)
				return str
			}
		}
	}

	helper.Trace(ctx, "sql.context."+shared.Default, true)
	return shared.Default
}

func (t TrxSqlDriverMultiDBLazy) getOne(ctx context.Context) (string, *sql.Tx) {
	key, tx, err := t.getOrBeginTx(ctx)
	if err != nil {
		panic(err)
	}

	return key, tx
}

// New helper: error-aware lazy transaction lookup.
func (t TrxSqlDriverMultiDBLazy) getOrBeginTx(ctx context.Context) (string, *sql.Tx, error) {
	if t.state == nil {
		return "", nil, errors.New("sql transaction driver is not initialized")
	}

	key := t.getDBKey(ctx)

	t.state.mu.Lock()
	defer t.state.mu.Unlock()

	if t.state.closed {
		return key, nil, ErrSqlTransactionClosed
	}

	if tx := t.tx[key]; tx != nil {
		return key, tx, nil
	}

	db := t.db[key]
	if db == nil {
		return key, nil, fmt.Errorf("sql db %q not found", key)
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: false})
	if err != nil {
		return key, nil, fmt.Errorf("begin sql transaction for db %q: %w", key, err)
	}

	t.tx[key] = tx

	return key, tx, nil
}

// BeginContext Optional public helper if callers want to force transaction creation earlier.
func (t TrxSqlDriverMultiDBLazy) BeginContext(ctx context.Context) error {
	_, _, err := t.getOrBeginTx(ctx)
	return err
}

func (t TrxSqlDriverMultiDBLazy) Tx(ctx context.Context) *sql.Tx {
	_, tx, err := t.getOrBeginTx(ctx)
	if err != nil {
		panic(err)
	}

	return tx
}

func (t TrxSqlDriverMultiDBLazy) Rollback() error {
	if t.state == nil {
		return nil
	}

	t.state.mu.Lock()

	if t.state.closed {
		t.state.mu.Unlock()
		return nil
	}

	t.state.closed = true

	txs := make(map[string]*sql.Tx, len(t.tx))
	for k, tx := range t.tx {
		txs[k] = tx
		delete(t.tx, k)
	}

	t.state.mu.Unlock()

	var rollbackErr error
	for key, tx := range txs {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			rollbackErr = fmt.Errorf("rollback sql transaction for db %q: %w", key, err)
		}
	}

	return rollbackErr
}

func (t TrxSqlDriverMultiDBLazy) Commit() error {
	if t.state == nil {
		return errors.New("sql transaction driver is not initialized")
	}

	t.state.mu.Lock()
	defer t.state.mu.Unlock()

	if t.state.closed {
		return ErrSqlTransactionClosed
	}

	for key, tx := range t.tx {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit sql transaction for db %q: %w", key, err)
		}
	}

	t.state.closed = true
	for k := range t.tx {
		delete(t.tx, k)
	}

	return nil
}

func (t TrxSqlDriverMultiDBLazy) PingContext(ctx context.Context) error {
	key := t.getDBKey(ctx)

	db := t.db[key]
	if db == nil {
		return fmt.Errorf("sql db %q not found", key)
	}

	return db.PingContext(ctx)
}

func (t TrxSqlDriverMultiDBLazy) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	_, tx, err := t.getOrBeginTx(ctx)
	if err != nil {
		return nil, err
	}

	return tx.ExecContext(ctx, query, args...)
}

func (t TrxSqlDriverMultiDBLazy) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	_, tx, err := t.getOrBeginTx(ctx)
	if err != nil {
		return nil, err
	}

	return tx.QueryContext(ctx, query, args...)
}

func (t TrxSqlDriverMultiDBLazy) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.Tx(ctx).QueryRowContext(ctx, query, args...)
}
