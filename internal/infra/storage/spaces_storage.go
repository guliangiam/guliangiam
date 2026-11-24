package storage

import (
	"context"
	"fmt"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DigitalOceanSpacesStorage stores objects in DigitalOcean Spaces via its
// S3-compatible API.
//
// 兼容差异登记（OPEN-API-75 P7，已按
// https://docs.digitalocean.com/products/spaces/reference/s3-compatibility/ 复核）：
//   - 端点为 https://<region>.digitaloceanspaces.com，region 形如 nyc3、sgp1、fra3；
//   - SigV4 与 AWS 一致（仅支持 SigV4，不支持 SigV2；payload 签名按 SDK 默认）；
//   - virtual-host 与 path-style 均支持，保持 SDK 默认 virtual-host；
//   - 不支持 S3 的 ACL/部分 API（如 Select），本实现仅用 Put/Get/Delete/List/Presign，
//     均在兼容矩阵内。
type DigitalOceanSpacesStorage struct {
	Provider
}

// NewDigitalOceanSpacesStorage creates a DigitalOcean Spaces storage provider.
// endpoint 为空时按 region 推导 https://<region>.digitaloceanspaces.com。
func NewDigitalOceanSpacesStorage(ctx context.Context, accessKey, secretKey, region, endpoint, bucket string) (Provider, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		if strings.TrimSpace(region) == "" {
			return nil, apperrors.NewInvalidArgument("digitalocean spaces storage requires region (e.g. nyc3) or a custom S3 endpoint")
		}
		endpoint = spacesEndpoint(region)
	}
	p, err := newS3Storage(ctx, accessKey, secretKey, defaultRegion(region), endpoint, bucket, false)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "digitalocean spaces storage")
	}
	return &DigitalOceanSpacesStorage{Provider: p}, nil
}

// spacesEndpoint returns the Spaces endpoint for a DO region slug.
func spacesEndpoint(region string) string {
	return fmt.Sprintf("https://%s.digitaloceanspaces.com", strings.ToLower(strings.TrimSpace(region)))
}

// ensure DigitalOceanSpacesStorage implements Provider at compile time.
var _ Provider = (*DigitalOceanSpacesStorage)(nil)
