package storage

import (
	"context"
	"fmt"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// COSStorage stores objects in Tencent Cloud COS using the S3-compatible API.
//
// 校准（OPEN-API-75 P7）：COS 原生签名 v5（q-sign-algorithm=sha1 HMAC-SHA1，
// https://cloud.tencent.com/document/product/436/7778）未接入；本实现走 COS 的
// S3 兼容层（SigV4，https://cloud.tencent.com/document/product/436/37421），
// 端点 https://cos.<region>.myqcloud.com 与现行文档一致；virtual-host 寻址要求
// bucket 名为 <name>-<appid> 形态（COS 强制 appid 后缀），region 参与 SigV4 scope。
type COSStorage struct {
	Provider
}

// NewCOSStorage creates a new Tencent Cloud COS storage provider.
// region should be a COS region like "ap-beijing", "ap-shanghai", etc.
// The endpoint is constructed as: https://cos.{region}.myqcloud.com
func NewCOSStorage(ctx context.Context, accessKey, secretKey, region, bucket string) (Provider, error) {
	endpoint := fmt.Sprintf("https://cos.%s.myqcloud.com", region)
	p, err := NewS3Storage(ctx, accessKey, secretKey, region, endpoint, bucket)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "cos storage")
	}
	return &COSStorage{Provider: p}, nil
}

// ensure COSStorage implements Provider at compile time.
var _ Provider = (*COSStorage)(nil)
