package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.uber.org/zap"
)

type S3Config struct {
	Bucket   string
	Region   string
	Endpoint string // optional: S3-compatible endpoint (MinIO, etc.)
}

func NewS3(ctx context.Context, cfg S3Config, log *zap.Logger) (*s3.Client, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("s3 bucket is empty")
	}

	opts := []func(*awsconfig.LoadOptions) error{}
	if cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(cfg.Region))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	var s3Opts []func(*s3.Options)
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	createCtx, createCancel := context.WithTimeout(ctx, pingTimeout)
	defer createCancel()

	_, createErr := client.CreateBucket(createCtx, &s3.CreateBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if createErr != nil {
		var ownedErr *types.BucketAlreadyOwnedByYou
		var existsErr *types.BucketAlreadyExists
		if !errors.As(createErr, &ownedErr) && !errors.As(createErr, &existsErr) {
			return nil, fmt.Errorf("create bucket %s: %w", cfg.Bucket, createErr)
		}
	}

	if _, err := client.HeadBucket(pingCtx, &s3.HeadBucketInput{Bucket: aws.String(cfg.Bucket)}); err != nil {
		return nil, fmt.Errorf("head bucket %s: %w", cfg.Bucket, err)
	}

	log.Info("connected to s3",
		zap.String("bucket", cfg.Bucket),
		zap.String("region", cfg.Region),
		zap.String("endpoint", cfg.Endpoint),
	)

	return client, nil
}
