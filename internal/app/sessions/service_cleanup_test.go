package sessions

import (
	"context"
	"testing"
	"time"

	crosstenantops "github.com/guliangiam/guliangiam/internal/domain/cross_tenant_operations"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	actionsdomain "github.com/guliangiam/guliangiam/internal/domain/actions"
	adapterdomain "github.com/guliangiam/guliangiam/internal/domain/adapters"
	agentdomain "github.com/guliangiam/guliangiam/internal/domain/agents"
	"github.com/guliangiam/guliangiam/internal/domain/applications"
	authorizationcodedomain "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
	captchaproviderdomain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	"github.com/guliangiam/guliangiam/internal/domain/castickets"
	certdomain "github.com/guliangiam/guliangiam/internal/domain/certs"
	consentdomain "github.com/guliangiam/guliangiam/internal/domain/consents"
	coupondomain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	"github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
	enforcerdomain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	entrydomain "github.com/guliangiam/guliangiam/internal/domain/entries"
	faceidproviderdomain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	"github.com/guliangiam/guliangiam/internal/domain/faceidsessions"
	formdomain "github.com/guliangiam/guliangiam/internal/domain/forms"
	gatewaydomain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	groupsdomain "github.com/guliangiam/guliangiam/internal/domain/groups"
	idvdomain "github.com/guliangiam/guliangiam/internal/domain/idv"
	igadomain "github.com/guliangiam/guliangiam/internal/domain/iga"
	invitationsdomain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	"github.com/guliangiam/guliangiam/internal/domain/kerberostickets"
	keysdomain "github.com/guliangiam/guliangiam/internal/domain/keys"
	ldapdomain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	mcpserverdomain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	meteringdomain "github.com/guliangiam/guliangiam/internal/domain/metering"
	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	modeldomain "github.com/guliangiam/guliangiam/internal/domain/models"
	notificationproviderdomain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	notificationsdomain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	orderdomain "github.com/guliangiam/guliangiam/internal/domain/orders"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	paymentdomain "github.com/guliangiam/guliangiam/internal/domain/payments"
	permissiondomain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	plandomain "github.com/guliangiam/guliangiam/internal/domain/plans"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	pricingdomain "github.com/guliangiam/guliangiam/internal/domain/pricings"
	productdomain "github.com/guliangiam/guliangiam/internal/domain/products"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/domain/qrcodesessions"
	radiusaccountingdomain "github.com/guliangiam/guliangiam/internal/domain/radiusaccountings"
	radiusconfigdomain "github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
	rebac "github.com/guliangiam/guliangiam/internal/domain/rebac"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	resourcedomain "github.com/guliangiam/guliangiam/internal/domain/resources"
	riskdomain "github.com/guliangiam/guliangiam/internal/domain/risk"
	roledomain "github.com/guliangiam/guliangiam/internal/domain/roles"
	ruledomain "github.com/guliangiam/guliangiam/internal/domain/rules"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	subscriptiondomain "github.com/guliangiam/guliangiam/internal/domain/subscriptions"
	syncerdomain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	ticketdomain "github.com/guliangiam/guliangiam/internal/domain/tickets"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	transactiondomain "github.com/guliangiam/guliangiam/internal/domain/transactions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	"github.com/guliangiam/guliangiam/internal/domain/web3identity"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	webhookeventdomain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	webhookdomain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	workloaddomain "github.com/guliangiam/guliangiam/internal/domain/workload"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type svcFakeClock struct {
	fixedTime time.Time
}

func (c svcFakeClock) Now() time.Time { return c.fixedTime }

// svcMockSessionRepo with configurable cleanup
type svcMockSessionRepo struct {
	sessions     []*sessiondomain.Session
	cleanupFn    func(ctx context.Context, cutoff time.Time) (int64, error)
	cleanupCount int64
}

func (r *svcMockSessionRepo) GetByID(ctx context.Context, id string) (*sessiondomain.Session, error) {
	for _, s := range r.sessions {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, apperrors.NewNotFound("session not found")
}

func (r *svcMockSessionRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*sessiondomain.Session, error) {
	for _, s := range r.sessions {
		if s.ID == id && s.OrganizationID == organizationID {
			return s, nil
		}
	}
	return nil, apperrors.NewNotFound("session not found")
}

func (r *svcMockSessionRepo) List(ctx context.Context, query sessiondomain.ListQuery) ([]*sessiondomain.Session, int64, error) {
	return r.sessions, int64(len(r.sessions)), nil
}

func (r *svcMockSessionRepo) Create(ctx context.Context, s *sessiondomain.Session) error {
	r.sessions = append(r.sessions, s)
	return nil
}

func (r *svcMockSessionRepo) Update(ctx context.Context, s *sessiondomain.Session) error { return nil }
func (r *svcMockSessionRepo) Touch(ctx context.Context, id string, t time.Time) error    { return nil }
func (r *svcMockSessionRepo) Revoke(ctx context.Context, id string, t time.Time) error   { return nil }
func (r *svcMockSessionRepo) RevokeForOrganization(ctx context.Context, id, organizationID string, t time.Time) error {
	return nil
}
func (r *svcMockSessionRepo) RevokeByUserExcept(ctx context.Context, uid, excl string, t time.Time) (int64, error) {
	return 0, nil
}
func (r *svcMockSessionRepo) RevokeByUserAndApp(ctx context.Context, uid, appID, excl string, t time.Time) (int64, error) {
	return 0, nil
}
func (r *svcMockSessionRepo) Delete(ctx context.Context, id string) error { return nil }
func (r *svcMockSessionRepo) DeleteForOrganization(ctx context.Context, id, organizationID string) error {
	return nil
}
func (r *svcMockSessionRepo) RevokeByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}
func (r *svcMockSessionRepo) RevokeByOrganization(ctx context.Context, organizationID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}

func (r *svcMockSessionRepo) CleanupExpired(ctx context.Context, cutoff time.Time) (int64, error) {
	if r.cleanupFn != nil {
		return r.cleanupFn(ctx, cutoff)
	}
	return r.cleanupCount, nil
}

// svcMockOrgRepo
type svcMockOrgRepo struct{}

func (r *svcMockOrgRepo) GetByID(ctx context.Context, id string) (*orgdomain.Organization, error) {
	return nil, apperrors.NewNotFound("not found")
}
func (r *svcMockOrgRepo) GetBySlug(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return nil, apperrors.NewNotFound("not found")
}

// GetBySlugPublic delegates to GetBySlug (tests don't need RLS bypass).
func (r *svcMockOrgRepo) GetBySlugPublic(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return r.GetBySlug(ctx, slug)
}

func (r *svcMockOrgRepo) List(ctx context.Context, query orgdomain.ListQuery) ([]*orgdomain.Organization, int64, error) {
	return nil, 0, nil
}
func (r *svcMockOrgRepo) Create(ctx context.Context, org *orgdomain.Organization) error { return nil }
func (r *svcMockOrgRepo) Update(ctx context.Context, org *orgdomain.Organization) error { return nil }
func (r *svcMockOrgRepo) UpdateSettings(ctx context.Context, s *orgdomain.OrganizationSettings) error {
	return nil
}
func (r *svcMockOrgRepo) UpdateTheme(ctx context.Context, s *orgdomain.OrganizationThemeSettings) error {
	return nil
}
func (r *svcMockOrgRepo) UpdateSecurityPolicy(ctx context.Context, s *orgdomain.OrganizationSecurityPolicy) error {
	return nil
}
func (r *svcMockOrgRepo) Delete(_ context.Context, _ string) error {
	return apperrors.NewUnimplemented("not implemented")
}
func (r *svcMockOrgRepo) HasDeletedWithSlug(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// svcTestUOW
type svcTestUOW struct {
	sessionRepo *svcMockSessionRepo
	orgRepo     *svcMockOrgRepo
}

func (u svcTestUOW) Applications() applications.Repository                        { return nil }
func (u svcTestUOW) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u svcTestUOW) Consents() consentdomain.Repository                           { return nil }
func (u svcTestUOW) Gateway() gatewaydomain.Repository                            { return nil }
func (u svcTestUOW) Groups() groupsdomain.Repository                              { return nil }
func (u svcTestUOW) Invitations() invitationsdomain.Repository                    { return nil }
func (u svcTestUOW) MfaChallenges() mfachallengedomain.Repository                 { return nil }
func (u svcTestUOW) Organizations() orgdomain.Repository                          { return u.orgRepo }
func (u svcTestUOW) Permissions() permissiondomain.Repository                     { return nil }
func (u svcTestUOW) Providers() providerdomain.Repository                         { return nil }
func (u svcTestUOW) Certs() certdomain.Repository                                 { return nil }
func (u svcTestUOW) CaptchaProviders() captchaproviderdomain.Repository           { return nil }
func (u svcTestUOW) NotificationProviders() notificationproviderdomain.Repository { return nil }
func (u svcTestUOW) Notifications() notificationsdomain.Repository                { return nil }
func (u svcTestUOW) Ldaps() ldapdomain.Repository                                 { return nil }
func (u svcTestUOW) Models() modeldomain.Repository                               { return nil }
func (u svcTestUOW) Adapters() adapterdomain.Repository                           { return nil }
func (u svcTestUOW) Agents() agentdomain.Repository                               { return nil }
func (u svcTestUOW) Enforcers() enforcerdomain.Repository                         { return nil }
func (u svcTestUOW) Forms() formdomain.Repository                                 { return nil }
func (u svcTestUOW) Orders() orderdomain.Repository                               { return nil }
func (u svcTestUOW) Payments() paymentdomain.Repository                           { return nil }
func (u svcTestUOW) Plans() plandomain.Repository                                 { return nil }

func (u svcTestUOW) PlatformAdmins() platformadmins.PlatformAdminRepository           { return nil }
func (u svcTestUOW) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository { return nil }
func (u svcTestUOW) CrossTenantOps() crosstenantops.TicketRepository                  { return nil }
func (u svcTestUOW) Pricings() pricingdomain.Repository                               { return nil }
func (u svcTestUOW) Products() productdomain.Repository                               { return nil }
func (u svcTestUOW) Records() recordsdomain.Repository                                { return nil }
func (u svcTestUOW) Resources() resourcedomain.Repository                             { return nil }
func (u svcTestUOW) Roles() roledomain.Repository                                     { return nil }
func (u svcTestUOW) Syncers() syncerdomain.Repository                                 { return nil }
func (u svcTestUOW) Sessions() sessiondomain.Repository                               { return u.sessionRepo }
func (u svcTestUOW) Subscriptions() subscriptiondomain.Repository                     { return nil }
func (u svcTestUOW) Tickets() ticketdomain.Repository                                 { return nil }
func (u svcTestUOW) Tokens() tokendomain.Repository                                   { return nil }
func (u svcTestUOW) Webhooks() webhookdomain.Repository                               { return nil }
func (u svcTestUOW) Users() userdomain.Repository                                     { return nil }
func (u svcTestUOW) Coupons() coupondomain.Repository                                 { return nil }
func (u svcTestUOW) CouponUsages() coupondomain.CouponUsageRepository                 { return nil }
func (u svcTestUOW) Entries() entrydomain.Repository                                  { return nil }
func (u svcTestUOW) McpServers() mcpserverdomain.Repository                           { return nil }
func (u svcTestUOW) RadiusAccountings() radiusaccountingdomain.Repository             { return nil }
func (u svcTestUOW) Verifications() verificationsdomain.Repository                    { return nil }
func (u svcTestUOW) WebAuthnCredentials() webauthndomain.Repository                   { return nil }
func (u svcTestUOW) FaceIdProviders() faceidproviderdomain.Repository                 { return nil }
func (u svcTestUOW) RadiusConfigs() radiusconfigdomain.Repository                     { return nil }
func (u svcTestUOW) Rules() ruledomain.Repository                                     { return nil }
func (u svcTestUOW) Transactions() transactiondomain.Repository                       { return nil }
func (u svcTestUOW) WebhookEvents() webhookeventdomain.Repository                     { return nil }
func (u svcTestUOW) Keys() keysdomain.Repository                                      { return nil }
func (u svcTestUOW) QRCodeSessions() qrcodesessions.Repository                        { return nil }
func (u svcTestUOW) DeviceAuthCodes() deviceauthcodes.Repository                      { return nil }

func (u svcTestUOW) Web3Identities() web3identity.Repository     { return nil }
func (u svcTestUOW) FaceIDSessions() faceidsessions.Repository   { return nil }
func (u svcTestUOW) CASTickets() castickets.Repository           { return nil }
func (u svcTestUOW) KerberosTickets() kerberostickets.Repository { return nil }
func (u svcTestUOW) LockoutAttempts() lockoutattempts.Repository { return nil }
func (u svcTestUOW) IDV() idvdomain.Repository                   { return nil }

func (u svcTestUOW) Workload() workloaddomain.Repository { return nil }

func (u svcTestUOW) Risk() riskdomain.Repository { return nil }

func (u svcTestUOW) Rebac() rebac.Repository { return nil }

func (u svcTestUOW) Actions() actionsdomain.Repository { return nil }

func (u svcTestUOW) Metering() meteringdomain.Repository { return nil }

func (u svcTestUOW) Outbox() domainoutbox.Repository { return nil }

func (u svcTestUOW) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}
func (u svcTestUOW) IGA() igadomain.Repository { return nil }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestService_CleanupExpiredSessions_Success(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	repo := &svcMockSessionRepo{cleanupCount: 5}
	orgRepo := &svcMockOrgRepo{}
	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UOW:   svcTestUOW{sessionRepo: repo, orgRepo: orgRepo},
	}

	deleted, err := svc.CleanupExpiredSessions(context.Background(), 7*24*time.Hour)
	if err != nil {
		t.Fatalf("CleanupExpiredSessions() error = %v", err)
	}
	if deleted != 5 {
		t.Errorf("deleted = %d, want 5", deleted)
	}
}

func TestService_CleanupExpiredSessions_NoExpired(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	repo := &svcMockSessionRepo{cleanupCount: 0}
	orgRepo := &svcMockOrgRepo{}
	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UOW:   svcTestUOW{sessionRepo: repo, orgRepo: orgRepo},
	}

	deleted, err := svc.CleanupExpiredSessions(context.Background(), 7*24*time.Hour)
	if err != nil {
		t.Fatalf("CleanupExpiredSessions() error = %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}
}

func TestService_CleanupExpiredSessions_RepositoryError(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	repo := &svcMockSessionRepo{
		cleanupFn: func(ctx context.Context, cutoff time.Time) (int64, error) {
			return 0, apperrors.NewInternal("database connection lost")
		},
	}
	orgRepo := &svcMockOrgRepo{}
	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UOW:   svcTestUOW{sessionRepo: repo, orgRepo: orgRepo},
	}

	_, err := svc.CleanupExpiredSessions(context.Background(), 7*24*time.Hour)
	if err == nil {
		t.Fatal("CleanupExpiredSessions() expected error, got nil")
	}
}

func TestService_CleanupExpiredSessions_ZeroRetention(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	var capturedCutoff time.Time
	repo := &svcMockSessionRepo{
		cleanupFn: func(ctx context.Context, cutoff time.Time) (int64, error) {
			capturedCutoff = cutoff
			return 10, nil
		},
	}
	orgRepo := &svcMockOrgRepo{}
	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UOW:   svcTestUOW{sessionRepo: repo, orgRepo: orgRepo},
	}

	deleted, err := svc.CleanupExpiredSessions(context.Background(), 0)
	if err != nil {
		t.Fatalf("CleanupExpiredSessions() error = %v", err)
	}
	if deleted != 10 {
		t.Errorf("deleted = %d, want 10", deleted)
	}
	// With zero retention, cutoff should equal now
	if !capturedCutoff.Equal(now) {
		t.Errorf("cutoff = %v, want %v", capturedCutoff, now)
	}
}

func TestService_ListSessions_Success(t *testing.T) {
	now := time.Now()
	repo := &svcMockSessionRepo{
		sessions: []*sessiondomain.Session{
			{ID: "sess-1", UserID: "user-1", Username: "alice", CreatedAt: now, UpdatedAt: now},
			{ID: "sess-2", UserID: "user-2", Username: "bob", CreatedAt: now, UpdatedAt: now},
		},
	}
	orgRepo := &svcMockOrgRepo{}
	svc := Service{UOW: svcTestUOW{sessionRepo: repo, orgRepo: orgRepo}}

	items, total, err := svc.ListSessions(context.Background(), ListSessionsQuery{UserID: "user-1", Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(items))
	}
}

func TestService_DeleteSession_Success(t *testing.T) {
	now := time.Now()
	repo := &mockSessionRepo{
		sessions: []*sessiondomain.Session{
			{ID: "sess-1", UserID: "user-1"},
		},
	}
	orgRepo := &mockOrgRepo{}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{sessionRepo: repo, orgRepo: orgRepo}}

	err := svc.DeleteSession(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
}

func TestService_DeleteSession_NotFound(t *testing.T) {
	repo := &mockSessionRepo{}
	orgRepo := &mockOrgRepo{}
	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UOW: testUnitOfWork{sessionRepo: repo, orgRepo: orgRepo}}

	err := svc.DeleteSession(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session, got nil")
	}
}
