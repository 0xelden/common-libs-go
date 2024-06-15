package models

import (
	"net/url"
	"time"

	"github.com/dgrijalva/jwt-go"
)

const (
	TimestampFormat string = "2006-01-02T15:04:05-0700"
	TokenTypeBearer string = "bearer"
)

type Request struct {
	Method      string
	URL         *url.URL
	Body        []byte
	CompositeID CompositeID
	Timestamp   time.Time
	Signature   string
	Token       string
}

type CompositeID string

type Composite struct {
	ID     CompositeID `json:"id"`
	Secret string      `json:"secret"`
}

type TokenClaims struct {
	jwt.StandardClaims
	UserID         string        `json:"id"`
	Username       string        `json:"username"`
	IsOrgAdmin     int           `json:"isorgadmin"`
	IsActive       int           `json:"isactive"`
	OrganizationId string        `json:"organization_id"`
	AppId          string        `json:"app"`
	Exp            int           `json:"exp"`
	UserAccess     []interface{} `json:"user_access"`
}

type Token struct {
	Type  string
	Value string
}
