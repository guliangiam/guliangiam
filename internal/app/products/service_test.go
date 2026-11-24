package products

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

type mockProductRepo struct{ products []*productdomain.Product }

func (r *mockProductRepo) GetByID(_ context.Context, id string) (*productdomain.Product, error) {
	return r.find(id, "")
}

func (r *mockProductRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*productdomain.Product, error) {
	return r.find(id, organizationID)
}

func (r *mockProductRepo) find(id, organizationID string) (*productdomain.Product, error) {
	for _, item := range r.products {
		if item.ID != id {
			continue
		}
		if organizationID == "" || item.OrganizationID == organizationID {
			return item, nil
		}
	}
	return nil, apperrors.NewNotFound("product not found")
}
func (r *mockProductRepo) List(_ context.Context, _ productdomain.ListQuery) ([]*productdomain.Product, int64, error) {
	return r.products, int64(len(r.products)), nil
}
func (r *mockProductRepo) Create(_ context.Context, item *productdomain.Product) error {
	r.products = append(r.products, item)
	return nil
}
func (r *mockProductRepo) Update(_ context.Context, item *productdomain.Product) error {
	for i, x := range r.products {
		if x.ID == item.ID {
			r.products[i] = item
			return nil
		}
	}
	return apperrors.NewNotFound("product not found")
}
func (r *mockProductRepo) Delete(_ context.Context, id, organizationID string) error {
	for i, x := range r.products {
		if x.ID == id && x.OrganizationID == organizationID {
			r.products = append(r.products[:i], r.products[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("product not found")
}

type testUnitOfWork struct{ productRepo *mockProductRepo }

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
func (u testUnitOfWork) Coupons() coupondomain.Repository                                 { return nil }
func (u testUnitOfWork) CouponUsages() coupondomain.CouponUsageRepository                 { return nil }
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

func newSvc(repo *mockProductRepo) Service {
	return Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: fakeUUIDGenerator{fixedID: "test-id"}, UOW: testUnitOfWork{productRepo: repo}}
}

func TestCreateProduct_Validation(t *testing.T) {
	svc := newSvc(&mockProductRepo{})
	tests := []struct {
		name    string
		cmd     CreateProductCommand
		wantErr bool
	}{
		{name: "valid", cmd: CreateProductCommand{OrganizationID: "org-1", Name: "p-1"}, wantErr: false},
		{name: "missing orgId", cmd: CreateProductCommand{Name: "x"}, wantErr: true},
		{name: "missing name", cmd: CreateProductCommand{OrganizationID: "org-1"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateProduct(context.Background(), tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateProduct_Success(t *testing.T) {
	repo := &mockProductRepo{}
	svc := newSvc(repo)
	err := svc.CreateProduct(context.Background(), CreateProductCommand{OrganizationID: "org-1", Name: "p-1", DisplayName: "Test"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(repo.products) != 1 {
		t.Fatalf("expected 1, got %d", len(repo.products))
	}
	if repo.products[0].ID != "test-id" {
		t.Errorf("ID = %q", repo.products[0].ID)
	}
	if repo.products[0].Revision != 1 {
		t.Errorf("Revision = %d", repo.products[0].Revision)
	}
}

func TestGetProduct(t *testing.T) {
	now := time.Now()
	repo := &mockProductRepo{products: []*productdomain.Product{{ID: "x-1", OrganizationID: "org-1", Name: "test", CreatedAt: now, UpdatedAt: now}}}
	svc := newSvc(repo)
	d, err := svc.GetProduct(context.Background(), "org-1", "x-1")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if d.ID != "x-1" {
		t.Errorf("ID = %q", d.ID)
	}
}

func TestListProducts(t *testing.T) {
	now := time.Now()
	repo := &mockProductRepo{products: []*productdomain.Product{
		{ID: "x-1", CreatedAt: now, UpdatedAt: now},
		{ID: "x-2", CreatedAt: now, UpdatedAt: now},
	}}
	svc := newSvc(repo)
	items, total, err := svc.ListProducts(context.Background(), ListProductsQuery{Page: 1, PageSize: 10})
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

func TestDeleteProduct(t *testing.T) {
	repo := &mockProductRepo{products: []*productdomain.Product{{ID: "x-1", OrganizationID: "org-1"}}}
	svc := newSvc(repo)
	if err := svc.DeleteProduct(context.Background(), "org-1", "x-1"); err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(repo.products) != 0 {
		t.Errorf("expected 0, got %d", len(repo.products))
	}
}

func TestPatchProduct_RevisionMismatch(t *testing.T) {
	repo := &mockProductRepo{products: []*productdomain.Product{{ID: "x-1", OrganizationID: "org-1", Revision: 2}}}
	svc := newSvc(repo)
	err := svc.PatchProduct(context.Background(), "org-1", PatchProductCommand{ID: "x-1", Revision: 1})
	if err == nil {
		t.Error("expected error")
	}
}

func TestPatchProduct_Success(t *testing.T) {
	now := time.Now()
	repo := &mockProductRepo{products: []*productdomain.Product{{ID: "x-1", OrganizationID: "org-1", Name: "test", Revision: 1, CreatedAt: now, UpdatedAt: now}}}
	svc := newSvc(repo)
	displayName := "Updated"
	err := svc.PatchProduct(context.Background(), "org-1", PatchProductCommand{ID: "x-1", DisplayName: &displayName, Revision: 1})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if repo.products[0].DisplayName != "Updated" {
		t.Errorf("DisplayName = %q", repo.products[0].DisplayName)
	}
	if repo.products[0].Revision != 2 {
		t.Errorf("Revision = %d", repo.products[0].Revision)
	}
}
func TestBuyProduct_NotAvailable(t *testing.T) {
	now := time.Now()
	repo := &mockProductRepo{products: []*productdomain.Product{{ID: "p-1", OrganizationID: "org-1", State: productdomain.StateDraft, CreatedAt: now, UpdatedAt: now}}}
	svc := newSvc(repo)
	_, err := svc.BuyProduct(context.Background(), "org-1", "p-1", "user", "", "", 1)
	if err == nil {
		t.Error("expected error for a product that is not active")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeUnprocessableEntity {
		t.Errorf("error code = %q, want %q", code, apperrors.CodeUnprocessableEntity)
	}
}

func TestBuyProduct_InsufficientQuantity(t *testing.T) {
	now := time.Now()
	repo := &mockProductRepo{products: []*productdomain.Product{{
		ID: "p-1", OrganizationID: "org-1", State: productdomain.StateActive, Quantity: 5, Sold: 4, CreatedAt: now, UpdatedAt: now,
	}}}
	svc := newSvc(repo)
	_, err := svc.BuyProduct(context.Background(), "org-1", "p-1", "user", "", "", 2)
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("error code = %q, want 422", appErr.Code)
	}
	if appErr.Meta["field"] != "quantity" || appErr.Meta["max"] != 1 {
		t.Fatalf("meta = %v, want field=quantity max=1", appErr.Meta)
	}
}
func TestBuyProduct_Active(t *testing.T) {
	now := time.Now()
	repo := &mockProductRepo{products: []*productdomain.Product{{ID: "p-1", OrganizationID: "org-1", State: productdomain.StateActive, CreatedAt: now, UpdatedAt: now}}}
	svc := newSvc(repo)
	d, err := svc.BuyProduct(context.Background(), "org-1", "p-1", "user", "", "", 1)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if d.ID != "p-1" {
		t.Errorf("ID = %q", d.ID)
	}
}
