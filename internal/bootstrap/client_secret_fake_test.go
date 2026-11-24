package bootstrap

import (
	"context"
	"time"

	applicationsdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
)

// fakeBootstrapClientSecrets 为 fakeApplicationRepository 补齐 Repository 新方法。
// 键带 organization_id，跨租户读取为空集。
var fakeBootstrapClientSecrets = map[string][]*applicationsdomain.ClientSecret{}

func fakeBootstrapClientSecretKey(organizationID, applicationID string) string {
	return organizationID + "|" + applicationID
}

func (r *fakeApplicationRepository) CreateClientSecret(_ context.Context, organizationID string, secret *applicationsdomain.ClientSecret) error {
	secret.OrganizationID = organizationID
	key := fakeBootstrapClientSecretKey(organizationID, secret.ApplicationID)
	fakeBootstrapClientSecrets[key] = append(fakeBootstrapClientSecrets[key], secret)
	return nil
}

func (r *fakeApplicationRepository) ListClientSecrets(_ context.Context, organizationID, applicationID string) ([]*applicationsdomain.ClientSecret, error) {
	return fakeBootstrapClientSecrets[fakeBootstrapClientSecretKey(organizationID, applicationID)], nil
}

func (r *fakeApplicationRepository) ListUsableClientSecrets(_ context.Context, organizationID, applicationID string, now time.Time) ([]*applicationsdomain.ClientSecret, error) {
	all := fakeBootstrapClientSecrets[fakeBootstrapClientSecretKey(organizationID, applicationID)]
	usable := make([]*applicationsdomain.ClientSecret, 0, len(all))
	for _, secret := range all {
		if secret.UsableAt(now) {
			usable = append(usable, secret)
		}
	}
	return usable, nil
}

func (r *fakeApplicationRepository) RotateClientSecret(_ context.Context, organizationID, applicationID string, next *applicationsdomain.ClientSecret, _, _ time.Time) error {
	key := fakeBootstrapClientSecretKey(organizationID, applicationID)
	for _, secret := range fakeBootstrapClientSecrets[key] {
		if secret.Status == applicationsdomain.ClientSecretStatusActive {
			secret.Status = applicationsdomain.ClientSecretStatusGrace
		}
	}
	next.OrganizationID = organizationID
	fakeBootstrapClientSecrets[key] = append(fakeBootstrapClientSecrets[key], next)
	return nil
}

func (r *fakeApplicationRepository) RetireClientSecrets(_ context.Context, organizationID, applicationID string, _ time.Time) error {
	for _, secret := range fakeBootstrapClientSecrets[fakeBootstrapClientSecretKey(organizationID, applicationID)] {
		secret.Status = applicationsdomain.ClientSecretStatusRetired
	}
	return nil
}

// fakeBootstrapClientJWKs / fakeBootstrapClientCertificates follow the same
// keyed-map pattern as fakeBootstrapClientSecrets: empty by default, so tests
// that do not register keys keep the previous "no active keys" behaviour.
var (
	fakeBootstrapClientJWKs         = map[string][]*applicationsdomain.ClientJWK{}
	fakeBootstrapClientCertificates = map[string][]*applicationsdomain.ClientCertificate{}
)

func (r *fakeApplicationRepository) ListActiveClientJWKs(_ context.Context, organizationID, applicationID string) ([]*applicationsdomain.ClientJWK, error) {
	return fakeBootstrapClientJWKs[fakeBootstrapClientSecretKey(organizationID, applicationID)], nil
}

func (r *fakeApplicationRepository) ReplaceClientJWKs(_ context.Context, _, _ string, _ []*applicationsdomain.ClientJWK, _ time.Time) error {
	return nil
}

func (r *fakeApplicationRepository) ListActiveClientCertificates(_ context.Context, organizationID, applicationID string) ([]*applicationsdomain.ClientCertificate, error) {
	return fakeBootstrapClientCertificates[fakeBootstrapClientSecretKey(organizationID, applicationID)], nil
}

func (r *fakeApplicationRepository) ReplaceClientCertificates(_ context.Context, _, _ string, _ []*applicationsdomain.ClientCertificate, _ time.Time) error {
	return nil
}
