package users

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Storage interface {
	UploadAvatar(ctx context.Context, userID int64, ext string, data []byte) (string, error)
	DeleteAvatar(ctx context.Context, avatarURL string) error
}

type S3Storage struct {
	client   *s3.Client
	bucket   string
	endpoint string
}

func NewS3Storage(client *s3.Client, bucket, endpoint string) *S3Storage {
	return &S3Storage{client: client, bucket: bucket, endpoint: endpoint}
}

func (s *S3Storage) UploadAvatar(ctx context.Context, userID int64, ext string, data []byte) (string, error) {
	key := fmt.Sprintf("avatars/%d%s", userID, ext)

	contentType := mimeFromExt(ext)

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("put avatar: %w", err)
	}

	return s.objectURL(key), nil
}

func (s *S3Storage) DeleteAvatar(ctx context.Context, avatarURL string) error {
	key := s.keyFromURL(avatarURL)
	if key == "" {
		return nil
	}

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete avatar: %w", err)
	}

	return nil
}

func (s *S3Storage) objectURL(key string) string {
	endpoint := strings.TrimRight(s.endpoint, "/")
	return fmt.Sprintf("%s/%s/%s", endpoint, s.bucket, key)
}

func (s *S3Storage) keyFromURL(avatarURL string) string {
	prefix := strings.TrimRight(s.endpoint, "/") + "/" + s.bucket + "/"
	if !strings.HasPrefix(avatarURL, prefix) {
		return ""
	}
	return strings.TrimPrefix(avatarURL, prefix)
}

func mimeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func ExtFromFilename(filename string) string {
	return strings.ToLower(path.Ext(filename))
}
