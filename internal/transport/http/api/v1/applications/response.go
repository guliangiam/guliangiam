package applications

import (
	"time"

	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type ListApplicationsResponse = presenter.ListResponse[appapplications.ApplicationListItem]
type ApplicationResponse = appapplications.ApplicationDetail
type ApplicationPublicResponse = appapplications.ApplicationPublicDetail

type ApplicationCustomScopeResponse = appapplications.ApplicationCustomScopeItem

type ApplicationProviderBindingResponseItem struct {
	ProviderID string `json:"providerId"`
}

type ApplicationProviderBindingsResponse struct {
	Items []ApplicationProviderBindingResponseItem `json:"items"`
}

// ApplicationClientCertificateResponseItem 是客户端证书的读侧投影：
// 只含指纹与元数据，证书原文（certificate）绝不出现。
type ApplicationClientCertificateResponseItem struct {
	FingerprintSHA256 string     `json:"fingerprintSha256"`
	NotAfter          *time.Time `json:"notAfter"`
	SelfSigned        bool       `json:"selfSigned"`
	Status            string     `json:"status"`
}

type ApplicationClientCertificatesResponse struct {
	Items []ApplicationClientCertificateResponseItem `json:"items"`
}

// PatchApplicationResponse 在成功响应里回传落库后的 revision。
// 前端据此立即同步乐观锁基线，无需等待详情 refetch。
type PatchApplicationResponse struct {
	Status   string `json:"status"`
	Revision int64  `json:"revision"`
}

type ReplaceProviderBindingsResponse struct {
	Status   string `json:"status"`
	Revision int64  `json:"revision"`
}

type CreateApplicationResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	// 一次性明文：仅在创建响应出现，读接口只暴露 clientSecretSet / clientSecretMasked。
	ClientSecretPlain  string `json:"clientSecretPlain,omitempty"`
	ClientSecretPrefix string `json:"clientSecretPrefix,omitempty"`
}

// RotateClientSecretResponse 是轮换响应；明文同样只出现一次。
type RotateClientSecretResponse struct {
	ClientSecretPlain  string    `json:"clientSecretPlain"`
	ClientSecretPrefix string    `json:"clientSecretPrefix"`
	RotatedAt          time.Time `json:"rotatedAt"`
	GraceUntil         time.Time `json:"graceUntil"`
}

type ApplicationDeleteSummaryResponse = appapplications.ApplicationDeleteSummary
