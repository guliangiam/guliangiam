package tickets

import (
	"context"
	"strings"
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
	domain "github.com/guliangiam/guliangiam/internal/domain/tickets"
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

type mockTicketRepo struct {
	tickets       []*domain.Ticket
	lastListQuery domain.ListQuery
}

func (r *mockTicketRepo) GetByID(_ context.Context, id, organizationID string) (*domain.Ticket, error) {
	for _, t := range r.tickets {
		if t.ID == id && t.OrganizationID == organizationID {
			return t, nil
		}
	}
	return nil, apperrors.NewNotFound("ticket not found")
}

func (r *mockTicketRepo) List(_ context.Context, q domain.ListQuery) ([]*domain.Ticket, int64, error) {
	r.lastListQuery = q
	return r.tickets, int64(len(r.tickets)), nil
}

func (r *mockTicketRepo) Create(_ context.Context, t *domain.Ticket) error {
	r.tickets = append(r.tickets, t)
	return nil
}

func (r *mockTicketRepo) Update(_ context.Context, t *domain.Ticket) error {
	for i, item := range r.tickets {
		if item.ID == t.ID {
			r.tickets[i] = t
			return nil
		}
	}
	return apperrors.NewNotFound("ticket not found")
}

func (r *mockTicketRepo) Delete(_ context.Context, id, organizationID string) error {
	for i, t := range r.tickets {
		if t.ID == id && t.OrganizationID == organizationID {
			r.tickets = append(r.tickets[:i], r.tickets[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("ticket not found")
}

type testUnitOfWork struct {
	ticketRepo *mockTicketRepo
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
func (u testUnitOfWork) Tickets() domain.Repository                                       { return u.ticketRepo }
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

func newSvc(repo *mockTicketRepo) Service {
	return Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "ticket-id"},
		UOW:   testUnitOfWork{ticketRepo: repo},
	}
}

func TestCreateTicket_Validation(t *testing.T) {
	svc := newSvc(&mockTicketRepo{})
	tests := []struct {
		name    string
		cmd     CreateTicketCommand
		wantErr bool
	}{
		{name: "valid", cmd: CreateTicketCommand{OrganizationID: "org-1", Name: "t-1"}, wantErr: false},
		{name: "missing orgId", cmd: CreateTicketCommand{Name: "t-1"}, wantErr: true},
		{name: "missing name", cmd: CreateTicketCommand{OrganizationID: "org-1"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateTicket(context.Background(), tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateTicket_DefaultStatus(t *testing.T) {
	repo := &mockTicketRepo{}
	svc := newSvc(repo)
	err := svc.CreateTicket(context.Background(), CreateTicketCommand{
		OrganizationID: "org-1",
		Name:           "t-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.tickets[0].Status != domain.StatusOpen {
		t.Errorf("Status = %q, want %q", repo.tickets[0].Status, domain.StatusOpen)
	}
	if repo.tickets[0].Revision != 1 {
		t.Errorf("Revision = %d, want 1", repo.tickets[0].Revision)
	}
}

func TestCreateTicket_CustomStatus(t *testing.T) {
	repo := &mockTicketRepo{}
	svc := newSvc(repo)
	err := svc.CreateTicket(context.Background(), CreateTicketCommand{
		OrganizationID: "org-1",
		Name:           "t-1",
		Status:         domain.StatusPending,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.tickets[0].Status != domain.StatusPending {
		t.Errorf("Status = %q, want %q", repo.tickets[0].Status, domain.StatusPending)
	}
}

func TestCreateTicket_DefaultType(t *testing.T) {
	repo := &mockTicketRepo{}
	svc := newSvc(repo)
	err := svc.CreateTicket(context.Background(), CreateTicketCommand{
		OrganizationID: "org-1",
		Name:           "t-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.tickets[0].Type != domain.TypeOrgRequest {
		t.Errorf("Type = %q, want %q", repo.tickets[0].Type, domain.TypeOrgRequest)
	}
}

func TestCreateTicket_KnownTypesAccepted(t *testing.T) {
	for _, want := range []string{domain.TypeOrgRequest, domain.TypeVendorFeedback} {
		repo := &mockTicketRepo{}
		svc := newSvc(repo)
		if err := svc.CreateTicket(context.Background(), CreateTicketCommand{
			OrganizationID: "org-1", Name: "t-1", Type: want,
		}); err != nil {
			t.Fatalf("unexpected error for type %q: %v", want, err)
		}
		if repo.tickets[0].Type != want {
			t.Errorf("Type = %q, want %q", repo.tickets[0].Type, want)
		}
	}
}

func TestCreateTicket_UnknownTypeRejected(t *testing.T) {
	repo := &mockTicketRepo{}
	svc := newSvc(repo)
	err := svc.CreateTicket(context.Background(), CreateTicketCommand{
		OrganizationID: "org-1", Name: "t-1", Type: "free_text",
	})
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeUnprocessableEntity {
		t.Errorf("error code = %q, want %q", code, apperrors.CodeUnprocessableEntity)
	}
	if len(repo.tickets) != 0 {
		t.Errorf("expected no ticket stored, got %d", len(repo.tickets))
	}
}

func TestPatchTicket_UnknownTypeRejected(t *testing.T) {
	repo := &mockTicketRepo{
		tickets: []*domain.Ticket{{ID: "t-1", OrganizationID: "org-1", Name: "test", Type: domain.TypeOrgRequest, Status: domain.StatusOpen, Revision: 1}},
	}
	svc := newSvc(repo)
	bad := "legacy_free_text"
	err := svc.PatchTicket(context.Background(), PatchTicketCommand{ID: "t-1", OrganizationID: "org-1", Type: &bad})
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeUnprocessableEntity {
		t.Errorf("error code = %q, want %q", code, apperrors.CodeUnprocessableEntity)
	}
	if repo.tickets[0].Type != domain.TypeOrgRequest {
		t.Errorf("Type = %q, want unchanged %q", repo.tickets[0].Type, domain.TypeOrgRequest)
	}
}

func TestPatchTicket_KnownTypeAccepted(t *testing.T) {
	repo := &mockTicketRepo{
		tickets: []*domain.Ticket{{ID: "t-1", OrganizationID: "org-1", Name: "test", Type: domain.TypeOrgRequest, Status: domain.StatusOpen, Revision: 1}},
	}
	svc := newSvc(repo)
	want := domain.TypeVendorFeedback
	if err := svc.PatchTicket(context.Background(), PatchTicketCommand{ID: "t-1", OrganizationID: "org-1", Type: &want}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.tickets[0].Type != want {
		t.Errorf("Type = %q, want %q", repo.tickets[0].Type, want)
	}
}

func TestListTickets_PriorityAndTypePassthrough(t *testing.T) {
	repo := &mockTicketRepo{}
	svc := newSvc(repo)
	_, _, err := svc.ListTickets(context.Background(), ListTicketsQuery{
		OrganizationID: "org-1",
		Priority:       "P1",
		Type:           domain.TypeVendorFeedback,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastListQuery.Priority != "P1" {
		t.Errorf("ListQuery.Priority = %q, want P1", repo.lastListQuery.Priority)
	}
	if repo.lastListQuery.Type != domain.TypeVendorFeedback {
		t.Errorf("ListQuery.Type = %q, want %q", repo.lastListQuery.Type, domain.TypeVendorFeedback)
	}
}

func TestPatchTicket_RevisionMismatch(t *testing.T) {
	repo := &mockTicketRepo{
		tickets: []*domain.Ticket{{ID: "t-1", OrganizationID: "org-1", Name: "test", Status: domain.StatusOpen, Revision: 2}},
	}
	svc := newSvc(repo)
	err := svc.PatchTicket(context.Background(), PatchTicketCommand{ID: "t-1", OrganizationID: "org-1", Revision: 1})
	if err == nil {
		t.Error("expected revision mismatch error")
	}
}

func TestPatchTicket_StatusTransition(t *testing.T) {
	now := time.Now()
	repo := &mockTicketRepo{
		tickets: []*domain.Ticket{{ID: "t-1", OrganizationID: "org-1", Name: "test", Status: domain.StatusOpen, Revision: 1, CreatedAt: now, UpdatedAt: now}},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "id"},
		UOW:   testUnitOfWork{ticketRepo: repo},
	}

	resolved := domain.StatusResolved
	err := svc.PatchTicket(context.Background(), PatchTicketCommand{ID: "t-1", OrganizationID: "org-1", Status: &resolved, Revision: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.tickets[0].Status != domain.StatusResolved {
		t.Errorf("Status = %q, want %q", repo.tickets[0].Status, domain.StatusResolved)
	}
	if repo.tickets[0].ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set")
	}
}

func TestDeleteTicket(t *testing.T) {
	repo := &mockTicketRepo{
		tickets: []*domain.Ticket{{ID: "t-1", OrganizationID: "org-1", Name: "test"}},
	}
	svc := newSvc(repo)
	if err := svc.DeleteTicket(context.Background(), "org-1", "t-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.tickets) != 0 {
		t.Errorf("expected 0 tickets, got %d", len(repo.tickets))
	}
}

type capturingNotifier struct {
	commands []NotificationCommand
	err      error
}

func (n *capturingNotifier) Notify(_ context.Context, command NotificationCommand) error {
	n.commands = append(n.commands, command)
	return n.err
}

func TestPatchTicket_StatusChangeNotifiesCreator(t *testing.T) {
	repo := &mockTicketRepo{
		tickets: []*domain.Ticket{{
			ID: "t-1", OrganizationID: "org-1", Name: "printer", DisplayName: "打印机故障",
			Status: domain.StatusOpen, Creator: "requester", Revision: 1,
		}},
	}
	svc := newSvc(repo)
	notifier := &capturingNotifier{}
	svc.Notifier = notifier

	resolved := domain.StatusResolved
	if err := svc.PatchTicket(context.Background(), PatchTicketCommand{
		ID: "t-1", OrganizationID: "org-1", Status: &resolved, Revision: 1,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.commands) != 1 {
		t.Fatalf("notifications = %d, want 1", len(notifier.commands))
	}
	got := notifier.commands[0]
	if got.EventType != "tickets.status_changed" || got.Category != "feedback" {
		t.Fatalf("unexpected notification: %+v", got)
	}
	if got.RecipientUsername != "requester" {
		t.Fatalf("recipient = %q, want requester", got.RecipientUsername)
	}
	if !strings.Contains(got.Content, "已解决") || !strings.Contains(got.Content, "打印机故障") {
		t.Fatalf("content = %q, want resolved status and display name", got.Content)
	}
}

func TestPatchTicket_AssigneeChangeNotifiesNewAssignee(t *testing.T) {
	repo := &mockTicketRepo{
		tickets: []*domain.Ticket{{
			ID: "t-1", OrganizationID: "org-1", Name: "printer",
			Status: domain.StatusOpen, Creator: "requester", Assignee: "agent-old", Revision: 1,
		}},
	}
	svc := newSvc(repo)
	notifier := &capturingNotifier{}
	svc.Notifier = notifier

	assignee := "agent-new"
	if err := svc.PatchTicket(context.Background(), PatchTicketCommand{
		ID: "t-1", OrganizationID: "org-1", Assignee: &assignee, Revision: 1,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.commands) != 1 {
		t.Fatalf("notifications = %d, want 1", len(notifier.commands))
	}
	got := notifier.commands[0]
	if got.EventType != "tickets.assigned" || got.RecipientUsername != "agent-new" {
		t.Fatalf("unexpected notification: %+v", got)
	}
}

func TestCreateTicket_NotifiesAssignee(t *testing.T) {
	repo := &mockTicketRepo{}
	svc := newSvc(repo)
	notifier := &capturingNotifier{}
	svc.Notifier = notifier

	if err := svc.CreateTicket(context.Background(), CreateTicketCommand{
		OrganizationID: "org-1", Name: "printer", Creator: "requester", Assignee: "agent-1",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var assigned bool
	for _, command := range notifier.commands {
		if command.EventType == "tickets.assigned" {
			assigned = true
			if command.RecipientUsername != "agent-1" {
				t.Fatalf("assigned recipient = %q, want agent-1", command.RecipientUsername)
			}
		}
	}
	if !assigned {
		t.Fatal("expected tickets.assigned notification for the assignee")
	}
}

func TestAddTicketMessage_Validation(t *testing.T) {
	svc := newSvc(&mockTicketRepo{})
	_, err := svc.AddTicketMessage(context.Background(), "org-1", "t-1", "user", "", "")
	if err == nil {
		t.Error("expected error for empty content")
	}
}

func TestAddTicketMessage_ClosedTicket(t *testing.T) {
	repo := &mockTicketRepo{
		tickets: []*domain.Ticket{
			{ID: "t-1", OrganizationID: "org-1", Name: "test", Status: domain.StatusClosed, Creator: "creator", Revision: 1},
		},
	}
	svc := newSvc(repo)
	_, err := svc.AddTicketMessage(context.Background(), "org-1", "t-1", "user", "hello", "")
	if err == nil {
		t.Error("expected error for closed ticket")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeUnprocessableEntity {
		t.Errorf("error code = %q, want %q", code, apperrors.CodeUnprocessableEntity)
	}
}

func TestAddTicketMessage_StatusTransition(t *testing.T) {
	now := time.Now()
	repo := &mockTicketRepo{
		tickets: []*domain.Ticket{
			{ID: "t-1", OrganizationID: "org-1", Name: "test", Status: domain.StatusOpen, Creator: "creator", Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "id"},
		UOW:   testUnitOfWork{ticketRepo: repo},
	}

	detail, err := svc.AddTicketMessage(context.Background(), "org-1", "t-1", "agent", "reply", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Status != domain.StatusPending {
		t.Errorf("Status = %q, want %q", detail.Status, domain.StatusPending)
	}
	if detail.Revision != 2 {
		t.Errorf("Revision = %d, want 2", detail.Revision)
	}
}

func TestAddTicketMessage_SameCreator(t *testing.T) {
	now := time.Now()
	repo := &mockTicketRepo{
		tickets: []*domain.Ticket{
			{ID: "t-1", OrganizationID: "org-1", Name: "test", Status: domain.StatusOpen, Creator: "creator", Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "id"},
		UOW:   testUnitOfWork{ticketRepo: repo},
	}

	detail, err := svc.AddTicketMessage(context.Background(), "org-1", "t-1", "creator", "more info", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Status != domain.StatusOpen {
		t.Errorf("Status should remain open when creator replies, got %q", detail.Status)
	}
}
