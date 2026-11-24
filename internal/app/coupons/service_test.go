package coupons

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

type fakeClock struct{ fixedTime time.Time }

func (c fakeClock) Now() time.Time { return c.fixedTime }

type fakeUUIDGenerator struct{ fixedID string }

func (g fakeUUIDGenerator) NewString() (string, error) { return g.fixedID, nil }

type mockCouponRepo struct {
	coupons []*coupondomain.Coupon
	updates int
}

func (r *mockCouponRepo) GetByID(_ context.Context, id string) (*coupondomain.Coupon, error) {
	for _, c := range r.coupons {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, apperrors.NewNotFound("coupon not found")
}

func (r *mockCouponRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*coupondomain.Coupon, error) {
	for _, c := range r.coupons {
		if c.ID == id && c.OrganizationID == organizationID {
			return c, nil
		}
	}
	return nil, apperrors.NewNotFound("coupon not found")
}
func (r *mockCouponRepo) List(_ context.Context, _ coupondomain.ListQuery) ([]*coupondomain.Coupon, int64, error) {
	return r.coupons, int64(len(r.coupons)), nil
}
func (r *mockCouponRepo) Create(_ context.Context, c *coupondomain.Coupon) error {
	r.coupons = append(r.coupons, c)
	return nil
}
func (r *mockCouponRepo) Update(_ context.Context, c *coupondomain.Coupon) error {
	r.updates++
	for i, item := range r.coupons {
		if item.ID == c.ID {
			r.coupons[i] = c
			return nil
		}
	}
	return apperrors.NewNotFound("coupon not found")
}
func (r *mockCouponRepo) Delete(_ context.Context, id, organizationID string) error {
	for i, c := range r.coupons {
		if c.ID == id && c.OrganizationID == organizationID {
			r.coupons = append(r.coupons[:i], r.coupons[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("coupon not found")
}

// GetByCode mirrors the exact-code, tenant-scoped SQL lookup (BE-C-07).
func (r *mockCouponRepo) GetByCode(_ context.Context, organizationID, code string) (*coupondomain.Coupon, error) {
	for _, c := range r.coupons {
		if c.OrganizationID == organizationID && c.Code == code {
			return c, nil
		}
	}
	return nil, apperrors.NewNotFound("coupon not found")
}

// IncrementUsage mirrors the atomic guarded SQL increment (BE-C-06): active
// state, total quantity and revision predicate, all evaluated against the
// stored row rather than a caller-held copy.
func (r *mockCouponRepo) IncrementUsage(_ context.Context, id, _ string, now time.Time, expectedRevision int64) (*coupondomain.Coupon, error) {
	for _, c := range r.coupons {
		if c.ID != id {
			continue
		}
		if c.State != coupondomain.StateActive || c.Revision != expectedRevision ||
			(c.Quantity > 0 && c.UsedCount >= c.Quantity) {
			return nil, apperrors.NewConflict("coupon usage increment rejected: limit reached, inactive, or revision mismatch")
		}
		c.UsedCount++
		c.Revision++
		c.UpdatedAt = now
		return c, nil
	}
	return nil, apperrors.NewConflict("coupon usage increment rejected: limit reached, inactive, or revision mismatch")
}

type mockCouponUsageRepo struct{ usages []*coupondomain.CouponUsage }

func (r *mockCouponUsageRepo) Create(_ context.Context, u *coupondomain.CouponUsage) error {
	r.usages = append(r.usages, u)
	return nil
}
func (r *mockCouponUsageRepo) ListByCoupon(_ context.Context, _ string, _, _ int) ([]*coupondomain.CouponUsage, int64, error) {
	return r.usages, int64(len(r.usages)), nil
}
func (r *mockCouponUsageRepo) CountByCouponAndUser(_ context.Context, _, _ string) (int64, error) {
	return 0, nil
}

type testUnitOfWork struct {
	couponRepo      *mockCouponRepo
	couponUsageRepo *mockCouponUsageRepo
}

func (u testUnitOfWork) Applications() applications.Repository                        { return nil }
func (u testUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u testUnitOfWork) Consents() consentdomain.Repository                           { return nil }
func (u testUnitOfWork) Gateway() gatewaydomain.Repository                            { return nil }
func (u testUnitOfWork) Groups() groupsdomain.Repository                              { return nil }
func (u testUnitOfWork) Invitations() invitationsdomain.Repository                    { return nil }
func (u testUnitOfWork) Orders() orderdomain.Repository                               { return nil }
func (u testUnitOfWork) MfaChallenges() mfachallengedomain.Repository                 { return nil }
func (u testUnitOfWork) Organizations() orgdomain.Repository                          { return nil }
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
func (u testUnitOfWork) Payments() paymentdomain.Repository                           { return nil }
func (u testUnitOfWork) Plans() plandomain.Repository                                 { return nil }

func (u testUnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository           { return nil }
func (u testUnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository { return nil }
func (u testUnitOfWork) CrossTenantOps() crosstenantops.TicketRepository                  { return nil }
func (u testUnitOfWork) Pricings() pricingdomain.Repository                               { return nil }
func (u testUnitOfWork) Products() productdomain.Repository                               { return nil }
func (u testUnitOfWork) Records() recordsdomain.Repository                                { return nil }
func (u testUnitOfWork) Resources() resourcedomain.Repository                             { return nil }
func (u testUnitOfWork) Roles() roledomain.Repository                                     { return nil }
func (u testUnitOfWork) Syncers() syncerdomain.Repository                                 { return nil }
func (u testUnitOfWork) Sessions() sessiondomain.Repository                               { return nil }
func (u testUnitOfWork) Subscriptions() subscriptiondomain.Repository                     { return nil }
func (u testUnitOfWork) Tickets() ticketdomain.Repository                                 { return nil }
func (u testUnitOfWork) Tokens() tokendomain.Repository                                   { return nil }
func (u testUnitOfWork) Webhooks() webhookdomain.Repository                               { return nil }
func (u testUnitOfWork) Users() userdomain.Repository                                     { return nil }
func (u testUnitOfWork) Coupons() coupondomain.Repository                                 { return u.couponRepo }
func (u testUnitOfWork) CouponUsages() coupondomain.CouponUsageRepository                 { return u.couponUsageRepo }
func (u testUnitOfWork) Entries() entrydomain.Repository                                  { return nil }
func (u testUnitOfWork) McpServers() mcpserverdomain.Repository                           { return nil }
func (u testUnitOfWork) RadiusAccountings() radiusaccountingdomain.Repository             { return nil }
func (u testUnitOfWork) Verifications() verificationsdomain.Repository                    { return nil }
func (u testUnitOfWork) WebAuthnCredentials() webauthndomain.Repository                   { return nil }
func (u testUnitOfWork) FaceIdProviders() faceidproviderdomain.Repository                 { return nil }
func (u testUnitOfWork) RadiusConfigs() radiusconfigdomain.Repository                     { return nil }
func (u testUnitOfWork) Rules() ruledomain.Repository                                     { return nil }
func (u testUnitOfWork) Transactions() transactiondomain.Repository                       { return nil }
func (u testUnitOfWork) WebhookEvents() webhookeventdomain.Repository                     { return nil }
func (u testUnitOfWork) Keys() keysdomain.Repository                                      { return nil }
func (u testUnitOfWork) QRCodeSessions() qrcodesessions.Repository                        { return nil }
func (u testUnitOfWork) DeviceAuthCodes() deviceauthcodes.Repository                      { return nil }

func (u testUnitOfWork) Web3Identities() web3identity.Repository     { return nil }
func (u testUnitOfWork) FaceIDSessions() faceidsessions.Repository   { return nil }
func (u testUnitOfWork) CASTickets() castickets.Repository           { return nil }
func (u testUnitOfWork) KerberosTickets() kerberostickets.Repository { return nil }
func (u testUnitOfWork) LockoutAttempts() lockoutattempts.Repository { return nil }
func (u testUnitOfWork) IDV() idvdomain.Repository                   { return nil }

func (u testUnitOfWork) Workload() workloaddomain.Repository { return nil }

func (u testUnitOfWork) Rebac() rebac.Repository { return nil }

func (u testUnitOfWork) Risk() riskdomain.Repository { return nil }

func (u testUnitOfWork) Actions() actionsdomain.Repository { return nil }

func (u testUnitOfWork) Metering() meteringdomain.Repository { return nil }

func (u testUnitOfWork) Outbox() domainoutbox.Repository { return nil }

func (u testUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}
func (u testUnitOfWork) IGA() igadomain.Repository { return nil }

func newSvc(repo *mockCouponRepo) Service {
	return Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test-id"},
		UOW:   testUnitOfWork{couponRepo: repo, couponUsageRepo: &mockCouponUsageRepo{}},
	}
}

func TestService_CreateCoupon_Validation(t *testing.T) {
	svc := newSvc(&mockCouponRepo{})
	tests := []struct {
		name    string
		cmd     CreateCouponCommand
		wantErr bool
	}{
		{name: "valid", cmd: CreateCouponCommand{OrganizationID: "org-1", Name: "c-1", Code: "SAVE10", DiscountType: "percentage", Discount: 10}, wantErr: false},
		{name: "missing orgId", cmd: CreateCouponCommand{Name: "c-1"}, wantErr: true},
		{name: "missing name", cmd: CreateCouponCommand{OrganizationID: "org-1"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateCoupon(context.Background(), tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_CreateCoupon_Success(t *testing.T) {
	repo := &mockCouponRepo{}
	svc := newSvc(repo)
	err := svc.CreateCoupon(context.Background(), CreateCouponCommand{
		OrganizationID: "org-1", Name: "c-1", DisplayName: "Test", Code: "SAVE10",
		DiscountType: "percentage", Discount: 10, State: coupondomain.StateActive,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(repo.coupons) != 1 {
		t.Fatalf("expected 1, got %d", len(repo.coupons))
	}
	c := repo.coupons[0]
	if c.ID != "test-id" {
		t.Errorf("ID = %q", c.ID)
	}
	if c.State != coupondomain.StateActive {
		t.Errorf("State = %q", c.State)
	}
	if c.Revision != 1 {
		t.Errorf("Revision = %d", c.Revision)
	}
}

func TestService_GetCoupon(t *testing.T) {
	now := time.Now()
	repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{{ID: "c-1", OrganizationID: "org-1", Name: "test", CreatedAt: now, UpdatedAt: now}}}
	svc := newSvc(repo)
	d, err := svc.GetCoupon(context.Background(), "org-1", "c-1")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if d.ID != "c-1" {
		t.Errorf("ID = %q", d.ID)
	}
}

func TestService_ListCoupons(t *testing.T) {
	now := time.Now()
	repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{
		{ID: "c-1", CreatedAt: now, UpdatedAt: now},
		{ID: "c-2", CreatedAt: now, UpdatedAt: now},
	}}
	svc := newSvc(repo)
	items, total, err := svc.ListCoupons(context.Background(), ListCouponsQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d", total)
	}
	if len(items) != 2 {
		t.Errorf("len = %d", len(items))
	}
}

func TestService_DeleteCoupon(t *testing.T) {
	repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{{ID: "c-1", OrganizationID: "org-1"}}}
	svc := newSvc(repo)
	if err := svc.DeleteCoupon(context.Background(), "org-1", "c-1"); err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(repo.coupons) != 0 {
		t.Errorf("expected 0, got %d", len(repo.coupons))
	}
}

func TestService_PatchCoupon_RevisionMismatch(t *testing.T) {
	repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{{ID: "c-1", OrganizationID: "org-1", Revision: 2}}}
	svc := newSvc(repo)
	err := svc.PatchCoupon(context.Background(), "org-1", PatchCouponCommand{ID: "c-1", Revision: 1})
	if err == nil {
		t.Error("expected error")
	}
}

func TestService_ValidateCoupon_EmptyCode(t *testing.T) {
	svc := newSvc(&mockCouponRepo{})
	r, err := svc.ValidateCoupon(context.Background(), "org-1", "", "", nil, 0)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if r.Valid {
		t.Error("expected Valid=false")
	}
}

func TestService_ValidateCoupon_Valid(t *testing.T) {
	now := time.Now()
	repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{
		{ID: "c-1", OrganizationID: "org-1", Code: "SAVE10", State: coupondomain.StateActive,
			DiscountType: "percentage", Discount: 10, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{couponRepo: repo, couponUsageRepo: &mockCouponUsageRepo{}}}
	r, err := svc.ValidateCoupon(context.Background(), "org-1", "SAVE10", "", nil, 0)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !r.Valid {
		t.Errorf("expected Valid=true, got %q", r.Message)
	}
}

func TestService_IncrementCouponUsage(t *testing.T) {
	now := time.Now()
	repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{{ID: "c-1", State: coupondomain.StateActive, Revision: 1, CreatedAt: now, UpdatedAt: now}}}
	usageRepo := &mockCouponUsageRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now}, UUID: fakeUUIDGenerator{fixedID: "uid"},
		UOW: testUnitOfWork{couponRepo: repo, couponUsageRepo: usageRepo},
	}
	if err := svc.IncrementCouponUsage(context.Background(), "c-1", "u-1", "o-1"); err != nil {
		t.Fatalf("error = %v", err)
	}
	if repo.coupons[0].UsedCount != 1 {
		t.Errorf("UsedCount = %d", repo.coupons[0].UsedCount)
	}
	if len(usageRepo.usages) != 1 {
		t.Errorf("usages = %d", len(usageRepo.usages))
	}
}

func TestService_PatchCoupon_Success(t *testing.T) {
	now := time.Now()
	repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{{ID: "c-1", OrganizationID: "org-1", Name: "test", Revision: 1, CreatedAt: now, UpdatedAt: now}}}
	svc := newSvc(repo)
	displayName := "Updated"
	err := svc.PatchCoupon(context.Background(), "org-1", PatchCouponCommand{ID: "c-1", DisplayName: &displayName, Revision: 1})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if repo.coupons[0].DisplayName != "Updated" {
		t.Errorf("DisplayName = %q", repo.coupons[0].DisplayName)
	}
	if repo.coupons[0].Revision != 2 {
		t.Errorf("Revision = %d", repo.coupons[0].Revision)
	}
}

func TestService_PatchCoupon_RejectsInvalidTimeWindow(t *testing.T) {
	validStart := "2025-01-01T00:00:00Z"
	validExpire := "2025-12-31T23:59:59Z"
	invalid := "not-a-date"
	tests := []struct {
		name      string
		start     *string
		expire    *string
		wantField string
	}{
		{name: "invalid start", start: &invalid, expire: &validExpire, wantField: "startTime"},
		{name: "invalid expire", start: &validStart, expire: &invalid, wantField: "expireTime"},
		{name: "expire before start", start: &validExpire, expire: &validStart, wantField: "expireTime"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			repo := &mockCouponRepo{coupons: []*coupondomain.Coupon{{ID: "c-1", OrganizationID: "org-1", Name: "test", Revision: 1, CreatedAt: now, UpdatedAt: now}}}
			svc := newSvc(repo)
			err := svc.PatchCoupon(context.Background(), "org-1", PatchCouponCommand{
				ID: "c-1", StartTime: tt.start, ExpireTime: tt.expire, Revision: 1,
			})
			if err == nil {
				t.Fatal("expected 422, got nil error")
			}
			appErr, ok := err.(*apperrors.Error)
			if !ok {
				t.Fatalf("error type = %T, want *apperrors.Error", err)
			}
			if appErr.Code != apperrors.CodeUnprocessableEntity {
				t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeUnprocessableEntity)
			}
			if field, _ := appErr.Meta["field"].(string); field != tt.wantField {
				t.Errorf("meta field = %q, want %q", field, tt.wantField)
			}
			if repo.updates != 0 {
				t.Errorf("invalid patch reached Update() %d times", repo.updates)
			}
		})
	}
}
