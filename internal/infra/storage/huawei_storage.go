package storage

import (
	"context"
	"fmt"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// HuaweiOBSStorage stores objects in Huawei Cloud OBS using the S3-compatible API.
//
// 校准（OPEN-API-75 P7）：OBS 原生签名（V4 AK/SK HMAC-SHA256，
// https://support.huaweicloud.com/intl/zh-cn/api-obs/obs_04_0011.html）未接入；
// 本实现走 OBS 的 S3 兼容层（SigV4，https://support.huaweicloud.com/intl/zh-cn/api-obs/obs_04_0007.html），
// 端点 https://obs.<region>.myhuaweicloud.com 与现行区域端点一致；
// OBS 的 S3 兼容层支持 virtual-host 与 path-style 两种寻址。
type HuaweiOBSStorage struct {
	Provider
}

// NewHuaweiOBSStorage creates a new Huawei Cloud OBS storage provider.
// region should be an OBS region like "cn-north-1", "cn-east-2", etc.
// The endpoint is constructed as: https://obs.{region}.myhuaweicloud.com
func NewHuaweiOBSStorage(ctx context.Context, accessKey, secretKey, region, bucket string) (Provider, error) {
	endpoint := fmt.Sprintf("https://obs.%s.myhuaweicloud.com", region)
	p, err := NewS3Storage(ctx, accessKey, secretKey, region, endpoint, bucket)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "huawei obs storage")
	}
	return &HuaweiOBSStorage{Provider: p}, nil
}

// ensure HuaweiOBSStorage implements Provider at compile time.
var _ Provider = (*HuaweiOBSStorage)(nil)
