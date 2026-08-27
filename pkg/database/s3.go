package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/goccy/go-json"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.uber.org/zap"
)

type S3Config struct {
	Bucket    string
	Region    string
	Endpoint  string
	AccessKey string
	SecretKey string
}

func NewS3(ctx context.Context, cfg S3Config, log *zap.Logger) (*s3.Client, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("s3 bucket is empty")
	}

	opts := []func(*awsconfig.LoadOptions) error{}
	if cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(cfg.Region))
	}
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     cfg.AccessKey,
					SecretAccessKey: cfg.SecretKey,
				}, nil
			}),
		))
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

	policyCtx, policyCancel := context.WithTimeout(ctx, pingTimeout)
	defer policyCancel()

	publicReadPolicy := map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Sid":       "PublicReadAvatars",
				"Effect":    "Allow",
				"Principal": "*",
				"Action":    "s3:GetObject",
				"Resource":  fmt.Sprintf("arn:aws:s3:::%s/avatars/*", cfg.Bucket),
			},
		},
	}

	policyBytes, err := json.Marshal(publicReadPolicy)
	if err != nil {
		return nil, fmt.Errorf("marshal bucket policy: %w", err)
	}

	if _, err := client.PutBucketPolicy(policyCtx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(cfg.Bucket),
		Policy: aws.String(string(policyBytes)),
	}); err != nil {
		log.Warn("failed to set bucket policy, avatars may not be publicly accessible", zap.Error(err))
	}

	log.Info(
		"connected to s3",
		zap.String("bucket", cfg.Bucket),
		zap.String("region", cfg.Region),
		zap.String("endpoint", cfg.Endpoint),
	)

	return client, nil
}
