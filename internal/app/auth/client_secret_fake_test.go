package auth

import (
	"context"
	"time"

	applications "github.com/guliangiam/guliangiam/internal/domain/applications"
)

// fakeAppClientSecrets 为 loginTestAppRepository 补齐 Repository 新方法。
// 键带 organization_id：替身按租户隔离，跨租户读取为空集，与真实仓储的
// 显式 organization_id 过滤保持一致。
var fakeAppClientSecrets = map[string][]*applications.ClientSecret{}

func fakeAppClientSecretKey(organizationID, applicationID string) string {
	return organizationID + "|" + applicationID
}

func (r *loginTestAppRepository) CreateClientSecret(_ context.Context, organizationID string, secret *applications.ClientSecret) error {
	secret.OrganizationID = organizationID
	key := fakeAppClientSecretKey(organizationID, secret.ApplicationID)
	fakeAppClientSecrets[key] = append(fakeAppClientSecrets[key], secret)
	return nil
}

func (r *loginTestAppRepository) ListClientSecrets(_ context.Context, organizationID, applicationID string) ([]*applications.ClientSecret, error) {
	return fakeAppClientSecrets[fakeAppClientSecretKey(organizationID, applicationID)], nil
}

func (r *loginTestAppRepository) ListUsableClientSecrets(_ context.Context, organizationID, applicationID string, now time.Time) ([]*applications.ClientSecret, error) {
	all := fakeAppClientSecrets[fakeAppClientSecretKey(organizationID, applicationID)]
	usable := make([]*applications.ClientSecret, 0, len(all))
	for _, secret := range all {
		if secret.UsableAt(now) {
			usable = append(usable, secret)
		}
	}
	return usable, nil
}

func (r *loginTestAppRepository) RotateClientSecret(_ context.Context, organizationID, applicationID string, next *applications.ClientSecret, _, _ time.Time) error {
	key := fakeAppClientSecretKey(organizationID, applicationID)
	for _, secret := range fakeAppClientSecrets[key] {
		if secret.Status == applications.ClientSecretStatusActive {
			secret.Status = applications.ClientSecretStatusGrace
		}
	}
	next.OrganizationID = organizationID
	fakeAppClientSecrets[key] = append(fakeAppClientSecrets[key], next)
	return nil
}

func (r *loginTestAppRepository) RetireClientSecrets(_ context.Context, organizationID, applicationID string, _ time.Time) error {
	for _, secret := range fakeAppClientSecrets[fakeAppClientSecretKey(organizationID, applicationID)] {
		secret.Status = applications.ClientSecretStatusRetired
	}
	return nil
}

// fakeAppClientJWKs mirrors the per-tenant filtering of app_client_jwks.
var fakeAppClientJWKs = map[string][]*applications.ClientJWK{}

func (r *loginTestAppRepository) ListActiveClientJWKs(_ context.Context, organizationID, applicationID string) ([]*applications.ClientJWK, error) {
	all := fakeAppClientJWKs[fakeAppClientSecretKey(organizationID, applicationID)]
	active := make([]*applications.ClientJWK, 0, len(all))
	for _, key := range all {
		if key.Status == applications.ClientJWKStatusActive {
			active = append(active, key)
		}
	}
	return active, nil
}

func (r *loginTestAppRepository) ReplaceClientJWKs(_ context.Context, organizationID, applicationID string, keys []*applications.ClientJWK, replacedAt time.Time) error {
	storeKey := fakeAppClientSecretKey(organizationID, applicationID)
	for _, existing := range fakeAppClientJWKs[storeKey] {
		if existing.Status == applications.ClientJWKStatusActive {
			existing.Status = applications.ClientJWKStatusRetired
			existing.RetiredAt = &replacedAt
		}
	}
	for _, incoming := range keys {
		incoming.OrganizationID = organizationID
		incoming.ApplicationID = applicationID
		incoming.Status = applications.ClientJWKStatusActive
		incoming.RetiredAt = nil
		fakeAppClientJWKs[storeKey] = append(fakeAppClientJWKs[storeKey], incoming)
	}
	return nil
}

// fakeAppClientCertificates mirrors the per-tenant filtering of
// app_client_certificates: the key includes organization_id, so a cross-tenant
// read is empty.
var fakeAppClientCertificates = map[string][]*applications.ClientCertificate{}

func (r *loginTestAppRepository) ListActiveClientCertificates(_ context.Context, organizationID, applicationID string) ([]*applications.ClientCertificate, error) {
	all := fakeAppClientCertificates[fakeAppClientSecretKey(organizationID, applicationID)]
	active := make([]*applications.ClientCertificate, 0, len(all))
	for _, certificate := range all {
		if certificate.Status == applications.ClientCertificateStatusActive {
			active = append(active, certificate)
		}
	}
	return active, nil
}

func (r *loginTestAppRepository) ReplaceClientCertificates(_ context.Context, organizationID, applicationID string, certificates []*applications.ClientCertificate, replacedAt time.Time) error {
	storeKey := fakeAppClientSecretKey(organizationID, applicationID)
	for _, existing := range fakeAppClientCertificates[storeKey] {
		if existing.Status == applications.ClientCertificateStatusActive {
			existing.Status = applications.ClientCertificateStatusRetired
			existing.RetiredAt = &replacedAt
		}
	}
	for _, incoming := range certificates {
		incoming.OrganizationID = organizationID
		incoming.ApplicationID = applicationID
		incoming.Status = applications.ClientCertificateStatusActive
		incoming.RetiredAt = nil
		fakeAppClientCertificates[storeKey] = append(fakeAppClientCertificates[storeKey], incoming)
	}
	return nil
}
