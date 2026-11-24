package storage

import (
	"context"
	"fmt"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BackblazeB2Storage stores objects in Backblaze B2 via its S3-compatible API.
//
// 兼容差异登记（OPEN-API-75 P7，已按
// https://backblaze.com/docs/cloud-storage-s3-compatible-api 复核）：
//   - 端点为 https://s3.<region>.backblazeb2.com，region 形如 us-west-004、eu00
//     （B2 区域名不兼容 AWS region 命名，必须原样参与 SigV4 scope）；
//   - 凭据为 application key ID / application key，SigV4 与 AWS 一致；
//   - 两种寻址均支持，保持 SDK 默认 virtual-host；
//   - 原生 B2 API（application key 授权 + accountId 头）未接入：统一走 S3 兼容层，
//     与 aws_s3 签名链复用，差异仅端点与 region 格式。
type BackblazeB2Storage struct {
	Provider
}

// NewBackblazeB2Storage creates a Backblaze B2 storage provider.
// endpoint 为空时按 region 推导 https://s3.<region>.backblazeb2.com。
func NewBackblazeB2Storage(ctx context.Context, accessKey, secretKey, region, endpoint, bucket string) (Provider, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		if strings.TrimSpace(region) == "" {
			return nil, apperrors.NewInvalidArgument("backblaze b2 storage requires region (e.g. us-west-004) or a custom S3 endpoint")
		}
		endpoint = b2Endpoint(region)
	}
	p, err := newS3Storage(ctx, accessKey, secretKey, defaultRegion(region), endpoint, bucket, false)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "backblaze b2 storage")
	}
	return &BackblazeB2Storage{Provider: p}, nil
}

// b2Endpoint returns the S3-compatible endpoint for a B2 region name.
func b2Endpoint(region string) string {
	return fmt.Sprintf("https://s3.%s.backblazeb2.com", strings.ToLower(strings.TrimSpace(region)))
}

// ensure BackblazeB2Storage implements Provider at compile time.
var _ Provider = (*BackblazeB2Storage)(nil)
