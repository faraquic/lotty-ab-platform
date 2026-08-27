package database_test

import (
	"context"
	"net"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/faraquic/lotty-ab-platform/pkg/database"
)

func TestNewRedis(t *testing.T) {
	log := zap.NewNop()

	t.Run("empty URL", func(t *testing.T) {
		client, err := database.NewRedis(context.Background(), "", log)
		if client != nil {
			t.Errorf("client = %v, want nil", client)
		}
		if err == nil {
			t.Fatal("err = nil, want error")
		}
		if err.Error() != "redis url is empty" {
			t.Errorf("err = %q, want %q", err.Error(), "redis url is empty")
		}
	})

	t.Run("invalid URL syntax", func(t *testing.T) {
		client, err := database.NewRedis(context.Background(), "://bad", log)
		if client != nil {
			t.Errorf("client = %v, want nil", client)
		}
		if err == nil {
			t.Fatal("err = nil, want error")
		}
	})

	t.Run("unreachable host — ping fails", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client, err := database.NewRedis(ctx, "192.0.2.1:6379", log)
		if client != nil {
			t.Errorf("client = %v, want nil on ping failure", client)
		}
		if err == nil {
			t.Fatal("err = nil, want ping error")
		}
	})

	t.Run("connection refused", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		client, err := database.NewRedis(ctx, "localhost:1", log)
		if client != nil {
			t.Errorf("client = %v, want nil", client)
		}
		if err == nil {
			t.Fatal("err = nil, want connection error")
		}
	})

	t.Run("plain host:port — toRedisURL adds scheme", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		if !isRedisReachable("localhost:6379") {
			t.Skip("redis not reachable at localhost:6379")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		client, err := database.NewRedis(ctx, "localhost:6379", log)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		defer (*client).Close()
	})

	t.Run("full redis URL — passthrough", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		if !isRedisReachable("localhost:6379") {
			t.Skip("redis not reachable at localhost:6379")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		client, err := database.NewRedis(ctx, "redis://localhost:6379", log)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		defer (*client).Close()
	})

	t.Run("wrong password — ping fails", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		if !isRedisReachable("localhost:6379") {
			t.Skip("redis not reachable at localhost:6379")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		client, err := database.NewRedis(ctx, "redis://:wrongpassword@localhost:6379", log)
		if client != nil {
			t.Errorf("client = %v, want nil on auth failure", client)
		}
		if err == nil {
			t.Fatal("err = nil, want auth error")
		}
	})
}

func isRedisReachable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
