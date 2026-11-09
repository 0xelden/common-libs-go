package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/0xelden/common-libs-go/shared"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "parse-token-test-secret"

func signTestToken(t *testing.T, method jwt.SigningMethod, key any, claims jwt.Claims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func TestParseToken(t *testing.T) {
	t.Setenv(shared.JwtSecret, testSecret)
	now := time.Now()

	valid := &AccessTokenClaims{UserId: "u1", RegisteredClaims: jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}}
	var got AccessTokenClaims
	if err := ParseToken(signTestToken(t, jwt.SigningMethodHS256, []byte(testSecret), valid), &got); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if got.UserId != "u1" {
		t.Fatalf("UserId = %q, want u1", got.UserId)
	}

	expired := &AccessTokenClaims{RegisteredClaims: jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(-time.Minute)),
	}}
	expiredTok := signTestToken(t, jwt.SigningMethodHS256, []byte(testSecret), expired)
	if err := ParseToken(expiredTok, &AccessTokenClaims{}); !errors.Is(err, jwt.ErrTokenExpired) {
		t.Fatalf("expired token err = %v, want ErrTokenExpired", err)
	}
	// WithoutClaimsValidation is how the auth service reads an expired token.
	if err := ParseToken(expiredTok, &AccessTokenClaims{}, jwt.WithoutClaimsValidation()); err != nil {
		t.Fatalf("WithoutClaimsValidation still rejected: %v", err)
	}

	// v4 rejected a future iat; v5 only does with WithIssuedAt.
	future := &AccessTokenClaims{RegisteredClaims: jwt.RegisteredClaims{
		IssuedAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}}
	if err := ParseToken(signTestToken(t, jwt.SigningMethodHS256, []byte(testSecret), future), &AccessTokenClaims{}); !errors.Is(err, jwt.ErrTokenUsedBeforeIssued) {
		t.Fatalf("future iat err = %v, want ErrTokenUsedBeforeIssued", err)
	}

	if err := ParseToken(signTestToken(t, jwt.SigningMethodHS256, []byte("other"), valid), &AccessTokenClaims{}); err == nil {
		t.Fatal("token signed with another secret accepted")
	}

	none := signTestToken(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, valid)
	if err := ParseToken(none, &AccessTokenClaims{}); err == nil {
		t.Fatal("alg=none token accepted")
	}
}
