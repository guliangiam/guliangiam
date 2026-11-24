package applications

import (
	"context"
	"time"
)

// app_client_certificates 的边界值。上传校验与 mTLS 绑定读取共用同一组常量，
// 仓储层不会收到超过上限的行。
const (
	// ClientCertificateMaxPerApplication 是单个客户端允许登记的活跃证书上限。
	// 多张只用于证书轮换窗口，不是证书仓库。
	ClientCertificateMaxPerApplication = 8
	// ClientCertificateMaxPEMBytes 限制单张证书 PEM 的大小。
	ClientCertificateMaxPEMBytes = 16 * 1024
	// ClientCertificateFingerprintLength 是 base64url(SHA-256(DER)) 的长度，
	// 与 app_client_certificates.fingerprint_sha256 VARCHAR(43) 对齐（RFC 8705 x5t#S256）。
	ClientCertificateFingerprintLength = 43
)

// ClientCertificateStatus 是 app_client_certificates.status 的域枚举。
type ClientCertificateStatus string

const (
	ClientCertificateStatusActive  ClientCertificateStatus = "active"
	ClientCertificateStatusRetired ClientCertificateStatus = "retired"
)

// ClientCertificate 是 app_client_certificates 的一行。
//
// 只保存公有证书 PEM 与指纹：私钥材料在上传校验阶段被拒绝，仓储层不会收到。
// OrganizationID 是租户归属，所有查询都以它为显式谓词（MySQL 无 RLS）。
type ClientCertificate struct {
	ID                string
	ApplicationID     string
	OrganizationID    string
	AuthMethod        string
	CertificatePEM    string
	FingerprintSHA256 string
	SubjectDN         string
	IssuerDN          string
	NotBefore         *time.Time
	NotAfter          *time.Time
	Status            ClientCertificateStatus
	RetiredAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Revision          int64
}

// ExpiringClientCertificate 是到期提醒扫描的投影：证书本身加上所属应用名称，
// 让提醒文案不必再逐条回查应用。
type ExpiringClientCertificate struct {
	ClientCertificate
	ApplicationName string
}

// ExpiringClientCertificateScanner 是后台到期提醒的可选扫描能力，只由维护作业在
// 系统作用域调用，不并入 Repository。
type ExpiringClientCertificateScanner interface {
	// ListExpiringClientCertificates 返回 status=active 且 not_after 落在
	// (now, deadline] 的客户端证书，按到期时间升序；limit 限制单次扫描行数。
	ListExpiringClientCertificates(ctx context.Context, now, deadline time.Time, limit int) ([]*ExpiringClientCertificate, error)
}

// UsableAt 判断证书在 now 时刻是否可用于绑定：必须 active，且落在自身的
// 有效期窗口内。有效期由证书自身声明，不由数据库默认值决定。
func (c ClientCertificate) UsableAt(now time.Time) bool {
	if c.Status != ClientCertificateStatusActive {
		return false
	}
	if c.NotBefore != nil && now.Before(*c.NotBefore) {
		return false
	}
	if c.NotAfter != nil && !now.Before(*c.NotAfter) {
		return false
	}
	return true
}
