package providers

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
)

// Reproduces the reported runtime state: a non-legacy identity IDVerification
// provider (registry metadata present) whose AccessKeySecret (password) must be
// persisted by a PATCH that sends `password`, exactly as the console does.
func TestUnifiedPatch_IdentityIDVPasswordPersists(t *testing.T) {
	now := time.Now()
	repo := &mockProviderRepo{
		providers: []*domain.Provider{
			{
				ID: "prov-idv", OrganizationID: "org-1", Name: "guliang-aliyun-idv",
				DisplayName: "GuliangTech Aliyun IDVerification",
				Family:      domain.FamilyIdentity, Kind: domain.Kind("aliyun_idv"),
				Category: domain.CategoryIDVerification, ProviderType: domain.ProviderType("AliyunIdv"),
				Admin: "ak-id", Password: "", Enabled: true, Revision: 1,
				CreatedAt: now, UpdatedAt: now,
			},
		},
		configs: []*domain.ProviderConfig{
			{
				ProviderID: "prov-idv", ConfigType: registryConfigType,
				Config: map[string]any{
					"family": "identity", "kind": "aliyun_idv", "status": "active",
					"variant": "", "detailId": "prov-idv",
				},
			},
			{
				ProviderID: "prov-idv", ConfigType: "default",
				Config: map[string]any{"idvSceneId": "1000022505"},
			},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{providerRepo: repo}}

	secret := "real-access-key-secret"
	admin := "ak-id"
	err := svc.PatchProvider(context.Background(), PatchProviderCommand{
		ID:             "prov-idv",
		OrganizationID: "org-1",
		Admin:          &admin,
		Password:       &secret,
		Revision:       1,
	})
	if err != nil {
		t.Fatalf("PatchProvider() error = %v", err)
	}
	if got := repo.providers[0].Password; got != secret {
		t.Fatalf("persisted Password = %q, want %q (AccessKeySecret must reach the native row)", got, secret)
	}
	if got := repo.providers[0].Admin; got != admin {
		t.Fatalf("persisted Admin = %q, want %q", got, admin)
	}
}
