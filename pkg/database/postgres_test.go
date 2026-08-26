package database_test

import (
	"context"
	"net"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/faraquic/lotty-ab-platform/pkg/database"
)

func TestNewPostgres(t *testing.T) {
	log := zap.NewNop()

	t.Run("empty DSN", func(t *testing.T) {
		pool, err := database.NewPostgres(context.Background(), "", 4, 2, log)
		if pool != nil {
			t.Errorf("pool = %v, want nil", pool)
		}
		if err == nil {
			t.Fatal("err = nil, want error")
		}
		if err.Error() != "postgres dsn is empty" {
			t.Errorf("err = %q, want %q", err.Error(), "postgres dsn is empty")
		}
	})

	t.Run("invalid DSN syntax", func(t *testing.T) {
		pool, err := database.NewPostgres(context.Background(), "not://a:valid/dsn", 4, 2, log)
		if pool != nil {
			t.Errorf("pool = %v, want nil", pool)
		}
		if err == nil {
			t.Fatal("err = nil, want error")
		}
		if err.Error() == "" {
			t.Error("error message is empty")
		}
	})

	t.Run("unreachable host — pool created then ping fails", func(t *testing.T) {
		// 192.0.2.1 is TEST-NET-1 (RFC 5737), guaranteed non-routable;
		// connection attempt times out rather than being refused instantly.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pool, err := database.NewPostgres(ctx, "postgres://u:p@192.0.2.1:5432/db?sslmode=disable&connect_timeout=1", 1, 1, log)
		if pool != nil {
			t.Errorf("pool = %v, want nil on ping failure", pool)
			pool.Close()
		}
		if err == nil {
			t.Fatal("err = nil, want ping error")
		}
		if err.Error() == "" {
			t.Error("error message is empty")
		}
	})

	t.Run("context cancelled before ping", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(2 * time.Millisecond) // ensure timeout fires

		pool, err := database.NewPostgres(ctx, "postgres://u:p@localhost:5432/db?sslmode=disable&connect_timeout=1", 1, 1, log)
		if pool != nil {
			t.Errorf("pool = %v, want nil", pool)
			pool.Close()
		}
		if err == nil {
			t.Fatal("err = nil, want timeout error")
		}
	})

	t.Run("connection refused — immediate error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		pool, err := database.NewPostgres(ctx, "postgres://u:p@localhost:1/db?sslmode=disable&connect_timeout=1", 1, 1, log)
		if pool != nil {
			t.Errorf("pool = %v, want nil", pool)
			pool.Close()
		}
		if err == nil {
			t.Fatal("err = nil, want connection error")
		}
	})

	t.Run("valid DSN — integration", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}

		dsn := "postgres://lotty:lottypassword@localhost:5433/labp?sslmode=disable"
		if !isReachable("localhost:5433") {
			t.Skip("postgres not reachable at localhost:5433")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pool, err := database.NewPostgres(ctx, dsn, 4, 2, log)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		defer pool.Close()

		if pool == nil {
			t.Fatal("pool is nil, want *pgxpool.Pool")
		}
	})

	t.Run("wrong password — ping fails", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}

		dsn := "postgres://lotty:wrongpassword@localhost:5433/labp?sslmode=disable&connect_timeout=1"
		if !isReachable("localhost:5433") {
			t.Skip("postgres not reachable at localhost:5433")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pool, err := database.NewPostgres(ctx, dsn, 1, 1, log)
		if pool != nil {
			t.Errorf("pool = %v, want nil on auth failure", pool)
			pool.Close()
		}
		if err == nil {
			t.Fatal("err = nil, want auth error")
		}
	})
}

// isReachable checks if a TCP address is reachable.
func isReachable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
