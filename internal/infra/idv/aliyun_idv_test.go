package idv

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 未知阿里云码会坍缩成泛化 503；每个分支必须同时把 provider 与原始错误码
// 放进 Meta，前端才能显示真实阿里云码（如 RAM 无权限/产品未开通），
// 而不是只剩「服务暂时不可用」。
func TestMapAliyunErrorCodeEveryBranchCarriesProviderMeta(t *testing.T) {
	cases := []struct {
		name string
		code string
		want apperrors.Code
	}{
		{"credential rejected", "InvalidAccessKeyId.NotFound", apperrors.CodeUnauthorized},
		{"denied access", "Forbidden.NoPermission", apperrors.CodeForbidden},
		{"throttled", "Throttling.User", apperrors.CodeUnavailable},
		{"rejected request", "InvalidParameter.CertNo", apperrors.CodeInvalidArgument},
		{"unknown default", "SomeBrandNewUpstreamCode", apperrors.CodeUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := mapAliyunErrorCode("aliyun_idv", tc.code, "message", "InitFaceVerify")
			appErr, ok := err.(*apperrors.Error)
			if !ok {
				t.Fatalf("expected *apperrors.Error, got %T", err)
			}
			if appErr.Code != tc.want {
				t.Errorf("code = %q, want %q", appErr.Code, tc.want)
			}
			if got := appErr.Meta["provider"]; got != "aliyun_idv" {
				t.Errorf("meta[provider] = %v, want aliyun_idv", got)
			}
			if got := appErr.Meta["providerCode"]; got != tc.code {
				t.Errorf("meta[providerCode] = %v, want %q", got, tc.code)
			}
		})
	}
}
