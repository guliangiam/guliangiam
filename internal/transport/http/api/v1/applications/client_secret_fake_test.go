package applications

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
)

// fakeByIDClientSecrets 为 byIDApplicationRepo 补齐 Repository 新方法。
// 键带 organization_id，跨租户读取为空集。
var fakeByIDClientSecrets = map[string][]*domain.ClientSecret{}

func fakeByIDClientSecretKey(organizationID, applicationID string) string {
	return organizationID + "|" + applicationID
}

func (r *byIDApplicationRepo) CreateClientSecret(_ context.Context, organizationID string, secret *domain.ClientSecret) error {
	secret.OrganizationID = organizationID
	key := fakeByIDClientSecretKey(organizationID, secret.ApplicationID)
	fakeByIDClientSecrets[key] = append(fakeByIDClientSecrets[key], secret)
	return nil
}

func (r *byIDApplicationRepo) ListClientSecrets(_ context.Context, organizationID, applicationID string) ([]*domain.ClientSecret, error) {
	return fakeByIDClientSecrets[fakeByIDClientSecretKey(organizationID, applicationID)], nil
}

func (r *byIDApplicationRepo) ListUsableClientSecrets(_ context.Context, organizationID, applicationID string, now time.Time) ([]*domain.ClientSecret, error) {
	all := fakeByIDClientSecrets[fakeByIDClientSecretKey(organizationID, applicationID)]
	usable := make([]*domain.ClientSecret, 0, len(all))
	for _, secret := range all {
		if secret.UsableAt(now) {
			usable = append(usable, secret)
		}
	}
	return usable, nil
}

func (r *byIDApplicationRepo) RotateClientSecret(_ context.Context, organizationID, applicationID string, next *domain.ClientSecret, _, _ time.Time) error {
	key := fakeByIDClientSecretKey(organizationID, applicationID)
	for _, secret := range fakeByIDClientSecrets[key] {
		if secret.Status == domain.ClientSecretStatusActive {
			secret.Status = domain.ClientSecretStatusGrace
		}
	}
	next.OrganizationID = organizationID
	fakeByIDClientSecrets[key] = append(fakeByIDClientSecrets[key], next)
	return nil
}

func (r *byIDApplicationRepo) RetireClientSecrets(_ context.Context, organizationID, applicationID string, _ time.Time) error {
	for _, secret := range fakeByIDClientSecrets[fakeByIDClientSecretKey(organizationID, applicationID)] {
		secret.Status = domain.ClientSecretStatusRetired
	}
	return nil
}

func (r *byIDApplicationRepo) ListActiveClientJWKs(_ context.Context, _, _ string) ([]*domain.ClientJWK, error) {
	return nil, nil
}

func (r *byIDApplicationRepo) ReplaceClientJWKs(_ context.Context, _, _ string, _ []*domain.ClientJWK, _ time.Time) error {
	return nil
}

func (r *byIDApplicationRepo) ListActiveClientCertificates(_ context.Context, _, _ string) ([]*domain.ClientCertificate, error) {
	return nil, nil
}

func (r *byIDApplicationRepo) ReplaceClientCertificates(_ context.Context, _, _ string, _ []*domain.ClientCertificate, _ time.Time) error {
	return nil
}
