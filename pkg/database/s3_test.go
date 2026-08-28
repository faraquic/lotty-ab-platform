package database_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

	t.Run("does not create bucket as side effect", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		if !isS3Reachable("localhost:9000") {
			t.Skip("MinIO not reachable at localhost:9000")
		}

		bucketName := "new-test-bucket-should-not-exist"

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := database.NewS3(ctx, database.S3Config{
			Bucket:    bucketName,
			Region:    "us-east-1",
			Endpoint:  "http://localhost:9000",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
		}, log)
		if err == nil {
			t.Fatal("NewS3 succeeded for non-existent bucket; expected failure")
		}

		verifyCfg, err := awsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithRegion("us-east-1"),
			awsconfig.WithCredentialsProvider(
				aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
					return aws.Credentials{
						AccessKeyID:     "minioadmin",
						SecretAccessKey: "minioadmin",
					}, nil
				}),
			),
		)
		if err != nil {
			t.Fatalf("load verify config: %v", err)
		}

		verifyClient := s3.NewFromConfig(verifyCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String("http://localhost:9000")
			o.UsePathStyle = true
		})

		_, headErr := verifyClient.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucketName)})
		if headErr == nil {
			t.Error("bucket was created by NewS3; expected it to not exist")
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
			Bucket:    "labp",
			Region:    "us-east-1",
			Endpoint:  "http://localhost:9000",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
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

	t.Run("HeadBucket error for missing bucket is not nil", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		if !isS3Reachable("localhost:9000") {
			t.Skip("MinIO not reachable at localhost:9000")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		_, err := database.NewS3(ctx, database.S3Config{
			Bucket:   "another-non-existent-bucket-99999",
			Region:   "us-east-1",
			Endpoint: "http://localhost:9000",
		}, log)
		if err == nil {
			t.Fatal("expected error for missing bucket")
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
