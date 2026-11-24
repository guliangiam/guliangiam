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
	castickets "github.com/guliangiam/guliangiam/internal/domain/castickets"
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
	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
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
// Fake platform implementations
// ---------------------------------------------------------------------------

type fakeClock struct {
	fixedTime time.Time
}

func (c fakeClock) Now() time.Time { return c.fixedTime }

// ---------------------------------------------------------------------------
// Mock session repository
// ---------------------------------------------------------------------------

type mockSessionRepo struct {
	sessions  []*domain.Session
	revokeErr error
	getErr    error
}

func (r *mockSessionRepo) GetByID(ctx context.Context, id string) (*domain.Session, error) {
	for _, s := range r.sessions {
		if s.ID == id {
			return s, nil
		}
	}
	if r.getErr != nil {
		return nil, r.getErr
	}
	return nil, apperrors.NewNotFound("session not found")
}

func (r *mockSessionRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Session, error) {
	for _, s := range r.sessions {
		if s.ID == id && s.OrganizationID == organizationID {
			return s, nil
		}
	}
	return nil, apperrors.NewNotFound("session not found")
}

func (r *mockSessionRepo) List(ctx context.Context, query domain.ListQuery) ([]*domain.Session, int64, error) {
	var result []*domain.Session
	for _, s := range r.sessions {
		if query.UserID != "" && s.UserID != query.UserID {
			continue
		}
		result = append(result, s)
	}
	return result, int64(len(result)), nil
}

func (r *mockSessionRepo) Create(ctx context.Context, session *domain.Session) error {
	r.sessions = append(r.sessions, session)
	return nil
}

func (r *mockSessionRepo) Update(ctx context.Context, session *domain.Session) error {
	for i, s := range r.sessions {
		if s.ID == session.ID {
			r.sessions[i] = session
			return nil
		}
	}
	return apperrors.NewNotFound("session not found")
}

func (r *mockSessionRepo) Touch(ctx context.Context, id string, touchedAt time.Time) error {
	for _, s := range r.sessions {
		if s.ID == id {
			s.LastSeenAt = touchedAt
			return nil
		}
	}
	return apperrors.NewNotFound("session not found")
}

func (r *mockSessionRepo) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	if r.revokeErr != nil {
		return r.revokeErr
	}
	for _, s := range r.sessions {
		if s.ID == id {
			s.RevokedAt = &revokedAt
			return nil
		}
	}
	return apperrors.NewNotFound("session not found")
}

func (r *mockSessionRepo) RevokeForOrganization(ctx context.Context, id, organizationID string, revokedAt time.Time) error {
	if r.revokeErr != nil {
		return r.revokeErr
	}
	for _, s := range r.sessions {
		if s.ID == id && s.OrganizationID == organizationID {
			s.RevokedAt = &revokedAt
			return nil
		}
	}
	return apperrors.NewNotFound("session not found")
}

func (r *mockSessionRepo) RevokeByUserExcept(ctx context.Context, userID, excludeSessionID string, revokedAt time.Time) (int64, error) {
	var count int64
	for _, s := range r.sessions {
		if s.UserID == userID && s.ID != excludeSessionID {
			s.RevokedAt = &revokedAt
			count++
		}
	}
	return count, nil
}

func (r *mockSessionRepo) RevokeByUserAndApp(ctx context.Context, userID, applicationID, excludeSessionID string, revokedAt time.Time) (int64, error) {
	var count int64
	for _, s := range r.sessions {
		if s.UserID == userID && s.ApplicationID == applicationID && s.ID != excludeSessionID {
			s.RevokedAt = &revokedAt
			count++
		}
	}
	return count, nil
}

func (r *mockSessionRepo) Delete(ctx context.Context, id string) error {
	for i, s := range r.sessions {
		if s.ID == id {
			r.sessions = append(r.sessions[:i], r.sessions[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("session not found")
}

func (r *mockSessionRepo) DeleteForOrganization(ctx context.Context, id, organizationID string) error {
	for i, s := range r.sessions {
		if s.ID == id && s.OrganizationID == organizationID {
			r.sessions = append(r.sessions[:i], r.sessions[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("session not found")
}

func (r *mockSessionRepo) CleanupExpired(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (r *mockSessionRepo) RevokeByOrganization(ctx context.Context, organizationID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}
func (r *mockSessionRepo) RevokeByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}

// ---------------------------------------------------------------------------
// Mock organization repository (for session policy resolution)
// ---------------------------------------------------------------------------

type mockOrgRepo struct {
	orgs []*orgdomain.Organization
}

func (r *mockOrgRepo) GetByID(ctx context.Context, id string) (*orgdomain.Organization, error) {
	for _, o := range r.orgs {
		if o.ID == id {
			return o, nil
		}
	}
	return nil, apperrors.NewNotFound("organization not found")
}

func (r *mockOrgRepo) GetBySlug(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return nil, apperrors.NewNotFound("not found")
}

// GetBySlugPublic delegates to GetBySlug (tests don't need RLS bypass).
func (r *mockOrgRepo) GetBySlugPublic(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return r.GetBySlug(ctx, slug)
}

func (r *mockOrgRepo) List(ctx context.Context, query orgdomain.ListQuery) ([]*orgdomain.Organization, int64, error) {
	return nil, 0, nil
}

func (r *mockOrgRepo) Create(ctx context.Context, org *orgdomain.Organization) error { return nil }

func (r *mockOrgRepo) Update(ctx context.Context, org *orgdomain.Organization) error { return nil }

func (r *mockOrgRepo) UpdateSettings(ctx context.Context, settings *orgdomain.OrganizationSettings) error {
	return nil
}

func (r *mockOrgRepo) UpdateTheme(ctx context.Context, theme *orgdomain.OrganizationThemeSettings) error {
	return nil
}

func (r *mockOrgRepo) UpdateSecurityPolicy(ctx context.Context, policy *orgdomain.OrganizationSecurityPolicy) error {
	return nil
}

func (r *mockOrgRepo) Delete(_ context.Context, _ string) error {
	return apperrors.NewUnimplemented("not implemented")
}

func (r *mockOrgRepo) HasDeletedWithSlug(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// ---------------------------------------------------------------------------
// Mock audit outbox repositories (用于验证 embedAuditEvent 被调用)
// ---------------------------------------------------------------------------

type mockRecordsRepo struct {
	records     []*recordsdomain.Record
	createCalls int
}

func (r *mockRecordsRepo) Create(_ context.Context, record *recordsdomain.Record) error {
	r.records = append(r.records, record)
	r.createCalls++
	return nil
}
func (r *mockRecordsRepo) List(context.Context, recordsdomain.ListQuery) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}
func (r *mockRecordsRepo) Search(context.Context, recordsdomain.SearchCriteria) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}
func (r *mockRecordsRepo) LoadLastChainEntry(context.Context, string) (seq int64, hash []byte, err error) {
	return 0, nil, nil
}
func (r *mockRecordsRepo) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}
func (r *mockRecordsRepo) InsertAnchor(context.Context, *recordsdomain.ChainAnchor) error {
	return nil
}
func (r *mockRecordsRepo) ListChainEntries(context.Context, string) ([]recordsdomain.ChainEntry, error) {
	return nil, nil
}

type mockOutboxRepo struct {
	events       []*domainoutbox.OutboxEvent
	enqueueCalls int
}

func (r *mockOutboxRepo) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	r.events = append(r.events, event)
	r.enqueueCalls++
	return nil
}
func (r *mockOutboxRepo) EnqueueBatch(context.Context, []*domainoutbox.OutboxEvent) error { return nil }

// ---------------------------------------------------------------------------
// Test UOW
// ---------------------------------------------------------------------------

type testUnitOfWork struct {
	sessionRepo *mockSessionRepo
	orgRepo     *mockOrgRepo
	recordsRepo *mockRecordsRepo // nil = embedAuditEvent 跳过写入
	outboxRepo  *mockOutboxRepo  // nil = embedAuditEvent 跳过写入
}

func (u testUnitOfWork) Applications() applications.Repository                        { return nil }
func (u testUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u testUnitOfWork) Consents() consentdomain.Repository                           { return nil }
func (u testUnitOfWork) Gateway() gatewaydomain.Repository                            { return nil }
func (u testUnitOfWork) Groups() groupsdomain.Repository                              { return nil }
func (u testUnitOfWork) Invitations() invitationsdomain.Repository                    { return nil }
func (u testUnitOfWork) MfaChallenges() mfachallengedomain.Repository                 { return nil }
func (u testUnitOfWork) Organizations() orgdomain.Repository                          { return u.orgRepo }
func (u testUnitOfWork) Permissions() permissiondomain.Repository                     { return nil }
func (u testUnitOfWork) Providers() providerdomain.Repository                         { return nil }
func (u testUnitOfWork) Certs() certdomain.Repository                                 { return nil }
func (u testUnitOfWork) CaptchaProviders() captchaproviderdomain.Repository           { return nil }
func (u testUnitOfWork) NotificationProviders() notificationproviderdomain.Repository { return nil }
func (u testUnitOfWork) Notifications() notificationsdomain.Repository                { return nil }
func (u testUnitOfWork) Ldaps() ldapdomain.Repository                                 { return nil }
func (u testUnitOfWork) Models() modeldomain.Repository                               { return nil }
func (u testUnitOfWork) Adapters() adapterdomain.Repository                           { return nil }
func (u testUnitOfWork) Agents() agentdomain.Repository                               { return nil }
func (u testUnitOfWork) Enforcers() enforcerdomain.Repository                         { return nil }
func (u testUnitOfWork) Forms() formdomain.Repository                                 { return nil }
func (u testUnitOfWork) Orders() orderdomain.Repository                               { return nil }
func (u testUnitOfWork) Payments() paymentdomain.Repository                           { return nil }
func (u testUnitOfWork) Plans() plandomain.Repository                                 { return nil }

func (u testUnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository           { return nil }
func (u testUnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository { return nil }
func (u testUnitOfWork) CrossTenantOps() crosstenantops.TicketRepository                  { return nil }
func (u testUnitOfWork) Pricings() pricingdomain.Repository                               { return nil }
func (u testUnitOfWork) Products() productdomain.Repository                               { return nil }
func (u testUnitOfWork) Records() recordsdomain.Repository {
	if u.recordsRepo != nil {
		return u.recordsRepo
	}
	return nil
}
func (u testUnitOfWork) Resources() resourcedomain.Repository                 { return nil }
func (u testUnitOfWork) Roles() roledomain.Repository                         { return nil }
func (u testUnitOfWork) Syncers() syncerdomain.Repository                     { return nil }
func (u testUnitOfWork) Sessions() domain.Repository                          { return u.sessionRepo }
func (u testUnitOfWork) Subscriptions() subscriptiondomain.Repository         { return nil }
func (u testUnitOfWork) Tickets() ticketdomain.Repository                     { return nil }
func (u testUnitOfWork) Tokens() tokendomain.Repository                       { return nil }
func (u testUnitOfWork) Webhooks() webhookdomain.Repository                   { return nil }
func (u testUnitOfWork) Users() userdomain.Repository                         { return nil }
func (u testUnitOfWork) Coupons() coupondomain.Repository                     { return nil }
func (u testUnitOfWork) CouponUsages() coupondomain.CouponUsageRepository     { return nil }
func (u testUnitOfWork) Entries() entrydomain.Repository                      { return nil }
func (u testUnitOfWork) McpServers() mcpserverdomain.Repository               { return nil }
func (u testUnitOfWork) RadiusAccountings() radiusaccountingdomain.Repository { return nil }
func (u testUnitOfWork) Verifications() verificationsdomain.Repository        { return nil }
func (u testUnitOfWork) WebAuthnCredentials() webauthndomain.Repository       { return nil }
func (u testUnitOfWork) FaceIdProviders() faceidproviderdomain.Repository     { return nil }
func (u testUnitOfWork) RadiusConfigs() radiusconfigdomain.Repository         { return nil }
func (u testUnitOfWork) Rules() ruledomain.Repository                         { return nil }
func (u testUnitOfWork) Transactions() transactiondomain.Repository           { return nil }
func (u testUnitOfWork) WebhookEvents() webhookeventdomain.Repository         { return nil }
func (u testUnitOfWork) Keys() keysdomain.Repository                          { return nil }
func (u testUnitOfWork) QRCodeSessions() qrcodesessions.Repository            { return nil }
func (u testUnitOfWork) DeviceAuthCodes() deviceauthcodes.Repository          { return nil }

func (u testUnitOfWork) Web3Identities() web3identity.Repository     { return nil }
func (u testUnitOfWork) FaceIDSessions() faceidsessions.Repository   { return nil }
func (u testUnitOfWork) CASTickets() castickets.Repository           { return nil }
func (u testUnitOfWork) KerberosTickets() kerberostickets.Repository { return nil }
func (u testUnitOfWork) LockoutAttempts() lockoutattempts.Repository { return nil }

func (u testUnitOfWork) IDV() idvdomain.Repository { return nil }

func (u testUnitOfWork) Workload() workloaddomain.Repository { return nil }

func (u testUnitOfWork) Rebac() rebac.Repository     { return nil }
func (u testUnitOfWork) Risk() riskdomain.Repository { return nil }

func (u testUnitOfWork) Actions() actionsdomain.Repository { return nil }

func (u testUnitOfWork) Metering() meteringdomain.Repository { return nil }

func (u testUnitOfWork) Outbox() domainoutbox.Repository {
	if u.outboxRepo != nil {
		return u.outboxRepo
	}
	return nil
}

func (u testUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}
func (u testUnitOfWork) IGA() igadomain.Repository { return nil }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestService_RevokeSession(t *testing.T) {
	t.Setenv("ISSUER_URL", "https://op.example.com")
	now := time.Now()
	repo := &mockSessionRepo{
		sessions: []*domain.Session{
			{ID: "sess-1", UserID: "user-1", OrganizationID: "org-1"},
		},
	}
	recordsRepo := &mockRecordsRepo{}
	outboxRepo := &mockOutboxRepo{}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{
		sessionRepo: repo,
		recordsRepo: recordsRepo,
		outboxRepo:  outboxRepo,
	}}

	err := svc.RevokeSession(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("RevokeSession() error = %v", err)
	}
	if repo.sessions[0].RevokedAt == nil {
		t.Error("expected RevokedAt to be set")
	}

	// § 审计同事务：Record 必须写
	if recordsRepo.createCalls != 1 {
		t.Errorf("audit Records.Create calls = %d, want 1", recordsRepo.createCalls)
	}
	if len(recordsRepo.records) > 0 && recordsRepo.records[0].Action != "session.revoked" {
		t.Errorf("audit Action = %q, want %q", recordsRepo.records[0].Action, "session.revoked")
	}
	if len(recordsRepo.records) > 0 && recordsRepo.records[0].ResourceID != "sess-1" {
		t.Errorf("audit ResourceID = %q, want %q", recordsRepo.records[0].ResourceID, "sess-1")
	}
	if len(recordsRepo.records) > 0 && recordsRepo.records[0].OrganizationID != "org-1" {
		t.Errorf("audit OrganizationID = %q, want %q", recordsRepo.records[0].OrganizationID, "org-1")
	}

	// § Outbox 事件必须入队：audit 事件 + SSF（CAEP）事件
	if outboxRepo.enqueueCalls != 2 {
		t.Errorf("outbox Enqueue calls = %d, want 2 (audit + ssf)", outboxRepo.enqueueCalls)
	}
	var auditEvent, ssfEvent bool
	for _, event := range outboxRepo.events {
		switch event.EventType {
		case "session.revoked":
			auditEvent = true
		case "ssf.session.revoked":
			ssfEvent = true
		}
	}
	if !auditEvent {
		t.Errorf("outbox events = %v, want audit event %q", outboxRepo.events, "session.revoked")
	}
	if !ssfEvent {
		t.Errorf("outbox events = %v, want ssf event %q", outboxRepo.events, "ssf.session.revoked")
	}
}

func TestService_RevokeSession_NotFound(t *testing.T) {
	repo := &mockSessionRepo{}
	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UOW: testUnitOfWork{sessionRepo: repo}}

	err := svc.RevokeSession(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session, got nil")
	}
}

func TestService_IsSessionDuplicated(t *testing.T) {
	now := time.Now()
	repo := &mockSessionRepo{
		sessions: []*domain.Session{
			{ID: "sess-1", UserID: "user-1", RevokedAt: nil},
			{ID: "sess-2", UserID: "user-1", RevokedAt: nil},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{sessionRepo: repo}}

	duplicated, err := svc.IsSessionDuplicated(context.Background(), "user-1", "sess-1")
	if err != nil {
		t.Fatalf("IsSessionDuplicated() error = %v", err)
	}
	if !duplicated {
		t.Error("expected duplicated = true when other active sessions exist")
	}
}

func TestService_IsSessionDuplicated_NotDuplicated(t *testing.T) {
	now := time.Now()
	repo := &mockSessionRepo{
		sessions: []*domain.Session{
			{ID: "sess-1", UserID: "user-1", RevokedAt: nil},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{sessionRepo: repo}}

	duplicated, err := svc.IsSessionDuplicated(context.Background(), "user-1", "sess-1")
	if err != nil {
		t.Fatalf("IsSessionDuplicated() error = %v", err)
	}
	if duplicated {
		t.Error("expected duplicated = false when only current session exists")
	}
}

func TestService_IsSessionDuplicated_RevokedIgnored(t *testing.T) {
	now := time.Now()
	revokedAt := now.Add(-1 * time.Hour)
	repo := &mockSessionRepo{
		sessions: []*domain.Session{
			{ID: "sess-1", UserID: "user-1", RevokedAt: nil},
			{ID: "sess-2", UserID: "user-1", RevokedAt: &revokedAt},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{sessionRepo: repo}}

	duplicated, err := svc.IsSessionDuplicated(context.Background(), "user-1", "sess-1")
	if err != nil {
		t.Fatalf("IsSessionDuplicated() error = %v", err)
	}
	if duplicated {
		t.Error("expected duplicated = false when other sessions are revoked")
	}
}

func TestService_RevokeOtherSessions(t *testing.T) {
	t.Setenv("ISSUER_URL", "https://op.example.com")
	now := time.Now()
	orgRepo := &mockOrgRepo{
		orgs: []*orgdomain.Organization{
			{ID: "org-1", Slug: "test-org"},
		},
	}
	repo := &mockSessionRepo{
		sessions: []*domain.Session{
			{ID: "sess-1", UserID: "user-1", OrganizationID: "org-1", ExpiresAt: now.Add(1 * time.Hour), LastSeenAt: now.Add(-1 * time.Minute)},
			{ID: "sess-2", UserID: "user-1", OrganizationID: "org-1"},
			{ID: "sess-3", UserID: "user-1", OrganizationID: "org-1"},
		},
	}
	recordsRepo := &mockRecordsRepo{}
	outboxRepo := &mockOutboxRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW: testUnitOfWork{
			sessionRepo: repo,
			orgRepo:     orgRepo,
			recordsRepo: recordsRepo,
			outboxRepo:  outboxRepo,
		},
	}

	result, err := svc.RevokeOtherSessions(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("RevokeOtherSessions() error = %v", err)
	}
	if result.RevokedCount != 2 {
		t.Errorf("RevokedCount = %d, want 2", result.RevokedCount)
	}

	// § 审计同事务
	if recordsRepo.createCalls != 1 {
		t.Errorf("audit Records.Create calls = %d, want 1", recordsRepo.createCalls)
	}
	if len(recordsRepo.records) > 0 && recordsRepo.records[0].Action != "session.revoked.others" {
		t.Errorf("audit Action = %q, want %q", recordsRepo.records[0].Action, "session.revoked.others")
	}
	if outboxRepo.enqueueCalls != 2 {
		t.Errorf("outbox Enqueue calls = %d, want 2 (audit + ssf)", outboxRepo.enqueueCalls)
	}
	var auditEvent, ssfEvent bool
	for _, event := range outboxRepo.events {
		switch event.EventType {
		case "session.revoked.others":
			auditEvent = true
		case "ssf.session.revoked":
			ssfEvent = true
		}
	}
	if !auditEvent {
		t.Errorf("outbox events = %v, want audit event %q", outboxRepo.events, "session.revoked.others")
	}
	if !ssfEvent {
		t.Errorf("outbox events = %v, want ssf event %q", outboxRepo.events, "ssf.session.revoked")
	}
}

func TestService_GetSession(t *testing.T) {
	now := time.Now()
	repo := &mockSessionRepo{
		sessions: []*domain.Session{
			{ID: "sess-1", UserID: "user-1", Username: "testuser", OrganizationID: "org-1", OrganizationSlug: "test-org", CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{sessionRepo: repo}}

	detail, err := svc.GetSession(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if detail.ID != "sess-1" {
		t.Errorf("ID = %q, want %q", detail.ID, "sess-1")
	}
	if detail.Username != "testuser" {
		t.Errorf("Username = %q, want %q", detail.Username, "testuser")
	}
}

func TestService_GetSession_NotFound(t *testing.T) {
	repo := &mockSessionRepo{}
	svc := Service{UOW: testUnitOfWork{sessionRepo: repo}}

	_, err := svc.GetSession(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session, got nil")
	}
}
