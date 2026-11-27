package api

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net"

	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

// isInfraError reports whether err comes from Postgres, Redis, the network or
// a deadline rather than from input the caller can fix. Such errors are
// answered as 500 with a generic message outside verbose mode, whatever code
// the handler passed, since their text can name hosts, users or pool state.
func isInfraError(err error) bool {
	if err == nil || errors.Is(err, redis.Nil) {
		return false
	}
	var (
		pqErr    *pq.Error
		netErr   net.Error
		redisErr redis.Error
	)
	return errors.As(err, &pqErr) ||
		errors.As(err, &netErr) ||
		errors.As(err, &redisErr) ||
		errors.Is(err, driver.ErrBadConn) ||
		errors.Is(err, sql.ErrConnDone) ||
		errors.Is(err, sql.ErrTxDone) ||
		errors.Is(err, context.DeadlineExceeded)
}
