package tokens

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	domain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// familySeqUUID 使每次签发拿到不同 ID，从而能区分「FamilyID=自身ID」与
// 「FamilyID=另生成的随机 UUID」（固定 ID 替身会把两者混为一谈）。
type familySeqUUID struct{ n int }

func (g *familySeqUUID) NewString() (string, error) {
	g.n++
	return fmt.Sprintf("id-%04d", g.n), nil
}

func familyTestService(now time.Time, repo *mockTokenRepo) Service {
	return Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &familySeqUUID{},
		UOW: testUnitOfWork{
			tokenRepo:   repo,
			recordsRepo: &mockRecordsRepo{},
			outboxRepo:  &mockOutboxRepo{},
			orgRepo: &mockOrgRepo{orgs: []*orgdomain.Organization{
				{ID: "org-1", Slug: "org-1"},
			}},
			userRepo: &mockUserRepo{users: []*userdomain.User{
				{ID: "user-1", OrganizationID: "org-1", Username: "jane"},
			}},
			appRepo: &mockAppRepo{apps: []*applications.Application{
				{ID: "app-1", OrganizationID: "org-1", Slug: "app-1"},
			}},
		},
	}
}

// BE-D-3：管理端签发路径首代 refresh token 必须自引用家族根。
func TestCreateToken_RefreshFamilyIsSelfRoot(t *testing.T) {
	now := time.Now()
	repo := &mockTokenRepo{}
	svc := familyTestService(now, repo)

	issued, err := svc.CreateToken(context.Background(), CreateTokenCommand{
		OrganizationID:    "org-1",
		UserID:            "user-1",
		ApplicationID:     "app-1",
		Scopes:            []string{"read"},
		ExpiresInSeconds:  3600,
		IssueRefreshToken: true,
	})
	require.NoError(t, err)
	require.NotEmpty(t, issued.RefreshToken)
	require.Len(t, repo.refreshTokens, 1)
	rt := repo.refreshTokens[0]
	assert.NotEmpty(t, rt.FamilyID, "family_id must not be NULL")
	assert.Equal(t, rt.ID, rt.FamilyID, "first generation must be its own family root")
	assert.NotEqual(t, rt.AccessTokenID, rt.FamilyID)
}

func refreshFixture(now time.Time, familyID string, revokedAt *time.Time) (*mockTokenRepo, *domain.RefreshToken) {
	access := &domain.Token{
		ID: "access-1", UserID: "user-1", OrganizationID: "org-1", ApplicationID: "app-1",
		Scopes: []string{"read"}, ExpiresAt: now.Add(time.Hour), CreatedAt: now.Add(-time.Minute),
	}
	rt := &domain.RefreshToken{
		ID: "refresh-1", AccessTokenID: access.ID, UserID: "user-1",
		OrganizationID: "org-1", ApplicationID: "app-1",
		RefreshTokenHash: hashTokenValue("gl_prt_seed"), FamilyID: familyID,
		Scopes: []string{"read"}, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now.Add(-time.Minute),
		RevokedAt: revokedAt,
	}
	return &mockTokenRepo{tokens: []*domain.Token{access}, refreshTokens: []*domain.RefreshToken{rt}}, rt
}

// 轮换保持家族不变。
func TestRefreshAccessToken_RotationPreservesFamily(t *testing.T) {
	now := time.Now()
	repo, _ := refreshFixture(now, "family-root", nil)
	svc := familyTestService(now, repo)

	issued, err := svc.RefreshAccessToken(context.Background(), RefreshAccessTokenCommand{
		OrganizationID: "org-1", RefreshToken: "gl_prt_seed",
	})
	require.NoError(t, err)
	require.NotEmpty(t, issued.RefreshToken)
	require.Len(t, repo.refreshTokens, 2)
	assert.Equal(t, "family-root", repo.refreshTokens[1].FamilyID)
}

// BE-D-3：历史 NULL 家族行轮换时以自身为家族根，后代不再落 NULL。
func TestRefreshAccessToken_LegacyNullFamilyAdoptsSelfRoot(t *testing.T) {
	now := time.Now()
	repo, rt := refreshFixture(now, "", nil)
	svc := familyTestService(now, repo)

	_, err := svc.RefreshAccessToken(context.Background(), RefreshAccessTokenCommand{
		OrganizationID: "org-1", RefreshToken: "gl_prt_seed",
	})
	require.NoError(t, err)
	require.Len(t, repo.refreshTokens, 2)
	assert.Equal(t, rt.ID, repo.refreshTokens[1].FamilyID,
		"legacy NULL-family row must rotate into a family rooted at itself")
}

// BE-D-3：reuse（超出宽限窗）必须触发家族吊销——包括历史 NULL 家族行。
func TestRefreshAccessToken_ReuseRevokesFamilyEvenForLegacyNullFamily(t *testing.T) {
	now := time.Now()
	revokedAt := now.Add(-time.Hour) // outside the 10s grace window
	repo, rt := refreshFixture(now, "", &revokedAt)
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &familySeqUUID{},
		UOW: testUnitOfWork{
			tokenRepo:   repo,
			recordsRepo: &mockRecordsRepo{},
			outboxRepo:  &mockOutboxRepo{},
		},
	}

	_, err := svc.RefreshAccessToken(context.Background(), RefreshAccessTokenCommand{
		OrganizationID: "org-1", RefreshToken: "gl_prt_seed",
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnauthorized, appErr.Code)
	assert.Equal(t, []string{rt.ID}, repo.revokedFamilies,
		"reuse of a legacy NULL-family token must still revoke its family")
}

// BE-D-19：bearer 秘密必须是 256-bit crypto/rand，URL-safe，且保持列宽兼容。
func TestNewTokenSecrets_HighEntropyURLSafe(t *testing.T) {
	svc := Service{}
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		value, prefix, hash, err := svc.newTokenSecret()
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(value, "gl_pat_"))
		assert.Len(t, value, len("gl_pat_")+43, "32 bytes base64url (no padding) = 43 chars")
		assert.Len(t, prefix, tokenPrefixMaxLength)
		assert.True(t, strings.HasPrefix(value, prefix))
		assert.Len(t, hash, 64)
		body, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, "gl_pat_"))
		require.NoError(t, err)
		assert.Len(t, body, 32)
		assert.False(t, seen[value], "secrets must not repeat")
		seen[value] = true

		refreshValue, refreshHash, err := svc.newRefreshTokenSecret()
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(refreshValue, "gl_prt_"))
		assert.Len(t, refreshValue, len("gl_prt_")+43)
		assert.Len(t, refreshHash, 64)
		assert.False(t, seen[refreshValue])
		seen[refreshValue] = true
	}
}
