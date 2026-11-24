package tokens

import (
	"context"
	"time"

	applications "github.com/guliangiam/guliangiam/internal/domain/applications"
)

// fakeTokenAppClientSecrets 为 mockAppRepo 补齐 Repository 新方法。
// 键带 organization_id，跨租户读取为空集。
var fakeTokenAppClientSecrets = map[string][]*applications.ClientSecret{}

func fakeTokenAppClientSecretKey(organizationID, applicationID string) string {
	return organizationID + "|" + applicationID
}

func (r *mockAppRepo) CreateClientSecret(_ context.Context, organizationID string, secret *applications.ClientSecret) error {
	secret.OrganizationID = organizationID
	key := fakeTokenAppClientSecretKey(organizationID, secret.ApplicationID)
	fakeTokenAppClientSecrets[key] = append(fakeTokenAppClientSecrets[key], secret)
	return nil
}

func (r *mockAppRepo) ListClientSecrets(_ context.Context, organizationID, applicationID string) ([]*applications.ClientSecret, error) {
	return fakeTokenAppClientSecrets[fakeTokenAppClientSecretKey(organizationID, applicationID)], nil
}

func (r *mockAppRepo) ListUsableClientSecrets(_ context.Context, organizationID, applicationID string, now time.Time) ([]*applications.ClientSecret, error) {
	all := fakeTokenAppClientSecrets[fakeTokenAppClientSecretKey(organizationID, applicationID)]
	usable := make([]*applications.ClientSecret, 0, len(all))
	for _, secret := range all {
		if secret.UsableAt(now) {
			usable = append(usable, secret)
		}
	}
	return usable, nil
}

func (r *mockAppRepo) RotateClientSecret(_ context.Context, organizationID, applicationID string, next *applications.ClientSecret, _, _ time.Time) error {
	key := fakeTokenAppClientSecretKey(organizationID, applicationID)
	for _, secret := range fakeTokenAppClientSecrets[key] {
		if secret.Status == applications.ClientSecretStatusActive {
			secret.Status = applications.ClientSecretStatusGrace
		}
	}
	next.OrganizationID = organizationID
	fakeTokenAppClientSecrets[key] = append(fakeTokenAppClientSecrets[key], next)
	return nil
}

func (r *mockAppRepo) RetireClientSecrets(_ context.Context, organizationID, applicationID string, _ time.Time) error {
	for _, secret := range fakeTokenAppClientSecrets[fakeTokenAppClientSecretKey(organizationID, applicationID)] {
		secret.Status = applications.ClientSecretStatusRetired
	}
	return nil
}

func (r *mockAppRepo) ListActiveClientJWKs(_ context.Context, _, _ string) ([]*applications.ClientJWK, error) {
	return nil, nil
}

func (r *mockAppRepo) ReplaceClientJWKs(_ context.Context, _, _ string, _ []*applications.ClientJWK, _ time.Time) error {
	return nil
}

func (r *mockAppRepo) ListActiveClientCertificates(_ context.Context, _, _ string) ([]*applications.ClientCertificate, error) {
	return nil, nil
}

func (r *mockAppRepo) ReplaceClientCertificates(_ context.Context, _, _ string, _ []*applications.ClientCertificate, _ time.Time) error {
	return nil
}
