package redis

import (
	"encoding/json"
	"errors"
	"fmt"

	redis "github.com/go-redis/redis/v7"

	"github.com/0xelden/common-libs-go/gateway/models"
	"github.com/0xelden/common-libs-go/gateway/service"
	"github.com/0xelden/common-libs-go/logger"
	"github.com/0xelden/common-libs-go/serror"
)

var (
	ErrNotFound = errors.New("not found")
)

type Registry struct {
	key  string
	conn *Connection
}

func NewRegistry(key string, conn *Connection) *Registry {
	return &Registry{key, conn}
}

func (reg Registry) WriteRaw(key string, body []byte) error {
	_, err := reg.conn.HSet(reg.key, key, body).Result()
	if err != nil {
		return fmt.Errorf("while writing to redis: %v", err)
	}

	return nil
}

func (reg Registry) Write(cfg service.Config) error {
	b, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("while marshaling json: %v", err)
	}

	return reg.WriteRaw(cfg.Key, b)
}

func (reg Registry) PublishRaw(body []byte) error {
	_, err := reg.conn.Publish(fmt.Sprintf("%s:channel", reg.key), body).Result()
	if err != nil {
		return fmt.Errorf("while publishing to redis: %v", err)
	}

	return nil
}

func (reg Registry) Publish(cfg service.Config) error {
	b, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("while marshaling json: %v", err)
	}

	return reg.PublishRaw(b)
}

func (reg Registry) GetByKeyRaw(key string) ([]byte, error) {
	res, err := reg.conn.HGet(reg.key, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, service.ErrConfigNotFound
		}

		return nil, fmt.Errorf("while reading from redis: %v", err)
	}

	return []byte(res), err
}

func (reg Registry) GetByKey(key string) (service.Config, error) {
	var cfg service.Config

	res, err := reg.GetByKeyRaw(key)
	if err != nil {
		return cfg, err
	}

	err = json.Unmarshal(res, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("while unmarshal json: %v", err)
	}

	return cfg, nil
}

func (reg Registry) GetRaw() (map[string][]byte, error) {
	rows := map[string][]byte{}

	res, err := reg.conn.HGetAll(reg.key).Result()
	if err != nil {
		return nil, fmt.Errorf("while reading from redis: %v", err)
	}

	for k, v := range res {
		rows[k] = []byte(v)
	}
	return rows, err
}

func (reg Registry) Get() ([]service.Config, error) {
	res, err := reg.GetRaw()
	if err != nil {
		return nil, err
	}

	configs := []service.Config{}
	for _, each := range res {
		var cfg service.Config

		err := json.Unmarshal(each, &cfg)
		if err != nil {
			return nil, fmt.Errorf("while unmarshalling json: %v", err)
		}

		configs = append(configs, cfg)
	}

	return configs, nil
}

func (reg Registry) Keys() ([]string, error) {
	res, err := reg.conn.HKeys(reg.key).Result()
	if err != nil {
		return nil, fmt.Errorf("while reading from redis: %v", err)
	}

	return res, nil
}

func (reg Registry) WatchRaw() (<-chan []byte, error) {
	rc := make(chan []byte)
	sub := reg.conn.PSubscribe(fmt.Sprintf("%s:channel", reg.key))
	ch := sub.Channel()

	go func(ch <-chan *redis.Message, cch chan<- []byte) {
		for v := range ch {
			cch <- []byte(v.Payload)
		}
	}(ch, rc)

	return rc, nil
}

func (reg Registry) Watch() (<-chan service.Config, error) {
	ch, err := reg.WatchRaw()
	if err != nil {
		return nil, err
	}

	rc := make(chan service.Config)
	go func(ch <-chan []byte, cch chan<- service.Config) {
		for {
			var cfg service.Config
			dat := <-ch

			err := json.Unmarshal(dat, &cfg)
			if err != nil {
				logger.Err(serror.NewFromErrorc(err, "while unmarshaling json"))
				continue
			}

			cch <- cfg
		}
	}(ch, rc)

	return rc, nil
}

func (reg Registry) GetRedisByID(id models.CompositeID) (models.Composite, error) {
	var composite models.Composite

	res, err := reg.conn.HGet(fmt.Sprintf("%s:composite", reg.key), string(id)).Result()
	if err != nil {
		if err == redis.Nil {
			return composite, errors.New("composite not found")
		}

		return composite, fmt.Errorf("while reading from redis: %v", err)
	}

	err = json.Unmarshal([]byte(res), &composite)
	if err != nil {
		return composite, fmt.Errorf("while unmarshal json: %v", err)
	}

	return composite, nil
}
