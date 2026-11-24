package organizations

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
	tenantidentities "github.com/guliangiam/guliangiam/internal/domain/tenant_identities"
	tenantinstances "github.com/guliangiam/guliangiam/internal/domain/tenant_instances"
	"github.com/guliangiam/guliangiam/internal/domain/tenants"
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
// Fakes: Clock, UUID Generator
// ---------------------------------------------------------------------------

type fakeClock struct {
	fixedTime time.Time
}

func (f fakeClock) Now() time.Time { return f.fixedTime }

type fakeUUIDGenerator struct {
	fixedID string
}

func (f fakeUUIDGenerator) NewString() (string, error) { return f.fixedID, nil }

// ---------------------------------------------------------------------------
// Stub UOW: delegates Organizations() to in-memory repository, all others nil
// ---------------------------------------------------------------------------

type testUnitOfWork struct {
	orgRepo    *testOrganizationRepository
	tenantRepo *testTenantRepository
	records    recordsdomain.Repository
}

// fakeRecordsRepo captures audit rows written inside the transaction. Only
// Create is exercised; the embedded interface keeps the chain methods out of
// scope.
type fakeRecordsRepo struct {
	recordsdomain.Repository

	created []*recordsdomain.Record
}

func (r *fakeRecordsRepo) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

func (u testUnitOfWork) Applications() applications.Repository { return nil }
func (u testUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository {
	return nil
}
func (u testUnitOfWork) Consents() consentdomain.Repository           { return nil }
func (u testUnitOfWork) Gateway() gatewaydomain.Repository            { return nil }
func (u testUnitOfWork) Groups() groupsdomain.Repository              { return nil }
func (u testUnitOfWork) Invitations() invitationsdomain.Repository    { return nil }
func (u testUnitOfWork) MfaChallenges() mfachallengedomain.Repository { return nil }
func (u testUnitOfWork) Organizations() orgdomain.Repository          { return u.orgRepo }
func (u testUnitOfWork) Permissions() permissiondomain.Repository     { return nil }
func (u testUnitOfWork) Providers() providerdomain.Repository         { return nil }
func (u testUnitOfWork) Certs() certdomain.Repository                 { return nil }
func (u testUnitOfWork) CaptchaProviders() captchaproviderdomain.Repository {
	return nil
}
func (u testUnitOfWork) NotificationProviders() notificationproviderdomain.Repository {
	return nil
}
func (u testUnitOfWork) Notifications() notificationsdomain.Repository { return nil }
func (u testUnitOfWork) Ldaps() ldapdomain.Repository                  { return nil }
func (u testUnitOfWork) Models() modeldomain.Repository                { return nil }
func (u testUnitOfWork) Adapters() adapterdomain.Repository            { return nil }
func (u testUnitOfWork) Agents() agentdomain.Repository                { return nil }
func (u testUnitOfWork) Enforcers() enforcerdomain.Repository          { return testEnforcerRepository{} }
func (u testUnitOfWork) Forms() formdomain.Repository                  { return nil }
func (u testUnitOfWork) Orders() orderdomain.Repository                { return nil }
func (u testUnitOfWork) Payments() paymentdomain.Repository            { return nil }
func (u testUnitOfWork) Plans() plandomain.Repository                  { return nil }

func (u testUnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository           { return nil }
func (u testUnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository { return nil }
func (u testUnitOfWork) CrossTenantOps() crosstenantops.TicketRepository                  { return nil }
func (u testUnitOfWork) Pricings() pricingdomain.Repository                               { return nil }
func (u testUnitOfWork) Products() productdomain.Repository                               { return nil }
func (u testUnitOfWork) Records() recordsdomain.Repository                                { return u.records }
func (u testUnitOfWork) Resources() resourcedomain.Repository                             { return nil }
func (u testUnitOfWork) Roles() roledomain.Repository                                     { return nil }
func (u testUnitOfWork) Syncers() syncerdomain.Repository                                 { return nil }
func (u testUnitOfWork) Sessions() sessiondomain.Repository                               { return nil }
func (u testUnitOfWork) Subscriptions() subscriptiondomain.Repository {
	return nil
}
func (u testUnitOfWork) Tickets() ticketdomain.Repository   { return nil }
func (u testUnitOfWork) Tokens() tokendomain.Repository     { return nil }
func (u testUnitOfWork) Webhooks() webhookdomain.Repository { return nil }
func (u testUnitOfWork) Users() userdomain.Repository       { return nil }
func (u testUnitOfWork) Coupons() coupondomain.Repository   { return nil }
func (u testUnitOfWork) CouponUsages() coupondomain.CouponUsageRepository {
	return nil
}
func (u testUnitOfWork) Entries() entrydomain.Repository        { return nil }
func (u testUnitOfWork) McpServers() mcpserverdomain.Repository { return nil }
func (u testUnitOfWork) RadiusAccountings() radiusaccountingdomain.Repository {
	return nil
}
func (u testUnitOfWork) Verifications() verificationsdomain.Repository {
	return nil
}
func (u testUnitOfWork) WebAuthnCredentials() webauthndomain.Repository {
	return nil
}
func (u testUnitOfWork) FaceIdProviders() faceidproviderdomain.Repository {
	return nil
}
func (u testUnitOfWork) RadiusConfigs() radiusconfigdomain.Repository {
	return nil
}
func (u testUnitOfWork) Rules() ruledomain.Repository               { return nil }
func (u testUnitOfWork) Transactions() transactiondomain.Repository { return nil }
func (u testUnitOfWork) WebhookEvents() webhookeventdomain.Repository {
	return nil
}
func (u testUnitOfWork) Keys() keysdomain.Repository                 { return nil }
func (u testUnitOfWork) QRCodeSessions() qrcodesessions.Repository   { return nil }
func (u testUnitOfWork) DeviceAuthCodes() deviceauthcodes.Repository { return nil }

func (u testUnitOfWork) Web3Identities() web3identity.Repository     { return nil }
func (u testUnitOfWork) FaceIDSessions() faceidsessions.Repository   { return nil }
func (u testUnitOfWork) CASTickets() castickets.Repository           { return nil }
func (u testUnitOfWork) KerberosTickets() kerberostickets.Repository { return nil }
func (u testUnitOfWork) LockoutAttempts() lockoutattempts.Repository { return nil }

func (u testUnitOfWork) IDV() idvdomain.Repository { return nil }

func (u testUnitOfWork) Workload() workloaddomain.Repository { return nil }

func (u testUnitOfWork) Rebac() rebac.Repository { return nil }

func (u testUnitOfWork) Risk() riskdomain.Repository { return nil }

func (u testUnitOfWork) Actions() actionsdomain.Repository { return nil }

func (u testUnitOfWork) Metering() meteringdomain.Repository { return nil }

func (u testUnitOfWork) Outbox() domainoutbox.Repository { return nil }

func (u testUnitOfWork) Tenants() tenants.Repository { return u.tenantRepo }

func (u testUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	orgSnapshot := u.orgRepo.snapshot()
	tenantSnapshot := u.tenantRepo.snapshot()
	err := fn(ctx, u)
	if err != nil {
		u.orgRepo.restore(orgSnapshot)
		u.tenantRepo.restore(tenantSnapshot)
	}
	return err
}
func (u testUnitOfWork) IGA() igadomain.Repository { return nil }

// ---------------------------------------------------------------------------
// Stub Organization Repository: in-memory CRUD
// ---------------------------------------------------------------------------

type testOrganizationRepository struct {
	items    map[string]*orgdomain.Organization
	slugs    map[string]string // slug -> id
	settings *orgdomain.OrganizationSettings
	deleted  map[string]bool
}

func newTestOrganizationRepository() *testOrganizationRepository {
	return &testOrganizationRepository{
		items:   make(map[string]*orgdomain.Organization),
		slugs:   make(map[string]string),
		deleted: make(map[string]bool),
	}
}

func (r *testOrganizationRepository) snapshot() map[string]*orgdomain.Organization {
	out := make(map[string]*orgdomain.Organization, len(r.items))
	for k, v := range r.items {
		out[k] = v
	}
	return out
}

func (r *testOrganizationRepository) restore(snapshot map[string]*orgdomain.Organization) {
	r.items = snapshot
	r.slugs = make(map[string]string, len(snapshot))
	for id, org := range snapshot {
		r.slugs[org.Slug] = id
	}
	r.deleted = make(map[string]bool)
}

func (r *testOrganizationRepository) GetByID(_ context.Context, id string) (*orgdomain.Organization, error) {
	org, ok := r.items[id]
	if !ok {
		return nil, apperrors.NewNotFound("organization not found")
	}
	return org, nil
}

func (r *testOrganizationRepository) GetBySlug(_ context.Context, slug string) (*orgdomain.Organization, error) {
	id, ok := r.slugs[slug]
	if !ok {
		return nil, apperrors.NewNotFound("organization not found")
	}
	return r.items[id], nil
}

// GetBySlugPublic delegates to GetBySlug (tests don't need RLS bypass).
func (r *testOrganizationRepository) GetBySlugPublic(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return r.GetBySlug(ctx, slug)
}

func (r *testOrganizationRepository) List(_ context.Context, query orgdomain.ListQuery) ([]*orgdomain.Organization, int64, error) {
	all := make([]*orgdomain.Organization, 0, len(r.items))
	for _, org := range r.items {
		all = append(all, org)
	}

	total := int64(len(all))
	start := (query.Page - 1) * query.PageSize
	if start >= int(total) {
		return []*orgdomain.Organization{}, total, nil
	}
	end := start + query.PageSize
	if end > int(total) {
		end = int(total)
	}

	return all[start:end], total, nil
}

func (r *testOrganizationRepository) Create(_ context.Context, org *orgdomain.Organization) error {
	if _, exists := r.slugs[org.Slug]; exists {
		return apperrors.NewAlreadyExists("organization slug already exists")
	}
	r.items[org.ID] = org
	r.slugs[org.Slug] = org.ID
	return nil
}

func (r *testOrganizationRepository) Update(_ context.Context, org *orgdomain.Organization) error {
	if _, ok := r.items[org.ID]; !ok {
		return apperrors.NewNotFound("organization not found")
	}
	// If slug changed, update slug index
	for slug, id := range r.slugs {
		if id == org.ID && slug != org.Slug {
			delete(r.slugs, slug)
			r.slugs[org.Slug] = org.ID
		}
	}
	r.items[org.ID] = org
	return nil
}

func (r *testOrganizationRepository) UpdateSettings(_ context.Context, settings *orgdomain.OrganizationSettings) error {
	settingsCopy := *settings
	r.settings = &settingsCopy
	return nil
}

func (r *testOrganizationRepository) UpdateTheme(_ context.Context, _ *orgdomain.OrganizationThemeSettings) error {
	return nil
}

func (r *testOrganizationRepository) UpdateSecurityPolicy(_ context.Context, _ *orgdomain.OrganizationSecurityPolicy) error {
	return nil
}

func (r *testOrganizationRepository) Delete(_ context.Context, id string) error {
	if _, ok := r.items[id]; !ok {
		return apperrors.NewNotFound("organization not found")
	}
	r.deleted[id] = true
	return nil
}

func (r *testOrganizationRepository) HasDeletedWithSlug(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// ---------------------------------------------------------------------------
// Helper: build a Service wired to fakes
// ---------------------------------------------------------------------------

func newTestService() (Service, *testOrganizationRepository) {
	repo := newTestOrganizationRepository()
	tenantRepo := newTestTenantRepository()
	uow := testUnitOfWork{orgRepo: repo, tenantRepo: tenantRepo}
	svc := NewService(
		fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		fakeUUIDGenerator{fixedID: "019abc-test-uuid-0001"},
		uow,
	)
	return svc, repo
}

func newTestServiceWithRecords() (Service, *testOrganizationRepository, *fakeRecordsRepo) {
	repo := newTestOrganizationRepository()
	tenantRepo := newTestTenantRepository()
	recordsRepo := &fakeRecordsRepo{}
	uow := testUnitOfWork{orgRepo: repo, tenantRepo: tenantRepo, records: recordsRepo}
	svc := NewService(
		fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		fakeUUIDGenerator{fixedID: "019abc-test-uuid-0001"},
		uow,
	)
	return svc, repo, recordsRepo
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestCreateOrganization_Success(t *testing.T) {
	svc, repo, recordsRepo := newTestServiceWithRecords()
	ctx := context.Background()

	const actorID = "019operator-0000-7000-8000-000000000002"
	cmd := CreateOrganizationCommand{
		Slug:                 "guliang",
		ActorID:              actorID,
		DisplayName:          "GuliangTech",
		WebsiteURL:           "https://guliang.tech",
		Status:               "active",
		Languages:            []string{"en", "zh-CN"},
		InitScore:            100,
		DefaultApplicationID: "app-1",
	}

	org, err := svc.CreateOrganization(ctx, cmd)
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v, want nil", err)
	}

	// Verify returned organization has correct fields
	if org.ID != "019abc-test-uuid-0001" {
		t.Errorf("org.ID = %q, want %q", org.ID, "019abc-test-uuid-0001")
	}
	if org.Slug != "guliang" {
		t.Errorf("org.Slug = %q, want %q", org.Slug, "guliang")
	}
	if org.DisplayName != "GuliangTech" {
		t.Errorf("org.DisplayName = %q, want %q", org.DisplayName, "GuliangTech")
	}
	if org.WebsiteURL != "https://guliang.tech" {
		t.Errorf("org.WebsiteURL = %q, want %q", org.WebsiteURL, "https://guliang.tech")
	}
	if org.Status != "active" {
		t.Errorf("org.Status = %q, want %q", org.Status, "active")
	}
	if org.InitScore != 100 {
		t.Errorf("org.InitScore = %d, want %d", org.InitScore, 100)
	}
	if org.Settings.DefaultApplicationID != "app-1" {
		t.Errorf("org.Settings.DefaultApplicationID = %q, want %q", org.Settings.DefaultApplicationID, "app-1")
	}
	if repo.settings == nil || repo.settings.DefaultApplicationID != "app-1" {
		t.Errorf("persisted default application = %v, want %q", repo.settings, "app-1")
	}
	if repo.settings == nil || len(repo.settings.Languages) != 2 || repo.settings.Languages[0] != "en" || repo.settings.Languages[1] != "zh-CN" {
		t.Errorf("persisted languages = %v, want [en zh-CN]", repo.settings)
	}
	if org.Revision != 1 {
		t.Errorf("org.Revision = %d, want 1", org.Revision)
	}
	if org.CreatedAt.IsZero() {
		t.Error("org.CreatedAt is zero, want non-zero")
	}

	// Verify organization was persisted in the repository
	persisted, err := repo.GetByID(ctx, org.ID)
	if err != nil {
		t.Fatalf("repo.GetByID() error = %v, want nil", err)
	}
	if persisted.Slug != org.Slug {
		t.Errorf("persisted.Slug = %q, want %q", persisted.Slug, org.Slug)
	}

	// Verify the audit row carries the operator as actor.
	if len(recordsRepo.created) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(recordsRepo.created))
	}
	record := recordsRepo.created[0]
	if record.Action != "organization.created" {
		t.Errorf("audit action = %q, want %q", record.Action, "organization.created")
	}
	if record.ActorID != actorID {
		t.Errorf("audit actorID = %q, want %q", record.ActorID, actorID)
	}
}

func TestCreateOrganization_DuplicateSlug(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	cmd := CreateOrganizationCommand{
		Slug:        "guliang",
		DisplayName: "GuliangTech",
		Status:      "active",
	}

	// First creation should succeed
	_, err := svc.CreateOrganization(ctx, cmd)
	if err != nil {
		t.Fatalf("first CreateOrganization() error = %v, want nil", err)
	}

	// Second creation with the same slug should return conflict
	cmd2 := CreateOrganizationCommand{
		Slug:        "guliang",
		DisplayName: "GuliangTech Duplicate",
		Status:      "active",
	}
	_, err = svc.CreateOrganization(ctx, cmd2)
	if err == nil {
		t.Fatal("second CreateOrganization() error = nil, want non-nil")
	}

	appErr, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if appErr.Code != apperrors.CodeAlreadyExists {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeAlreadyExists)
	}
}

func TestGetOrganization_Success(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	// Seed an organization via Create
	cmd := CreateOrganizationCommand{
		Slug:        "guliang",
		DisplayName: "GuliangTech",
		WebsiteURL:  "https://guliang.tech",
		Status:      "active",
	}
	created, err := svc.CreateOrganization(ctx, cmd)
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v, want nil", err)
	}

	// Retrieve by ID
	detail, err := svc.GetOrganization(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetOrganization() error = %v, want nil", err)
	}

	if detail.ID != created.ID {
		t.Errorf("detail.ID = %q, want %q", detail.ID, created.ID)
	}
	if detail.Slug != "guliang" {
		t.Errorf("detail.Slug = %q, want %q", detail.Slug, "guliang")
	}
	if detail.DisplayName != "GuliangTech" {
		t.Errorf("detail.DisplayName = %q, want %q", detail.DisplayName, "GuliangTech")
	}
	if detail.Status != "active" {
		t.Errorf("detail.Status = %q, want %q", detail.Status, "active")
	}
	if detail.Revision != 1 {
		t.Errorf("detail.Revision = %d, want 1", detail.Revision)
	}
}

func TestGetOrganization_NotFound(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	_, err := svc.GetOrganization(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("GetOrganization() error = nil, want non-nil for nonexistent ID")
	}

	appErr, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if appErr.Code != apperrors.CodeNotFound {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeNotFound)
	}
}

func TestListOrganizations_Success(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	// Seed multiple organizations
	for i, cmd := range []CreateOrganizationCommand{
		{Slug: "org-alpha", DisplayName: "Alpha", Status: "active"},
		{Slug: "org-beta", DisplayName: "Beta", Status: "active"},
		{Slug: "org-gamma", DisplayName: "Gamma", Status: "inactive"},
	} {
		// Each call reuses the same fixed UUID; override for uniqueness
		svc.UUID = fakeUUIDGenerator{fixedID: "019abc-test-uuid-00" + string(rune('0'+i+1))}
		if _, err := svc.CreateOrganization(ctx, cmd); err != nil {
			t.Fatalf("CreateOrganization(%d) error = %v", i, err)
		}
	}

	// List with page size 2
	items, total, err := svc.ListOrganizations(ctx, ListOrganizationsQuery{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("ListOrganizations() error = %v, want nil", err)
	}

	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(items))
	}

	// Page 2 should have 1 item
	items2, total2, err := svc.ListOrganizations(ctx, ListOrganizationsQuery{Page: 2, PageSize: 2})
	if err != nil {
		t.Fatalf("ListOrganizations(page 2) error = %v, want nil", err)
	}
	if total2 != 3 {
		t.Errorf("total2 = %d, want 3", total2)
	}
	if len(items2) != 1 {
		t.Errorf("len(items2) = %d, want 1", len(items2))
	}
}

func TestDeleteOrganization_BuiltIn(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()

	// Seed the built-in organization
	builtinOrg := &orgdomain.Organization{
		ID:        "builtin-org-id",
		Slug:      "built-in",
		Status:    "active",
		CreatedAt: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC),
		Revision:  1,
	}
	repo.items[builtinOrg.ID] = builtinOrg
	repo.slugs[builtinOrg.Slug] = builtinOrg.ID

	err := svc.DeleteOrganization(ctx, DeleteOrganizationCommand{ID: builtinOrg.ID})
	if err == nil {
		t.Fatal("DeleteOrganization() expected error for built-in org, got nil")
	}

	appErr, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if appErr.Code != apperrors.CodeForbidden {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeForbidden)
	}
}

func TestDeleteOrganization_NotFound(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	err := svc.DeleteOrganization(ctx, DeleteOrganizationCommand{ID: "nonexistent-id"})
	if err == nil {
		t.Fatal("DeleteOrganization() expected error for nonexistent ID, got nil")
	}
}

func TestDeleteOrganization_EmptyID(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	err := svc.DeleteOrganization(ctx, DeleteOrganizationCommand{ID: ""})
	if err == nil {
		t.Fatal("DeleteOrganization() expected error for empty ID, got nil")
	}

	appErr, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeInvalidArgument)
	}
}

func TestUpdateOrganization_RevisionMismatch(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()

	created, err := svc.CreateOrganization(ctx, CreateOrganizationCommand{
		Slug:        "guliang",
		DisplayName: "GuliangTech",
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v, want nil", err)
	}

	// Update the revision in the repo to simulate a concurrent update
	persisted, _ := repo.GetByID(ctx, created.ID)
	// Modify the revision directly in the stored object
	persisted.Revision = 10

	// Use a stale revision (1) that doesn't match the current (10)
	staleRevision := int64(1)
	newDisplayName := "GuliangTech Updated"
	err = svc.PatchOrganization(ctx, PatchOrganizationCommand{
		ID:          created.ID,
		DisplayName: &newDisplayName,
		Revision:    staleRevision, // stale revision
	})
	if err == nil {
		t.Fatal("PatchOrganization() expected error for revision mismatch, got nil")
	}

	appErr, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if appErr.Code != apperrors.CodeConflict {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeConflict)
	}
}

func TestGetOrganizationBySlug_Success(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	cmd := CreateOrganizationCommand{
		Slug:        "guliang",
		DisplayName: "GuliangTech",
		Status:      "active",
	}
	_, err := svc.CreateOrganization(ctx, cmd)
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v, want nil", err)
	}

	detail, err := svc.GetOrganizationBySlug(ctx, "guliang")
	if err != nil {
		t.Fatalf("GetOrganizationBySlug() error = %v, want nil", err)
	}
	if detail.Slug != "guliang" {
		t.Errorf("detail.Slug = %q, want %q", detail.Slug, "guliang")
	}
}

func TestGetOrganizationBySlug_NotFound(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	_, err := svc.GetOrganizationBySlug(ctx, "nonexistent-slug")
	if err == nil {
		t.Fatal("GetOrganizationBySlug() error = nil, want non-nil for nonexistent slug")
	}
}

func TestUpdateOrganization_Success(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()

	// Seed an organization
	created, err := svc.CreateOrganization(ctx, CreateOrganizationCommand{
		Slug:        "guliang",
		DisplayName: "GuliangTech",
		WebsiteURL:  "https://guliang.tech",
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v, want nil", err)
	}

	// Patch display name and website
	newDisplayName := "GuliangTech Updated"
	newWebsiteURL := "https://updated.guliang.tech"
	err = svc.PatchOrganization(ctx, PatchOrganizationCommand{
		ID:          created.ID,
		DisplayName: &newDisplayName,
		WebsiteURL:  &newWebsiteURL,
		Revision:    created.Revision,
	})
	if err != nil {
		t.Fatalf("PatchOrganization() error = %v, want nil", err)
	}

	// Verify the update was persisted
	updated, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("repo.GetByID() error = %v, want nil", err)
	}
	if updated.DisplayName != newDisplayName {
		t.Errorf("updated.DisplayName = %q, want %q", updated.DisplayName, newDisplayName)
	}
	if updated.WebsiteURL != newWebsiteURL {
		t.Errorf("updated.WebsiteURL = %q, want %q", updated.WebsiteURL, newWebsiteURL)
	}
	if updated.Revision != 2 {
		t.Errorf("updated.Revision = %d, want 2", updated.Revision)
	}
	// Slug should remain unchanged
	if updated.Slug != "guliang" {
		t.Errorf("updated.Slug = %q, want %q", updated.Slug, "guliang")
	}
}

// ---------------------------------------------------------------------------
// Stub Tenant Repository: in-memory identity + topology + primary region
// ---------------------------------------------------------------------------

type testTenantRepository struct {
	identities map[string]*tenantidentities.TenantIdentity
	topologies map[string]*tenantinstances.TenantInstance
	// noPrimaryRegion forces PrimaryRegion() to return NotFound.
	noPrimaryRegion bool
}

func newTestTenantRepository() *testTenantRepository {
	return &testTenantRepository{
		identities: make(map[string]*tenantidentities.TenantIdentity),
		topologies: make(map[string]*tenantinstances.TenantInstance),
	}
}

func (r *testTenantRepository) snapshot() map[string]any {
	out := make(map[string]any, len(r.identities)+len(r.topologies))
	for k, v := range r.identities {
		out["identity:"+k] = v
	}
	for k, v := range r.topologies {
		out["topology:"+k] = v
	}
	return out
}

func (r *testTenantRepository) restore(snapshot map[string]any) {
	r.identities = make(map[string]*tenantidentities.TenantIdentity)
	r.topologies = make(map[string]*tenantinstances.TenantInstance)
	for _, value := range snapshot {
		switch typed := value.(type) {
		case *tenantidentities.TenantIdentity:
			r.identities[typed.OrganizationID] = typed
		case *tenantinstances.TenantInstance:
			r.topologies[typed.ID] = typed
		}
	}
}

func (r *testTenantRepository) CreateIdentity(_ context.Context, identity *tenantidentities.TenantIdentity) error {
	if _, exists := r.identities[identity.OrganizationID]; exists {
		return apperrors.NewAlreadyExists("tenant identity already exists")
	}
	r.identities[identity.OrganizationID] = identity
	return nil
}

func (r *testTenantRepository) GetIdentity(_ context.Context, organizationID string) (*tenantidentities.TenantIdentity, error) {
	identity, ok := r.identities[organizationID]
	if !ok {
		return nil, apperrors.NewNotFound("tenant identity not found")
	}
	return identity, nil
}

func (r *testTenantRepository) DecommissionIdentity(_ context.Context, organizationID string, at time.Time) error {
	identity, ok := r.identities[organizationID]
	if !ok {
		return nil
	}
	if identity.Status != tenantidentities.StatusDecommissioned {
		identity.Status = tenantidentities.StatusDecommissioned
		identity.DecommissionedAt = &at
		identity.UpdatedAt = at
		identity.Revision++
	}
	return nil
}

func (r *testTenantRepository) CreatePrimaryTopology(_ context.Context, instance *tenantinstances.TenantInstance) error {
	for _, existing := range r.topologies {
		if existing.OrganizationID == instance.OrganizationID && existing.IsPrimary && existing.Status != tenantinstances.StatusDecommissioned {
			return apperrors.NewAlreadyExists("tenant already has a primary topology")
		}
	}
	r.topologies[instance.ID] = instance
	return nil
}

func (r *testTenantRepository) GetPrimaryTopology(_ context.Context, organizationID string) (*tenantinstances.TenantInstance, error) {
	for _, instance := range r.topologies {
		if instance.OrganizationID == organizationID && instance.IsPrimary && instance.Status != tenantinstances.StatusDecommissioned {
			return instance, nil
		}
	}
	return nil, apperrors.NewNotFound("tenant primary topology not found")
}

func (r *testTenantRepository) DecommissionTopologies(_ context.Context, organizationID string, at time.Time) error {
	for _, instance := range r.topologies {
		if instance.OrganizationID != organizationID || instance.Status == tenantinstances.StatusDecommissioned {
			continue
		}
		instance.Status = tenantinstances.StatusDecommissioned
		instance.UpdatedAt = at
		instance.Revision++
	}
	return nil
}

func (r *testTenantRepository) PrimaryRegion(_ context.Context) (*tenantinstances.Region, error) {
	if r.noPrimaryRegion {
		return nil, apperrors.NewNotFound("no primary deployment region is configured")
	}
	return &tenantinstances.Region{ID: "region-primary", RegionCode: "cn-north", CountryCode: "CN"}, nil
}

// ---------------------------------------------------------------------------
// Tenant provisioning / retirement regression tests
// ---------------------------------------------------------------------------

func TestCreateOrganization_ProvisionsIdentityAndPrimaryTopology(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()

	org, err := svc.CreateOrganization(ctx, CreateOrganizationCommand{
		Slug:        "guliang",
		DisplayName: "GuliangTech",
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v, want nil", err)
	}

	tx := svc.UOW.(testUnitOfWork)
	identity := tx.tenantRepo.identities[org.ID]
	if identity == nil {
		t.Fatal("tenant identity row was not created")
	}
	if identity.Status != tenantidentities.StatusActive {
		t.Errorf("identity.Status = %q, want %q", identity.Status, tenantidentities.StatusActive)
	}
	if identity.IsolationTier != tenantidentities.IsolationTierRow {
		t.Errorf("identity.IsolationTier = %q, want %q", identity.IsolationTier, tenantidentities.IsolationTierRow)
	}
	if identity.IsBuiltIn {
		t.Error("identity.IsBuiltIn = true, want false")
	}
	if identity.ProvisionedAt == nil {
		t.Error("identity.ProvisionedAt = nil, want provisioned")
	}

	topology, err := tx.tenantRepo.GetPrimaryTopology(ctx, org.ID)
	if err != nil {
		t.Fatalf("GetPrimaryTopology() error = %v, want nil", err)
	}
	if !topology.IsPrimary {
		t.Error("topology.IsPrimary = false, want true")
	}
	if topology.Status != tenantinstances.StatusActive {
		t.Errorf("topology.Status = %q, want %q", topology.Status, tenantinstances.StatusActive)
	}
	if topology.RegionCode != "cn-north" || topology.DataResidencyCountry != "CN" {
		t.Errorf("topology region = (%q, %q), want (cn-north, CN)", topology.RegionCode, topology.DataResidencyCountry)
	}
	if topology.PlacementKind != tenantinstances.PlacementSharedCluster {
		t.Errorf("topology.PlacementKind = %q, want %q", topology.PlacementKind, tenantinstances.PlacementSharedCluster)
	}

	// Organization itself must still be persisted.
	if _, err := repo.GetByID(ctx, org.ID); err != nil {
		t.Fatalf("organization missing after provisioning: %v", err)
	}
}

func TestCreateOrganization_RollsBackWhenPrimaryRegionMissing(t *testing.T) {
	svc, repo := newTestService()
	svc.UOW.(testUnitOfWork).tenantRepo.noPrimaryRegion = true
	ctx := context.Background()

	_, err := svc.CreateOrganization(ctx, CreateOrganizationCommand{
		Slug:        "no-region",
		DisplayName: "No Region",
		Status:      "active",
	})
	if err == nil {
		t.Fatal("CreateOrganization() error = nil, want region resolution failure")
	}
	if _, lookupErr := repo.GetBySlug(ctx, "no-region"); lookupErr == nil {
		t.Fatal("organization row survived a failed provisioning transaction; want rollback")
	}
}

func TestCreateOrganization_RollsBackWhenIdentityConflicts(t *testing.T) {
	svc, repo := newTestService()
	tx := svc.UOW.(testUnitOfWork)
	tx.tenantRepo.identities["019abc-test-uuid-0001"] = &tenantidentities.TenantIdentity{
		OrganizationID: "019abc-test-uuid-0001",
		Status:         tenantidentities.StatusActive,
		IsolationTier:  tenantidentities.IsolationTierRow,
	}
	ctx := context.Background()

	_, err := svc.CreateOrganization(ctx, CreateOrganizationCommand{
		Slug:        "identity-conflict",
		DisplayName: "Identity Conflict",
		Status:      "active",
	})
	if err == nil {
		t.Fatal("CreateOrganization() error = nil, want identity conflict")
	}
	if _, lookupErr := repo.GetBySlug(ctx, "identity-conflict"); lookupErr == nil {
		t.Fatal("organization row survived an identity conflict; want rollback")
	}
}

func TestDeleteOrganization_DecommissionsIdentityAndTopology(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	org, err := svc.CreateOrganization(ctx, CreateOrganizationCommand{
		Slug:        "retire-me",
		DisplayName: "Retire Me",
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v, want nil", err)
	}

	if err := svc.DeleteOrganization(ctx, DeleteOrganizationCommand{ID: org.ID}); err != nil {
		t.Fatalf("DeleteOrganization() error = %v, want nil", err)
	}

	tx := svc.UOW.(testUnitOfWork)
	identity := tx.tenantRepo.identities[org.ID]
	if identity == nil {
		t.Fatal("tenant identity row disappeared; want soft decommission")
	}
	if identity.Status != tenantidentities.StatusDecommissioned {
		t.Errorf("identity.Status = %q, want %q", identity.Status, tenantidentities.StatusDecommissioned)
	}
	if identity.DecommissionedAt == nil {
		t.Error("identity.DecommissionedAt = nil, want stamped")
	}

	topology, err := tx.tenantRepo.GetPrimaryTopology(ctx, org.ID)
	if err == nil && topology.Status != tenantinstances.StatusDecommissioned {
		t.Errorf("topology status = %q, want decommissioned", topology.Status)
	}
	found := false
	for _, instance := range tx.tenantRepo.topologies {
		if instance.OrganizationID != org.ID {
			continue
		}
		found = true
		if instance.Status != tenantinstances.StatusDecommissioned {
			t.Errorf("topology %q status = %q, want decommissioned", instance.ID, instance.Status)
		}
	}
	if !found {
		t.Fatal("tenant topology row disappeared; want soft decommission")
	}
}

// testEnforcerRepository 只覆盖开通租户时的 Create（DEF-34）；其余方法不应被调用。
type testEnforcerRepository struct{ enforcerdomain.Repository }

func (testEnforcerRepository) Create(_ context.Context, _ *enforcerdomain.Enforcer) error { return nil }
