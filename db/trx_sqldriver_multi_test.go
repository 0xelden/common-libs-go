package db_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/0xelden/common-libs-go/db"
	"github.com/0xelden/common-libs-go/shared"
)

const multiTrxTestDriverName = "common_multi_trx_test"

var (
	multiTrxTestDriverOnce sync.Once
	multiTrxTestRegistry   = struct {
		sync.Mutex
		db map[string]*multiTrxTestDBState
	}{db: map[string]*multiTrxTestDBState{}}
	multiTrxTestDBSeq atomic.Uint64
)

type multiTrxTestDBState struct {
	mu sync.Mutex

	begin    int
	commit   int
	rollback int
	exec     int
	query    int

	failCommit bool
}

func (s *multiTrxTestDBState) snapshot() multiTrxTestDBState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return multiTrxTestDBState{
		begin:    s.begin,
		commit:   s.commit,
		rollback: s.rollback,
		exec:     s.exec,
		query:    s.query,
	}
}

func (s *multiTrxTestDBState) setFailCommit(fail bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failCommit = fail
}

type multiTrxDriverCase struct {
	name           string
	expectedClosed error
	new            func(context.Context, map[string]*sql.DB) (db.TrxDriver, error)
}

func multiTrxDriverCases() []multiTrxDriverCase {
	return []multiTrxDriverCase{
		{
			name:           "eager",
			expectedClosed: sql.ErrTxDone,
			new: func(ctx context.Context, dbs map[string]*sql.DB) (db.TrxDriver, error) {
				return db.NewSqlTransactionMultiDB(ctx, dbs)
			},
		},
		{
			name:           "lazy",
			expectedClosed: db.ErrSqlTransactionClosed,
			new: func(ctx context.Context, dbs map[string]*sql.DB) (db.TrxDriver, error) {
				return db.NewSqlTransactionMultiDBLazy(ctx, dbs)
			},
		},
	}
}

func TestSqlTransactionMultiDBBeginTiming(t *testing.T) {
	for _, tc := range multiTrxDriverCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			dbs, states := newMultiTrxTestDBs(t, shared.Default, "tenant")

			trx, err := tc.new(ctx, dbs)
			if err != nil {
				t.Fatalf("new transaction: %v", err)
			}
			defer trx.Rollback()

			wantDefaultBegins := map[string]int{
				"eager": 1,
				"lazy":  1,
			}[tc.name]
			wantTenantBegins := map[string]int{
				"eager": 1,
				"lazy":  0,
			}[tc.name]
			assertBeginCount(t, states, shared.Default, wantDefaultBegins)
			assertBeginCount(t, states, "tenant", wantTenantBegins)
		})
	}
}

func TestSqlTransactionMultiDBLazyBeginUsesConstructorContext(t *testing.T) {
	ctx := db.ExecuteIn(context.Background(), "tenant")
	dbs, states := newMultiTrxTestDBs(t, shared.Default, "tenant")

	trx, err := db.NewSqlTransactionMultiDBLazy(ctx, dbs)
	if err != nil {
		t.Fatalf("new lazy transaction: %v", err)
	}
	defer trx.Rollback()

	assertBeginCount(t, states, shared.Default, 0)
	assertBeginCount(t, states, "tenant", 1)
}

func TestSqlTransactionMultiDBReusesTransactionForSameDBKey(t *testing.T) {
	for _, tc := range multiTrxDriverCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			dbs, states := newMultiTrxTestDBs(t, shared.Default)

			trx, err := tc.new(ctx, dbs)
			if err != nil {
				t.Fatalf("new transaction: %v", err)
			}
			defer trx.Rollback()

			if _, err := trx.ExecContext(ctx, "INSERT INTO test_table(id) VALUES(1)"); err != nil {
				t.Fatalf("first exec: %v", err)
			}
			if _, err := trx.ExecContext(ctx, "INSERT INTO test_table(id) VALUES(2)"); err != nil {
				t.Fatalf("second exec: %v", err)
			}

			assertBeginCount(t, states, shared.Default, 1)
		})
	}
}

func TestSqlTransactionMultiDBUsesSeparateTransactionForDifferentDBKeys(t *testing.T) {
	for _, tc := range multiTrxDriverCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			tenantCtx := db.ExecuteIn(ctx, "tenant")
			dbs, states := newMultiTrxTestDBs(t, shared.Default, "tenant")

			trx, err := tc.new(ctx, dbs)
			if err != nil {
				t.Fatalf("new transaction: %v", err)
			}
			defer trx.Rollback()

			if _, err := trx.ExecContext(ctx, "INSERT INTO test_table(id) VALUES(1)"); err != nil {
				t.Fatalf("default exec: %v", err)
			}
			if _, err := trx.ExecContext(tenantCtx, "INSERT INTO test_table(id) VALUES(2)"); err != nil {
				t.Fatalf("tenant exec: %v", err)
			}

			assertBeginCount(t, states, shared.Default, 1)
			assertBeginCount(t, states, "tenant", 1)
		})
	}
}

func TestSqlTransactionMultiDBRollbackAfterNoUsageSucceeds(t *testing.T) {
	for _, tc := range multiTrxDriverCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			dbs, states := newMultiTrxTestDBs(t, shared.Default, "tenant")

			trx, err := tc.new(ctx, dbs)
			if err != nil {
				t.Fatalf("new transaction: %v", err)
			}

			if err := trx.Rollback(); err != nil {
				t.Fatalf("rollback: %v", err)
			}

			wantRollbacks := map[string]int{
				"eager": 2,
				"lazy":  1,
			}[tc.name]
			if got := totalRollbacks(states); got != wantRollbacks {
				t.Fatalf("rollback count = %d, want %d", got, wantRollbacks)
			}
		})
	}
}

func TestSqlTransactionMultiDBCommitAfterNoUsageSucceeds(t *testing.T) {
	for _, tc := range multiTrxDriverCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			dbs, states := newMultiTrxTestDBs(t, shared.Default, "tenant")

			trx, err := tc.new(ctx, dbs)
			if err != nil {
				t.Fatalf("new transaction: %v", err)
			}

			if err := trx.Commit(); err != nil {
				t.Fatalf("commit: %v", err)
			}

			wantCommits := map[string]int{
				"eager": 2,
				"lazy":  1,
			}[tc.name]
			if got := totalCommits(states); got != wantCommits {
				t.Fatalf("commit count = %d, want %d", got, wantCommits)
			}
		})
	}
}

func TestSqlTransactionMultiDBCommitFailureLeavesRemainingTransactionsRollbackable(t *testing.T) {
	for _, tc := range multiTrxDriverCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			tenantCtx := db.ExecuteIn(ctx, "tenant")
			dbs, states := newMultiTrxTestDBs(t, shared.Default, "tenant")
			for _, state := range states {
				state.setFailCommit(true)
			}

			trx, err := tc.new(ctx, dbs)
			if err != nil {
				t.Fatalf("new transaction: %v", err)
			}

			if _, err := trx.ExecContext(ctx, "INSERT INTO test_table(id) VALUES(1)"); err != nil {
				t.Fatalf("default exec: %v", err)
			}
			if _, err := trx.ExecContext(tenantCtx, "INSERT INTO test_table(id) VALUES(2)"); err != nil {
				t.Fatalf("tenant exec: %v", err)
			}

			if err := trx.Commit(); err == nil {
				t.Fatal("commit error = nil, want error")
			}

			if err := trx.Rollback(); err != nil {
				t.Fatalf("rollback after failed commit: %v", err)
			}

			if got := totalRollbacks(states); got != 1 {
				t.Fatalf("rollback count after failed commit = %d, want 1", got)
			}
		})
	}
}

func TestSqlTransactionMultiDBOperationsAfterCloseReturnError(t *testing.T) {
	for _, tc := range multiTrxDriverCases() {
		t.Run(tc.name+"/commit", func(t *testing.T) {
			ctx := context.Background()
			dbs, _ := newMultiTrxTestDBs(t, shared.Default)

			trx, err := tc.new(ctx, dbs)
			if err != nil {
				t.Fatalf("new transaction: %v", err)
			}
			if _, err := trx.ExecContext(ctx, "INSERT INTO test_table(id) VALUES(1)"); err != nil {
				t.Fatalf("exec before commit: %v", err)
			}
			if err := trx.Commit(); err != nil {
				t.Fatalf("commit: %v", err)
			}

			_, err = trx.ExecContext(ctx, "INSERT INTO test_table(id) VALUES(2)")
			if !errors.Is(err, tc.expectedClosed) {
				t.Fatalf("exec after commit error = %v, want %v", err, tc.expectedClosed)
			}
		})

		t.Run(tc.name+"/rollback", func(t *testing.T) {
			ctx := context.Background()
			dbs, _ := newMultiTrxTestDBs(t, shared.Default)

			trx, err := tc.new(ctx, dbs)
			if err != nil {
				t.Fatalf("new transaction: %v", err)
			}
			if _, err := trx.ExecContext(ctx, "INSERT INTO test_table(id) VALUES(1)"); err != nil {
				t.Fatalf("exec before rollback: %v", err)
			}
			if err := trx.Rollback(); err != nil {
				t.Fatalf("rollback: %v", err)
			}

			_, err = trx.ExecContext(ctx, "INSERT INTO test_table(id) VALUES(2)")
			if !errors.Is(err, tc.expectedClosed) {
				t.Fatalf("exec after rollback error = %v, want %v", err, tc.expectedClosed)
			}
		})
	}
}

func newMultiTrxTestDBs(t *testing.T, keys ...string) (map[string]*sql.DB, map[string]*multiTrxTestDBState) {
	t.Helper()

	multiTrxTestDriverOnce.Do(func() {
		sql.Register(multiTrxTestDriverName, multiTrxTestDriver{})
	})

	dbs := make(map[string]*sql.DB, len(keys))
	states := make(map[string]*multiTrxTestDBState, len(keys))
	for _, key := range keys {
		name := fmt.Sprintf("%s-%d-%s", t.Name(), multiTrxTestDBSeq.Add(1), key)
		state := &multiTrxTestDBState{}

		multiTrxTestRegistry.Lock()
		multiTrxTestRegistry.db[name] = state
		multiTrxTestRegistry.Unlock()

		sqlDB, err := sql.Open(multiTrxTestDriverName, name)
		if err != nil {
			t.Fatalf("open test db %q: %v", key, err)
		}

		dbs[key] = sqlDB
		states[key] = state

		t.Cleanup(func() {
			_ = sqlDB.Close()

			multiTrxTestRegistry.Lock()
			delete(multiTrxTestRegistry.db, name)
			multiTrxTestRegistry.Unlock()
		})
	}

	return dbs, states
}

func assertBeginCount(t *testing.T, states map[string]*multiTrxTestDBState, key string, want int) {
	t.Helper()

	got := states[key].snapshot().begin
	if got != want {
		t.Fatalf("%s begin count = %d, want %d", key, got, want)
	}
}

func totalCommits(states map[string]*multiTrxTestDBState) int {
	var total int
	for _, state := range states {
		total += state.snapshot().commit
	}
	return total
}

func totalRollbacks(states map[string]*multiTrxTestDBState) int {
	var total int
	for _, state := range states {
		total += state.snapshot().rollback
	}
	return total
}

type multiTrxTestDriver struct{}

func (multiTrxTestDriver) Open(name string) (driver.Conn, error) {
	multiTrxTestRegistry.Lock()
	state := multiTrxTestRegistry.db[name]
	multiTrxTestRegistry.Unlock()
	if state == nil {
		return nil, fmt.Errorf("unknown test db %q", name)
	}

	return &multiTrxTestConn{state: state}, nil
}

type multiTrxTestConn struct {
	state *multiTrxTestDBState
}

func (c *multiTrxTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not implemented in multi transaction test driver")
}

func (c *multiTrxTestConn) Close() error {
	return nil
}

func (c *multiTrxTestConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *multiTrxTestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.state.mu.Lock()
	c.state.begin++
	c.state.mu.Unlock()

	return &multiTrxTestTx{state: c.state}, nil
}

func (c *multiTrxTestConn) Ping(context.Context) error {
	return nil
}

func (c *multiTrxTestConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	c.state.mu.Lock()
	c.state.exec++
	c.state.mu.Unlock()

	return driver.RowsAffected(1), nil
}

func (c *multiTrxTestConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	c.state.mu.Lock()
	c.state.query++
	c.state.mu.Unlock()

	return multiTrxTestRows{}, nil
}

type multiTrxTestTx struct {
	state *multiTrxTestDBState
}

func (tx *multiTrxTestTx) Commit() error {
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()

	tx.state.commit++
	if tx.state.failCommit {
		return errors.New("commit failed")
	}

	return nil
}

func (tx *multiTrxTestTx) Rollback() error {
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()

	tx.state.rollback++
	return nil
}

type multiTrxTestRows struct{}

func (multiTrxTestRows) Columns() []string {
	return []string{"id"}
}

func (multiTrxTestRows) Close() error {
	return nil
}

func (multiTrxTestRows) Next([]driver.Value) error {
	return io.EOF
}
