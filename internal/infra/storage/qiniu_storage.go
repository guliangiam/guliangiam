package storage

import (
	"context"
	"fmt"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// QiniuStorage stores objects in Qiniu Cloud Storage using the S3-compatible API.
type QiniuStorage struct {
	Provider
}

// NewQiniuStorage creates a new Qiniu Cloud storage provider.
// region should be a Qiniu region like "z0" (East China), "z1" (North China), "z2" (South China), etc.
// The S3-compatible endpoint is constructed as: https://s3-{region}.qiniucs.com
func NewQiniuStorage(ctx context.Context, accessKey, secretKey, region, bucket string) (Provider, error) {
	endpoint := fmt.Sprintf("https://s3-%s.qiniucs.com", region)
	p, err := NewS3Storage(ctx, accessKey, secretKey, region, endpoint, bucket)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "qiniu storage")
	}
	return &QiniuStorage{Provider: p}, nil
}

// ensure QiniuStorage implements Provider at compile time.
var _ Provider = (*QiniuStorage)(nil)
