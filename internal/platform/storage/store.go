package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Store interface {
	Put(ctx context.Context, key string, data []byte) error
}

type S3 struct {
	client *minio.Client
	bucket string
}

type S3Config struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	UseSSL    bool

	Region string
}

func NewS3(cfg S3Config) (*S3, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("images: S3 endpoint, bucket and keys are all required")
	}
	if cfg.Region == "" {
		cfg.Region = "garage"
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,

		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, err
	}
	return &S3{client: client, bucket: cfg.Bucket}, nil
}

// Put uploads an object.
func (s *S3) Put(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType:  "image/webp",
		CacheControl: "public, max-age=31536000, immutable",
	})
	if err != nil {
		return fmt.Errorf("images: put %s: %w", key, err)
	}
	return nil
}

type Object struct {
	Key  string
	Size int64
}

// Walk calls fn for every object, in key order.
func (s *S3) Walk(ctx context.Context, fn func(Object) error) error {
	for info := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Recursive: true}) {
		if info.Err != nil {
			return fmt.Errorf("images: list: %w", info.Err)
		}
		if err := fn(Object{Key: info.Key, Size: info.Size}); err != nil {
			return err
		}
	}
	return nil
}

// Open returns the content of an object.
func (s *S3) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("images: get %s: %w", key, err)
	}
	return obj, nil
}
