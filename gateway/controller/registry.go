package controller

import (
	"fmt"

	redis "github.com/go-redis/redis/v7"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	redisreg "github.com/0xelden/common-libs-go/gateway/controller/redis"
	"github.com/0xelden/common-libs-go/gateway/controller/resolver"
	"github.com/0xelden/common-libs-go/gateway/service"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/logger"
	"github.com/0xelden/common-libs-go/shared"
)

const (
	DefaultRegistryKey = "gateway"
)

type RegistryConfig struct {
	Address  string
	Password string
}

type Registry struct {
	conn     *redisreg.Connection
	reader   service.RegistryReader
	writer   service.RegistryWriter
	resolver resolver.Resolver
}

type Connection interface {
	Kind() string
}

func InitRegistry(cfg RegistryConfig) (*Registry, error) {
	logger.Infof("initializing registry with controller %v", cfg)
	redisConn, err := redisreg.NewConnection(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
	})
	if err != nil {
		return nil, err
	}
	reg := redisreg.NewRegistry(helper.Env(shared.AppKeyGateway, DefaultRegistryKey), redisConn)

	resolv, errx := service.NewResolver()
	if errx != nil {
		return nil, errx.Cause()
	}

	return &Registry{redisConn, reg, reg, resolv}, nil
}

func (reg Registry) GetConnection(key string) (*grpc.ClientConn, error) {
	logger.Infof("get service controller %s on registry", key)
	cfg, err := reg.reader.GetByKey(key)
	if err != nil {
		return nil, fmt.Errorf("while fetching controller from registry: %v", err)
	}

	connURL := reg.resolver.GenerateURL(cfg.Host, helper.IntToString(cfg.Port))
	logger.Infof("try dialing to service %s(%s) via gRPC", key, connURL)

	const grpcServiceConfig = `{"loadBalancingPolicy":"round_robin"}`

	conn, err := grpc.Dial(
		connURL,
		grpc.WithDefaultServiceConfig(grpcServiceConfig),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(150*1024*1024),
		),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
	)
	if err != nil {
		return nil, fmt.Errorf("while dialing server: %v", err)
	}
	logger.Infof("connected with %s", key)

	return conn, nil
}

func (reg Registry) ForkConnection() Connection {
	return reg.conn
}
