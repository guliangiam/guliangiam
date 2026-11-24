package organizations

import (
	"context"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/app/uow/mock"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	orgmock "github.com/guliangiam/guliangiam/internal/domain/organizations/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func setupOrgTest(t *testing.T) (*gomock.Controller, *mock.MockUnitOfWork, *orgmock.MockRepository, Service) {
	ctrl := gomock.NewController(t)
	uowMock := mock.NewMockUnitOfWork(ctrl)
	orgRepo := orgmock.NewMockRepository(ctrl)
	svc := Service{
		Clock: clock.SystemClock{},
		UUID:  platformuuid.V7Generator{},
		UOW:   uowMock,
	}
	return ctrl, uowMock, orgRepo, svc
}

func TestGetOrganizationNames_Success(t *testing.T) {
	ctrl, uowMock, orgRepo, svc := setupOrgTest(t)
	defer ctrl.Finish()

	orgs := []*orgdomain.Organization{
		{ID: "org-1", DisplayName: "Org One", Slug: "org-one"},
		{ID: "org-2", DisplayName: "Org Two", Slug: "org-two"},
	}
	uowMock.EXPECT().Organizations().Return(orgRepo)
	orgRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(orgs, int64(2), nil)

	result, err := svc.GetOrganizationNames(context.Background())
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "org-1", result[0].ID)
	assert.Equal(t, "Org One", result[0].DisplayName)
	assert.Equal(t, "org-one", result[0].Slug)
	assert.Equal(t, "org-2", result[1].ID)
}

func TestGetOrganizationNames_Empty(t *testing.T) {
	ctrl, uowMock, orgRepo, svc := setupOrgTest(t)
	defer ctrl.Finish()

	uowMock.EXPECT().Organizations().Return(orgRepo)
	orgRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*orgdomain.Organization{}, int64(0), nil)

	result, err := svc.GetOrganizationNames(context.Background())
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetOrganizationNames_Error(t *testing.T) {
	ctrl, uowMock, orgRepo, svc := setupOrgTest(t)
	defer ctrl.Finish()

	uowMock.EXPECT().Organizations().Return(orgRepo)
	orgRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), assert.AnError)

	_, err := svc.GetOrganizationNames(context.Background())
	assert.Error(t, err)
}

func TestUpdateOrganizationSecurityPolicy_Success(t *testing.T) {
	ctrl, uowMock, orgRepo, svc := setupOrgTest(t)
	defer ctrl.Finish()

	org := &orgdomain.Organization{
		ID:       "org-1",
		Revision: 1,
		Security: orgdomain.OrganizationSecurityPolicy{HasMasterKey: true},
	}

	uowMock.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			txMock := mock.NewMockUnitOfWork(ctrl)
			txMock.EXPECT().Organizations().Return(orgRepo).Times(2)
			orgRepo.EXPECT().GetByID(gomock.Any(), "org-1").Return(org, nil)
			orgRepo.EXPECT().UpdateSecurityPolicy(gomock.Any(), gomock.Any()).Return(nil)
			return fn(ctx, txMock)
		},
	)

	err := svc.UpdateOrganizationSecurityPolicy(context.Background(), UpdateOrganizationSecurityPolicyCommand{
		OrganizationID: "org-1",
		PasswordPolicy: map[string]any{"minLength": 8},
		MfaPolicy:      map[string]any{"enabled": true},
	})
	require.NoError(t, err)
}

func TestUpdateOrganizationSecurityPolicy_ValidationError(t *testing.T) {
	ctrl, _, _, svc := setupOrgTest(t)
	defer ctrl.Finish()

	err := svc.UpdateOrganizationSecurityPolicy(context.Background(), UpdateOrganizationSecurityPolicyCommand{
		OrganizationID: "org-1",
		PasswordPolicy: map[string]any{"minLength": -1}, // negative value is invalid
	})
	assert.Error(t, err)
}

func TestUpdateOrganizationSettings_Success(t *testing.T) {
	ctrl, uowMock, orgRepo, svc := setupOrgTest(t)
	defer ctrl.Finish()

	org := &orgdomain.Organization{
		ID:       "org-1",
		Revision: 1,
	}

	uowMock.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			txMock := mock.NewMockUnitOfWork(ctrl)
			txMock.EXPECT().Organizations().Return(orgRepo).Times(2)
			orgRepo.EXPECT().GetByID(gomock.Any(), "org-1").Return(org, nil)
			orgRepo.EXPECT().UpdateSettings(gomock.Any(), gomock.Any()).Return(nil)
			return fn(ctx, txMock)
		},
	)

	accountMenu := "custom"
	mfaRemember := 24
	dcrPolicy := "auto"
	err := svc.UpdateOrganizationSettings(context.Background(), UpdateOrganizationSettingsCommand{
		OrganizationID: "org-1",
		Languages:      []string{"en", "zh-CN"},
		AccountItems: []AccountItemDTO{
			{Name: "email", Visible: true},
		},
		MfaItems: []MfaItemDTO{
			{Name: "totp", Rule: "required"},
		},
		AccountMenu:        &accountMenu,
		MfaRememberInHours: &mfaRemember,
		DcrPolicy:          &dcrPolicy,
	})
	require.NoError(t, err)
}

func TestUpdateOrganizationSettings_DuplicateLanguage(t *testing.T) {
	ctrl, _, _, svc := setupOrgTest(t)
	defer ctrl.Finish()

	err := svc.UpdateOrganizationSettings(context.Background(), UpdateOrganizationSettingsCommand{
		OrganizationID: "org-1",
		Languages:      []string{"en", "en"},
	})
	assert.Error(t, err)
}

func TestUpdateOrganizationTheme_Success(t *testing.T) {
	ctrl, uowMock, orgRepo, svc := setupOrgTest(t)
	defer ctrl.Finish()

	org := &orgdomain.Organization{
		ID:       "org-1",
		Revision: 1,
	}

	uowMock.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			txMock := mock.NewMockUnitOfWork(ctrl)
			txMock.EXPECT().Organizations().Return(orgRepo).Times(2)
			orgRepo.EXPECT().GetByID(gomock.Any(), "org-1").Return(org, nil)
			orgRepo.EXPECT().UpdateTheme(gomock.Any(), gomock.Any()).Return(nil)
			return fn(ctx, txMock)
		},
	)

	err := svc.UpdateOrganizationTheme(context.Background(), UpdateOrganizationThemeCommand{
		OrganizationID: "org-1",
		ThemeName:      "dark",
		Tokens:         map[string]any{"primary": "#000"},
		CustomCSS:      "body { background: #000; }",
	})
	require.NoError(t, err)
}

func TestUpdateOrganizationTheme_ValidationError(t *testing.T) {
	ctrl, _, _, svc := setupOrgTest(t)
	defer ctrl.Finish()

	err := svc.UpdateOrganizationTheme(context.Background(), UpdateOrganizationThemeCommand{
		OrganizationID: "",
	})
	assert.Error(t, err)
}

func TestUpdateOrganizationSecurityPolicy_CallsSubtable(t *testing.T) {
	ctrl, uowMock, orgRepo, svc := setupOrgTest(t)
	defer ctrl.Finish()

	org := &orgdomain.Organization{
		ID:       "org-1",
		Revision: 1,
		Security: orgdomain.OrganizationSecurityPolicy{HasMasterKey: true},
	}

	uowMock.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			txMock := mock.NewMockUnitOfWork(ctrl)
			txMock.EXPECT().Organizations().Return(orgRepo).Times(2)
			orgRepo.EXPECT().GetByID(gomock.Any(), "org-1").Return(org, nil)
			// ⚠️ 子表端点不再调 repo.Update 主表（避免与并行子表端点 CAS 竞态）。
			// 主表 revision 只由 PATCH /organizations/:id 单点管理。
			orgRepo.EXPECT().UpdateSecurityPolicy(gomock.Any(), gomock.Any()).DoAndReturn(
				func(ctx context.Context, p *orgdomain.OrganizationSecurityPolicy) error {
					assert.Equal(t, "org-1", p.OrganizationID)
					assert.True(t, p.HasMasterKey, "GetByID 拿到的 HasMasterKey 应保留")
					return nil
				},
			)
			return fn(ctx, txMock)
		},
	)

	err := svc.UpdateOrganizationSecurityPolicy(context.Background(), UpdateOrganizationSecurityPolicyCommand{
		OrganizationID: "org-1",
		PasswordPolicy: map[string]any{"minLength": 8},
	})
	require.NoError(t, err)
}
