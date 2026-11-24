package applications

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

const (
	queryListActiveClientCertificates = `
SELECT ` + clientCertificateColumns + `
FROM guliangiam.app_client_certificates
WHERE organization_id = $1 AND application_id = $2 AND status = 'active'
ORDER BY created_at ASC, id ASC`

	queryRetireActiveClientCertificates = `
UPDATE guliangiam.app_client_certificates
SET status = 'retired',
    retired_at = $3,
    updated_at = $3,
    revision = revision + 1
WHERE organization_id = $1 AND application_id = $2 AND status = 'active'`

	// 唯一索引是 (organization_id, fingerprint_sha256)：同一租户内一个指纹只属于
	// 一个应用，因此冲突时把行改绑到本次 applicationID 并重新激活。
	queryUpsertClientCertificate = `
INSERT INTO guliangiam.app_client_certificates (
    id, organization_id, application_id, auth_method, certificate_pem,
    fingerprint_sha256, subject_dn, issuer_dn, not_before, not_after, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'active')
ON CONFLICT (organization_id, fingerprint_sha256) DO UPDATE
SET application_id = EXCLUDED.application_id,
    auth_method = EXCLUDED.auth_method,
    certificate_pem = EXCLUDED.certificate_pem,
    subject_dn = EXCLUDED.subject_dn,
    issuer_dn = EXCLUDED.issuer_dn,
    not_before = EXCLUDED.not_before,
    not_after = EXCLUDED.not_after,
    status = 'active',
    retired_at = NULL,
    updated_at = $11,
    revision = guliangiam.app_client_certificates.revision + 1
WHERE guliangiam.app_client_certificates.organization_id = EXCLUDED.organization_id
RETURNING created_at, updated_at, revision`

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
FROM guliangiam.app_client_certificates c
JOIN guliangiam.applications a ON a.id = c.application_id
WHERE c.status = 'active'
  AND c.not_after IS NOT NULL
  AND c.not_after > $1
  AND c.not_after <= $2
ORDER BY c.not_after ASC, c.id ASC
LIMIT $3`
)

var _ domain.ExpiringClientCertificateScanner = Repository{}

// ListExpiringClientCertificates 扫描即将过期的 mTLS 客户端证书。仅供维护作业
// 在系统作用域调用：不做组织过滤，跨租户返回候选行由调用方按组织分组投递。
func (r Repository) ListExpiringClientCertificates(ctx context.Context, now, deadline time.Time, limit int) ([]*domain.ExpiringClientCertificate, error) {
	rows, err := r.db.Query(ctx, queryListExpiringClientCertificates, now, deadline, limit)
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

// ListActiveClientCertificates returns the active certificate bindings for a
// client. The organization predicate is explicit and comes from the resolved
// application row, never from the presented TLS certificate.
func (r Repository) ListActiveClientCertificates(ctx context.Context, organizationID, applicationID string) ([]*domain.ClientCertificate, error) {
	rows, err := r.db.Query(ctx, queryListActiveClientCertificates, organizationID, applicationID)
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

// ReplaceClientCertificates retires every active row and then upserts the
// supplied set in one transaction.
func (r Repository) ReplaceClientCertificates(ctx context.Context, organizationID, applicationID string, certificates []*domain.ClientCertificate, replacedAt time.Time) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx shared.DBTX) error {
		if _, err := tx.Exec(ctx, queryRetireActiveClientCertificates, organizationID, applicationID, replacedAt); err != nil {
			return err
		}
		for _, certificate := range certificates {
			if certificate == nil {
				continue
			}
			var createdAt, updatedAt time.Time
			var revision int64
			if err := tx.QueryRow(ctx, queryUpsertClientCertificate,
				certificate.ID, organizationID, applicationID, certificate.AuthMethod,
				certificate.CertificatePEM, certificate.FingerprintSHA256,
				certificate.SubjectDN, certificate.IssuerDN,
				certificate.NotBefore, certificate.NotAfter, replacedAt,
			).Scan(&createdAt, &updatedAt, &revision); err != nil {
				return err
			}
			certificate.ApplicationID = applicationID
			certificate.OrganizationID = organizationID
			certificate.Status = domain.ClientCertificateStatusActive
			certificate.RetiredAt = nil
			certificate.CreatedAt = createdAt
			certificate.UpdatedAt = updatedAt
			certificate.Revision = revision
		}
		return nil
	})
}
