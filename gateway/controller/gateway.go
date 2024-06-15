package controller

import (
	"fmt"

	"github.com/0xelden/common-libs-go/gateway/controller/resolver"
	"github.com/0xelden/common-libs-go/gateway/service"
	"github.com/0xelden/common-libs-go/logger"
	"github.com/0xelden/common-libs-go/serror"
)

type Config struct {
	Port         int
	Key          string
	Name         string
	BaseEndpoint string
}

type Gateway struct {
	fwd      service.Forwarder
	reg      service.RegistryReader
	resolver resolver.Resolver
}

func New(fwd service.Forwarder, reg service.RegistryReader) Gateway {
	return Gateway{
		fwd: fwd,
		reg: reg,
	}
}

func (g *Gateway) Open() error {
	resolv, errx := service.NewResolver()
	if errx != nil {
		return errx.Cause()
	}

	g.resolver = resolv

	configs, err := g.reg.Get()
	if err != nil {
		return fmt.Errorf("while reading controller from registry: %v", err)
	}

	for _, cfg := range configs {
		go func(cfg service.Config) {
			c, err := service.NewComposite(g.resolver, cfg)
			if err != nil {
				logger.Err(serror.NewFromErrorc(err, fmt.Sprintf("while registering service %s", cfg.Key)))
				return
			}

			logger.Infof("service %s successful registered.", cfg.Key)
			g.fwd.Mount(c)
		}(cfg)
	}

	ch, err := g.reg.Watch()
	if err != nil {
		return err
	}

	go func(ch <-chan service.Config, fwd service.Forwarder) {
		for cfg := range ch {
			go func(cfg service.Config) {
				logger.Info("incoming server registration...")

				c, err := service.NewComposite(g.resolver, cfg)
				if err != nil {
					logger.Err(serror.NewFromErrorc(err, fmt.Sprintf("while registering service %s", cfg.Key)))
					return
				}

				logger.Infof("service %s successful registered.", cfg.Key)
				if cfg.TypeConn != "http" {
					fwd.Mount(c)
				}
			}(cfg)
		}
	}(ch, g.fwd)

	return nil
}
