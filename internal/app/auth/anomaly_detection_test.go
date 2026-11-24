package auth

import (
	"context"
	"errors"
	"testing"
	"time"

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
	crosstenantops "github.com/guliangiam/guliangiam/internal/domain/cross_tenant_operations"
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
	kerberostickets "github.com/guliangiam/guliangiam/internal/domain/kerberostickets"
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
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
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
)

// ─── fakeSessionRepo 实现 sessions.Repository.List ────────────────────

type fakeSessionRepo struct {
	sessions []*sessiondomain.Session
	listErr  error
}

func (r *fakeSessionRepo) GetByID(_ context.Context, _ string) (*sessiondomain.Session, error) {
	return nil, nil
}
func (r *fakeSessionRepo) GetByIDForOrganization(_ context.Context, _, _ string) (*sessiondomain.Session, error) {
	return nil, nil
}
func (r *fakeSessionRepo) List(_ context.Context, q sessiondomain.ListQuery) ([]*sessiondomain.Session, int64, error) {
	if r.listErr != nil {
		return nil, 0, r.listErr
	}
	var out []*sessiondomain.Session
	for _, s := range r.sessions {
		if q.UserID != "" && s.UserID != q.UserID {
			continue
		}
		out = append(out, s)
	}
	return out, int64(len(out)), nil
}
func (r *fakeSessionRepo) Create(_ context.Context, _ *sessiondomain.Session) error { return nil }
func (r *fakeSessionRepo) Update(_ context.Context, _ *sessiondomain.Session) error { return nil }
func (r *fakeSessionRepo) Touch(_ context.Context, _ string, _ time.Time) error     { return nil }
func (r *fakeSessionRepo) Revoke(_ context.Context, _ string, _ time.Time) error    { return nil }
func (r *fakeSessionRepo) RevokeForOrganization(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}
func (r *fakeSessionRepo) RevokeByUserExcept(_ context.Context, _, _ string, _ time.Time) (int64, error) {
	return 0, nil
}
func (r *fakeSessionRepo) RevokeByUserAndApp(_ context.Context, _, _, _ string, _ time.Time) (int64, error) {
	return 0, nil
}
func (r *fakeSessionRepo) RevokeByUser(_ context.Context, _ string, _ time.Time) (int64, error) {
	return 0, nil
}
func (r *fakeSessionRepo) RevokeByOrganization(_ context.Context, _ string, _ time.Time) (int64, error) {
	return 0, nil
}
func (r *fakeSessionRepo) Delete(_ context.Context, _ string) error { return nil }
func (r *fakeSessionRepo) DeleteForOrganization(_ context.Context, _, _ string) error {
	return nil
}
func (r *fakeSessionRepo) CleanupExpired(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

// ─── anomalyTestUnitOfWork: AuthUoW 全存根，仅 Sessions() 真实 ─────────────

type anomalyTestUnitOfWork struct {
	sessions *fakeSessionRepo
}

var _ appuow.AuthUoW = (*anomalyTestUnitOfWork)(nil)

func (u anomalyTestUnitOfWork) Applications() applications.Repository                  { return nil }
func (u anomalyTestUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository { return nil }
func (u anomalyTestUnitOfWork) Certs() certdomain.Repository                           { return nil }
func (u anomalyTestUnitOfWork) Adapters() adapterdomain.Repository                     { return nil }
func (u anomalyTestUnitOfWork) Agents() agentdomain.Repository                         { return nil }
func (u anomalyTestUnitOfWork) Enforcers() enforcerdomain.Repository                   { return nil }
func (u anomalyTestUnitOfWork) Models() modeldomain.Repository                         { return nil }
func (u anomalyTestUnitOfWork) Syncers() syncerdomain.Repository                       { return nil }
func (u anomalyTestUnitOfWork) Webhooks() webhookdomain.Repository                     { return nil }
func (u anomalyTestUnitOfWork) Ldaps() ldapdomain.Repository                           { return nil }
func (u anomalyTestUnitOfWork) Forms() formdomain.Repository                           { return nil }
func (u anomalyTestUnitOfWork) Orders() orderdomain.Repository                         { return nil }
func (u anomalyTestUnitOfWork) Payments() paymentdomain.Repository                     { return nil }
func (u anomalyTestUnitOfWork) Plans() plandomain.Repository                           { return nil }

func (u anomalyTestUnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository { return nil }
func (u anomalyTestUnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository {
	return nil
}
func (u anomalyTestUnitOfWork) CrossTenantOps() crosstenantops.TicketRepository      { return nil }
func (u anomalyTestUnitOfWork) Pricings() pricingdomain.Repository                   { return nil }
func (u anomalyTestUnitOfWork) Products() productdomain.Repository                   { return nil }
func (u anomalyTestUnitOfWork) Subscriptions() subscriptiondomain.Repository         { return nil }
func (u anomalyTestUnitOfWork) Tickets() ticketdomain.Repository                     { return nil }
func (u anomalyTestUnitOfWork) Resources() resourcedomain.Repository                 { return nil }
func (u anomalyTestUnitOfWork) Consents() consentdomain.Repository                   { return nil }
func (u anomalyTestUnitOfWork) Gateway() gatewaydomain.Repository                    { return nil }
func (u anomalyTestUnitOfWork) Groups() groupsdomain.Repository                      { return nil }
func (u anomalyTestUnitOfWork) Invitations() invitationsdomain.Repository            { return nil }
func (u anomalyTestUnitOfWork) MfaChallenges() mfachallengedomain.Repository         { return nil }
func (u anomalyTestUnitOfWork) Organizations() orgdomain.Repository                  { return nil }
func (u anomalyTestUnitOfWork) Permissions() permissiondomain.Repository             { return nil }
func (u anomalyTestUnitOfWork) Providers() providerdomain.Repository                 { return nil }
func (u anomalyTestUnitOfWork) Roles() roledomain.Repository                         { return nil }
func (u anomalyTestUnitOfWork) Sessions() sessiondomain.Repository                   { return u.sessions }
func (u anomalyTestUnitOfWork) Tokens() tokendomain.Repository                       { return nil }
func (u anomalyTestUnitOfWork) Users() userdomain.Repository                         { return nil }
func (u anomalyTestUnitOfWork) Records() recordsdomain.Repository                    { return nil }
func (u anomalyTestUnitOfWork) Coupons() coupondomain.Repository                     { return nil }
func (u anomalyTestUnitOfWork) CouponUsages() coupondomain.CouponUsageRepository     { return nil }
func (u anomalyTestUnitOfWork) Entries() entrydomain.Repository                      { return nil }
func (u anomalyTestUnitOfWork) McpServers() mcpserverdomain.Repository               { return nil }
func (u anomalyTestUnitOfWork) RadiusAccountings() radiusaccountingdomain.Repository { return nil }
func (u anomalyTestUnitOfWork) Verifications() verificationsdomain.Repository        { return nil }
func (u anomalyTestUnitOfWork) WebAuthnCredentials() webauthndomain.Repository       { return nil }
func (u anomalyTestUnitOfWork) CaptchaProviders() captchaproviderdomain.Repository   { return nil }
func (u anomalyTestUnitOfWork) NotificationProviders() notificationproviderdomain.Repository {
	return nil
}
func (u anomalyTestUnitOfWork) Notifications() notificationsdomain.Repository    { return nil }
func (u anomalyTestUnitOfWork) FaceIdProviders() faceidproviderdomain.Repository { return nil }
func (u anomalyTestUnitOfWork) Rules() ruledomain.Repository                     { return nil }
func (u anomalyTestUnitOfWork) RadiusConfigs() radiusconfigdomain.Repository     { return nil }
func (u anomalyTestUnitOfWork) Keys() keysdomain.Repository                      { return nil }
func (u anomalyTestUnitOfWork) Transactions() transactiondomain.Repository       { return nil }
func (u anomalyTestUnitOfWork) WebhookEvents() webhookeventdomain.Repository     { return nil }
func (u anomalyTestUnitOfWork) QRCodeSessions() qrcodesessions.Repository        { return nil }
func (u anomalyTestUnitOfWork) DeviceAuthCodes() deviceauthcodes.Repository      { return nil }

func (u anomalyTestUnitOfWork) Web3Identities() web3identity.Repository     { return nil }
func (u anomalyTestUnitOfWork) FaceIDSessions() faceidsessions.Repository   { return nil }
func (u anomalyTestUnitOfWork) CASTickets() castickets.Repository           { return nil }
func (u anomalyTestUnitOfWork) KerberosTickets() kerberostickets.Repository { return nil }
func (u anomalyTestUnitOfWork) LockoutAttempts() lockoutattempts.Repository { return nil }
func (u anomalyTestUnitOfWork) IDV() idvdomain.Repository                   { return nil }
func (u anomalyTestUnitOfWork) Workload() workloaddomain.Repository         { return nil }
func (u anomalyTestUnitOfWork) Rebac() rebac.Repository                     { return nil }
func (u anomalyTestUnitOfWork) Risk() riskdomain.Repository                 { return nil }
func (u anomalyTestUnitOfWork) Actions() actionsdomain.Repository           { return nil }
func (u anomalyTestUnitOfWork) Metering() meteringdomain.Repository         { return nil }
func (u anomalyTestUnitOfWork) Outbox() domainoutbox.Repository             { return nil }
func (u anomalyTestUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, nil)
}
func (u anomalyTestUnitOfWork) IGA() igadomain.Repository { return nil }

// newAnomalyService 构造最小 Service（UOW + nil GeoLookup 跳过地理定位分支）
func newAnomalyService(repo *fakeSessionRepo) Service {
	return Service{
		Clock:     fakeClock{fixedTime: time.Now()},
		UOW:       &anomalyTestUnitOfWork{sessions: repo},
		GeoLookup: nil,
	}
}

func TestDetectAnomalousLogin_EmptyIP_NoReport(t *testing.T) {
	repo := &fakeSessionRepo{}
	svc := newAnomalyService(repo)
	report, err := svc.DetectAnomalousLogin(context.Background(), "user-1", "")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if report.IsAnomalous {
		t.Error("IsAnomalous=true, want false for empty IP")
	}
}

func TestDetectAnomalousLogin_WhitespaceIP_TreatedAsEmpty(t *testing.T) {
	repo := &fakeSessionRepo{}
	svc := newAnomalyService(repo)
	report, err := svc.DetectAnomalousLogin(context.Background(), "user-1", "   ")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if report.IsAnomalous {
		t.Error("IsAnomalous=true, want false for whitespace-only IP")
	}
}

func TestDetectAnomalousLogin_NoPriorSessions_NotAnomalous(t *testing.T) {
	repo := &fakeSessionRepo{sessions: []*sessiondomain.Session{}}
	svc := newAnomalyService(repo)
	report, err := svc.DetectAnomalousLogin(context.Background(), "user-1", "10.0.0.1")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if report.IsAnomalous {
		t.Error("IsAnomalous=true, want false when no prior sessions")
	}
}

func TestDetectAnomalousLogin_KnownIP_NotAnomalous(t *testing.T) {
	repo := &fakeSessionRepo{sessions: []*sessiondomain.Session{
		{UserID: "user-1", IPAddress: "10.0.0.1"},
	}}
	svc := newAnomalyService(repo)
	report, err := svc.DetectAnomalousLogin(context.Background(), "user-1", "10.0.0.1")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if report.IsAnomalous {
		t.Error("IsAnomalous=true, want false when IP is known")
	}
}

func TestDetectAnomalousLogin_NewIP_IsAnomalous(t *testing.T) {
	repo := &fakeSessionRepo{sessions: []*sessiondomain.Session{
		{UserID: "user-1", IPAddress: "10.0.0.1"},
		{UserID: "user-1", IPAddress: "10.0.0.2"},
	}}
	svc := newAnomalyService(repo)
	report, err := svc.DetectAnomalousLogin(context.Background(), "user-1", "192.168.1.100")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !report.IsAnomalous {
		t.Fatal("IsAnomalous=false, want true for new IP")
	}
	if !report.NewIP {
		t.Error("NewIP=false, want true")
	}
	if len(report.Reasons) == 0 {
		t.Error("Reasons empty")
	}
}

func TestDetectAnomalousLogin_DBError_GracefulDegrade(t *testing.T) {
	repo := &fakeSessionRepo{listErr: errors.New("db down")}
	svc := newAnomalyService(repo)
	report, err := svc.DetectAnomalousLogin(context.Background(), "user-1", "10.0.0.1")
	if err != nil {
		t.Fatalf("err=%v, want nil (graceful degrade)", err)
	}
	if report.IsAnomalous {
		t.Error("IsAnomalous=true on DB error, want false")
	}
}

func TestDetectAnomalousLogin_IPWhitespaceTrimmed(t *testing.T) {
	repo := &fakeSessionRepo{sessions: []*sessiondomain.Session{
		{UserID: "user-1", IPAddress: "10.0.0.1"},
	}}
	svc := newAnomalyService(repo)
	report, err := svc.DetectAnomalousLogin(context.Background(), "user-1", "  10.0.0.1  ")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if report.IsAnomalous {
		t.Error("IsAnomalous=true, want false — whitespace trimmed IP matches known")
	}
}
