package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/shared"
)

// AccessTokenClaims is the lean claim set actually issued by the auth service
// (uzzeet-usm-service): identity fields only. Company/portal/role/menu data
// lives in Redis at ses-data-<user_id> (see SessionEnvelope), keyed by KeyData.
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

// SessionEnvelope is a partial view of the session payload the auth service
// stores at ses-data-<user_id>. Only the fields common's own middleware needs
// are modeled here; unrecognized fields (apps/portals/menu tree, etc.) are
// ignored by json.Unmarshal.
type SessionEnvelope struct {
	Company struct {
		Id string `json:"id"`
	} `json:"company"`
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

// ParseToken parses and validates a JWT signed with the JWT_SECRET env var
// (shared.JwtSecret), rejecting tokens signed with anything other than an
// HMAC algorithm (algorithm confusion protection).
func ParseToken(token string, claims jwt.Claims, options ...jwt.ParserOption) error {
	tkn, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(helper.Env(shared.JwtSecret)), nil
	}, options...)
	if err != nil {
		return err
	}
	if !tkn.Valid {
		return errors.New("token verify failed")
	}
	return nil
}
