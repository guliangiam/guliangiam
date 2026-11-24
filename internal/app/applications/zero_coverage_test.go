package applications

import (
	"context"
	"errors"
	"testing"
	"time"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	consentdomain "github.com/guliangiam/guliangiam/internal/domain/consents"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
)

// ── Tokens mock for GetApplicationDeleteSummary ──────────────────────────────
type mockTokenRepo struct {
	tokens  []*tokendomain.Token
	listErr error
}

func (m *mockTokenRepo) GetByID(ctx context.Context, id string) (*tokendomain.Token, error) {
	return nil, nil
}

func (m *mockTokenRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*tokendomain.Token, error) {
	return nil, nil
}
func (m *mockTokenRepo) GetByTokenHash(ctx context.Context, h string) (*tokendomain.Token, error) {
	return nil, nil
}
func (m *mockTokenRepo) GetByDeviceSecretHash(ctx context.Context, h string) (*tokendomain.Token, error) {
	return nil, nil
}
func (m *mockTokenRepo) GetRefreshByTokenHash(ctx context.Context, h string) (*tokendomain.RefreshToken, error) {
	return nil, nil
}
func (m *mockTokenRepo) List(ctx context.Context, q tokendomain.ListQuery) ([]*tokendomain.Token, int64, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.tokens, int64(len(m.tokens)), nil
}

func (m *mockTokenRepo) Create(ctx context.Context, t *tokendomain.Token) error { return nil }
func (m *mockTokenRepo) UpdateDeviceSecret(ctx context.Context, tokenID, deviceSecretHash string, expiresIn int) error {
	return nil
}
func (m *mockTokenRepo) CreateRefresh(ctx context.Context, r *tokendomain.RefreshToken) error {
	return nil
}
func (m *mockTokenRepo) Revoke(ctx context.Context, id string, revokedAt time.Time) error { return nil }
func (m *mockTokenRepo) RevokeRefresh(ctx context.Context, id string, revokedAt time.Time) error {
	return nil
}
func (m *mockTokenRepo) RevokeByQuery(ctx context.Context, q tokendomain.RevokeQuery, revokedAt time.Time) (int64, error) {
	return 0, nil
}
func (m *mockTokenRepo) RevokeByFamilyID(ctx context.Context, familyID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}
func (m *mockTokenRepo) RevokeRefreshByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}
func (m *mockTokenRepo) RevokeByAuthorizationCodeID(ctx context.Context, codeID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}
func (m *mockTokenRepo) Delete(ctx context.Context, id string) error { return nil }

// Consents mock
type mockConsentRepo struct {
	count    int64
	countErr error
}

func (m *mockConsentRepo) GetByUserAndApplication(ctx context.Context, u, a string) (*consentdomain.Consent, error) {
	return nil, nil
}
func (m *mockConsentRepo) CountActiveByApplication(ctx context.Context, appID string) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.count, nil
}
func (m *mockConsentRepo) Upsert(ctx context.Context, c *consentdomain.Consent) error { return nil }
func (m *mockConsentRepo) Revoke(ctx context.Context, u, a string) error              { return nil }

// Extended UoW: adds Tokens + Consents
type appDeleteTestUOW struct {
	testUnitOfWork
	tokenRepo   *mockTokenRepo
	consentRepo *mockConsentRepo
}

func (u appDeleteTestUOW) Tokens() tokendomain.Repository     { return u.tokenRepo }
func (u appDeleteTestUOW) Consents() consentdomain.Repository { return u.consentRepo }

func newAppWithMocks(appRepo *mockApplicationRepo, tokenRepo *mockTokenRepo, consentRepo *mockConsentRepo) Service {
	return Service{
		Clock: fakeClock{fixedTime: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)},
		UOW: appDeleteTestUOW{
			testUnitOfWork: testUnitOfWork{appRepo: appRepo},
			tokenRepo:      tokenRepo,
			consentRepo:    consentRepo,
		},
	}
}

// ── GetDefaultApplication ────────────────────────────────────────────────────

func TestGetDefaultApplication_Validation(t *testing.T) {
	svc := newAppWithMocks(&mockApplicationRepo{}, nil, nil)
	_, err := svc.GetDefaultApplication(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestGetDefaultApplication_NotFound(t *testing.T) {
	svc := newAppWithMocks(&mockApplicationRepo{}, nil, nil)
	_, err := svc.GetDefaultApplication(context.Background(), "org1")
	if err == nil {
		t.Fatal("expected not found")
	}
}

func TestGetDefaultApplication_Success(t *testing.T) {
	app := applicationStub("app1")
	app.DisplayName = "Default App"
	app.RedirectURIs = []string{"https://example.com/callback"}
	appRepo := &mockApplicationRepo{apps: []*applicationdomain.Application{app}}
	svc := newAppWithMocks(appRepo, nil, nil)

	detail, err := svc.GetDefaultApplication(context.Background(), "org1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.ID != "app1" || detail.DisplayName != "Default App" {
		t.Fatalf("wrong detail: %+v", detail)
	}
	if len(detail.RedirectURIs) != 1 {
		t.Fatalf("redirect URIs missing: %v", detail.RedirectURIs)
	}
}

// ── GetApplicationDeleteSummary ───────────────────────────────────────────────

func TestGetDeleteSummary_Success(t *testing.T) {
	app := applicationStub("app-del")
	app.Slug = "myapp"
	app.DisplayName = "My App"
	app.ProviderBindings = []applicationdomain.ApplicationProviderBinding{{ProviderID: "p1"}}

	tokenRepo := &mockTokenRepo{tokens: make([]*tokendomain.Token, 3)} // 3 active tokens
	consentRepo := &mockConsentRepo{count: 5}
	appRepo := &mockApplicationRepo{apps: []*applicationdomain.Application{app}}
	svc := newAppWithMocks(appRepo, tokenRepo, consentRepo)

	summary, err := svc.GetApplicationDeleteSummary(context.Background(), "org1", "app-del")
	if err != nil {
		t.Fatal(err)
	}
	if summary.ApplicationID != "app-del" || summary.Slug != "myapp" {
		t.Fatalf("wrong summary: %+v", summary)
	}
	if summary.ProviderBindingCount != 1 {
		t.Fatalf("bindings: %d", summary.ProviderBindingCount)
	}
	if summary.ActiveTokenCount != 3 {
		t.Fatalf("tokens: %d", summary.ActiveTokenCount)
	}
	if summary.ActiveConsentCount != 5 {
		t.Fatalf("consents: %d", summary.ActiveConsentCount)
	}
}

func TestGetDeleteSummary_AppNotFound(t *testing.T) {
	svc := newAppWithMocks(&mockApplicationRepo{}, &mockTokenRepo{}, &mockConsentRepo{})
	_, err := svc.GetApplicationDeleteSummary(context.Background(), "org1", "nonexistent")
	if err == nil {
		t.Fatal("expected not found")
	}
}

func TestGetDeleteSummary_TokenListError(t *testing.T) {
	app := applicationStub("app2")
	tokenRepo := &mockTokenRepo{listErr: errors.New("token db down")}
	appRepo := &mockApplicationRepo{apps: []*applicationdomain.Application{app}}
	svc := newAppWithMocks(appRepo, tokenRepo, &mockConsentRepo{})

	_, err := svc.GetApplicationDeleteSummary(context.Background(), "org1", "app2")
	if err == nil {
		t.Fatal("expected token list error")
	}
}

func TestGetDeleteSummary_ConsentCountError(t *testing.T) {
	app := applicationStub("app3")
	consentRepo := &mockConsentRepo{countErr: errors.New("consent db down")}
	appRepo := &mockApplicationRepo{apps: []*applicationdomain.Application{app}}
	svc := newAppWithMocks(appRepo, &mockTokenRepo{}, consentRepo)

	_, err := svc.GetApplicationDeleteSummary(context.Background(), "org1", "app3")
	if err == nil {
		t.Fatal("expected consent count error")
	}
}

// ── ExportApplication ─────────────────────────────────────────────────────────

func TestExportApplication_Success(t *testing.T) {
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	app := applicationStub("exp-app")
	app.Slug = "exported-app"
	app.DisplayName = "Exported App"
	app.ClientID = "c123"
	app.RedirectURIs = []string{"https://exp.example.com/cb"}
	appRepo := &mockApplicationRepo{apps: []*applicationdomain.Application{app}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{appRepo: appRepo}}

	payload, err := svc.ExportApplication(context.Background(), "org1", "exp-app")
	if err != nil {
		t.Fatal(err)
	}
	if payload.Version != ExportVersion {
		t.Fatalf("version=%s", payload.Version)
	}
	if !payload.ExportedAt.Equal(now) {
		t.Fatalf("exportedAt=%v want %v", payload.ExportedAt, now)
	}
	if payload.Export.Slug != "exported-app" || payload.Export.ClientID != "c123" {
		t.Fatalf("wrong export: %+v", payload.Export)
	}
}

func TestExportApplication_NotFound(t *testing.T) {
	svc := Service{Clock: fakeClock{}, UOW: testUnitOfWork{appRepo: &mockApplicationRepo{}}}
	_, err := svc.ExportApplication(context.Background(), "org1", "ghost")
	if err == nil {
		t.Fatal("expected not found")
	}
}

// ── ImportApplication ─────────────────────────────────────────────────────────

func TestImportApplication_Success(t *testing.T) {
	appRepo := &mockApplicationRepo{}
	svc := Service{Clock: fakeClock{}, UUID: &fakeUUIDGenerator{fixedID: "new-uuid"}, UOW: testUnitOfWork{appRepo: appRepo}}

	payload := ImportPayload{
		OrganizationID: "org1",
		Export: ApplicationExportDTO{
			Slug:         "imported-app",
			DisplayName:  "Imported App",
			Category:     "oauth",
			RedirectURIs: []string{"https://imp.example.com/cb"},
			GrantTypes:   []string{grantTypeAuthorizationCode},
		},
	}
	app, err := svc.ImportApplication(context.Background(), payload)
	if err != nil {
		t.Fatal(err)
	}
	if app.OrganizationID != "org1" || app.Slug != "imported-app" {
		t.Fatalf("wrong app: %+v", app)
	}
	if len(appRepo.apps) != 1 {
		t.Fatal("app not created in repo")
	}
}

func TestImportApplication_OrgRequired(t *testing.T) {
	svc := Service{Clock: fakeClock{}, UUID: &fakeUUIDGenerator{}, UOW: testUnitOfWork{appRepo: &mockApplicationRepo{}}}
	_, err := svc.ImportApplication(context.Background(), ImportPayload{Export: ApplicationExportDTO{Slug: "s"}})
	if err == nil {
		t.Fatal("expected org validation error")
	}
}

func TestImportApplication_SlugRequired(t *testing.T) {
	svc := Service{Clock: fakeClock{}, UUID: &fakeUUIDGenerator{}, UOW: testUnitOfWork{appRepo: &mockApplicationRepo{}}}
	_, err := svc.ImportApplication(context.Background(), ImportPayload{OrganizationID: "o", Export: ApplicationExportDTO{}})
	if err == nil {
		t.Fatal("expected slug validation error")
	}
}

func TestImportApplication_SlugConflict(t *testing.T) {
	existing := applicationStub("old")
	existing.Slug = "taken-slug"
	appRepo := &mockApplicationRepo{apps: []*applicationdomain.Application{existing}}
	svc := Service{Clock: fakeClock{}, UUID: &fakeUUIDGenerator{}, UOW: testUnitOfWork{appRepo: appRepo}}

	_, err := svc.ImportApplication(context.Background(), ImportPayload{
		OrganizationID: "org1",
		Export:         ApplicationExportDTO{Slug: "taken-slug", DisplayName: "Duplicate", GrantTypes: []string{grantTypeAuthorizationCode}},
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

// ── 工具函数 ────────────────────────────────────────────────────────────────
func applicationStub(id string) *applicationdomain.Application {
	return &applicationdomain.Application{
		ID:             id,
		OrganizationID: "org1",
	}
}
