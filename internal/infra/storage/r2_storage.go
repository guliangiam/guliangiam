package storage

import (
	"context"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// CloudflareR2Storage stores objects in Cloudflare R2 via its S3-compatible API.
//
// 兼容差异登记（OPEN-API-75 P7，已按
// https://developers.cloudflare.com/r2/api/s3/api/ 复核）：
//   - 端点为 https://<account_id>.r2.cloudflarestorage.com（由调用方以 Host 提供）；
//   - region 固定为 "auto"（R2 不接受真实 AWS region 参与 SigV4 scope）；
//   - SigV4 与 payload 签名与 AWS 一致（SDK 默认 UNSIGNED-PAYLOAD 仅用于
//     presigned URL，R2 支持）；两种寻址（bucket-style / path-style）均可用，
//     保持 SDK 默认 virtual-host，自定义域名场景需 path-style 时另行配置。
type CloudflareR2Storage struct {
	Provider
}

// NewCloudflareR2Storage creates a Cloudflare R2 storage provider.
// endpoint must be the full R2 S3 endpoint
// ("https://<account_id>.r2.cloudflarestorage.com"); region defaults to "auto".
func NewCloudflareR2Storage(ctx context.Context, accessKey, secretKey, region, endpoint, bucket string) (Provider, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, apperrors.NewInvalidArgument("cloudflare r2 storage requires the S3 endpoint (https://<account_id>.r2.cloudflarestorage.com)")
	}
	if strings.TrimSpace(region) == "" {
		region = "auto"
	}
	p, err := newS3Storage(ctx, accessKey, secretKey, region, endpoint, bucket, false)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "cloudflare r2 storage")
	}
	return &CloudflareR2Storage{Provider: p}, nil
}

// ensure CloudflareR2Storage implements Provider at compile time.
var _ Provider = (*CloudflareR2Storage)(nil)
