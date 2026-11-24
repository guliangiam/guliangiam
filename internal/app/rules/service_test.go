package rules

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
	domain "github.com/guliangiam/guliangiam/internal/domain/rules"
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
}

func (g fakeUUIDGenerator) NewString() (string, error) { return g.fixedID, nil }

// ---------------------------------------------------------------------------
// Mock rule repository
// ---------------------------------------------------------------------------

type mockRuleRepo struct {
	rules []*domain.Rule
}

func (r *mockRuleRepo) GetByID(ctx context.Context, id, organizationID string) (*domain.Rule, error) {
	for _, rule := range r.rules {
		if rule.ID == id && rule.OrganizationID == organizationID {
			// 返回快照：真实仓储按行扫描，事务回滚不会把内存改动带回存储。
			snapshot := *rule
			return &snapshot, nil
		}
	}
	return nil, apperrors.NewNotFound("rule not found")
}

func (r *mockRuleRepo) List(ctx context.Context, query domain.ListQuery) ([]*domain.Rule, int64, error) {
	return r.rules, int64(len(r.rules)), nil
}

func (r *mockRuleRepo) Create(ctx context.Context, rule *domain.Rule) error {
	r.rules = append(r.rules, rule)
	return nil
}

func (r *mockRuleRepo) Update(ctx context.Context, rule *domain.Rule) error {
	for i, rl := range r.rules {
		if rl.ID == rule.ID {
			r.rules[i] = rule
			return nil
		}
	}
	return apperrors.NewNotFound("rule not found")
}

func (r *mockRuleRepo) Delete(ctx context.Context, id, organizationID string) error {
	for i, rl := range r.rules {
		if rl.ID == id && rl.OrganizationID == organizationID {
			r.rules = append(r.rules[:i], r.rules[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("rule not found")
}

// ---------------------------------------------------------------------------
// Test UOW
// ---------------------------------------------------------------------------

type testUnitOfWork struct {
	ruleRepo *mockRuleRepo
}

func (u testUnitOfWork) Applications() applications.Repository                        { return nil }
func (u testUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u testUnitOfWork) Consents() consentdomain.Repository                           { return nil }
func (u testUnitOfWork) Gateway() gatewaydomain.Repository                            { return nil }
func (u testUnitOfWork) Groups() groupsdomain.Repository                              { return nil }
func (u testUnitOfWork) Invitations() invitationsdomain.Repository                    { return nil }
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
func (u testUnitOfWork) Orders() orderdomain.Repository                               { return nil }
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
func (u testUnitOfWork) Coupons() coupondomain.Repository                                 { return nil }
func (u testUnitOfWork) CouponUsages() coupondomain.CouponUsageRepository                 { return nil }
func (u testUnitOfWork) Entries() entrydomain.Repository                                  { return nil }
func (u testUnitOfWork) McpServers() mcpserverdomain.Repository                           { return nil }
func (u testUnitOfWork) RadiusAccountings() radiusaccountingdomain.Repository             { return nil }
func (u testUnitOfWork) Verifications() verificationsdomain.Repository                    { return nil }
func (u testUnitOfWork) WebAuthnCredentials() webauthndomain.Repository                   { return nil }
func (u testUnitOfWork) FaceIdProviders() faceidproviderdomain.Repository                 { return nil }
func (u testUnitOfWork) RadiusConfigs() radiusconfigdomain.Repository                     { return nil }
func (u testUnitOfWork) Rules() domain.Repository                                         { return u.ruleRepo }
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

func TestService_NewRule(t *testing.T) {
	clock := fakeClock{fixedTime: time.Now()}
	uuid := fakeUUIDGenerator{fixedID: "019test-rule-id"}
	svc := Service{Clock: clock, UUID: uuid, UOW: testUnitOfWork{}}

	enabled := true
	rule, err := svc.NewRule(CreateRuleCommand{
		OrganizationID: "org-1",
		Name:           "IP Block Rule",
		Scope:          "org",
		RuleType:       "IP",
		Conditions:     []map[string]any{{"field": "ip", "operator": "eq", "value": "1.2.3.4"}},
		Action:         "Block",
		Priority:       10,
		IsEnabled:      &enabled,
	})
	if err != nil {
		t.Fatalf("NewRule() error = %v", err)
	}
	if rule.ID != "019test-rule-id" {
		t.Errorf("ID = %q, want %q", rule.ID, "019test-rule-id")
	}
	if rule.OrganizationID != "org-1" {
		t.Errorf("OrganizationID = %q, want %q", rule.OrganizationID, "org-1")
	}
	if rule.Name != "IP Block Rule" {
		t.Errorf("Name = %q, want %q", rule.Name, "IP Block Rule")
	}
	if !rule.IsEnabled {
		t.Error("expected IsEnabled = true")
	}
	if rule.Revision != 1 {
		t.Errorf("Revision = %d, want 1", rule.Revision)
	}
}

func TestService_NewRule_DefaultEnabled(t *testing.T) {
	clock := fakeClock{fixedTime: time.Now()}
	uuid := fakeUUIDGenerator{fixedID: "019test-rule-id"}
	svc := Service{Clock: clock, UUID: uuid, UOW: testUnitOfWork{}}

	rule, err := svc.NewRule(CreateRuleCommand{
		OrganizationID: "org-1",
		Name:           "Default Rule",
		Scope:          "org",
		RuleType:       "IP",
		Action:         "Allow",
	})
	if err != nil {
		t.Fatalf("NewRule() error = %v", err)
	}
	if !rule.IsEnabled {
		t.Error("expected IsEnabled = true by default")
	}
}

func TestService_CreateRule_Validation(t *testing.T) {
	repo := &mockRuleRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "019test-rule-id"},
		UOW:   testUnitOfWork{ruleRepo: repo},
	}

	tests := []struct {
		name    string
		command CreateRuleCommand
		wantErr bool
	}{
		{
			name: "valid command",
			command: CreateRuleCommand{
				OrganizationID: "org-1",
				Name:           "Test Rule",
				Scope:          "org",
				RuleType:       "IP",
				Conditions:     []map[string]any{{"field": "ip", "operator": "eq", "value": "1.2.3.4"}},
				Action:         "Allow",
			},
			wantErr: false,
		},
		{
			name: "missing organizationId",
			command: CreateRuleCommand{
				Name:     "Test Rule",
				Scope:    "org",
				RuleType: "IP",
				Action:   "Allow",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			command: CreateRuleCommand{
				OrganizationID: "org-1",
				Scope:          "org",
				RuleType:       "IP",
				Action:         "Allow",
			},
			wantErr: true,
		},
		{
			name: "missing scope",
			command: CreateRuleCommand{
				OrganizationID: "org-1",
				Name:           "Test Rule",
				RuleType:       "IP",
				Action:         "Allow",
			},
			wantErr: true,
		},
		{
			name: "missing ruleType",
			command: CreateRuleCommand{
				OrganizationID: "org-1",
				Name:           "Test Rule",
				Scope:          "org",
				Action:         "Allow",
			},
			wantErr: true,
		},
		{
			name: "missing action",
			command: CreateRuleCommand{
				OrganizationID: "org-1",
				Name:           "Test Rule",
				Scope:          "org",
				RuleType:       "IP",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateRule(context.Background(), tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_PatchRule_RevisionMismatch(t *testing.T) {
	now := time.Now()
	repo := &mockRuleRepo{
		rules: []*domain.Rule{
			{ID: "rule-1", OrganizationID: "org-1", Revision: 2},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   testUnitOfWork{ruleRepo: repo},
	}

	err := svc.PatchRule(context.Background(), "org-1", PatchRuleCommand{
		ID:       "rule-1",
		Revision: 1, // mismatch
	})
	if err == nil {
		t.Error("expected error for revision mismatch, got nil")
	}
}

func TestService_PatchRule_Success(t *testing.T) {
	now := time.Now()
	repo := &mockRuleRepo{
		rules: []*domain.Rule{
			{
				ID:             "rule-1",
				OrganizationID: "org-1",
				Name:           "Old",
				RuleType:       "IP",
				Conditions:     []map[string]any{{"field": "ip", "operator": "eq", "value": "1.2.3.4"}},
				Revision:       1,
			},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   testUnitOfWork{ruleRepo: repo},
	}

	newName := "Updated"
	err := svc.PatchRule(context.Background(), "org-1", PatchRuleCommand{
		ID:   "rule-1",
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("PatchRule() error = %v", err)
	}
	if repo.rules[0].Name != "Updated" {
		t.Errorf("Name = %q, want %q", repo.rules[0].Name, "Updated")
	}
	if repo.rules[0].Revision != 2 {
		t.Errorf("Revision = %d, want 2", repo.rules[0].Revision)
	}
}

func TestService_DeleteRule(t *testing.T) {
	repo := &mockRuleRepo{
		rules: []*domain.Rule{
			{ID: "rule-1", OrganizationID: "org-1"},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UOW:   testUnitOfWork{ruleRepo: repo},
	}

	err := svc.DeleteRule(context.Background(), "org-1", "rule-1")
	if err != nil {
		t.Fatalf("DeleteRule() error = %v", err)
	}
	if len(repo.rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(repo.rules))
	}
}

func TestService_GetRule(t *testing.T) {
	now := time.Now()
	repo := &mockRuleRepo{
		rules: []*domain.Rule{
			{ID: "rule-1", OrganizationID: "org-1", Name: "Test Rule", Scope: "org", RuleType: "IP", Action: "Allow", CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{ruleRepo: repo}}

	detail, err := svc.GetRule(context.Background(), "org-1", "rule-1")
	if err != nil {
		t.Fatalf("GetRule() error = %v", err)
	}
	if detail.ID != "rule-1" {
		t.Errorf("ID = %q, want %q", detail.ID, "rule-1")
	}
	if detail.Name != "Test Rule" {
		t.Errorf("Name = %q, want %q", detail.Name, "Test Rule")
	}
}

func TestService_ListRules(t *testing.T) {
	now := time.Now()
	repo := &mockRuleRepo{
		rules: []*domain.Rule{
			{ID: "rule-1", OrganizationID: "org-1", Name: "Rule 1", CreatedAt: now, UpdatedAt: now},
			{ID: "rule-2", OrganizationID: "org-1", Name: "Rule 2", CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{ruleRepo: repo}}

	items, total, err := svc.ListRules(context.Background(), ListRulesQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListRules() error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(items))
	}
}
