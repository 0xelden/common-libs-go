package redis

import (
	"fmt"

	redis "github.com/go-redis/redis/v7"
)

type Connection struct {
	*redis.Client
}

func NewConnection(opts *redis.Options) (*Connection, error) {
	conn := redis.NewClient(opts)
	_, err := conn.Ping().Result()
	if err != nil {
		return nil, fmt.Errorf("while pinging redis: %v", err)
	}

	return &Connection{conn}, nil
}

func (Connection) Kind() string {
	return "redis"
}
