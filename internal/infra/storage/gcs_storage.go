package storage

import (
	"context"
	"errors"
	"io"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCSStorage implements storage using Google Cloud Storage.
//
// 校准（OPEN-API-75 P7）：经 cloud.google.com/go/storage 官方客户端，
// JSON service key（option.WithCredentialsFile）或 ADC OAuth 默认凭据，
// 流量走 storage.googleapis.com/storage/v1（JSON API 现行端点，
// https://cloud.google.com/storage/docs/json_api/v1）。
type GCSStorage struct {
	client     *storage.Client
	bucketName string
}

// NewGCSStorage creates a new Google Cloud Storage provider.
// config must contain "bucket" key for the bucket name.
// Optional "credentials" key for JSON credentials path.
func NewGCSStorage(bucketName string, opts ...option.ClientOption) (*GCSStorage, error) {
	client, err := storage.NewClient(context.Background(), opts...)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "create GCS client")
	}

	return &GCSStorage{
		client:     client,
		bucketName: bucketName,
	}, nil
}

func (s *GCSStorage) Put(ctx context.Context, key string, data []byte, contentType string) error {
	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(key)
	w := obj.NewWriter(ctx)
	if contentType != "" {
		w.ContentType = contentType
	}
	if _, err := w.Write(data); err != nil {
		return apperrors.WrapInternalf(err, "write to GCS")
	}
	return w.Close()
}

func (s *GCSStorage) Get(ctx context.Context, key string) ([]byte, error) {
	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(key)
	r, err := obj.NewReader(ctx)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "read from GCS")
	}
	defer r.Close()

	data := make([]byte, r.Attrs.Size)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, apperrors.WrapInternalf(err, "read GCS object")
	}
	return data, nil
}

func (s *GCSStorage) Delete(ctx context.Context, key string) error {
	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(key)
	if err := obj.Delete(ctx); err != nil {
		return apperrors.WrapInternalf(err, "delete from GCS")
	}
	return nil
}

// objectIterator is the subset of storage.ObjectIterator that List consumes;
// keeping it as an interface makes the error handling unit-testable.
type objectIterator interface {
	Next() (*storage.ObjectAttrs, error)
}

func (s *GCSStorage) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	bucket := s.client.Bucket(s.bucketName)
	return listObjects(bucket.Objects(ctx, &storage.Query{Prefix: prefix}))
}

func listObjects(it objectIterator) ([]ObjectInfo, error) {
	var objects []ObjectInfo
	for {
		attrs, err := it.Next()
		if err != nil {
			if errors.Is(err, iterator.Done) {
				break
			}
			// Any other error (permissions, network, bucket missing) must not be
			// reported as an empty successful listing.
			return nil, apperrors.WrapInternalf(err, "list GCS objects")
		}
		objects = append(objects, ObjectInfo{
			Key:          attrs.Name,
			Size:         attrs.Size,
			LastModified: attrs.Updated,
			ContentType:  attrs.ContentType,
		})
	}
	return objects, nil
}

func (s *GCSStorage) GetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(key)

	opts := &storage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(expires),
	}
	signedURL, err := storage.SignedURL(s.bucketName, key, opts)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "generate signed URL")
	}

	_ = obj // bucket reference kept for potential future use
	return signedURL, nil
}

func (s *GCSStorage) PutStream(ctx context.Context, key string, reader io.Reader, contentType string) error {
	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(key)
	w := obj.NewWriter(ctx)
	if contentType != "" {
		w.ContentType = contentType
	}
	if _, err := io.Copy(w, reader); err != nil {
		return apperrors.WrapInternalf(err, "stream to GCS")
	}
	return w.Close()
}
