package applications

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

const (
	btClientCertificates = "`app_client_certificates`"

	queryListActiveClientCertificates = `
SELECT ` + clientCertificateColumns + `
FROM ` + btClientCertificates + `
WHERE organization_id = ? AND application_id = ? AND status = 'active'
ORDER BY created_at ASC, id ASC`

	queryRetireActiveClientCertificates = `
UPDATE ` + btClientCertificates + `
SET status = 'retired',
    retired_at = ?,
    updated_at = ?,
    revision = revision + 1
WHERE organization_id = ? AND application_id = ? AND status = 'active'`

	// MySQL 没有 RLS，organization_id 必须是语句的一部分。唯一索引
	// (organization_id, fingerprint_sha256) 只在同一租户内命中，因此
	// ON DUPLICATE KEY UPDATE 不会跨租户改绑。
	queryUpsertClientCertificate = `
INSERT INTO ` + btClientCertificates + ` (
    id, organization_id, application_id, auth_method, certificate_pem,
    fingerprint_sha256, subject_dn, issuer_dn, not_before, not_after,
    status, created_at, updated_at, revision
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, 0)
ON DUPLICATE KEY UPDATE
    organization_id = VALUES(organization_id),
    application_id = VALUES(application_id),
    auth_method = VALUES(auth_method),
    certificate_pem = VALUES(certificate_pem),
    subject_dn = VALUES(subject_dn),
    issuer_dn = VALUES(issuer_dn),
    not_before = VALUES(not_before),
    not_after = VALUES(not_after),
    status = 'active',
    retired_at = NULL,
    updated_at = VALUES(updated_at),
    revision = revision + 1`

	clientCertificateColumns = `
id, organization_id, application_id, auth_method, certificate_pem,
fingerprint_sha256, subject_dn, issuer_dn, not_before, not_after, status,
retired_at, created_at, updated_at, revision`

	// queryListExpiringClientCertificates 是系统作用域的跨组织扫描：只取 active
	// 且 not_after 落在提醒窗口内（now, deadline] 的证书，并带出应用展示名。
	queryListExpiringClientCertificates = `
SELECT c.id, c.organization_id, c.application_id, c.auth_method, c.certificate_pem,
       c.fingerprint_sha256, c.subject_dn, c.issuer_dn, c.not_before, c.not_after, c.status,
       c.retired_at, c.created_at, c.updated_at, c.revision,
       COALESCE(NULLIF(a.display_name, ''), a.slug) AS application_name
FROM ` + btClientCertificates + ` c
JOIN ` + btApp + ` a ON a.id = c.application_id
WHERE c.status = 'active'
  AND c.not_after IS NOT NULL
  AND c.not_after > ?
  AND c.not_after <= ?
ORDER BY c.not_after ASC, c.id ASC
LIMIT ?`
)

var _ domain.ExpiringClientCertificateScanner = Repository{}

// ListExpiringClientCertificates 扫描即将过期的 mTLS 客户端证书。仅供维护作业
// 在系统作用域调用：不做组织过滤，跨租户返回候选行由调用方按组织分组投递。
func (r Repository) ListExpiringClientCertificates(ctx context.Context, now, deadline time.Time, limit int) ([]*domain.ExpiringClientCertificate, error) {
	rows, err := r.db.QueryContext(ctx, queryListExpiringClientCertificates, now, deadline, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*domain.ExpiringClientCertificate, 0)
	for rows.Next() {
		var item domain.ExpiringClientCertificate
		var status string
		if err := rows.Scan(
			&item.ID, &item.OrganizationID, &item.ApplicationID,
			&item.AuthMethod, &item.CertificatePEM,
			&item.FingerprintSHA256, &item.SubjectDN, &item.IssuerDN,
			&item.NotBefore, &item.NotAfter, &status,
			&item.RetiredAt, &item.CreatedAt, &item.UpdatedAt,
			&item.Revision, &item.ApplicationName,
		); err != nil {
			return nil, err
		}
		item.Status = domain.ClientCertificateStatus(status)
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// ListActiveClientCertificates 返回某应用登记为 active 的证书（MySQL 镜像，受 OPEN-API-37 约束）。
func (r Repository) ListActiveClientCertificates(ctx context.Context, organizationID, applicationID string) ([]*domain.ClientCertificate, error) {
	rows, err := r.db.QueryContext(ctx, queryListActiveClientCertificates, organizationID, applicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	certificates := make([]*domain.ClientCertificate, 0)
	for rows.Next() {
		var certificate domain.ClientCertificate
		var status string
		if err := rows.Scan(
			&certificate.ID, &certificate.OrganizationID, &certificate.ApplicationID,
			&certificate.AuthMethod, &certificate.CertificatePEM,
			&certificate.FingerprintSHA256, &certificate.SubjectDN, &certificate.IssuerDN,
			&certificate.NotBefore, &certificate.NotAfter, &status,
			&certificate.RetiredAt, &certificate.CreatedAt, &certificate.UpdatedAt,
			&certificate.Revision,
		); err != nil {
			return nil, err
		}
		certificate.Status = domain.ClientCertificateStatus(status)
		certificates = append(certificates, &certificate)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return certificates, nil
}

// ReplaceClientCertificates 在同一事务内先退役所有 active 行，再 upsert 新集合。
func (r Repository) ReplaceClientCertificates(ctx context.Context, organizationID, applicationID string, certificates []*domain.ClientCertificate, replacedAt time.Time) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx mysqlshared.DBTX) error {
		if _, err := tx.ExecContext(ctx, queryRetireActiveClientCertificates, replacedAt, replacedAt, organizationID, applicationID); err != nil {
			return err
		}
		for _, certificate := range certificates {
			if certificate == nil {
				continue
			}
			if _, err := tx.ExecContext(ctx, queryUpsertClientCertificate,
				certificate.ID, organizationID, applicationID, certificate.AuthMethod,
				certificate.CertificatePEM, certificate.FingerprintSHA256,
				certificate.SubjectDN, certificate.IssuerDN,
				certificate.NotBefore, certificate.NotAfter, replacedAt, replacedAt,
			); err != nil {
				return err
			}
			certificate.ApplicationID = applicationID
			certificate.OrganizationID = organizationID
			certificate.Status = domain.ClientCertificateStatusActive
			certificate.RetiredAt = nil
			certificate.CreatedAt = replacedAt
			certificate.UpdatedAt = replacedAt
		}
		return nil
	})
}
