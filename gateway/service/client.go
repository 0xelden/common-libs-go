package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/0xelden/common-libs-go/gateway/controller/resolver"
	"github.com/0xelden/common-libs-go/gateway/packets"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/logger"
)

type Forwarder interface {
	Mount(*Composite)
}

type Composite struct {
	packets.ServiceClient
	Key             string
	Endpoint        string
	Connection      *grpc.ClientConn
	Url             string
	ProtectedRoutes map[string][]protectedRoute
}

func NewComposite(resolv resolver.Resolver, cfg Config) (*Composite, error) {
	connURL := resolv.GenerateURL(cfg.Host, helper.IntToString(cfg.Port))

	if cfg.TypeConn == "http" {
		logger.Infof("connecting http composite %s(%s)", cfg.Key, connURL)

		return &Composite{
			Key:             cfg.Key,
			Endpoint:        cfg.gatewayEndpoint,
			Connection:      nil,
			Url:             connURL,
			ServiceClient:   nil,
			ProtectedRoutes: nil,
		}, nil
	}

	logger.Infof("connecting gRPC composite %s(%s)", cfg.Key, connURL)

	var err error
	var conn *grpc.ClientConn
	var sc packets.ServiceClient
	var res *packets.Ack

	i := 0
	max := 10
	for {
		if i > 0 {
			cooldown := 1
			switch {
			case i > 8:
				cooldown = 5

			case i > 5 && i <= 8:
				cooldown = 3
			}

			logger.Infof("will retrying (%d/%d) in %d seconds...", i, max, cooldown)
			time.Sleep(time.Duration(cooldown) * time.Second)
		}

		const grpcServiceConfig = `{"loadBalancingPolicy":"round_robin"}`

		conn, err = grpc.Dial(
			connURL,
			grpc.WithDefaultServiceConfig(grpcServiceConfig),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
			grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
		)
		if err != nil {
			if i < max {
				logger.Warnf("failed to dial service %s, detail: %v", cfg.Key, err)

				i++
				continue
			}

			return nil, fmt.Errorf("while dialing server: %v", err)
		}

		host, err := os.Hostname()
		if err != nil {
			host = "?"
		}

		logger.Infof("doing handshake with %s", cfg.Key)
		sc = packets.NewServiceClient(conn)
		res, err = sc.Handshake(context.Background(), &packets.AckRequest{
			From: host,
		})
		if err != nil {
			if i < max {
				logger.Warnf("failed to handshaking with service %s, detail: %v", cfg.Key, err)

				i++
				continue
			}

			return nil, fmt.Errorf("while handshaking with service %s: %v", cfg.Key, err)
		}

		break
	}

	logger.Infof("handshake has been served by %s", res.Server)
	if !cfg.Check(res.Checksum) {
		return nil, errors.New("invalid service checksum")
	}

	ProtectedRoutes := make(map[string][]protectedRoute)
	for method, prs := range res.ProtectedRoutes {
		for _, route := range prs.Routes {
			pattern, err := regexp.Compile(route.Pattern)
			if err != nil {
				return nil, err
			}

			if route.Method == "" {
				route.Method = "protect"
				if route.IsStrict {
					route.Method = "strict"
				}
			}

			ProtectedRoutes[method] = append(ProtectedRoutes[method], protectedRoute{
				pattern: pattern,
				method:  route.Method,
			})
		}
	}

	return &Composite{
		Key:             cfg.Key,
		Endpoint:        cfg.gatewayEndpoint,
		Connection:      conn,
		Url:             "",
		ServiceClient:   sc,
		ProtectedRoutes: ProtectedRoutes,
	}, nil
}

func (c Composite) Keys() string {
	return c.Key
}

func (c Composite) Endpoints() string {
	return c.Endpoint
}

func (c Composite) Stop() error {
	err := c.Connection.Close()
	if err != nil {
		return fmt.Errorf("while closing composite connection: %v", err)
	}

	return nil
}

func (c Composite) IsNeedProtection(method string, path string) (bool, bool, bool) {
	if routes, ok := c.ProtectedRoutes[method]; ok {
		for _, route := range routes {
			if route.pattern.MatchString(path) {
				switch route.method {
				case "strict":
					return true, true, false

				case "private":
					return true, false, true

				case "protect":
					return true, false, false

				default:
					return false, false, false
				}
			}
		}
	}

	return false, false, false
}
