package redis

import (
	"context"
	"fmt"

	redis "github.com/redis/go-redis/v9"
)

type Connection struct {
	*redis.Client
}

func NewConnection(opts *redis.Options) (*Connection, error) {
	conn := redis.NewClient(opts)
	_, err := conn.Ping(context.Background()).Result()
	if err != nil {
		return nil, fmt.Errorf("while pinging redis: %v", err)
	}

	return &Connection{conn}, nil
}

func (Connection) Kind() string {
	return "redis"
}
