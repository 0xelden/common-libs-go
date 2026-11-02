package connection

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/0xelden/common-libs-go/helper"
	_ "github.com/joho/godotenv/autoload"
	"github.com/redis/go-redis/v9"
)

func TestConnectRedis(t *testing.T) {
	if helper.Env("REDIS_HOST") == "" {
		t.Skip("REDIS_HOST not set")
	}

	client, serr := ConnectRedis()
	if serr != nil {
		t.Fatalf("ConnectRedis() serr = %v", serr)
	}
	t.Cleanup(func() { _ = client.Close() })

	var (
		ctx = context.Background()
		key = fmt.Sprintf("common-libs-go:test:%d", time.Now().UnixNano())
	)
	t.Cleanup(func() { _ = client.Del(ctx, key).Err() })

	if err := client.Set(ctx, key, "value", time.Minute).Err(); err != nil {
		t.Fatalf("Set() err = %v", err)
	}
	got, err := client.Get(ctx, key).Result()
	if err != nil || got != "value" {
		t.Fatalf("Get() = %q, %v, want %q, nil", got, err, "value")
	}

	if err := client.Del(ctx, key).Err(); err != nil {
		t.Fatalf("Del() err = %v", err)
	}
	if _, err := client.Get(ctx, key).Result(); !errors.Is(err, redis.Nil) {
		t.Fatalf("Get() after Del err = %v, want redis.Nil", err)
	}
}
