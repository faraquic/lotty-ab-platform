package database_test

import (
	"context"
	"net"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/faraquic/lotty-ab-platform/pkg/database"
)

func TestNewS3(t *testing.T) {
	log := zap.NewNop()

	t.Run("empty bucket", func(t *testing.T) {
		client, err := database.NewS3(context.Background(), database.S3Config{Bucket: ""}, log)
		if client != nil {
			t.Errorf("client = %v, want nil", client)
		}
		if err == nil {
			t.Fatal("err = nil, want error")
		}
		if err.Error() != "s3 bucket is empty" {
			t.Errorf("err = %q, want %q", err.Error(), "s3 bucket is empty")
		}
	})

	t.Run("unreachable endpoint", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client, err := database.NewS3(ctx, database.S3Config{
			Bucket:   "test-bucket",
			Region:   "us-east-1",
			Endpoint: "http://192.0.2.1:9000",
		}, log)
		if client != nil {
			t.Errorf("client = %v, want nil", client)
		}
		if err == nil {
			t.Fatal("err = nil, want error")
		}
	})

	t.Run("connection refused", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		client, err := database.NewS3(ctx, database.S3Config{
			Bucket:   "test-bucket",
			Region:   "us-east-1",
			Endpoint: "http://localhost:1",
		}, log)
		if client != nil {
			t.Errorf("client = %v, want nil", client)
		}
		if err == nil {
			t.Fatal("err = nil, want error")
		}
	})

	t.Run("non-existent bucket on MinIO", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		if !isS3Reachable("localhost:9000") {
			t.Skip("MinIO not reachable at localhost:9000")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		client, err := database.NewS3(ctx, database.S3Config{
			Bucket:   "non-existent-bucket-12345",
			Region:   "us-east-1",
			Endpoint: "http://localhost:9000",
		}, log)
		if client != nil {
			t.Errorf("client = %v, want nil", client)
		}
		if err == nil {
			t.Fatal("err = nil, want error for non-existent bucket")
		}
	})

	t.Run("valid bucket on MinIO", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		if !isS3Reachable("localhost:9000") {
			t.Skip("MinIO not reachable at localhost:9000")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		client, err := database.NewS3(ctx, database.S3Config{
			Bucket:   "labp",
			Region:   "us-east-1",
			Endpoint: "http://localhost:9000",
		}, log)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if client == nil {
			t.Fatal("client is nil, want *s3.Client")
		}
	})

	t.Run("context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(2 * time.Millisecond)

		client, err := database.NewS3(ctx, database.S3Config{
			Bucket:   "test-bucket",
			Region:   "us-east-1",
			Endpoint: "http://localhost:9000",
		}, log)
		if client != nil {
			t.Errorf("client = %v, want nil", client)
		}
		if err == nil {
			t.Fatal("err = nil, want timeout error")
		}
	})
}

func isS3Reachable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
