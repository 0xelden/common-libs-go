package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	redis "github.com/redis/go-redis/v9"

	"github.com/0xelden/common-libs-go/gateway/models"
	"github.com/0xelden/common-libs-go/gateway/service"
)

func newTestRegistry(t *testing.T) (*Registry, *Connection) {
	t.Helper()
	addr := os.Getenv("REDIS_HOST")
	if addr == "" {
		t.Skip("REDIS_HOST not set")
	}

	conn, err := NewConnection(&redis.Options{Addr: addr})
	if err != nil {
		t.Fatalf("NewConnection() err = %v", err)
	}

	key := fmt.Sprintf("common-libs-go:test:registry:%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_ = conn.Del(context.Background(), key, key+":composite").Err()
		_ = conn.Close()
	})

	return NewRegistry(key, conn), conn
}

func TestRegistryWriteRead(t *testing.T) {
	reg, _ := newTestRegistry(t)

	cfg := service.Config{Host: "127.0.0.1", Port: 8080, Key: "svc-a", Name: "svc-a"}
	if err := reg.Write(cfg); err != nil {
		t.Fatalf("Write() err = %v", err)
	}

	got, err := reg.GetByKey(cfg.Key)
	if err != nil {
		t.Fatalf("GetByKey() err = %v", err)
	}
	if got.Host != cfg.Host || got.Port != cfg.Port || got.Key != cfg.Key {
		t.Fatalf("GetByKey() = %+v, want %+v", got, cfg)
	}

	keys, err := reg.Keys()
	if err != nil || len(keys) != 1 || keys[0] != cfg.Key {
		t.Fatalf("Keys() = %v, %v, want [%s], nil", keys, err, cfg.Key)
	}

	all, err := reg.Get()
	if err != nil || len(all) != 1 {
		t.Fatalf("Get() = %v, %v, want 1 config", all, err)
	}

	if _, err := reg.GetByKey("missing"); !errors.Is(err, service.ErrConfigNotFound) {
		t.Fatalf("GetByKey(missing) err = %v, want ErrConfigNotFound", err)
	}
}

func TestRegistryGetRedisByID(t *testing.T) {
	reg, conn := newTestRegistry(t)

	want := models.Composite{ID: "composite-1", Secret: "secret"}
	b, _ := json.Marshal(want)
	if err := conn.HSet(context.Background(), reg.key+":composite", string(want.ID), b).Err(); err != nil {
		t.Fatalf("HSet() err = %v", err)
	}

	got, err := reg.GetRedisByID(want.ID)
	if err != nil || got != want {
		t.Fatalf("GetRedisByID() = %+v, %v, want %+v, nil", got, err, want)
	}

	if _, err := reg.GetRedisByID("missing"); err == nil {
		t.Fatal("GetRedisByID(missing) err = nil, want error")
	}
}

func TestRegistryPublishWatch(t *testing.T) {
	reg, _ := newTestRegistry(t)

	ch, err := reg.WatchRaw()
	if err != nil {
		t.Fatalf("WatchRaw() err = %v", err)
	}

	// PSUBSCRIBE is confirmed asynchronously, so keep publishing until the
	// subscriber sees a message instead of racing a single Publish.
	want := []byte("hello")
	deadline := time.After(5 * time.Second)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if err := reg.PublishRaw(want); err != nil {
			t.Fatalf("PublishRaw() err = %v", err)
		}
		select {
		case got := <-ch:
			if string(got) != string(want) {
				t.Fatalf("WatchRaw() got %q, want %q", got, want)
			}
			return
		case <-tick.C:
		case <-deadline:
			t.Fatal("WatchRaw() received nothing within 5s")
		}
	}
}
