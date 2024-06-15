package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	uap "github.com/Gohelraj/go_ip_device_parser"
	"github.com/gin-gonic/gin"
	"github.com/go-errors/errors"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v4"
	"github.com/0xelden/common-libs-go/acl"
	"github.com/0xelden/common-libs-go/api"
	"github.com/0xelden/common-libs-go/cli"
	"github.com/0xelden/common-libs-go/connection"
	"github.com/0xelden/common-libs-go/helper"
	authModels "github.com/0xelden/common-libs-go/models/auth"
	"github.com/0xelden/common-libs-go/shared"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type middleware struct {
	disableGuards    bool
	disableTelemetry bool
	isWebsocket      bool
	publicPath       []string
	noRefPath        []string
	builtinPath      []string
}

type Option func(*middleware)

func RegisterService(httpBasePoint string, service *gin.Engine, options ...Option) error {
	if httpBasePoint == "" {
		return errors.New("Middleware registration failed, invalid service http basepoint")
	}

	m := &middleware{
		publicPath:  []string{},
		builtinPath: []string{},
		noRefPath:   []string{},
	}

	service.Use(HandleNotfound())

	for _, opt := range options {
		opt(m)
	}

	if !m.disableTelemetry {
		service.Use(otelgin.Middleware(httpBasePoint))
		service.Use(TraceRequest())
	}

	if m.isWebsocket {
		client, _ := connection.ConnectRedis()
		service.Use(SetWsCredential(client, "wsk"))
	}

	service.Use(SetMetadata(httpBasePoint, m.noRefPath...))

	if !m.disableGuards {
		service.Use(GuardRoute(m.publicPath...))
	}

	return nil
}

// WithoutGuards mark all path as public
//
//goland:noinspection GoUnusedExportedFunction
func WithoutGuards() Option {
	return func(m *middleware) {
		m.disableGuards = true
	}
}

//goland:noinspection GoUnusedExportedFunction
func WithWebsocket() Option {
	return func(m *middleware) {
		m.isWebsocket = true
	}
}

// WithPublicRoute add custom public uri list
// an example of publicPath to the login endpoint is /credential/login
func WithPublicRoute(publicPath ...string) Option {
	return func(m *middleware) {
		m.disableGuards = false
		m.publicPath = append(m.publicPath, publicPath...)
	}
}

// WithNoRefRoute disable X-Portal or Referer validation on these specific path
//
//goland:noinspection GoUnusedExportedFunction
func WithNoRefRoute(noRefPath ...string) Option {
	return func(m *middleware) {
		m.noRefPath = append(m.noRefPath, noRefPath...)
	}
}

// WithBuiltinRoute add custom builtin uri list
// builtin uri need to be checked for token validity
// but not with acess control, since it is builtin functionality all portal can access it
//
//goland:noinspection GoUnusedExportedFunction
func WithBuiltinRoute(builtinPath ...string) Option {
	return func(m *middleware) {
		m.disableGuards = false
		m.builtinPath = builtinPath
	}
}

// WithoutTelemetry create service without telemetry
//
//goland:noinspection GoUnusedExportedFunction
func WithoutTelemetry() Option {
	return func(m *middleware) {
		m.disableTelemetry = true
	}
}

func HandleNotfound() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.FullPath() == "" {
			api.Error(c, errors.New("route not found"), http.StatusNotFound)
			c.Abort()
		}
	}
}

func SetWsCredential(client *redis.Client, key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("Sec-WebSocket-Protocol")
		if id == "" {
			id = c.Query(key + "-debug-key")
		} else {
			id = id[8:]
		}
		if !helper.IsValidUUID(id) {
			_ = c.AbortWithError(http.StatusInternalServerError, errors.New("invalid ws credential"))
			return
		}
		c.Writer.Header().Set("Sec-WebSocket-Protocol", id)
		tok, _ := client.Get(c.Request.Context(), key+"-"+id).Result()
		if len(tok) > 100 {
			c.Request.Header.Set("Authorization", "Bearer "+tok)
			c.Request.Header.Set("X-Portal", "XPortal")
		} else {
			_ = c.AbortWithError(http.StatusInternalServerError, errors.New("invalid ws credentials"))
		}
	}
}

// TraceRequest trace additional request attributes
func TraceRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		if span := trace.SpanFromContext(c.Request.Context()); span.IsRecording() {
			ua := uap.ParseUserAgentAndClientIP(c.Request)
			dt := "unknown"
			if ua.IsDesktop {
				dt = "desktop"
			} else if ua.IsMobile {
				dt = "mobile"
			} else if ua.IsTablet {
				dt = "tablet"
			} else if ua.IsBot {
				dt = "bot"
			}
			// duplicates, gin otel alrady have these
			//span.SetAttributes(attribute.String("ua.browser.name", ua.Agent.Browser.Name))
			//span.SetAttributes(attribute.String("ua.browser.version", ua.Agent.Browser.Version))
			//span.SetAttributes(attribute.String("ua.os.name", ua.Agent.Os.Name))
			//span.SetAttributes(attribute.String("ua.os.version", ua.Agent.Os.Version))
			// duplicate end
			span.SetAttributes(attribute.String("ua.device.name", ua.Agent.Device.Name))
			span.SetAttributes(attribute.String("ua.device.type", dt))
			span.SetAttributes(attribute.String("http.route_url", c.Request.Host+c.Request.URL.String()))
			span.SetAttributes(attribute.String("http.client_ip", ua.IP))

			// trace git commit id
			span.SetAttributes(attribute.String("x.commit_id", helper.If(cli.Commit != "", cli.Commit, helper.First(helper.GitCommitID()))))
		}
	}
}

// SetMetadata parse all contextual metadata of a http request and set it on ctx
func SetMetadata(basepoint string, noRefURI ...string) gin.HandlerFunc {
	if len(basepoint) == 0 {
		panic("invalid service http basepoint")
	}
	var service string
	if bp := strings.Split(basepoint, "/"); len(bp) != 2 {
		panic("invalid service http basepoint format")
	} else {
		service = bp[1]
	}
	noref := map[string]struct{}{}
	for _, uri := range noRefURI {
		noref[uri] = struct{}{}
	}
	return func(ctx *gin.Context) {
		if val, ok := ctx.Request.Header["Authorization"]; ok && len(val) > 0 {
			if parts := strings.Split(val[0], " "); len(parts) > 1 {
				ctx.Set(shared.X_Token, parts[1])
			}
		}

		// set empty uuid as default
		ctx.Set(shared.X_PortalId, shared.EmptyUuid)
		ctx.Set(shared.X_RoleId, shared.EmptyUuid)
		ctx.Set(shared.X_UserId, shared.EmptyUuid)

		// set portal domain
		dom := ctx.Request.Header.Get("X-Portal")
		if len(dom) < 3 {
			dom = ctx.Request.Header.Get("Referer")
			_, skipValidation := noref[ctx.FullPath()]
			if !skipValidation && len(dom) < 3 {
				api.Error(ctx, errors.New("X-Portal header must be set"))
				ctx.Abort()
				return
			}
		}

		ctx.Set(shared.X_PortalDomain, dom)
		ctx.Set(shared.X_Service, service)
		if parts := strings.Split(ctx.Request.URL.Path, "/"); len(parts) > 2 {
			ctx.Set(shared.X_Controller, parts[1])
			ctx.Set(shared.X_Action, parts[2])
			ctx.Set(shared.X_Slug, fmt.Sprintf("%s/%s/%s", service, parts[1], parts[2]))
			ctx.Set(shared.X_Level, acl.ParseAccessLevel(parts[2]).String())
		}
	}
}

var (
	guardRedisMu     sync.Mutex
	guardRedisClient *redis.Client
)

// guardSessionRedis lazily connects (retrying on a prior failed attempt) a
// Redis client used by GuardRoute to look up session liveness/data, mirroring
// the on-demand connection pattern SetWsCredential already relies on.
func guardSessionRedis() *redis.Client {
	guardRedisMu.Lock()
	defer guardRedisMu.Unlock()
	if guardRedisClient == nil {
		guardRedisClient, _ = connection.ConnectRedis()
	}
	return guardRedisClient
}

// GuardRoute guard a route with a bearer token
// a publicPath to the login endpoint is /credential/login
func GuardRoute(publicPath ...string) gin.HandlerFunc {
	public := map[string]struct{}{}
	for _, uri := range publicPath {
		public[uri] = struct{}{}
	}
	return func(ctx *gin.Context) {
		if _, isPublic := public[ctx.FullPath()]; isPublic {
			return
		}

		token := ctx.GetString(shared.X_Token)
		if token == "" {
			api.Error(ctx, errors.New("Token required"), http.StatusUnauthorized)
			ctx.Abort()
			return
		}

		claims := authModels.AccessTokenClaims{}
		err := authModels.ParseToken(token, &claims)
		if err != nil {
			msg := errors.New("Invalid token")
			if errors.Is(err, jwt.ErrTokenExpired) {
				msg = errors.New("Token expired")
			}
			api.Error(ctx, msg, http.StatusUnauthorized)
			ctx.Abort()
			return
		}

		sessionExpired := errors.New("session not found or expired")
		rdb := guardSessionRedis()
		if rdb == nil {
			api.Error(ctx, sessionExpired, http.StatusUnauthorized)
			ctx.Abort()
			return
		}

		reqCtx := ctx.Request.Context()

		// liveness marker set at login, deleted on revoke; its absence means
		// the token has been revoked even if the JWT itself hasn't expired.
		if _, err := rdb.Get(reqCtx, fmt.Sprintf("session-%s:%s", claims.UserId, claims.ID)).Result(); err != nil {
			api.Error(ctx, sessionExpired, http.StatusUnauthorized)
			ctx.Abort()
			return
		}

		var session authModels.SessionEnvelope
		raw, err := rdb.Get(reqCtx, fmt.Sprintf("ses-data-%s", claims.UserId)).Result()
		if err != nil || json.Unmarshal([]byte(raw), &session) != nil {
			api.Error(ctx, sessionExpired, http.StatusUnauthorized)
			ctx.Abort()
			return
		}

		ctx.Set(shared.X_UserName, claims.Username)
		ctx.Set(shared.X_UserId, claims.UserId)
		ctx.Set(shared.X_UserEmail, claims.Email)
		ctx.Set(shared.X_UserPhone, claims.PhoneNumber)

		if span := trace.SpanFromContext(reqCtx); span.IsRecording() {
			span.SetAttributes(
				attribute.String(shared.X_UserId, claims.UserId),
				attribute.String(shared.X_PortalDomain, ctx.GetString(shared.X_PortalDomain)),
			)
		}

		// TODO 1.0 : uncomment to implement access control
		// TODO: `slug` here only contains up to controller level, but `got` var can be up to action level

		// ok := false
		// got := ctx.GetString("x_slug")
		// for _, slug := range atClaims.Access {
		// 	segments := strings.Split(got, "/")

		// 	var filteredSegments []string
		// 	for _, segment := range segments {
		// 		if segment != "" {
		// 			filteredSegments = append(filteredSegments, segment)
		// 		}
		// 	}
		// 	firstTwoSegments := filteredSegments
		// 	if len(filteredSegments) > 2 {
		// 		firstTwoSegments = filteredSegments[:2]
		// 	}
		// 	ctlSegments := "/" + strings.Join(firstTwoSegments, "/")

		// 	if ctlSegments == slug {
		// 		ok = true
		// 		break
		// 	}
		// }
		// if !ok {
		// 	api.Error(ctx, errors.New("Unauthorized access to "+got), http.StatusUnauthorized)
		// 	ctx.Abort()
		// 	return
		// }
		// TODO 1.0
	}
}
