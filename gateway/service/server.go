package service

import (
	"fmt"
	"net"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	"github.com/0xelden/common-libs-go/gateway/packets"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/logger"
	"github.com/0xelden/common-libs-go/shared"
)

type Server struct {
	cfg      Config
	svc      *Service
	instance *grpc.Server
	listener net.Listener
	reg      RegistryWriter
}

func NewServer(cfg Config, reg RegistryWriter) (*Server, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return nil, fmt.Errorf("while opening listener: %v", err)
	}

	return &Server{
		cfg: cfg,
		instance: grpc.NewServer(
			grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
			grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
		),
		listener: listener,
		reg:      reg,
	}, nil
}

func NewServerHttp(cfg Config, reg RegistryWriter) (*Server, error) {
	return &Server{
		cfg: cfg,
		instance: grpc.NewServer(
			grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
			grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
		),
		reg: reg,
	}, nil
}

func (svr *Server) AsGatewayService(baseEndpoint string) *Service {
	svr.cfg.gatewayEndpoint = baseEndpoint
	svc := &Service{
		key:          helper.Env(shared.AppName, shared.AppName),
		namespace:    helper.Env(shared.AppNamespace, shared.NamespaceDefault),
		baseEndpoint: baseEndpoint,
		checksum:     svr.cfg.checksum(),
		router: router{
			routes:          make(map[string][]Route),
			protectedRoutes: make(map[string][]protectedRoute),
		},
	}

	packets.RegisterServiceServer(svr.instance, svc)

	return svc
}

func (svr Server) Instance() *grpc.Server {
	return svr.instance
}

func (svr Server) Start() error {
	err := svr.reg.Write(svr.cfg)
	if err != nil {
		return fmt.Errorf("while writing controller: %v", err)
	}

	if svr.cfg.HasGatewayEndpoint() {
		logger.Infof("send notify to gateway with controller %v", svr.cfg)
		err := svr.reg.Publish(svr.cfg)
		if err != nil {
			return fmt.Errorf("while publishing controller: %v", err)
		}
	}

	logger.Infof("service is listening on %s.", fmt.Sprintf("%s:%d", svr.cfg.Host, svr.cfg.Port))
	return svr.instance.Serve(svr.listener)
}

func (svr Server) Write() error {
	err := svr.reg.Write(svr.cfg)
	if err != nil {
		return fmt.Errorf("while writing controller: %v", err)
	}

	if svr.cfg.HasGatewayEndpoint() {
		logger.Infof("send notify to gateway with controller %v", svr.cfg)
		err := svr.reg.Publish(svr.cfg)
		if err != nil {
			return fmt.Errorf("while publishing controller: %v", err)
		}
	}

	return nil
}

func (svr Server) Stop() error {
	svr.Stop()

	err := svr.listener.Close()
	if err != nil {
		return fmt.Errorf("while closing listener: %v", err)
	}

	return nil
}
