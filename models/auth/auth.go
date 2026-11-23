package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/shared"
	"github.com/golang-jwt/jwt/v5"
)

type AccessTokenClaims struct {
	UserId       string     `form:"user_id" json:"user_id"`
	Username     string     `form:"username" json:"username"`
	Email        string     `form:"email" json:"email"`
	Name         string     `form:"name" json:"name"`
	ProfileImage string     `form:"profile_image" json:"profile_image"`
	PhoneNumber  string     `form:"phone_number" json:"phone_number"`
	LastLogin    *time.Time `form:"last_login" json:"last_login,omitempty"`
	KeyData      *string    `form:"key_data" json:"key_data,omitempty"`
	jwt.RegisteredClaims
}

// SessionRole is one role grant in the session payload. A user holds a list
// of these, not a single role.
type SessionRole struct {
	Id    string `json:"id"`
	Name  string `json:"name"`
	Level *int64 `json:"level"`
}

// SessionEnvelope is a partial view of the session payload the auth service
// stores at ses-data-<user_id>. Only the fields common's own middleware needs
// are modeled here; unrecognized fields (menu tree, permissions, etc.) are
// ignored by json.Unmarshal.
type SessionEnvelope struct {
	Roles       []SessionRole `json:"roles"`
	Permissions []string      `json:"permissions"`

	Company struct {
		Id string `json:"id"`
	} `json:"company"`
}

// RoleIds returns the ids of every role in the session, skipping entries with
// no id so a malformed row can't inject an empty id into a permission check.
// The result is nil when the user holds no roles.
func (s SessionEnvelope) RoleIds() []string {
	var ids []string
	for _, role := range s.Roles {
		if role.Id != "" {
			ids = append(ids, role.Id)
		}
	}
	return ids
}

// JoinRoleIds encodes role ids for transport in a string-only context value
// (see shared.RoleIdSeparator).
func JoinRoleIds(ids []string) string {
	return strings.Join(ids, shared.RoleIdSeparator)
}

// RoleIdsFromContext reads back the role ids GuardRoute put on the request.
//
// It accepts a *gin.Context (which resolves string keys against its own Keys)
// as well as the context.Context derived by api.SetContext, so a handler and
// anything downstream of it read roles the same way. The result is nil when
// the caller holds no roles.
func RoleIdsFromContext(ctx context.Context) []string {
	raw := helper.ContextGetString(ctx, shared.X_RoleIds)
	if raw == "" {
		return nil
	}
	var ids []string
	for _, id := range strings.Split(raw, shared.RoleIdSeparator) {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// HasRole reports whether the caller holds roleId.
func HasRole(ctx context.Context, roleId string) bool {
	if roleId == "" {
		return false
	}
	for _, id := range RoleIdsFromContext(ctx) {
		if id == roleId {
			return true
		}
	}
	return false
}

type RefreshToken struct {
	UserId string `form:"user_id" json:"user_id"`
	jwt.RegisteredClaims
}

type LoginResult struct {
	RefreshToken string            `form:"refresh_token" json:"refresh_token"`
	AccessToken  string            `form:"access_token" json:"access_token"`
	Info         AccessTokenClaims `form:"info" json:"info"`
}

type SendOtpResult struct {
	Key   string    `json:"key"`
	Until time.Time `json:"until"`
}

const (
	MinJwtSecretLen          = 32
	DefaultAccessTokenIssuer = "usm"
)

// JwtSecret returns the HMAC key from JWT_SECRET, refusing one shorter than
// MinJwtSecretLen bytes (RFC 7518 §3.2) so an empty or weak key never signs
// or verifies a token.
func JwtSecret() ([]byte, error) {
	secret := helper.Env(shared.JwtSecret)
	if len(secret) < MinJwtSecretLen {
		return nil, fmt.Errorf("%s must be at least %d bytes, got %d", shared.JwtSecret, MinJwtSecretLen, len(secret))
	}
	return []byte(secret), nil
}

// AccessTokenOptions requires the access-token issuer (JWT_ISSUER, default
// "usm") so a refresh token signed with the same key is not accepted as an
// access token, and the audience when JWT_AUDIENCE is set.
func AccessTokenOptions() []jwt.ParserOption {
	options := []jwt.ParserOption{jwt.WithIssuer(helper.Env(shared.JwtIssuer, DefaultAccessTokenIssuer))}
	if aud := helper.Env(shared.JwtAudience); aud != "" {
		options = append(options, jwt.WithAudience(aud))
	}
	return options
}

// ParseToken parses and validates a JWT signed with the JWT_SECRET env var
// (shared.JwtSecret), rejecting tokens signed with anything other than an
// HMAC algorithm (algorithm confusion protection) and tokens without exp.
// jwt/v5 stopped rejecting tokens issued in the future by default, so
// WithIssuedAt restores the v4 behavior; callers' options are applied after it.
func ParseToken(token string, claims jwt.Claims, options ...jwt.ParserOption) error {
	options = append([]jwt.ParserOption{jwt.WithIssuedAt(), jwt.WithExpirationRequired()}, options...)
	tkn, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return JwtSecret()
	}, options...)
	if err != nil {
		return err
	}
	if !tkn.Valid {
		return errors.New("token verify failed")
	}
	return nil
}
