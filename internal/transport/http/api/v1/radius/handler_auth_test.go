package radius

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/guliangiam/guliangiam/cred"
	appradius "github.com/guliangiam/guliangiam/internal/app/radius"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	orgmock "github.com/guliangiam/guliangiam/internal/domain/organizations/mock"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	usersmock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// BE-B-5: a bare username (no org slug) is rejected without any repository read.
func TestHandler_AuthenticateUser_RequiresOrgSlug(t *testing.T) {
	h := &Handler{}
	ok, err := h.authenticateUser(context.Background(), "alice", "pw")
	assert.NoError(t, err)
	assert.False(t, ok)
}

// BE-B-5 regression: the slug→tenant resolution runs under System scope and the
// credential read runs under the resolved tenant's strict scope. With the old
// scope-less packet context both FORCE-RLS reads failed closed and every
// RADIUS authentication was rejected.
func TestHandler_AuthenticateUser_ResolvesTenantScopes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := appuowmock.NewMockUnitOfWork(ctrl)
	orgRepo := orgmock.NewMockRepository(ctrl)
	userRepo := usersmock.NewMockRepository(ctrl)

	uowMock.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			assert.Equal(t, tenantctx.TenantFilterSystem, tenantctx.TenantScopeFromContext(ctx).Mode,
				"slug resolution must run under System scope")
			return fn(ctx, uowMock)
		})
	uowMock.EXPECT().Organizations().Return(orgRepo)
	orgRepo.EXPECT().GetBySlug(gomock.Any(), "acme").Return(&orgdomain.Organization{ID: "org-1", Slug: "acme"}, nil)

	hash := cred.GetCredManager("bcrypt").GetHashedPassword("dummy-test-password", "")
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().GetByUsername(gomock.Any(), "org-1", "alice").DoAndReturn(
		func(ctx context.Context, orgID, username string) (*users.User, error) {
			scope := tenantctx.TenantScopeFromContext(ctx)
			assert.Equal(t, tenantctx.TenantFilterStrict, scope.Mode,
				"credential read must run under the resolved tenant scope")
			assert.Equal(t, "org-1", scope.OrganizationID)
			return &users.User{
				ID:             "user-1",
				OrganizationID: "org-1",
				Username:       "alice",
				Credentials: users.UserCredentials{
					HasPassword:  true,
					PasswordHash: hash,
					PasswordAlgo: "bcrypt",
				},
			}, nil
		})

	h := &Handler{UOW: uowMock}
	ok, err := h.authenticateUser(context.Background(), "acme/alice", "dummy-test-password")
	assert.NoError(t, err)
	assert.True(t, ok, "valid credentials under the resolved tenant must authenticate")
}

// BE-B-5: an unresolvable slug fails closed without touching the user store.
func TestHandler_AuthenticateUser_OrgLookupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := appuowmock.NewMockUnitOfWork(ctrl)
	orgRepo := orgmock.NewMockRepository(ctrl)

	uowMock.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uowMock)
		})
	uowMock.EXPECT().Organizations().Return(orgRepo)
	orgRepo.EXPECT().GetBySlug(gomock.Any(), "ghost").Return(nil, assert.AnError)

	h := &Handler{UOW: uowMock}
	ok, err := h.authenticateUser(context.Background(), "ghost/alice", "pw")
	assert.NoError(t, err)
	assert.False(t, ok)
}

// lifecycleFakeServer implements appradius.Server plus the structural
// SetAcctHandler surface the transport binds through.
type lifecycleFakeServer struct {
	startCtx context.Context
	started  bool
	stopped  bool
	acct     func(ctx context.Context, username, sessionID, eventType string, bytesIn, bytesOut int64)
}

func (s *lifecycleFakeServer) SetAuthHandler(func(ctx context.Context, username, password string) (bool, error)) {
}

func (s *lifecycleFakeServer) SetAcctHandler(fn func(ctx context.Context, username, sessionID, eventType string, bytesIn, bytesOut int64)) {
	s.acct = fn
}

func (s *lifecycleFakeServer) Start(ctx context.Context) error {
	s.startCtx = ctx
	s.started = true
	return nil
}

func (s *lifecycleFakeServer) Stop() error {
	s.stopped = true
	return nil
}

// BE-B-13 regression: StartServer wires the accounting handler and starts the
// server on a detached lifecycle context — cancelling the HTTP request context
// must not stop the server; StopServer does.
func TestHandler_StartServer_DetachedLifecycleAndAcctWiring(t *testing.T) {
	srv := &lifecycleFakeServer{}
	h := &Handler{
		RadiusSvc: appradius.NewService(func(addr, secret string) appradius.Server { return srv }),
		AccountingHandler: func(ctx context.Context, username, sessionID, eventType string, bytesIn, bytesOut int64) {
		},
	}

	reqCtx, cancelReq := context.WithCancel(context.Background())
	require.NoError(t, h.StartServer(reqCtx, ":1812", "shared"))
	require.True(t, srv.started)
	require.NotNil(t, srv.acct, "accounting handler must be wired")

	cancelReq()
	assert.NoError(t, srv.startCtx.Err(), "server lifecycle must survive the HTTP request context")

	require.NoError(t, h.StopServer())
	assert.True(t, srv.stopped)
	assert.Error(t, srv.startCtx.Err(), "StopServer must cancel the lifecycle context")
	assert.False(t, h.GetStatus().Running)
}

// Without an injected accounting handler the server keeps its ack-only default
// (no structural assertion is attempted).
func TestHandler_StartServer_NoAccountingHandler(t *testing.T) {
	srv := &lifecycleFakeServer{}
	h := &Handler{
		RadiusSvc: appradius.NewService(func(addr, secret string) appradius.Server { return srv }),
	}
	require.NoError(t, h.StartServer(context.Background(), ":1812", "shared"))
	assert.Nil(t, srv.acct)
	require.NoError(t, h.StopServer())
}
