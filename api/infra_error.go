package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net"
	"net/http"

	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

// inputErrorCode maps Postgres errors caused by the submitted data: class 23
// (unique, foreign key, check, not null) to 409 and class 22 (malformed or
// out-of-range values) to 400.
func inputErrorCode(err error) (int, bool) {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return 0, false
	}
	switch pqErr.Code.Class() {
	case "23":
		return http.StatusConflict, true
	case "22":
		return http.StatusBadRequest, true
	}
	return 0, false
}

// isInfraError reports whether err comes from Postgres, Redis, the network or
// a deadline rather than from input the caller can fix. Such errors are
// answered as 500 with a generic message outside verbose mode, whatever code
// the handler passed, since their text can name hosts, users or pool state.
func isInfraError(err error) bool {
	if err == nil || errors.Is(err, redis.Nil) {
		return false
	}
	var (
		pqErr       *pq.Error
		netErr      net.Error
		redisErr    redis.Error
		verifyErr   *tls.CertificateVerificationError
		unknownCA   x509.UnknownAuthorityError
		hostnameErr x509.HostnameError
		invalidCert x509.CertificateInvalidError
		recordErr   tls.RecordHeaderError
		alertErr    tls.AlertError
	)
	return errors.As(err, &pqErr) ||
		errors.As(err, &netErr) ||
		errors.As(err, &redisErr) ||
		errors.As(err, &verifyErr) ||
		errors.As(err, &unknownCA) ||
		errors.As(err, &hostnameErr) ||
		errors.As(err, &invalidCert) ||
		errors.As(err, &recordErr) ||
		errors.As(err, &alertErr) ||
		errors.Is(err, redis.ErrClosed) ||
		errors.Is(err, redis.ErrPoolTimeout) ||
		errors.Is(err, redis.ErrPoolExhausted) ||
		errors.Is(err, driver.ErrBadConn) ||
		errors.Is(err, sql.ErrConnDone) ||
		errors.Is(err, sql.ErrTxDone) ||
		errors.Is(err, context.DeadlineExceeded)
}
