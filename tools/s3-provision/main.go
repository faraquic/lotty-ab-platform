package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func main() {
	endpoint := flag.String("endpoint", "http://localhost:9000", "S3-compatible endpoint")
	bucket := flag.String("bucket", "", "bucket name (required)")
	region := flag.String("region", "us-east-1", "S3 region")
	accessKey := flag.String("access-key", "minioadmin", "S3 access key")
	secretKey := flag.String("secret-key", "minioadmin", "S3 secret key")
	flag.Parse()

	if *bucket == "" {
		fmt.Fprintln(os.Stderr, "error: -bucket is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(*region),
		awsconfig.WithCredentialsProvider(
			aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     *accessKey,
					SecretAccessKey: *secretKey,
				}, nil
			}),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load aws config: %v\n", err)
		os.Exit(1)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(*endpoint)
		o.UsePathStyle = true
	})

	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(*bucket),
	})
	if err != nil {
		var ownedErr *types.BucketAlreadyOwnedByYou
		var existsErr *types.BucketAlreadyExists
		if !errors.As(err, &ownedErr) && !errors.As(err, &existsErr) {
			fmt.Fprintf(os.Stderr, "create bucket %s: %v\n", *bucket, err)
			os.Exit(1)
		}
	}

	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(*bucket)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "head bucket %s: %v\n", *bucket, err)
		os.Exit(1)
	}

	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Sid": "PublicReadAvatars",
				"Effect": "Allow",
				"Principal": {"AWS": ["*"]},
				"Action": ["s3:GetObject"],
				"Resource": ["arn:aws:s3:::%s/avatars/*"]
			}
		]
	}`, *bucket)

	_, err = client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(*bucket),
		Policy: aws.String(policy),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "set bucket policy %s: %v\n", *bucket, err)
		os.Exit(1)
	}

	fmt.Printf("provisioned bucket %s\n", *bucket)
}
