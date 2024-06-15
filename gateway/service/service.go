package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"

	"google.golang.org/grpc/metadata"

	"github.com/0xelden/common-libs-go/gateway/models"
	"github.com/0xelden/common-libs-go/gateway/packets"
	"github.com/0xelden/common-libs-go/logger"
)

type Service struct {
	key          string
	namespace    string
	baseEndpoint string
	checksum     string
	router       router
}

func (svc Service) BaseEndpoint() string {
	return svc.baseEndpoint
}

func (svc Service) Key() string {
	return svc.key
}

func (svc Service) Handshake(ctx context.Context, ackr *packets.AckRequest) (*packets.Ack, error) {
	logger.Infof("retrive incoming handshake from gateway(%s)", ackr.From)

	pr := make(map[string]*packets.ProtectedRoutes)
	for method, routes := range svc.router.protectedRoutes {
		protectedRoutes := []*packets.ProtectedRoute{}
		for _, route := range routes {
			protectedRoutes = append(protectedRoutes, &packets.ProtectedRoute{
				Method:  route.method,
				Pattern: route.pattern.String(),
			})
		}

		pr[method] = &packets.ProtectedRoutes{
			Routes: protectedRoutes,
		}
	}

	host, err := os.Hostname()
	if err != nil {
		host = "?"
	}

	return &packets.Ack{
		Server:          host,
		Checksum:        svc.checksum,
		Namespace:       svc.namespace,
		ProtectedRoutes: pr,
	}, nil
}

func (svc Service) Dispatch(ctx context.Context, req *packets.Request) (*packets.Response, error) {
	var (
		clientInfo models.ClientInfo
		authInfo   models.AuthorizationInfo
	)

	header := make(map[string]string)
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		for key, vals := range md {
			if http.CanonicalHeaderKey(key) == http.CanonicalHeaderKey(models.ClientInfoContextValueKey) {
				err := json.Unmarshal([]byte(vals[0]), &clientInfo)
				if err != nil {
					return nil, err
				}

				continue
			}

			if http.CanonicalHeaderKey(key) == http.CanonicalHeaderKey(models.AuthorizationInfoContextValueKey) {
				err := json.Unmarshal([]byte(vals[0]), &authInfo)
				if err != nil {
					return nil, err
				}

				continue
			}

			header[http.CanonicalHeaderKey(key)] = vals[0]
		}
	}

	u, err := url.Parse(req.Path)
	if err != nil {
		return Context{}.JSONResponse(http.StatusBadRequest, models.ResponseBody{
			Error:      "Jalur tidak ditemukan",
			Controller: req.Path,
			Action:     req.Method,
		}).buildResponse(ctx)
	}

	query := make(map[string]string)
	for key, vals := range u.Query() {
		query[key] = vals[0]
	}

	sCtx := &Context{
		requestContext{
			path:       u.Path,
			method:     req.Method,
			body:       req.Body,
			header:     header,
			query:      query,
			params:     make(map[string]string),
			authInfo:   authInfo,
			clientInfo: clientInfo,
		},
		responseContext{
			header: make(map[string]string),
		},
	}

	return svc.router.route(sCtx).buildResponse(ctx)
}
