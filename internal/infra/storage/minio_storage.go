package storage

import (
	"context"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// MinIOStorage stores objects in a MinIO server using the S3-compatible API.
//
// 校准（OPEN-API-75 P7）：MinIO 默认要求 path-style 寻址（virtual-host 需通配符
// DNS + 通配符证书，服务端默认不开启，已按 https://min.io/docs/minio/linux/index.html
// S3 API 兼容性说明复核）。本实现经 newS3Storage 强制 UsePathStyle=true，并沿用
// aws-sdk-go-v2 的 SigV4 签名链（MinIO 完整支持 SigV4）。
type MinIOStorage struct {
	Provider
}

// NewMinIOStorage creates a new MinIO storage provider.
// endpoint should be the full MinIO endpoint, e.g. "http://localhost:9000" or "https://minio.example.com".
// region is typically "us-east-1" for MinIO unless configured otherwise.
func NewMinIOStorage(ctx context.Context, accessKey, secretKey, region, endpoint, bucket string) (Provider, error) {
	p, err := newS3Storage(ctx, accessKey, secretKey, defaultRegion(region), endpoint, bucket, true)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "minio storage")
	}
	return &MinIOStorage{Provider: p}, nil
}

// ensure MinIOStorage implements Provider at compile time.
var _ Provider = (*MinIOStorage)(nil)
