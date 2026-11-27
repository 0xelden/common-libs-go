package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xelden/common-libs-go/shared"
)

const fallbackDriverName = "common_replica_fallback_test"

var (
	fallbackDriverOnce sync.Once
	fallbackDown       sync.Map
	fallbackExecErr    sync.Map
	fallbackExecs      sync.Map
	fallbackOpens      sync.Map
)

type fallbackDriver struct{}

func (fallbackDriver) Open(name string) (driver.Conn, error) {
	n, _ := fallbackOpens.LoadOrStore(name, &atomic.Int32{})
	n.(*atomic.Int32).Add(1)
	if down(name) {
		return nil, driver.ErrBadConn
	}
	return fallbackConn{name: name}, nil
}

func down(name string) bool {
	v, ok := fallbackDown.Load(name)
	return ok && v.(*atomic.Bool).Load()
}

func setDown(name string, value bool) {
	v, _ := fallbackDown.LoadOrStore(name, &atomic.Bool{})
	v.(*atomic.Bool).Store(value)
}

type fallbackConn struct{ name string }

func (c fallbackConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (c fallbackConn) Close() error                        { return nil }
func (c fallbackConn) Begin() (driver.Tx, error)           { return nil, errors.New("not supported") }

func (c fallbackConn) Ping(context.Context) error {
	if down(c.name) {
		return driver.ErrBadConn
	}
	return nil
}

func (c fallbackConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	if down(c.name) {
		return nil, driver.ErrBadConn
	}
	return &fallbackRows{value: c.name}, nil
}

func (c fallbackConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	if down(c.name) {
		return nil, driver.ErrBadConn
	}
	n, _ := fallbackExecs.LoadOrStore(c.name, &atomic.Int32{})
	n.(*atomic.Int32).Add(1)
	if err, ok := fallbackExecErr.Load(c.name); ok {
		return nil, err.(error)
	}
	return driver.RowsAffected(1), nil
}

type fallbackRows struct {
	value string
	done  bool
}

func (r *fallbackRows) Columns() []string { return []string{"src"} }
func (r *fallbackRows) Close() error      { return nil }
func (r *fallbackRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = r.value
	return nil
}

func openFallback(t *testing.T, name string) *sql.DB {
	fallbackDriverOnce.Do(func() { sql.Register(fallbackDriverName, fallbackDriver{}) })
	setDown(name, false)
	conn, err := sql.Open(fallbackDriverName, name)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxIdleConns(0)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func newFallbackDriver(t *testing.T) (*SqlDriverMulti, string, string, *time.Time) {
	primary, replica := t.Name()+"/primary", t.Name()+"/replica"
	d, _ := NewSqlDriverMulti(map[string]*sql.DB{
		shared.Default:                 openFallback(t, primary),
		shared.Default + replicaSuffix: openFallback(t, replica),
	})
	now := time.Unix(1_700_000_000, 0)
	d.health.now = func() time.Time { return now }
	return d, primary, replica, &now
}

func replicaCtx() context.Context {
	return context.WithValue(context.Background(), shared.ExecuteIn, shared.Default+replicaSuffix)
}

func source(t *testing.T, d *SqlDriverMulti, ctx context.Context) (viaQuery, viaRow string) {
	t.Helper()
	rows, err := d.QueryContext(ctx, "select")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	for rows.Next() {
		if err := rows.Scan(&viaQuery); err != nil {
			t.Fatal(err)
		}
	}
	_ = rows.Close()
	if err := d.QueryRowContext(ctx, "select").Scan(&viaRow); err != nil {
		t.Fatalf("QueryRowContext: %v", err)
	}
	return viaQuery, viaRow
}

func TestReplicaFallsBackToPrimary(t *testing.T) {
	d, primary, replica, now := newFallbackDriver(t)
	ctx := replicaCtx()

	if q, r := source(t, d, ctx); q != replica || r != replica {
		t.Fatalf("healthy replica: got %q/%q, want %q", q, r, replica)
	}

	setDown(replica, true)
	if q, _ := source(t, d, ctx); q != primary {
		t.Fatalf("QueryContext on a down replica went to %q, want the primary", q)
	}
	if _, r := source(t, d, ctx); r != primary {
		t.Fatalf("QueryRowContext while the replica is marked down went to %q, want the primary", r)
	}
	if _, err := d.ExecContext(ctx, "update"); err != nil {
		t.Fatalf("ExecContext on a down replica should retry on the primary: %v", err)
	}
	if err := d.PingContext(ctx); err == nil {
		t.Fatal("PingContext must report the replica itself, not fall back")
	}

	setDown(replica, false)
	*now = now.Add(replicaRecheck + time.Second)
	if q, r := source(t, d, ctx); q != replica || r != replica {
		t.Fatalf("recovered replica: got %q/%q, want %q", q, r, replica)
	}
}

func TestReplicaHealthCheckCatchesQueryRow(t *testing.T) {
	d, primary, replica, now := newFallbackDriver(t)
	ctx := replicaCtx()

	setDown(replica, true)
	*now = now.Add(replicaRecheck + time.Second)
	var got string
	if err := d.QueryRowContext(ctx, "select").Scan(&got); err != nil || got != primary {
		t.Fatalf("QueryRowContext with a down replica: got %q err %v, want %q", got, err, primary)
	}
}

func execs(name string) int32 {
	n, ok := fallbackExecs.Load(name)
	if !ok {
		return 0
	}
	return n.(*atomic.Int32).Load()
}

func TestExecIsNotRetriedAfterItMayHaveRun(t *testing.T) {
	d, primary, replica, _ := newFallbackDriver(t)
	ctx := replicaCtx()

	timeout := &net.OpError{Op: "read", Net: "tcp", Err: errors.New("i/o timeout")}
	fallbackExecErr.Store(replica, timeout)
	t.Cleanup(func() { fallbackExecErr.Delete(replica) })

	if _, err := d.ExecContext(ctx, "update"); !errors.Is(err, timeout) {
		t.Fatalf("ExecContext error = %v, want the replica's timeout", err)
	}
	if got := execs(replica); got != 1 {
		t.Fatalf("replica ran the statement %d times, want 1", got)
	}
	if got := execs(primary); got != 0 {
		t.Fatalf("primary ran the statement %d times, want 0", got)
	}
	if q, _ := source(t, d, ctx); q != primary {
		t.Fatalf("after the timeout reads went to %q, want the primary", q)
	}
}

func opens(name string) int32 {
	n, ok := fallbackOpens.Load(name)
	if !ok {
		return 0
	}
	return n.(*atomic.Int32).Load()
}

func TestPrimaryFailureDuringFallbackIsNotRetried(t *testing.T) {
	d, primary, replica, now := newFallbackDriver(t)
	ctx := replicaCtx()

	setDown(replica, true)
	if q, _ := source(t, d, ctx); q != primary {
		t.Fatalf("down replica: reads went to %q, want the primary", q)
	}

	setDown(primary, true)
	before := opens(primary)
	if _, err := d.QueryContext(context.Background(), "select"); err == nil {
		t.Fatal("a down primary must fail")
	}
	direct := opens(primary) - before

	*now = now.Add(4 * time.Second)
	before = opens(primary)
	if _, err := d.QueryContext(ctx, "select"); err == nil {
		t.Fatal("a down primary must fail")
	}
	if got := opens(primary) - before; got != direct {
		t.Fatalf("fallback query opened the primary %d times, a direct one %d", got, direct)
	}

	setDown(replica, false)
	*now = now.Add(2 * time.Second)
	if q, _ := source(t, d, ctx); q != replica {
		t.Fatalf("recovered replica while the primary is down: reads went to %q", q)
	}
}

func TestBusyReplicaPoolIsNotMarkedDown(t *testing.T) {
	d, _, replica, now := newFallbackDriver(t)
	ctx := replicaCtx()

	pool := d.db[shared.Default+replicaSuffix]
	pool.SetMaxOpenConns(1)
	held, err := pool.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()

	*now = now.Add(replicaRecheck + time.Second)
	start := time.Now()
	to := d.getOne(ctx)
	if to.conn != pool {
		t.Fatal("a replica with a busy pool was swapped for the primary")
	}
	if waited := time.Since(start); waited >= replicaPingTimeout {
		t.Fatalf("getOne waited %s on the busy pool", waited)
	}
	_ = held.Close()
	if q, _ := source(t, d, ctx); q != replica {
		t.Fatalf("reads went to %q, want %q", q, replica)
	}
}

func TestPrimaryErrorsAreNotRetried(t *testing.T) {
	d, primary, _, _ := newFallbackDriver(t)
	setDown(primary, true)
	if _, err := d.QueryContext(context.Background(), "select"); err == nil {
		t.Fatal("a primary failure must surface, there is nothing to fall back to")
	}
}
