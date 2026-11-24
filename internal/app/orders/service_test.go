package orders

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
	domain "github.com/guliangiam/guliangiam/internal/domain/orders"
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
// Fake platform implementations
// ---------------------------------------------------------------------------

type fakeClock struct {
	fixedTime time.Time
}

func (c fakeClock) Now() time.Time { return c.fixedTime }

type fakeUUIDGenerator struct {
	fixedID string
	callNum int
	altID   string
}

func (g *fakeUUIDGenerator) NewString() (string, error) {
	g.callNum++
	if g.callNum > 1 && g.altID != "" {
		return g.altID, nil
	}
	return g.fixedID, nil
}

// ---------------------------------------------------------------------------
// Mock order repository
// ---------------------------------------------------------------------------

type mockOrderRepo struct {
	orders []*domain.Order
}

func (r *mockOrderRepo) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	return r.find(id, "")
}

func (r *mockOrderRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Order, error) {
	return r.find(id, organizationID)
}

func (r *mockOrderRepo) find(id, organizationID string) (*domain.Order, error) {
	for _, o := range r.orders {
		if o.ID != id {
			continue
		}
		if organizationID == "" || o.OrganizationID == organizationID {
			return o, nil
		}
	}
	return nil, apperrors.NewNotFound("order not found")
}

func (r *mockOrderRepo) GetByTradeNo(ctx context.Context, tradeNo string) (*domain.Order, error) {
	for _, o := range r.orders {
		if o.TradeNo == tradeNo {
			return o, nil
		}
	}
	return nil, apperrors.NewNotFound("order not found by trade no")
}

func (r *mockOrderRepo) List(ctx context.Context, query domain.ListQuery) ([]*domain.Order, int64, error) {
	return r.orders, int64(len(r.orders)), nil
}

func (r *mockOrderRepo) Create(ctx context.Context, order *domain.Order) error {
	r.orders = append(r.orders, order)
	return nil
}

func (r *mockOrderRepo) Update(ctx context.Context, order *domain.Order) error {
	for i, o := range r.orders {
		if o.ID == order.ID {
			r.orders[i] = order
			return nil
		}
	}
	return apperrors.NewNotFound("order not found")
}

func (r *mockOrderRepo) Delete(ctx context.Context, id, organizationID string) error {
	for i, o := range r.orders {
		if o.ID != id {
			continue
		}
		if organizationID != "" && o.OrganizationID != organizationID {
			break
		}
		r.orders = append(r.orders[:i], r.orders[i+1:]...)
		return nil
	}
	return apperrors.NewNotFound("order not found")
}

// mockProductRepo is the catalog the order service prices a basket against. The
// service only reads it through the tenant-scoped lookup; the embedded port
// keeps this fake compatible with the repository's remaining surface.
type mockProductRepo struct {
	productdomain.Repository
	products []*productdomain.Product
}

func (r *mockProductRepo) GetByID(ctx context.Context, id string) (*productdomain.Product, error) {
	for _, p := range r.products {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, apperrors.NewNotFound("product not found")
}

func (r *mockProductRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*productdomain.Product, error) {
	for _, p := range r.products {
		if p.ID == id && p.OrganizationID == organizationID {
			return p, nil
		}
	}
	return nil, apperrors.NewNotFound("product not found")
}

// IncrementSold mirrors the infra atomic guard (BE-C-11): the SQL increments
// sold under `sold+q <= quantity`; the fake applies the same rule so placement
// tests exercise the real reservation semantics.
func (r *mockProductRepo) IncrementSold(_ context.Context, id, organizationID string, quantity int, now time.Time) (*productdomain.Product, error) {
	p, err := r.GetByIDForOrganization(context.Background(), id, organizationID)
	if err != nil {
		return nil, err
	}
	if p.State != productdomain.StateActive {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "product is not available for purchase", map[string]any{"field": "state", "value": string(p.State)})
	}
	if quantity <= 0 {
		return nil, apperrors.NewInvalidArgument("quantity must be positive")
	}
	if p.Quantity > 0 && p.Sold+quantity > p.Quantity {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "insufficient product quantity", map[string]any{"field": "quantity", "value": quantity, "max": p.Quantity - p.Sold})
	}
	p.Sold += quantity
	p.UpdatedAt = now
	p.Revision++
	return p, nil
}

// ReleaseSold returns reserved stock, floored at zero (BE-C-11).
func (r *mockProductRepo) ReleaseSold(_ context.Context, id, organizationID string, quantity int, now time.Time) (*productdomain.Product, error) {
	p, err := r.GetByIDForOrganization(context.Background(), id, organizationID)
	if err != nil {
		return nil, err
	}
	if quantity <= 0 {
		return nil, apperrors.NewInvalidArgument("quantity must be positive")
	}
	p.Sold -= quantity
	if p.Sold < 0 {
		p.Sold = 0
	}
	p.UpdatedAt = now
	p.Revision++
	return p, nil
}

// sellableProduct is the catalog fixture the placement tests buy: an `active`
// product is the only sellable one (§1.5), so every basket test needs one.
func sellableProduct(id, name string, price float64, currency string) *productdomain.Product {
	return &productdomain.Product{
		ID: id, OrganizationID: "org-1", Name: name, DisplayName: name,
		Price: price, Currency: currency, State: productdomain.StateActive,
	}
}

// ---------------------------------------------------------------------------
// Test UOW
// ---------------------------------------------------------------------------

type testUnitOfWork struct {
	orderRepo       *mockOrderRepo
	productRepo     *mockProductRepo
	couponRepo      coupondomain.Repository
	couponUsageRepo coupondomain.CouponUsageRepository
}

func (u testUnitOfWork) Applications() applications.Repository                        { return nil }
func (u testUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u testUnitOfWork) Consents() consentdomain.Repository                           { return nil }
func (u testUnitOfWork) Gateway() gatewaydomain.Repository                            { return nil }
func (u testUnitOfWork) Groups() groupsdomain.Repository                              { return nil }
func (u testUnitOfWork) Invitations() invitationsdomain.Repository                    { return nil }
func (u testUnitOfWork) Orders() domain.Repository                                    { return u.orderRepo }
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

func (u testUnitOfWork) Payments() paymentdomain.Repository { return nil }
func (u testUnitOfWork) Plans() plandomain.Repository       { return nil }

func (u testUnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository           { return nil }
func (u testUnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository { return nil }
func (u testUnitOfWork) CrossTenantOps() crosstenantops.TicketRepository                  { return nil }
func (u testUnitOfWork) Pricings() pricingdomain.Repository                               { return nil }
func (u testUnitOfWork) Products() productdomain.Repository                               { return u.productRepo }
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

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestService_CreateOrder(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019order-test-id-aaaaa"},
		UOW:   testUnitOfWork{orderRepo: repo},
	}
	ctx := context.Background()
	err := svc.CreateOrder(ctx, CreateOrderCommand{
		OrganizationID: "org-001",
		Name:           "order-001",
		DisplayName:    "Test Order",
		User:           "user-001",
		Price:          99.99,
		Currency:       "USD",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if len(repo.orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(repo.orders))
	}
	o := repo.orders[0]
	if o.ID != "019order-test-id-aaaaa" {
		t.Errorf("expected ID '019order-test-id-aaaaa', got %q", o.ID)
	}
	if o.State != domain.StateCreated {
		t.Errorf("expected State 'Created', got %q", o.State)
	}
	if o.Price != 99.99 {
		t.Errorf("expected Price 99.99, got %f", o.Price)
	}
}

func TestService_CreateOrder_Validation(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  &fakeUUIDGenerator{fixedID: "test"},
		UOW:   testUnitOfWork{orderRepo: &mockOrderRepo{}},
	}
	ctx := context.Background()
	tests := []struct {
		name    string
		cmd     CreateOrderCommand
		wantErr string
	}{
		{name: "missing organizationId", cmd: CreateOrderCommand{Name: "order-1"}, wantErr: "organizationId is required"},
		{name: "missing name", cmd: CreateOrderCommand{OrganizationID: "org-1"}, wantErr: "name is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateOrder(ctx, tt.cmd)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestService_CancelOrder(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", OrganizationID: "org-1", State: domain.StateCreated, Revision: 1, CreatedAt: now, UpdatedAt: now},
		{ID: "order-2", OrganizationID: "org-1", State: domain.StatePaid, Revision: 1, CreatedAt: now, UpdatedAt: now, PaidAt: &now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	ctx := context.Background()
	err := svc.CancelOrder(ctx, "org-1", "order-1", 1)
	if err != nil {
		t.Fatalf("CancelOrder(Created) error = %v", err)
	}
	if repo.orders[0].State != domain.StateCancelled {
		t.Errorf("expected State 'Cancelled', got %q", repo.orders[0].State)
	}
	err = svc.CancelOrder(ctx, "org-1", "order-2", 1)
	if err == nil {
		t.Fatalf("expected error for CancelOrder(Paid), got nil")
	}
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodePaymentRefundRequired {
		t.Fatalf("CancelOrder(Paid) error = %v, want code %s", err, apperrors.CodePaymentRefundRequired)
	}
}

// stubQueryPayment installs a channel verdict for the duration of a test.
func stubQueryPayment(t *testing.T, fn QueryPaymentFunc) {
	t.Helper()
	previous := sharedPaymentAdapters.Load()
	SetPaymentAdapters(nil, fn)
	t.Cleanup(func() { sharedPaymentAdapters.Store(previous) })
}

// stubCreatePayment installs a channel trade creation for the duration of a test.
func stubCreatePayment(t *testing.T, fn CreatePaymentFunc) {
	t.Helper()
	previous := sharedPaymentAdapters.Load()
	SetPaymentAdapters(fn, nil)
	t.Cleanup(func() { sharedPaymentAdapters.Store(previous) })
}

// TestService_PayOrder_RequiresChannelConfirmation pins the core correction:
// the order reaches `paid` only when the channel says the money moved, never
// because the caller asked.
func TestService_PayOrder_RequiresChannelConfirmation(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", OrganizationID: "org-1", State: domain.StatePaymentPending, TradeNo: "trade-1", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	stubQueryPayment(t, func(context.Context, string) (paymentdomain.Outcome, error) {
		return paymentdomain.OutcomeSucceeded, nil
	})

	if err := svc.PayOrder(context.Background(), "org-1", "order-1", "pay-ref-001"); err != nil {
		t.Fatalf("PayOrder() error = %v", err)
	}
	o := repo.orders[0]
	if o.State != domain.StatePaid {
		t.Errorf("State = %q, want %q", o.State, domain.StatePaid)
	}
	if o.PaidAt == nil {
		t.Error("PaidAt = nil, want the settlement instant recorded with the paid state")
	}
	if o.Payment != "pay-ref-001" {
		t.Errorf("Payment = %q, want %q", o.Payment, "pay-ref-001")
	}
}

// TestService_PayOrder_ChannelStillPending keeps the order payable when the
// channel has not settled yet.
func TestService_PayOrder_ChannelStillPending(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", OrganizationID: "org-1", State: domain.StatePaymentPending, TradeNo: "trade-1", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	stubQueryPayment(t, func(context.Context, string) (paymentdomain.Outcome, error) {
		return paymentdomain.OutcomePending, nil
	})

	if err := svc.PayOrder(context.Background(), "org-1", "order-1", ""); err != nil {
		t.Fatalf("PayOrder() error = %v", err)
	}
	if repo.orders[0].State != domain.StatePaymentPending {
		t.Errorf("State = %q, want it to stay %q", repo.orders[0].State, domain.StatePaymentPending)
	}
}

func TestService_PayOrder_ChannelRejected(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", OrganizationID: "org-1", State: domain.StatePaymentPending, TradeNo: "trade-1", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	stubQueryPayment(t, func(context.Context, string) (paymentdomain.Outcome, error) {
		return paymentdomain.OutcomeFailed, nil
	})

	if err := svc.PayOrder(context.Background(), "org-1", "order-1", ""); err != nil {
		t.Fatalf("PayOrder() error = %v", err)
	}
	o := repo.orders[0]
	if o.State != domain.StateFailed {
		t.Errorf("State = %q, want %q", o.State, domain.StateFailed)
	}
	if o.Message == "" {
		t.Error("Message = empty, want the channel rejection recorded")
	}
	if !o.CanTransitionTo(domain.StatePaymentPending) {
		t.Error("a failed order must stay retryable")
	}
}

// TestService_PayOrder_IdempotentOnSettledOrder covers AC-FE-016
// 「不产生重复支付副作用」: repeating `:pay` on a settled order must be a no-op.
func TestService_PayOrder_IdempotentOnSettledOrder(t *testing.T) {
	now := time.Now()
	paidAt := now.Add(-time.Hour)
	repo := &mockOrderRepo{orders: []*domain.Order{
		{
			ID: "order-1", OrganizationID: "org-1", State: domain.StatePaid, TradeNo: "trade-1",
			PaidAt: &paidAt, Revision: 7, CreatedAt: now, UpdatedAt: now,
		},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	called := false
	stubQueryPayment(t, func(context.Context, string) (paymentdomain.Outcome, error) {
		called = true
		return paymentdomain.OutcomeSucceeded, nil
	})

	if err := svc.PayOrder(context.Background(), "org-1", "order-1", "pay-ref-002"); err != nil {
		t.Fatalf("PayOrder() error = %v", err)
	}
	o := repo.orders[0]
	if called {
		t.Error("settled order must not be re-queried against the channel")
	}
	if o.Revision != 7 {
		t.Errorf("Revision = %d, want it untouched at 7", o.Revision)
	}
	if o.State != domain.StatePaid {
		t.Errorf("State = %q, want %q", o.State, domain.StatePaid)
	}
}

// TestService_PayOrder_NoTradeRequestsOne pins that an order that never reached
// the channel gets a trade before it is considered payable, instead of being
// marked paid without any channel evidence.
func TestService_PayOrder_NoTradeRequestsOne(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", OrganizationID: "org-1", Name: "order-1", DisplayName: "Order", State: domain.StateCreated, Price: 10, Currency: "USD", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	stubCreatePayment(t, func(context.Context, string, float64, string, string, string, string) (string, string, error) {
		return "https://pay.example.com/trade-1", "trade-1", nil
	})

	if err := svc.PayOrder(context.Background(), "org-1", "order-1", ""); err != nil {
		t.Fatalf("PayOrder() error = %v", err)
	}
	o := repo.orders[0]
	if o.State != domain.StatePaymentPending {
		t.Errorf("State = %q, want %q", o.State, domain.StatePaymentPending)
	}
	if o.TradeNo != "trade-1" {
		t.Errorf("TradeNo = %q, want %q", o.TradeNo, "trade-1")
	}
	if o.PaymentURL == "" {
		t.Error("PaymentURL = empty, want the channel's redirect URL recorded")
	}
}

// TestService_PayOrder_GatewayDownKeepsOrderRetryable asserts the failure is
// neither swallowed nor turned into an invented state: the order stays where it
// was and says why.
func TestService_PayOrder_GatewayDownKeepsOrderRetryable(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", OrganizationID: "org-1", Name: "order-1", State: domain.StateCreated, Price: 10, Currency: "USD", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	stubCreatePayment(t, func(context.Context, string, float64, string, string, string, string) (string, string, error) {
		return "", "", apperrors.NewUnavailable("channel down")
	})

	if err := svc.PayOrder(context.Background(), "org-1", "order-1", ""); err != nil {
		t.Fatalf("PayOrder() error = %v", err)
	}
	o := repo.orders[0]
	if o.State != domain.StateCreated {
		t.Errorf("State = %q, want it untouched at %q", o.State, domain.StateCreated)
	}
	if o.Message == "" {
		t.Error("Message = empty, want the channel failure recorded for the reader")
	}
	if !o.CanTransitionTo(domain.StatePaymentPending) {
		t.Error("order must remain payable after a channel failure")
	}
}

// TestService_PayOrder_ChannelUnreachableOnQuery surfaces the channel error
// instead of guessing a state.
func TestService_PayOrder_ChannelUnreachableOnQuery(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", OrganizationID: "org-1", State: domain.StatePaymentPending, TradeNo: "trade-1", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	stubQueryPayment(t, func(context.Context, string) (paymentdomain.Outcome, error) {
		return paymentdomain.OutcomeUnknown, apperrors.NewUnavailable("channel down")
	})

	err := svc.PayOrder(context.Background(), "org-1", "order-1", "")
	if err == nil {
		t.Fatal("PayOrder() error = nil, want the channel failure reported")
	}
	if repo.orders[0].State != domain.StatePaymentPending {
		t.Errorf("State = %q, want it untouched at %q", repo.orders[0].State, domain.StatePaymentPending)
	}
}

func TestService_PayOrder_InvalidState(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", OrganizationID: "org-1", State: domain.StateCancelled, Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	ctx := context.Background()
	err := svc.PayOrder(ctx, "org-1", "order-1", "")
	if err == nil {
		t.Fatalf("expected error for PayOrder(Cancelled), got nil")
	}
}

func TestService_DeleteOrder(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", OrganizationID: "org-1", State: domain.StateCreated, Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	ctx := context.Background()
	err := svc.DeleteOrder(ctx, "org-1", "order-1")
	if err != nil {
		t.Fatalf("DeleteOrder() error = %v", err)
	}
	if len(repo.orders) != 0 {
		t.Errorf("expected 0 orders after delete, got %d", len(repo.orders))
	}
}

func TestService_DeleteOrder_NotFound(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	ctx := context.Background()
	err := svc.DeleteOrder(ctx, "org-1", "nonexistent")
	if err == nil {
		t.Fatalf("expected error for nonexistent order, got nil")
	}
}

func TestService_GetOrder_Success(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", OrganizationID: "org-1", Name: "test-order", DisplayName: "Test Order", State: domain.StateCreated, Price: 99.99, Currency: "USD", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{UOW: testUnitOfWork{orderRepo: repo}}
	ctx := context.Background()
	detail, err := svc.GetOrder(ctx, "org-1", "order-1")
	if err != nil {
		t.Fatalf("GetOrder() error = %v", err)
	}
	if detail.ID != "order-1" {
		t.Errorf("ID = %q, want %q", detail.ID, "order-1")
	}
	if detail.State != domain.StateCreated {
		t.Errorf("State = %q, want %q", detail.State, domain.StateCreated)
	}
	if detail.Price != 99.99 {
		t.Errorf("Price = %f, want 99.99", detail.Price)
	}
}

func TestService_GetOrder_NotFound(t *testing.T) {
	repo := &mockOrderRepo{}
	svc := Service{UOW: testUnitOfWork{orderRepo: repo}}
	ctx := context.Background()
	_, err := svc.GetOrder(ctx, "org-1", "nonexistent")
	if err == nil {
		t.Fatalf("expected error for nonexistent order, got nil")
	}
}

func TestService_ListOrders_Success(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", Name: "order-1", State: domain.StateCreated, CreatedAt: now, UpdatedAt: now},
		{ID: "order-2", Name: "order-2", State: domain.StatePaid, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{UOW: testUnitOfWork{orderRepo: repo}}
	ctx := context.Background()
	items, total, err := svc.ListOrders(ctx, ListOrdersQuery{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("ListOrders() error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(items))
	}
}

func TestService_PatchOrder_Success(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", OrganizationID: "org-1", Name: "test-order", DisplayName: "Test Order", State: domain.StateCreated, Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	ctx := context.Background()

	newDisplayName := "Updated Order"
	err := svc.PatchOrder(ctx, "org-1", PatchOrderCommand{
		ID:          "order-1",
		DisplayName: &newDisplayName,
		Revision:    1,
	})
	if err != nil {
		t.Fatalf("PatchOrder() error = %v", err)
	}
	if repo.orders[0].DisplayName != "Updated Order" {
		t.Errorf("DisplayName = %q, want %q", repo.orders[0].DisplayName, "Updated Order")
	}
	if repo.orders[0].Revision != 2 {
		t.Errorf("Revision = %d, want 2", repo.orders[0].Revision)
	}
}

// TestService_PatchOrder_DoesNotTouchLifecycle pins that the edit path cannot
// move the state machine: the registry is the only writer of `orders.state`.
func TestService_PatchOrder_DoesNotTouchLifecycle(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", OrganizationID: "org-1", Name: "test-order", State: domain.StatePaymentPending, Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	ctx := context.Background()

	newName := "Renamed"
	if err := svc.PatchOrder(ctx, "org-1", PatchOrderCommand{
		ID: "order-1", DisplayName: &newName, Revision: 1,
	}); err != nil {
		t.Fatalf("PatchOrder() error = %v", err)
	}
	if repo.orders[0].State != domain.StatePaymentPending {
		t.Errorf("State = %q, want it untouched at %q", repo.orders[0].State, domain.StatePaymentPending)
	}
}

func TestService_PatchOrder_RevisionMismatch(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", OrganizationID: "org-1", Name: "test-order", State: domain.StateCreated, Revision: 5, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	ctx := context.Background()

	newDisplayName := "Updated Order"
	err := svc.PatchOrder(ctx, "org-1", PatchOrderCommand{
		ID:          "order-1",
		DisplayName: &newDisplayName,
		Revision:    3, // stale
	})
	if err == nil {
		t.Fatal("PatchOrder() expected error for revision mismatch, got nil")
	}
}

func TestService_PatchOrder_NotFound(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	ctx := context.Background()

	newDisplayName := "Updated Order"
	err := svc.PatchOrder(ctx, "org-1", PatchOrderCommand{
		ID:          "nonexistent",
		DisplayName: &newDisplayName,
	})
	if err == nil {
		t.Fatal("PatchOrder() expected error for nonexistent order, got nil")
	}
}

func TestService_CancelOrder_RevisionMismatch(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", OrganizationID: "org-1", State: domain.StateCreated, Revision: 5, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	ctx := context.Background()

	err := svc.CancelOrder(ctx, "org-1", "order-1", 3) // stale revision
	if err == nil {
		t.Fatal("CancelOrder() expected error for revision mismatch, got nil")
	}
}

func TestService_CancelOrder_PendingState(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", OrganizationID: "org-1", State: domain.StatePaymentPending, Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	ctx := context.Background()

	err := svc.CancelOrder(ctx, "org-1", "order-1", 1)
	if err != nil {
		t.Fatalf("CancelOrder(Pending) error = %v", err)
	}
	if repo.orders[0].State != domain.StateCancelled {
		t.Errorf("expected State 'Cancelled', got %q", repo.orders[0].State)
	}
}

func TestService_CancelOrder_NotFound(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}
	ctx := context.Background()

	err := svc.CancelOrder(ctx, "org-1", "nonexistent", 1)
	if err == nil {
		t.Fatal("CancelOrder() expected error for nonexistent order, got nil")
	}
}

// ---------------------------------------------------------------------------
// Payment lifecycle: placing an order and applying channel callbacks
// ---------------------------------------------------------------------------

func TestService_PlaceOrder_RequestsATrade(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "order-1"},
		UOW:   testUnitOfWork{orderRepo: repo, productRepo: &mockProductRepo{products: []*productdomain.Product{sellableProduct("plan-1", "plan-1", 10, "USD")}}},
	}
	stubCreatePayment(t, func(context.Context, string, float64, string, string, string, string) (string, string, error) {
		return "https://pay.example.com/trade-1", "trade-1", nil
	})

	detail, err := svc.PlaceOrder(context.Background(), CreateOrderCommand{
		OrganizationID: "org-1", Name: "order-1", User: "user-1",
		Products: []string{"plan-1"}, Price: 10, Currency: "USD",
	})
	if err != nil {
		t.Fatalf("PlaceOrder() error = %v", err)
	}
	if detail.State != domain.StatePaymentPending {
		t.Errorf("State = %q, want %q", detail.State, domain.StatePaymentPending)
	}
	if !domain.IsAwaitingPayment(detail.State) {
		t.Error("a placed order must be reported as awaiting payment so the client keeps polling")
	}
	if len(detail.ProductInfos) != 1 || detail.ProductInfos[0].Name != "plan-1" {
		t.Errorf("ProductInfos = %+v, want the catalog snapshot of the bought product", detail.ProductInfos)
	}
}

// TestService_PlaceOrder_RejectsAClientPriceThatDisagreesWithTheCatalog pins the
// money rule: the catalog decides what a basket costs, so a caller that claims a
// different price is refused rather than discounted.
func TestService_PlaceOrder_RejectsAClientPriceThatDisagreesWithTheCatalog(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "order-1"},
		UOW: testUnitOfWork{orderRepo: repo,
			productRepo: &mockProductRepo{products: []*productdomain.Product{sellableProduct("plan-1", "plan-1", 999, "USD")}}},
	}
	stubCreatePayment(t, func(context.Context, string, float64, string, string, string, string) (string, string, error) {
		return "https://pay.example.com/trade-1", "trade-1", nil
	})

	_, err := svc.PlaceOrder(context.Background(), CreateOrderCommand{
		OrganizationID: "org-1", Name: "order-1", Products: []string{"plan-1"}, Price: 0.01,
	})
	if err == nil {
		t.Fatal("PlaceOrder() error = nil, want the catalog price to win over the claimed one")
	}
	if len(repo.orders) != 0 {
		t.Errorf("orders = %d, want no order written for a tampered price", len(repo.orders))
	}
}

// TestService_PlaceOrder_PricesAndNamesAnUnlabelledBasket pins that a
// buyer-driven checkout neither sends money nor invents a business key: the
// server reads the price from the catalog and derives a unique name.
func TestService_PlaceOrder_PricesAndNamesAnUnlabelledBasket(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "0190f0aa-bbbb-cccc-dddd-eeeeffff0000"},
		UOW: testUnitOfWork{orderRepo: repo,
			productRepo: &mockProductRepo{products: []*productdomain.Product{sellableProduct("prod-1", "pro-license", 25, "EUR")}}},
	}
	stubCreatePayment(t, func(ctx context.Context, _ string, amount float64, currency string, _ string, _ string, _ string) (string, string, error) {
		if amount != 25 || currency != "EUR" {
			t.Errorf("channel asked for %g %s, want 25 EUR read from the catalog", amount, currency)
		}
		return "https://pay.example.com/trade-1", "trade-1", nil
	})

	detail, err := svc.PlaceOrder(context.Background(), CreateOrderCommand{
		OrganizationID: "org-1", Products: []string{"prod-1"},
	})
	if err != nil {
		t.Fatalf("PlaceOrder() error = %v", err)
	}
	if detail.Price != 25 || detail.Currency != "EUR" {
		t.Errorf("Price/Currency = %g %s, want 25 EUR from the catalog", detail.Price, detail.Currency)
	}
	if detail.Name != "pro-license-0190f0aa" {
		t.Errorf("Name = %q, want a name derived from the product and the order id", detail.Name)
	}
	if detail.DisplayName != "pro-license" {
		t.Errorf("DisplayName = %q, want the product's display name", detail.DisplayName)
	}
}

// TestService_PlaceOrder_RefusesAnUnsellableProduct pins that buying is not a
// partial operation: a basket containing a product that is not `active` fails
// whole.
func TestService_PlaceOrder_RefusesAnUnsellableProduct(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{}
	draft := sellableProduct("prod-draft", "prod-draft", 5, "USD")
	draft.State = productdomain.StateDraft
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "order-1"},
		UOW: testUnitOfWork{orderRepo: repo,
			productRepo: &mockProductRepo{products: []*productdomain.Product{sellableProduct("prod-1", "prod-1", 10, "USD"), draft}}},
	}
	stubCreatePayment(t, func(context.Context, string, float64, string, string, string, string) (string, string, error) {
		return "https://pay.example.com/trade-1", "trade-1", nil
	})

	_, err := svc.PlaceOrder(context.Background(), CreateOrderCommand{
		OrganizationID: "org-1", Name: "order-1", Products: []string{"prod-1", "prod-draft"},
	})
	if err == nil {
		t.Fatal("PlaceOrder() error = nil, want a not-for-sale rejection")
	}
	if len(repo.orders) != 0 {
		t.Errorf("orders = %d, want no order written for an unsellable basket", len(repo.orders))
	}
}

// TestService_PlaceOrder_GatewayDownStaysCreated pins that an order whose trade
// was never created is not left looking payable-but-broken: it stays `created`
// (the only reachable state) and carries the reason.
func TestService_PlaceOrder_GatewayDownStaysCreated(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "order-1"},
		UOW: testUnitOfWork{orderRepo: repo,
			productRepo: &mockProductRepo{products: []*productdomain.Product{sellableProduct("plan-1", "plan-1", 10, "USD")}}},
	}
	stubCreatePayment(t, func(context.Context, string, float64, string, string, string, string) (string, string, error) {
		return "", "", apperrors.NewUnavailable("channel down")
	})

	detail, err := svc.PlaceOrder(context.Background(), CreateOrderCommand{
		OrganizationID: "org-1", Name: "order-1", User: "user-1",
		Products: []string{"plan-1"}, Price: 10, Currency: "USD",
	})
	if err != nil {
		t.Fatalf("PlaceOrder() error = %v", err)
	}
	if detail.State != domain.StateCreated {
		t.Errorf("State = %q, want %q", detail.State, domain.StateCreated)
	}
	if detail.Message == "" {
		t.Error("Message = empty, want the channel failure recorded")
	}
	if domain.IsTerminal(detail.State) {
		t.Error("a failed trade request must not be reported as terminal")
	}
}

// TestService_PlaceOrder_RequiresAProduct pins the create-path validation that
// precedes any channel call.
func TestService_PlaceOrder_RequiresAProduct(t *testing.T) {
	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: &fakeUUIDGenerator{fixedID: "order-1"}}
	_, err := svc.PlaceOrder(context.Background(), CreateOrderCommand{
		OrganizationID: "org-1", Name: "order-1",
	})
	if err == nil {
		t.Fatal("PlaceOrder() error = nil, want a validation error for an empty product list")
	}
}

func TestService_NotifyOrderPayment_ConfirmsPayment(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", State: domain.StatePaymentPending, TradeNo: "trade-1", Price: 10, Currency: "USD", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}

	if err := svc.NotifyOrderPayment(context.Background(), "trade-1", "alipay", "TRADE_SUCCESS", 10, "USD"); err != nil {
		t.Fatalf("NotifyOrderPayment() error = %v", err)
	}
	o := repo.orders[0]
	if o.State != domain.StatePaid {
		t.Errorf("State = %q, want %q", o.State, domain.StatePaid)
	}
	if o.PaidAt == nil {
		t.Error("PaidAt = nil, want the callback's settlement instant recorded")
	}
}

// TestService_NotifyOrderPayment_RejectsAmountMismatch pins the amount binding:
// a verified callback that confirms a different amount than the order was
// created with must not transition the order.
func TestService_NotifyOrderPayment_RejectsAmountMismatch(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", State: domain.StatePaymentPending, TradeNo: "trade-1", Price: 10, Currency: "USD", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}

	if err := svc.NotifyOrderPayment(context.Background(), "trade-1", "alipay", "TRADE_SUCCESS", 1, "USD"); err == nil {
		t.Fatal("NotifyOrderPayment() error = nil, want an error for an amount mismatch")
	}
	if repo.orders[0].State != domain.StatePaymentPending {
		t.Errorf("State = %q, want it untouched at %q", repo.orders[0].State, domain.StatePaymentPending)
	}
}

// TestService_NotifyOrderPayment_RejectsCurrencyMismatch pins the currency binding.
func TestService_NotifyOrderPayment_RejectsCurrencyMismatch(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", State: domain.StatePaymentPending, TradeNo: "trade-1", Price: 10, Currency: "USD", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}

	if err := svc.NotifyOrderPayment(context.Background(), "trade-1", "alipay", "TRADE_SUCCESS", 10, "EUR"); err == nil {
		t.Fatal("NotifyOrderPayment() error = nil, want an error for a currency mismatch")
	}
	if repo.orders[0].State != domain.StatePaymentPending {
		t.Errorf("State = %q, want it untouched at %q", repo.orders[0].State, domain.StatePaymentPending)
	}
}

// TestService_NotifyOrderPayment_RejectsSuccessWithoutAmount pins the fail-closed
// rule: a payment confirmation that carries no verifiable amount cannot be bound
// to the order and must not transition it.
func TestService_NotifyOrderPayment_RejectsSuccessWithoutAmount(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", State: domain.StatePaymentPending, TradeNo: "trade-1", Price: 10, Currency: "USD", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}

	if err := svc.NotifyOrderPayment(context.Background(), "trade-1", "alipay", "TRADE_SUCCESS", 0, ""); err == nil {
		t.Fatal("NotifyOrderPayment() error = nil, want an error for a confirmation without an amount")
	}
	if repo.orders[0].State != domain.StatePaymentPending {
		t.Errorf("State = %q, want it untouched at %q", repo.orders[0].State, domain.StatePaymentPending)
	}
}

// TestService_NotifyOrderPayment_DuplicateEventIdempotent pins event-level
// idempotency: re-delivering the same verified event must not transition the
// order twice.
func TestService_NotifyOrderPayment_DuplicateEventIdempotent(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", State: domain.StatePaymentPending, TradeNo: "trade-1", Price: 10, Currency: "USD", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}

	for i := 0; i < 2; i++ {
		if err := svc.NotifyOrderPayment(context.Background(), "trade-1", "alipay", "TRADE_SUCCESS", 10, "USD"); err != nil {
			t.Fatalf("NotifyOrderPayment() round %d error = %v", i+1, err)
		}
	}
	o := repo.orders[0]
	if o.State != domain.StatePaid {
		t.Errorf("State = %q, want %q", o.State, domain.StatePaid)
	}
	if o.Revision != 2 {
		t.Errorf("Revision = %d, want exactly one transition (2)", o.Revision)
	}
}

// TestService_NotifyOrderPayment_IgnoresUnknownStatus pins that an unparsed
// third-party string is not a fact about the order.
func TestService_NotifyOrderPayment_IgnoresUnknownStatus(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", State: domain.StatePaymentPending, TradeNo: "trade-1", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}

	if err := svc.NotifyOrderPayment(context.Background(), "trade-1", "alipay", "SOMETHING_NEW", 0, ""); err != nil {
		t.Fatalf("NotifyOrderPayment() error = %v", err)
	}
	if repo.orders[0].State != domain.StatePaymentPending {
		t.Errorf("State = %q, want it untouched at %q", repo.orders[0].State, domain.StatePaymentPending)
	}
	if repo.orders[0].Revision != 1 {
		t.Errorf("Revision = %d, want it untouched at 1", repo.orders[0].Revision)
	}
}

// TestService_NotifyOrderPayment_IgnoresDuplicateCallback covers callback
// re-delivery, which every gateway performs until it receives an ack.
func TestService_NotifyOrderPayment_IgnoresDuplicateCallback(t *testing.T) {
	now := time.Now()
	paidAt := now.Add(-time.Minute)
	repo := &mockOrderRepo{orders: []*domain.Order{
		{
			ID: "order-1", State: domain.StatePaid, TradeNo: "trade-1",
			PaidAt: &paidAt, Revision: 4, CreatedAt: now, UpdatedAt: now,
		},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}

	if err := svc.NotifyOrderPayment(context.Background(), "trade-1", "alipay", "TRADE_SUCCESS", 0, ""); err != nil {
		t.Fatalf("NotifyOrderPayment() error = %v", err)
	}
	o := repo.orders[0]
	if o.Revision != 4 {
		t.Errorf("Revision = %d, want it untouched at 4", o.Revision)
	}
	if !o.PaidAt.Equal(paidAt) {
		t.Errorf("PaidAt = %v, want the original settlement instant %v", o.PaidAt, paidAt)
	}
}

// TestService_NotifyOrderPayment_IgnoresOutOfOrderVerdict pins that a verdict
// whose target state is unreachable is a no-op instead of an error: the gateway
// would otherwise retry the callback forever.
func TestService_NotifyOrderPayment_IgnoresOutOfOrderVerdict(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{orders: []*domain.Order{
		{ID: "order-1", State: domain.StateCompleted, TradeNo: "trade-1", Revision: 3, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}

	if err := svc.NotifyOrderPayment(context.Background(), "trade-1", "alipay", "WAIT_BUYER_PAY", 0, ""); err != nil {
		t.Fatalf("NotifyOrderPayment() error = %v", err)
	}
	if repo.orders[0].State != domain.StateCompleted {
		t.Errorf("State = %q, want it untouched at %q", repo.orders[0].State, domain.StateCompleted)
	}
}

func TestService_NotifyOrderPayment_UnknownTrade(t *testing.T) {
	now := time.Now()
	repo := &mockOrderRepo{}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{orderRepo: repo}}

	if err := svc.NotifyOrderPayment(context.Background(), "trade-missing", "alipay", "TRADE_SUCCESS", 0, ""); err == nil {
		t.Fatal("NotifyOrderPayment() error = nil, want a not-found error for an unknown trade")
	}
}
