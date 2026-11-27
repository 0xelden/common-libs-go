package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/shared"
	"github.com/lib/pq"
)

var (
	_ MultiDriver = SqlDriverMulti{}
)

const (
	replicaSuffix = ":replica"
	// replicaRecheck is how long a replica stays on the primary after a
	// failure, and how often a healthy replica is re-pinged.
	replicaRecheck     = 5 * time.Second
	replicaPingTimeout = 300 * time.Millisecond
)

type SqlDriverMulti struct {
	db     map[string]*sql.DB
	health *replicaHealth
}

func NewSqlDriverMulti(DB map[string]*sql.DB) (*SqlDriverMulti, error) {
	return &SqlDriverMulti{db: DB, health: newReplicaHealth()}, nil
}

func (t SqlDriverMulti) SupportMultiDb() {
}

// pick resolves the connection key from shared.ExecuteIn, then
// shared.X_PortalCode, then shared.Default.
func (t SqlDriverMulti) pick(ctx context.Context) (string, *sql.DB) {
	if val := ctx.Value(shared.ExecuteIn); val != nil {
		if str, ok := val.(string); ok {
			if tx, ok2 := t.db[str]; ok2 {
				return str, tx
			}
		}
	}
	if val := ctx.Value(shared.X_PortalCode); val != nil {
		if str, ok := val.(string); ok {
			if tx, ok2 := t.db[str]; ok2 {
				return str, tx
			}
		}
	}
	return shared.Default, t.db[shared.Default]
}

// primaryOf returns the primary behind a "<code>:replica" key, when the
// replica is a separate connection.
func (t SqlDriverMulti) primaryOf(key string, replica *sql.DB) (*sql.DB, bool) {
	code, ok := strings.CutSuffix(key, replicaSuffix)
	if !ok {
		return nil, false
	}
	primary, ok := t.db[code]
	if !ok || primary == nil || primary == replica {
		return nil, false
	}
	return primary, true
}

// target is the connection a statement runs on. fallback is set only when
// conn is a replica, so errors from a primary are never retried.
type target struct {
	key      string
	conn     *sql.DB
	fallback *sql.DB
}

// getOne returns the connection for ctx. A replica that failed recently, or
// that does not answer a periodic ping, is swapped for its primary.
func (t SqlDriverMulti) getOne(ctx context.Context) target {
	key, conn := t.pick(ctx)
	primary, ok := t.primaryOf(key, conn)
	if !ok || t.health == nil {
		helper.Trace(ctx, "sql.context."+key, true)
		return target{key: key, conn: conn}
	}
	if !t.health.usable(key, conn) {
		helper.Trace(ctx, "sql.context."+key+".fallback", true)
		return target{key: key, conn: primary}
	}
	helper.Trace(ctx, "sql.context."+key, true)
	return target{key: key, conn: conn, fallback: primary}
}

// PingContext reports the selected connection itself, without falling back,
// so health checks see a replica that is down.
func (t SqlDriverMulti) PingContext(ctx context.Context) error {
	_, conn := t.pick(ctx)
	return conn.PingContext(ctx)
}

// ExecContext retries on the primary only for driver.ErrBadConn, which means
// the statement never reached the server; any other error may follow a write
// that already ran.
func (t SqlDriverMulti) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	to := t.getOne(ctx)
	res, err := to.conn.ExecContext(ctx, query, args...)
	if primary, ok := t.retryOnPrimary(ctx, to, err); ok && errors.Is(err, driver.ErrBadConn) {
		return primary.ExecContext(ctx, query, args...)
	}
	return res, err
}

func (t SqlDriverMulti) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	to := t.getOne(ctx)
	rows, err := to.conn.QueryContext(ctx, query, args...)
	if primary, retry := t.retryOnPrimary(ctx, to, err); retry {
		return primary.QueryContext(ctx, query, args...)
	}
	return rows, err
}

// QueryRowContext cannot see the error before Scan, so it relies on the
// replica health check in getOne.
func (t SqlDriverMulti) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.getOne(ctx).conn.QueryRowContext(ctx, query, args...)
}

// retryOnPrimary marks the replica in to down after a connection-level error
// and returns its primary. It does nothing when to already ran on a primary.
func (t SqlDriverMulti) retryOnPrimary(ctx context.Context, to target, err error) (*sql.DB, bool) {
	if to.fallback == nil || err == nil || ctx.Err() != nil || !isConnError(err) {
		return nil, false
	}
	t.health.markDown(to.key)
	helper.Trace(ctx, "sql.context."+to.key+".fallback", err.Error())
	return to.fallback, true
}

// isConnError reports errors that mean the server could not be reached or
// dropped the session, as opposed to errors in the statement itself.
func isConnError(err error) bool {
	var (
		netErr net.Error
		pqErr  *pq.Error
	)
	if errors.Is(err, driver.ErrBadConn) || errors.Is(err, sql.ErrConnDone) || errors.As(err, &netErr) {
		return true
	}
	if errors.As(err, &pqErr) {
		code := string(pqErr.Code)
		return strings.HasPrefix(code, "08") || code == "57P01" || code == "57P02" || code == "57P03" || code == "53300"
	}
	return false
}

type replicaState struct {
	downUntil time.Time
	checkedAt time.Time
	checking  bool
}

type replicaHealth struct {
	mu    sync.Mutex
	state map[string]*replicaState
	now   func() time.Time
}

func newReplicaHealth() *replicaHealth {
	return &replicaHealth{state: map[string]*replicaState{}, now: time.Now}
}

func (h *replicaHealth) get(key string) *replicaState {
	s, ok := h.state[key]
	if !ok {
		s = &replicaState{}
		h.state[key] = s
	}
	return s
}

func (h *replicaHealth) markDown(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.get(key)
	s.downUntil = h.now().Add(replicaRecheck)
	s.checkedAt = h.now()
}

// usable reports whether the replica should serve this request. While it is
// marked down the primary is used; otherwise one caller at a time pings it
// every replicaRecheck, and the others keep the last known state. A replica
// whose pool is fully in use is busy serving, so it is not pinged: the ping
// would only wait for a free connection.
func (h *replicaHealth) usable(key string, conn *sql.DB) bool {
	h.mu.Lock()
	s := h.get(key)
	now := h.now()
	if now.Before(s.downUntil) {
		h.mu.Unlock()
		return false
	}
	if s.checking || now.Sub(s.checkedAt) < replicaRecheck {
		h.mu.Unlock()
		return true
	}
	if st := conn.Stats(); st.MaxOpenConnections > 0 && st.InUse >= st.MaxOpenConnections {
		s.checkedAt = now
		h.mu.Unlock()
		return true
	}
	s.checking = true
	h.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), replicaPingTimeout)
	err := conn.PingContext(ctx)
	cancel()

	h.mu.Lock()
	defer h.mu.Unlock()
	s.checking = false
	s.checkedAt = h.now()
	if err != nil {
		s.downUntil = s.checkedAt.Add(replicaRecheck)
		return false
	}
	return true
}
