package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/0xelden/common-libs-go/logger"
)

var (
	ErrConfigNotFound = errors.New("controller not found")
)

type RegistryWriter interface {
	Publish(Config) error
	Write(Config) error
}

type RegistryReader interface {
	Get() ([]Config, error)
	GetByKey(string) (Config, error)
	Watch() (<-chan Config, error)
}

type Registry interface {
	RegistryWriter
	RegistryReader
}

type Config struct {
	Host            string
	Port            int
	Key             string
	Name            string
	Namespace       string
	TypeConn        string
	gatewayEndpoint string
}

type jsonConfig struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	Key             string `json:"key"`
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	TypeConn        string `json:"typeconn"`
	GatewayEndpoint string `json:"gateway_endpoint"`
}

func (cfg Config) MarshalJSON() ([]byte, error) {
	jc := jsonConfig{
		Host:            cfg.Host,
		Port:            cfg.Port,
		Key:             cfg.Key,
		Name:            cfg.Name,
		Namespace:       cfg.Namespace,
		TypeConn:        cfg.TypeConn,
		GatewayEndpoint: cfg.gatewayEndpoint,
	}

	return json.Marshal(jc)
}

func (cfg *Config) UnmarshalJSON(v []byte) error {
	var tmp jsonConfig

	err := json.Unmarshal(v, &tmp)
	if err != nil {
		return err
	}

	cfg.Host = tmp.Host
	cfg.Port = tmp.Port
	cfg.Key = tmp.Key
	cfg.Name = tmp.Name
	cfg.Namespace = tmp.Namespace
	cfg.TypeConn = tmp.TypeConn
	cfg.gatewayEndpoint = tmp.GatewayEndpoint

	return nil
}

func (cfg Config) GatewayEndpoint() string {
	return cfg.gatewayEndpoint
}

func (cfg Config) HasGatewayEndpoint() bool {
	return cfg.gatewayEndpoint != ""
}

func (cfg Config) Check(checksum string) bool {
	logger.Infof("comparing checksum %s = %s", checksum, cfg.checksum())
	return strings.EqualFold(checksum, cfg.checksum())
}

func (cfg Config) checksum() string {
	s256 := sha256.Sum256([]byte(fmt.Sprintf(
		"<%s:%d:%s:%s:%s:%s:%s>",
		cfg.Host,
		cfg.Port,
		cfg.Key,
		strings.ToLower(cfg.Namespace),
		strings.ToLower(cfg.TypeConn),
		strings.ToLower(strings.Replace(cfg.Name, " ", "", -1)),
		strings.Trim(cfg.gatewayEndpoint, "/"),
	)))

	return hex.EncodeToString(s256[:])
}
