package storage

import (
	"context"
	"fmt"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// OSSStorage stores objects in Alibaba Cloud OSS using the S3-compatible API.
//
// 校准（OPEN-API-75 P7）：OSS 原生 V4 签名（OSS4-HMAC-SHA256，
// https://help.aliyun.com/zh/oss/developer-reference/recommend-to-use-signature-version-4）
// 与 OSS V1 签名均未接入；本实现走 OSS 的 S3 兼容层（SigV4，
// https://help.aliyun.com/zh/oss/developer-reference/compatibility-with-amazon-s3），
// 端点 https://oss-<region>.aliyuncs.com 与现行区域端点一致；path-style 兼容、
// virtual-host 默认（SDK 行为与 OSS 兼容层一致）。
type OSSStorage struct {
	Provider
}

// NewOSSStorage creates a new Alibaba Cloud OSS storage provider.
// region should be an OSS region like "cn-hangzhou", "cn-beijing", etc.
// The endpoint is constructed as: https://oss-{region}.aliyuncs.com
func NewOSSStorage(ctx context.Context, accessKey, secretKey, region, bucket string) (Provider, error) {
	endpoint := fmt.Sprintf("https://oss-%s.aliyuncs.com", region)
	p, err := NewS3Storage(ctx, accessKey, secretKey, region, endpoint, bucket)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "oss storage")
	}
	return &OSSStorage{Provider: p}, nil
}

// ensure OSSStorage implements Provider at compile time.
var _ Provider = (*OSSStorage)(nil)
