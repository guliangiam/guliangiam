package iga_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/app/iga"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainiga "github.com/guliangiam/guliangiam/internal/domain/iga"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	domainsessions "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/require"
)

var workflowNow = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

type workflowClock struct{ at time.Time }

func (c workflowClock) Now() time.Time { return c.at }

type workflowTransactionContextKey struct{}

func workflowInTransaction(ctx context.Context) bool {
	inTransaction, _ := ctx.Value(workflowTransactionContextKey{}).(bool)
	return inTransaction
}

type workflowRepo struct {
	domainiga.Repository
	request                    *domainiga.AccessRequest
	lifecycle                  *domainiga.AccessLifecycle
	createdLifecycle           *domainiga.AccessLifecycle
	validity                   domainiga.AccessTargetValidity
	policies                   []*domainiga.JITPolicy
	bindings                   []*domainiga.JITBinding
	messages                   []*domainiga.TicketMessage
	sodRules                   []*domainiga.SODRule
	effectiveRoleIDs           []string
	sodRulePages               []int
	sodRulePageSizes           []int
	sodReadsOutsideTransaction int
	accessListInTx             bool
	accessGetInTx              bool
	messagesListInTx           bool
	lifecycleListInTx          bool
	lifecycleGetInTx           bool
	updates                    int
}

func (r *workflowRepo) GetAccessRequest(ctx context.Context, orgID, id string, _ bool) (*domainiga.AccessRequest, error) {
	r.accessGetInTx = workflowInTransaction(ctx)
	if r.request == nil || r.request.OrganizationID != orgID || r.request.ID != id {
		return nil, errors.New("access request not found")
	}
	item := *r.request
	return &item, nil
}

func (r *workflowRepo) ListAccessRequests(ctx context.Context, q domainiga.ListAccessRequestsQuery) ([]*domainiga.AccessRequest, int64, error) {
	r.accessListInTx = workflowInTransaction(ctx)
	if r.request == nil || r.request.OrganizationID != q.OrganizationID {
		return []*domainiga.AccessRequest{}, 0, nil
	}
	item := *r.request
	return []*domainiga.AccessRequest{&item}, 1, nil
}

func (r *workflowRepo) ListTicketMessages(ctx context.Context, orgID, ticketID string) ([]*domainiga.TicketMessage, error) {
	r.messagesListInTx = workflowInTransaction(ctx)
	if r.request == nil || r.request.OrganizationID != orgID || r.request.ID != ticketID {
		return []*domainiga.TicketMessage{}, nil
	}
	return r.messages, nil
}

func (r *workflowRepo) UpdateAccessRequest(_ context.Context, item *domainiga.AccessRequest, expected domainiga.RequestStatus) (bool, error) {
	if r.request == nil || r.request.Status != expected || r.request.Revision != item.Revision {
		return false, nil
	}
	item.Revision++
	stored := *item
	r.request = &stored
	r.updates++
	return true, nil
}

func (r *workflowRepo) AddTicketMessage(_ context.Context, message *domainiga.TicketMessage) error {
	stored := *message
	r.messages = append(r.messages, &stored)
	return nil
}

func (r *workflowRepo) ValidateAccessTargets(context.Context, string, string, string, string, string) (domainiga.AccessTargetValidity, error) {
	return r.validity, nil
}

func (r *workflowRepo) ResolveDefaultEnforcer(context.Context, string) (string, error) {
	return "enforcer-1", nil
}

func (r *workflowRepo) ListSODRules(ctx context.Context, q domainiga.ListSODRulesQuery) ([]*domainiga.SODRule, int64, error) {
	r.sodRulePages = append(r.sodRulePages, q.Page)
	r.sodRulePageSizes = append(r.sodRulePageSizes, q.PageSize)
	if !workflowInTransaction(ctx) {
		r.sodReadsOutsideTransaction++
	}
	total := int64(len(r.sodRules))
	page := q.Page
	if page < 1 {
		page = 1
	}
	start := (page - 1) * q.PageSize
	if q.PageSize <= 0 || start >= len(r.sodRules) {
		return []*domainiga.SODRule{}, total, nil
	}
	end := start + q.PageSize
	if end > len(r.sodRules) {
		end = len(r.sodRules)
	}
	return r.sodRules[start:end], total, nil
}

func (r *workflowRepo) ListEffectiveRoleIDs(context.Context, string, string, time.Time) ([]string, error) {
	return append([]string(nil), r.effectiveRoleIDs...), nil
}

func (r *workflowRepo) CreateLifecycle(_ context.Context, lifecycle *domainiga.AccessLifecycle) error {
	stored := *lifecycle
	r.createdLifecycle = &stored
	return nil
}

func (r *workflowRepo) CreateJITBinding(_ context.Context, binding *domainiga.JITBinding) error {
	stored := *binding
	r.bindings = append(r.bindings, &stored)
	return nil
}

func (r *workflowRepo) CreateJITPolicy(_ context.Context, policy *domainiga.JITPolicy) error {
	stored := *policy
	r.policies = append(r.policies, &stored)
	return nil
}

func (r *workflowRepo) ListLifecycle(ctx context.Context, q domainiga.ListLifecycleQuery) ([]*domainiga.AccessLifecycle, int64, error) {
	r.lifecycleListInTx = workflowInTransaction(ctx)
	if r.lifecycle == nil || r.lifecycle.OrganizationID != q.OrganizationID {
		return []*domainiga.AccessLifecycle{}, 0, nil
	}
	item := *r.lifecycle
	return []*domainiga.AccessLifecycle{&item}, 1, nil
}

func (r *workflowRepo) GetLifecycle(ctx context.Context, id, orgID string) (*domainiga.AccessLifecycle, error) {
	r.lifecycleGetInTx = workflowInTransaction(ctx)
	return r.getLifecycle(id, orgID)
}

func (r *workflowRepo) GetLifecycleForUpdate(_ context.Context, orgID, id string) (*domainiga.AccessLifecycle, error) {
	return r.getLifecycle(id, orgID)
}

func (r *workflowRepo) getLifecycle(id, orgID string) (*domainiga.AccessLifecycle, error) {
	if r.lifecycle == nil || r.lifecycle.ID != id || r.lifecycle.OrganizationID != orgID {
		return nil, errors.New("lifecycle not found")
	}
	item := *r.lifecycle
	return &item, nil
}

type workflowSessions struct {
	domainsessions.Repository
	session  *domainsessions.Session
	listInTx bool
	updates  int
}

func (r *workflowSessions) GetByID(_ context.Context, id string) (*domainsessions.Session, error) {
	if r.session == nil || r.session.ID != id {
		return nil, errors.New("session not found")
	}
	item := *r.session
	return &item, nil
}

func (r *workflowSessions) Update(_ context.Context, session *domainsessions.Session) error {
	stored := *session
	r.session = &stored
	r.updates++
	return nil
}

func (r *workflowSessions) List(ctx context.Context, query domainsessions.ListQuery) ([]*domainsessions.Session, int64, error) {
	r.listInTx = workflowInTransaction(ctx)
	if r.session == nil || r.session.OrganizationID != query.OrganizationID || r.session.UserID != query.UserID {
		return []*domainsessions.Session{}, 0, nil
	}
	item := *r.session
	return []*domainsessions.Session{&item}, 1, nil
}

type workflowRecords struct {
	domainrecords.Repository
	items []*domainrecords.Record
}

func (r *workflowRecords) Create(_ context.Context, record *domainrecords.Record) error {
	stored := *record
	r.items = append(r.items, &stored)
	return nil
}

type workflowOutbox struct {
	domainoutbox.Repository
	items []*domainoutbox.OutboxEvent
}

func (r *workflowOutbox) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	stored := *event
	r.items = append(r.items, &stored)
	return nil
}

type workflowUOW struct {
	appuow.UnitOfWork
	iga               *workflowRepo
	sessions          *workflowSessions
	records           *workflowRecords
	outbox            *workflowOutbox
	transactionScopes []tenantctx.TenantScope
}

func (u *workflowUOW) IGA() domainiga.Repository           { return u.iga }
func (u *workflowUOW) Sessions() domainsessions.Repository { return u.sessions }
func (u *workflowUOW) Records() domainrecords.Repository   { return u.records }
func (u *workflowUOW) Outbox() domainoutbox.Repository     { return u.outbox }
func (u *workflowUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	u.transactionScopes = append(u.transactionScopes, tenantctx.TenantScopeFromContext(ctx))
	return fn(context.WithValue(ctx, workflowTransactionContextKey{}, true), u)
}

func newWorkflowService(repo *workflowRepo, sessions *workflowSessions) (iga.Service, *workflowUOW) {
	uow := &workflowUOW{iga: repo, sessions: sessions, records: &workflowRecords{}, outbox: &workflowOutbox{}}
	return iga.Service{Clock: workflowClock{at: workflowNow}, UOW: uow}, uow
}

func newWorkflowRequest(status domainiga.RequestStatus, revision int64) *domainiga.AccessRequest {
	return &domainiga.AccessRequest{
		ID: "ticket-1", OrganizationID: "org-uuid", RequesterID: "user-1",
		RoleID: "role-1", ApplicationID: "app-1", EnforcerID: "enforcer-1",
		Resource: "orders", Action: "get", Reason: "temporary support",
		Status: status, StartsAt: workflowNow.Add(-time.Hour), ExpiresAt: workflowNow.Add(time.Hour),
		CreatedAt: workflowNow.Add(-2 * time.Hour), UpdatedAt: workflowNow.Add(-time.Hour), Revision: revision,
	}
}

func validWorkflowTargets() domainiga.AccessTargetValidity {
	return domainiga.AccessTargetValidity{
		UserValid: true, RoleValid: true, ApplicationValid: true, EnforcerValid: true,
		Subject: "alice", OrganizationSlug: "acme",
	}
}

func actionCommand(actor string, revision int64) iga.AccessRequestActionCommand {
	return iga.AccessRequestActionCommand{OrganizationID: "org-uuid", RequestID: "ticket-1", ActorID: actor, Revision: revision}
}

func TestApproveAccessRequest_TwoLevelsProvisionJITWithSlug(t *testing.T) {
	repo := &workflowRepo{request: newWorkflowRequest(domainiga.RequestPendingManager, 1), validity: validWorkflowTargets()}
	service, uow := newWorkflowService(repo, &workflowSessions{})

	managerResult, err := service.ApproveAccessRequest(context.Background(), "manager", actionCommand("manager-1", 1))
	require.NoError(t, err)
	require.Equal(t, string(domainiga.RequestPendingSAP), managerResult.Status)
	require.Equal(t, "manager-1", managerResult.ManagerApproverID)
	require.Equal(t, int64(3), managerResult.Revision)
	require.Empty(t, repo.policies)

	sapResult, err := service.ApproveAccessRequest(context.Background(), "sap", actionCommand("sap-1", managerResult.Revision))
	require.NoError(t, err)
	require.Equal(t, string(domainiga.RequestProvisioned), sapResult.Status)
	require.Equal(t, "sap-1", sapResult.SAPApproverID)
	require.Equal(t, int64(5), sapResult.Revision)
	require.NotEmpty(t, sapResult.LifecycleID)
	require.NotNil(t, repo.createdLifecycle)
	require.Len(t, repo.createdLifecycle.PolicyID, 36)
	require.Len(t, repo.bindings, 1)
	require.Len(t, repo.policies, 1)
	policy := repo.policies[0]
	require.Equal(t, "alice", policy.Subject)
	require.Equal(t, "acme", policy.OrganizationSlug)
	require.NotEqual(t, "org-uuid", policy.OrganizationSlug)
	require.Equal(t, sapResult.LifecycleID, policy.LifecycleID)
	require.Equal(t, repo.createdLifecycle.StartsAt, policy.StartsAt)
	require.Equal(t, repo.createdLifecycle.ExpiresAt, policy.ExpiresAt)
	require.Len(t, repo.messages, 4)
	require.Equal(t, "approved_manager", repo.messages[0].Action)
	require.Equal(t, "pending_sap", repo.messages[1].Action)
	require.Equal(t, "approved_sap", repo.messages[2].Action)
	require.Equal(t, "provisioned", repo.messages[3].Action)
	require.Len(t, uow.outbox.items, 4)
	require.Equal(t, "iga.jit.granted", uow.outbox.items[3].EventType)
}

func TestWithdrawAccessRequest_RequesterCanWithdrawPendingSAP(t *testing.T) {
	request := newWorkflowRequest(domainiga.RequestPendingSAP, 3)
	request.ManagerApproverID = "manager-1"
	repo := &workflowRepo{request: request}
	service, uow := newWorkflowService(repo, &workflowSessions{})

	result, err := service.WithdrawAccessRequest(context.Background(), actionCommand("user-1", 3))
	require.NoError(t, err)
	require.Equal(t, string(domainiga.RequestWithdrawn), result.Status)
	require.Equal(t, int64(4), result.Revision)
	require.Len(t, repo.messages, 1)
	require.Equal(t, "withdrawn", repo.messages[0].Action)
	require.Len(t, uow.records.items, 1)
	require.Equal(t, "iga.ticket.withdrawn", uow.records.items[0].Action)
}

func TestProvisionAccessRequest_RequiresSlugBeforeCreatingArtifacts(t *testing.T) {
	request := newWorkflowRequest(domainiga.RequestApproved, 1)
	request.SAPApproverID = "sap-1"
	validity := validWorkflowTargets()
	validity.OrganizationSlug = ""
	repo := &workflowRepo{request: request, validity: validity}
	service, _ := newWorkflowService(repo, &workflowSessions{})

	result, err := service.ProvisionAccessRequest(context.Background(), actionCommand("sap-1", 1))
	require.Error(t, err)
	require.Nil(t, result)
	require.ErrorContains(t, err, "organization slug")
	require.Nil(t, repo.createdLifecycle)
	require.Empty(t, repo.bindings)
	require.Empty(t, repo.policies)
	require.Zero(t, repo.updates)
}

func TestProvisionAccessRequest_ApprovedRequestProvisionsOnce(t *testing.T) {
	request := newWorkflowRequest(domainiga.RequestApproved, 1)
	request.SAPApproverID = "sap-1"
	repo := &workflowRepo{request: request, validity: validWorkflowTargets()}
	service, _ := newWorkflowService(repo, &workflowSessions{})

	result, err := service.ProvisionAccessRequest(context.Background(), actionCommand("sap-1", 1))
	require.NoError(t, err)
	require.Equal(t, string(domainiga.RequestProvisioned), result.Status)
	require.Equal(t, int64(2), result.Revision)
	require.Equal(t, result.LifecycleID, repo.createdLifecycle.ID)
	require.Equal(t, result.LifecycleID, repo.bindings[0].LifecycleID)
	require.Equal(t, "acme", repo.policies[0].OrganizationSlug)

	second, err := service.ProvisionAccessRequest(context.Background(), actionCommand("sap-1", result.Revision))
	require.Error(t, err)
	require.Nil(t, second)
	require.Len(t, repo.policies, 1)
}

func TestCreateLifecycle_RejectsSODConflictInsideGrantTransaction(t *testing.T) {
	repo := &workflowRepo{
		validity:         validWorkflowTargets(),
		sodRules:         []*domainiga.SODRule{{OrganizationID: "org-uuid", RoleAID: "role-1", RoleBID: "role-2"}},
		effectiveRoleIDs: []string{"role-2"},
	}
	service, _ := newWorkflowService(repo, &workflowSessions{})

	result, err := service.CreateLifecycle(context.Background(), "org-uuid", "user-1", "role-1", workflowNow.Add(time.Hour))
	require.ErrorContains(t, err, "sod conflict")
	require.Nil(t, result)
	require.Equal(t, []int{1}, repo.sodRulePages)
	require.Zero(t, repo.sodReadsOutsideTransaction)
	require.Nil(t, repo.createdLifecycle)
	require.Empty(t, repo.bindings)
	require.Empty(t, repo.policies)
}

func TestProvisionAccessRequest_RechecksSODConflictBeforeCreatingArtifacts(t *testing.T) {
	request := newWorkflowRequest(domainiga.RequestApproved, 1)
	request.SAPApproverID = "sap-1"
	repo := &workflowRepo{
		request:          request,
		validity:         validWorkflowTargets(),
		sodRules:         []*domainiga.SODRule{{OrganizationID: "org-uuid", RoleAID: "role-1", RoleBID: "role-2"}},
		effectiveRoleIDs: []string{"role-2"},
	}
	service, _ := newWorkflowService(repo, &workflowSessions{})

	result, err := service.ProvisionAccessRequest(context.Background(), actionCommand("sap-1", 1))
	require.ErrorContains(t, err, "sod conflict")
	require.Nil(t, result)
	require.Equal(t, []int{1}, repo.sodRulePages)
	require.Zero(t, repo.sodReadsOutsideTransaction)
	require.Nil(t, repo.createdLifecycle)
	require.Empty(t, repo.bindings)
	require.Empty(t, repo.policies)
	require.Zero(t, repo.updates)
}

func TestAccessRequestReadsUseTenantTransaction(t *testing.T) {
	repo := &workflowRepo{request: newWorkflowRequest(domainiga.RequestSubmitted, 1)}
	service, uow := newWorkflowService(repo, &workflowSessions{})
	ctx := tenantctx.WithTenantScope(context.Background(), "org-uuid", "acme", tenantctx.TenantFilterStrict)

	items, total, err := service.ListAccessRequests(ctx, iga.ListAccessRequestsQuery{OrganizationID: "org-uuid", Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(1), total)
	detail, err := service.GetAccessRequest(ctx, "org-uuid", "ticket-1")
	require.NoError(t, err)
	require.Equal(t, "ticket-1", detail.Request.ID)
	require.True(t, repo.accessListInTx)
	require.True(t, repo.accessGetInTx)
	require.True(t, repo.messagesListInTx)
	require.Len(t, uow.transactionScopes, 2)
	for _, scope := range uow.transactionScopes {
		require.Equal(t, tenantctx.TenantFilterStrict, scope.Mode)
		require.Equal(t, "org-uuid", scope.OrganizationID)
	}
}

func TestLifecycleReadsUseTenantTransaction(t *testing.T) {
	repo := &workflowRepo{lifecycle: &domainiga.AccessLifecycle{
		ID: "lifecycle-1", OrganizationID: "org-uuid", UserID: "user-1", RoleID: "role-1",
		Status: domainiga.LifecycleActive, GrantedAt: workflowNow, StartsAt: workflowNow,
		ExpiresAt: workflowNow.Add(time.Hour), CreatedAt: workflowNow, Revision: 1,
	}}
	service, uow := newWorkflowService(repo, &workflowSessions{})
	ctx := tenantctx.WithTenantScope(context.Background(), "org-uuid", "acme", tenantctx.TenantFilterStrict)

	items, total, err := service.ListLifecycle(ctx, iga.ListLifecycleQuery{OrganizationID: "org-uuid", Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(1), total)
	item, err := service.GetLifecycle(ctx, "org-uuid", "lifecycle-1")
	require.NoError(t, err)
	require.Equal(t, "lifecycle-1", item.ID)
	require.True(t, repo.lifecycleListInTx)
	require.True(t, repo.lifecycleGetInTx)
	require.Len(t, uow.transactionScopes, 2)
	for _, scope := range uow.transactionScopes {
		require.Equal(t, tenantctx.TenantFilterStrict, scope.Mode)
		require.Equal(t, "org-uuid", scope.OrganizationID)
	}
}

func TestIGAValidation_Answers422(t *testing.T) {
	svc := iga.Service{Clock: workflowClock{at: workflowNow}}
	ctx := context.Background()

	checks := []struct {
		name string
		err  error
	}{
		{"window", func() error {
			_, err := svc.CreateAccessRequest(ctx, iga.CreateAccessRequestCommand{
				OrganizationID: "org", RequesterID: "user", RoleID: "role", ApplicationID: "app",
				Resource: "res", Action: "read",
				StartsAt: workflowNow.Add(time.Hour), ExpiresAt: workflowNow,
			})
			return err
		}()},
		{"expires_at", func() error {
			_, err := svc.CreateAccessRequest(ctx, iga.CreateAccessRequestCommand{
				OrganizationID: "org", RequesterID: "user", RoleID: "role", ApplicationID: "app",
				Resource: "res", Action: "read",
				StartsAt: workflowNow, ExpiresAt: workflowNow.Add(-time.Hour),
			})
			return err
		}()},
		{"approval_stage", func() error {
			_, err := svc.ApproveAccessRequest(ctx, "bogus", iga.AccessRequestActionCommand{})
			return err
		}()},
		{"sod_roles", func() error {
			_, err := svc.CreateSODRule(ctx, iga.CreateSODRuleCommand{OrganizationID: "org", Name: "rule", RoleAID: "role", RoleBID: "role"})
			return err
		}()},
		{"lifecycle_status", func() error {
			_, err := svc.UpdateLifecycleStatus(ctx, "org", "life", iga.LifecycleStatus("active"))
			return err
		}()},
	}
	for _, tt := range checks {
		t.Run(tt.name, func(t *testing.T) {
			appErr := apperrors.FromError(tt.err)
			require.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
			require.NotNil(t, appErr.Meta["field"])
		})
	}
}
