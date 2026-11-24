package platformadmins

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appplatformadmins "github.com/guliangiam/guliangiam/internal/app/platform_admins"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

const (
	testOperatorUserID  = "01a0ea76-0c00-7000-8000-0000000000a1"
	testOperatorAdminID = "01a0ea76-0c00-7000-8000-0000000000a2"
	testTargetUserID    = "01a0ea76-0c00-7000-8000-0000000000b1"
	testAdminID         = "01a0ea76-0c00-7000-8000-0000000000b2"
	testScopeID         = "01a0ea76-0c00-7000-8000-0000000000c1"
)

// stubService 是 PlatformAdminService 窄接口的测试替身：记录收到的命令，
// 用于断言审计字段来自可信身份而不是请求体。
type stubService struct {
	admin  *appplatformadmins.PlatformAdminDetail
	admins []appplatformadmins.PlatformAdminDetail
	total  int64
	scope  *appplatformadmins.ScopeDetail
	scopes []appplatformadmins.ScopeDetail
	err    error

	byUserID    map[string]*appplatformadmins.PlatformAdminDetail
	byUserIDErr error

	gotCreate      appplatformadmins.CreatePlatformAdminCommand
	gotUpdate      appplatformadmins.UpdatePlatformAdminCommand
	gotSetActive   appplatformadmins.SetActiveCommand
	gotDeleteID    string
	gotListQuery   appplatformadmins.ListPlatformAdminsQuery
	gotScopeCreate appplatformadmins.CreateScopeCommand
	gotApprove     appplatformadmins.ApproveScopeCommand
	gotDeleteScope string
	gotScopesQuery appplatformadmins.ListScopesQuery
}

var _ PlatformAdminService = (*stubService)(nil)

func (s *stubService) CreatePlatformAdmin(_ context.Context, cmd appplatformadmins.CreatePlatformAdminCommand) (*appplatformadmins.PlatformAdminDetail, error) {
	s.gotCreate = cmd
	return s.admin, s.err
}
func (s *stubService) UpdatePlatformAdmin(_ context.Context, cmd appplatformadmins.UpdatePlatformAdminCommand) (*appplatformadmins.PlatformAdminDetail, error) {
	s.gotUpdate = cmd
	return s.admin, s.err
}
func (s *stubService) SetActive(_ context.Context, cmd appplatformadmins.SetActiveCommand) error {
	s.gotSetActive = cmd
	return s.err
}
func (s *stubService) DeletePlatformAdmin(_ context.Context, cmd appplatformadmins.DeletePlatformAdminCommand) error {
	s.gotDeleteID = cmd.ID
	return s.err
}
func (s *stubService) ListPlatformAdmins(_ context.Context, q appplatformadmins.ListPlatformAdminsQuery) ([]appplatformadmins.PlatformAdminDetail, int64, error) {
	s.gotListQuery = q
	return s.admins, s.total, s.err
}
func (s *stubService) GetPlatformAdmin(_ context.Context, _ string) (*appplatformadmins.PlatformAdminDetail, error) {
	return s.admin, s.err
}
func (s *stubService) GetPlatformAdminByUserID(_ context.Context, userID string) (*appplatformadmins.PlatformAdminDetail, error) {
	if s.byUserIDErr != nil {
		return nil, s.byUserIDErr
	}
	return s.byUserID[userID], nil
}
func (s *stubService) CreateScope(_ context.Context, cmd appplatformadmins.CreateScopeCommand) (*appplatformadmins.ScopeDetail, error) {
	s.gotScopeCreate = cmd
	return s.scope, s.err
}
func (s *stubService) ApproveScope(_ context.Context, cmd appplatformadmins.ApproveScopeCommand) error {
	s.gotApprove = cmd
	return s.err
}
func (s *stubService) DeleteScope(_ context.Context, cmd appplatformadmins.DeleteScopeCommand) error {
	s.gotDeleteScope = cmd.ScopeID
	return s.err
}
func (s *stubService) ListScopes(_ context.Context, q appplatformadmins.ListScopesQuery) ([]appplatformadmins.ScopeDetail, error) {
	s.gotScopesQuery = q
	return s.scopes, s.err
}
func (s *stubService) GetScope(_ context.Context, _ string) (*appplatformadmins.ScopeDetail, error) {
	return s.scope, s.err
}

func operatorContext() context.Context {
	return authtransport.WithSessionContext(context.Background(), authtransport.SessionContext{
		UserID: testOperatorUserID,
	})
}

func activeOperatorStub() *stubService {
	return &stubService{
		byUserID: map[string]*appplatformadmins.PlatformAdminDetail{
			testOperatorUserID: {ID: testOperatorAdminID, UserID: testOperatorUserID, IsActive: true},
		},
	}
}

func httpStatusOf(t *testing.T, err error) int {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "want *apperrors.Error, got %T: %v", err, err)
	return appErr.HTTPStatus()
}

// BE-A-05：未接线时所有端点必须 fail-closed 501，而不是 panic/静默成功。
func TestUnwired_FailClosed501(t *testing.T) {
	h := Handler{}
	ctx := operatorContext()
	active := true

	_, err := h.List(ctx, appplatformadmins.ListPlatformAdminsQuery{})
	assert.Equal(t, 501, httpStatusOf(t, err))
	_, err = h.Get(ctx, testAdminID)
	assert.Equal(t, 501, httpStatusOf(t, err))
	_, err = h.Create(ctx, CreatePlatformAdminRequest{UserID: testTargetUserID})
	assert.Equal(t, 501, httpStatusOf(t, err))
	_, err = h.Patch(ctx, testAdminID, PatchPlatformAdminRequest{})
	assert.Equal(t, 501, httpStatusOf(t, err))
	assert.Equal(t, 501, httpStatusOf(t, h.SetActive(ctx, testAdminID, &active)))
	assert.Equal(t, 501, httpStatusOf(t, h.Delete(ctx, testAdminID)))
	_, err = h.ListScopes(ctx, testAdminID, false)
	assert.Equal(t, 501, httpStatusOf(t, err))
	_, err = h.CreateScope(ctx, testAdminID, CreateScopeRequest{})
	assert.Equal(t, 501, httpStatusOf(t, err))
	_, err = h.GetScope(ctx, testScopeID)
	assert.Equal(t, 501, httpStatusOf(t, err))
	assert.Equal(t, 501, httpStatusOf(t, h.ApproveScope(ctx, testScopeID, ApproveScopeRequest{})))
	assert.Equal(t, 501, httpStatusOf(t, h.DeleteScope(ctx, testScopeID)))
}

func TestCreate_CreatedByFromTrustedOperator(t *testing.T) {
	stub := activeOperatorStub()
	stub.admin = &appplatformadmins.PlatformAdminDetail{ID: testAdminID, UserID: testTargetUserID}
	h := NewHandler(stub)

	detail, err := h.Create(operatorContext(), CreatePlatformAdminRequest{UserID: testTargetUserID})
	require.NoError(t, err)
	assert.Equal(t, testAdminID, detail.ID)
	assert.Equal(t, testTargetUserID, stub.gotCreate.UserID)
	// 审计字段是操作者的 users.id（FK→users.id），不是 body 声称值。
	assert.Equal(t, testOperatorUserID, stub.gotCreate.CreatedBy)
}

func TestCreate_RequiresSession(t *testing.T) {
	h := NewHandler(activeOperatorStub())
	_, err := h.Create(context.Background(), CreatePlatformAdminRequest{UserID: testTargetUserID})
	assert.Equal(t, 401, httpStatusOf(t, err))
}

func TestCreate_OperatorNotRegistered_FailClosed403(t *testing.T) {
	stub := activeOperatorStub()
	stub.byUserIDErr = apperrors.NewNotFound("platform admin not found")
	h := NewHandler(stub)
	_, err := h.Create(operatorContext(), CreatePlatformAdminRequest{UserID: testTargetUserID})
	assert.Equal(t, 403, httpStatusOf(t, err))
}

func TestCreate_OperatorInactive_FailClosed403(t *testing.T) {
	stub := activeOperatorStub()
	stub.byUserID[testOperatorUserID] = &appplatformadmins.PlatformAdminDetail{ID: testOperatorAdminID, IsActive: false}
	h := NewHandler(stub)
	_, err := h.Create(operatorContext(), CreatePlatformAdminRequest{UserID: testTargetUserID})
	assert.Equal(t, 403, httpStatusOf(t, err))
}

func TestCreate_Validation400(t *testing.T) {
	h := NewHandler(activeOperatorStub())
	_, err := h.Create(operatorContext(), CreatePlatformAdminRequest{})
	assert.Equal(t, 400, httpStatusOf(t, err))
	_, err = h.Create(operatorContext(), CreatePlatformAdminRequest{UserID: "not-a-uuid"})
	assert.Equal(t, 400, httpStatusOf(t, err))
}

func TestCreate_ConflictPropagates409(t *testing.T) {
	stub := activeOperatorStub()
	stub.err = apperrors.NewConflict("user is already registered as platform admin")
	h := NewHandler(stub)
	_, err := h.Create(operatorContext(), CreatePlatformAdminRequest{UserID: testTargetUserID})
	assert.Equal(t, 409, httpStatusOf(t, err))
}

func TestGet_NotFoundPropagates404(t *testing.T) {
	stub := activeOperatorStub()
	stub.err = apperrors.NewNotFound("platform admin not found")
	h := NewHandler(stub)
	_, err := h.Get(operatorContext(), testAdminID)
	assert.Equal(t, 404, httpStatusOf(t, err))
}

func TestGet_InvalidID400(t *testing.T) {
	h := NewHandler(activeOperatorStub())
	_, err := h.Get(operatorContext(), "not-a-uuid")
	assert.Equal(t, 400, httpStatusOf(t, err))
}

func TestPatch_CreatedByPinnedToOperator(t *testing.T) {
	stub := activeOperatorStub()
	stub.admin = &appplatformadmins.PlatformAdminDetail{ID: testAdminID}
	h := NewHandler(stub)
	_, err := h.Patch(operatorContext(), testAdminID, PatchPlatformAdminRequest{})
	require.NoError(t, err)
	assert.Equal(t, testAdminID, stub.gotUpdate.ID)
	assert.Equal(t, testOperatorUserID, stub.gotUpdate.CreatedBy)
}

func TestSetActive_RequiresActiveFlag(t *testing.T) {
	h := NewHandler(activeOperatorStub())
	assert.Equal(t, 400, httpStatusOf(t, h.SetActive(operatorContext(), testAdminID, nil)))
}

func TestSetActive_PassesCommand(t *testing.T) {
	stub := activeOperatorStub()
	h := NewHandler(stub)
	active := false
	require.NoError(t, h.SetActive(operatorContext(), testAdminID, &active))
	assert.Equal(t, testAdminID, stub.gotSetActive.ID)
	assert.False(t, stub.gotSetActive.Active)
}

func TestDelete_PassesID(t *testing.T) {
	stub := activeOperatorStub()
	h := NewHandler(stub)
	require.NoError(t, h.Delete(operatorContext(), testAdminID))
	assert.Equal(t, testAdminID, stub.gotDeleteID)
}

func TestList_MapsQueryAndPagination(t *testing.T) {
	stub := activeOperatorStub()
	stub.admins = []appplatformadmins.PlatformAdminDetail{{ID: testAdminID}}
	stub.total = 1
	h := NewHandler(stub)
	active := true
	q := appplatformadmins.ListPlatformAdminsQuery{Page: 2, PageSize: 5, Active: &active, Keyword: "kw"}
	response, err := h.List(operatorContext(), q)
	require.NoError(t, err)
	assert.Equal(t, q, stub.gotListQuery)
	require.Len(t, response.Items, 1)
	assert.Equal(t, int64(1), response.Page.Total)
	assert.Equal(t, 2, response.Page.Page)
}

func TestListScopes_EnvelopeAndQuery(t *testing.T) {
	stub := activeOperatorStub()
	stub.scopes = []appplatformadmins.ScopeDetail{{ID: testScopeID}}
	h := NewHandler(stub)
	response, err := h.ListScopes(operatorContext(), testAdminID, true)
	require.NoError(t, err)
	assert.Equal(t, testAdminID, stub.gotScopesQuery.PlatformAdminID)
	assert.True(t, stub.gotScopesQuery.OnlyActive)
	require.Len(t, response.Items, 1)
	assert.Equal(t, int64(1), response.Page.Total)
}

func TestCreateScope_ApproverAndAuditFromTrustedIdentity(t *testing.T) {
	stub := activeOperatorStub()
	stub.scope = &appplatformadmins.ScopeDetail{ID: testScopeID}
	h := NewHandler(stub)
	expires := time.Now().UTC().Add(24 * time.Hour)
	_, err := h.CreateScope(operatorContext(), testAdminID, CreateScopeRequest{
		ScopeType:     "targeted",
		TargetTenants: []string{"tenant-1"},
		ExpiresAt:     expires,
	})
	require.NoError(t, err)
	assert.Equal(t, testAdminID, stub.gotScopeCreate.PlatformAdminID)
	require.NotNil(t, stub.gotScopeCreate.ApprovedBy)
	assert.Equal(t, testOperatorUserID, *stub.gotScopeCreate.ApprovedBy)
	assert.Equal(t, testOperatorUserID, stub.gotScopeCreate.CreatedBy)
	assert.Equal(t, expires, stub.gotScopeCreate.ExpiresAt)
}

func TestCreateScope_RequiresSession(t *testing.T) {
	h := NewHandler(activeOperatorStub())
	_, err := h.CreateScope(context.Background(), testAdminID, CreateScopeRequest{ScopeType: "all"})
	assert.Equal(t, 401, httpStatusOf(t, err))
}

func TestCreateScope_SelfApprovalPropagates403(t *testing.T) {
	stub := activeOperatorStub()
	stub.err = apperrors.NewForbidden("no self-approval: cannot approve your own scope")
	h := NewHandler(stub)
	_, err := h.CreateScope(operatorContext(), testOperatorAdminID, CreateScopeRequest{ScopeType: "all"})
	assert.Equal(t, 403, httpStatusOf(t, err))
}

func TestApproveScope_ApproverFromTrustedOperator(t *testing.T) {
	stub := activeOperatorStub()
	h := NewHandler(stub)
	require.NoError(t, h.ApproveScope(operatorContext(), testScopeID, ApproveScopeRequest{}))
	assert.Equal(t, testScopeID, stub.gotApprove.ScopeID)
	assert.Equal(t, testOperatorUserID, stub.gotApprove.ApproverID)
}

func TestApproveScope_RequiresSession(t *testing.T) {
	h := NewHandler(activeOperatorStub())
	assert.Equal(t, 401, httpStatusOf(t, h.ApproveScope(context.Background(), testScopeID, ApproveScopeRequest{})))
}

func TestDeleteScope_PassesID(t *testing.T) {
	stub := activeOperatorStub()
	h := NewHandler(stub)
	require.NoError(t, h.DeleteScope(operatorContext(), testScopeID))
	assert.Equal(t, testScopeID, stub.gotDeleteScope)
}
