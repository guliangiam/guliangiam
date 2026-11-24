package applications

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
)

// fakeClientSecretStore 支撑接管 Repository 新方法的测试替身。
// 键是 organization_id + application_id：替身本身也按租户隔离，跨租户读取
// 得到空集合，与真实仓储的显式 organization_id 过滤保持一致。
var fakeClientSecretStore = map[string][]*domain.ClientSecret{}

func fakeClientSecretKey(organizationID, applicationID string) string {
	return organizationID + "|" + applicationID
}

func (r *mockApplicationRepo) CreateClientSecret(_ context.Context, organizationID string, secret *domain.ClientSecret) error {
	secret.OrganizationID = organizationID
	key := fakeClientSecretKey(organizationID, secret.ApplicationID)
	fakeClientSecretStore[key] = append(fakeClientSecretStore[key], secret)
	return nil
}

func (r *mockApplicationRepo) ListClientSecrets(_ context.Context, organizationID, applicationID string) ([]*domain.ClientSecret, error) {
	return fakeClientSecretStore[fakeClientSecretKey(organizationID, applicationID)], nil
}

func (r *mockApplicationRepo) ListUsableClientSecrets(_ context.Context, organizationID, applicationID string, now time.Time) ([]*domain.ClientSecret, error) {
	all := fakeClientSecretStore[fakeClientSecretKey(organizationID, applicationID)]
	usable := make([]*domain.ClientSecret, 0, len(all))
	for _, secret := range all {
		if secret.UsableAt(now) {
			usable = append(usable, secret)
		}
	}
	return usable, nil
}

func (r *mockApplicationRepo) RotateClientSecret(_ context.Context, organizationID, applicationID string, next *domain.ClientSecret, _, _ time.Time) error {
	key := fakeClientSecretKey(organizationID, applicationID)
	for _, secret := range fakeClientSecretStore[key] {
		if secret.Status == domain.ClientSecretStatusActive {
			secret.Status = domain.ClientSecretStatusGrace
		}
	}
	next.OrganizationID = organizationID
	fakeClientSecretStore[key] = append(fakeClientSecretStore[key], next)
	return nil
}

func (r *mockApplicationRepo) RetireClientSecrets(_ context.Context, organizationID, applicationID string, _ time.Time) error {
	for _, secret := range fakeClientSecretStore[fakeClientSecretKey(organizationID, applicationID)] {
		secret.Status = domain.ClientSecretStatusRetired
	}
	return nil
}

// fakeClientJWKStore mirrors the per-tenant filtering of app_client_jwks: the
// key includes organization_id, so a cross-tenant read is empty.
var fakeClientJWKStore = map[string][]*domain.ClientJWK{}

func (r *mockApplicationRepo) ListActiveClientJWKs(_ context.Context, organizationID, applicationID string) ([]*domain.ClientJWK, error) {
	all := fakeClientJWKStore[fakeClientSecretKey(organizationID, applicationID)]
	active := make([]*domain.ClientJWK, 0, len(all))
	for _, key := range all {
		if key.Status == domain.ClientJWKStatusActive {
			active = append(active, key)
		}
	}
	return active, nil
}

func (r *mockApplicationRepo) ReplaceClientJWKs(_ context.Context, organizationID, applicationID string, keys []*domain.ClientJWK, replacedAt time.Time) error {
	storeKey := fakeClientSecretKey(organizationID, applicationID)
	for _, existing := range fakeClientJWKStore[storeKey] {
		if existing.Status == domain.ClientJWKStatusActive {
			existing.Status = domain.ClientJWKStatusRetired
			existing.RetiredAt = &replacedAt
		}
	}
	for _, incoming := range keys {
		incoming.OrganizationID = organizationID
		incoming.ApplicationID = applicationID
		incoming.Status = domain.ClientJWKStatusActive
		incoming.RetiredAt = nil
		fakeClientJWKStore[storeKey] = append(fakeClientJWKStore[storeKey], incoming)
	}
	return nil
}

// fakeClientCertificateStore mirrors the per-tenant filtering of
// app_client_certificates: the key includes organization_id, so a cross-tenant
// read is empty.
var fakeClientCertificateStore = map[string][]*domain.ClientCertificate{}

func (r *mockApplicationRepo) ListActiveClientCertificates(_ context.Context, organizationID, applicationID string) ([]*domain.ClientCertificate, error) {
	all := fakeClientCertificateStore[fakeClientSecretKey(organizationID, applicationID)]
	active := make([]*domain.ClientCertificate, 0, len(all))
	for _, certificate := range all {
		if certificate.Status == domain.ClientCertificateStatusActive {
			active = append(active, certificate)
		}
	}
	return active, nil
}

func (r *mockApplicationRepo) ReplaceClientCertificates(_ context.Context, organizationID, applicationID string, certificates []*domain.ClientCertificate, replacedAt time.Time) error {
	storeKey := fakeClientSecretKey(organizationID, applicationID)
	for _, existing := range fakeClientCertificateStore[storeKey] {
		if existing.Status == domain.ClientCertificateStatusActive {
			existing.Status = domain.ClientCertificateStatusRetired
			existing.RetiredAt = &replacedAt
		}
	}
	for _, incoming := range certificates {
		incoming.OrganizationID = organizationID
		incoming.ApplicationID = applicationID
		incoming.Status = domain.ClientCertificateStatusActive
		incoming.RetiredAt = nil
		fakeClientCertificateStore[storeKey] = append(fakeClientCertificateStore[storeKey], incoming)
	}
	return nil
}
