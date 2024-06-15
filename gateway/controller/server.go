package controller

import (
	"github.com/0xelden/common-libs-go/gateway/service"
)

type ServerConfig struct {
	Host      string
	Port      int
	Key       string
	Name      string
	Namespace string
	TypeConn  string
}

func NewServer(cfg ServerConfig, reg *Registry) (*service.Server, error) {
	return service.NewServer(service.Config{
		Host:      cfg.Host,
		Port:      cfg.Port,
		Key:       cfg.Key,
		Name:      cfg.Name,
		Namespace: cfg.Namespace,
		TypeConn:  cfg.TypeConn,
	}, reg.writer)
}

func NewServerHttp(cfg ServerConfig, reg *Registry) (*service.Server, error) {
	return service.NewServerHttp(service.Config{
		Host:      cfg.Host,
		Port:      cfg.Port,
		Key:       cfg.Key,
		Name:      cfg.Name,
		Namespace: cfg.Namespace,
		TypeConn:  cfg.TypeConn,
	}, reg.writer)
}
