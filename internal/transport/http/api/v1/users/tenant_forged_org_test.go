package users

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	apporders "github.com/guliangiam/guliangiam/internal/app/orders"
	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	ordersdomain "github.com/guliangiam/guliangiam/internal/domain/orders"
	ordersmock "github.com/guliangiam/guliangiam/internal/domain/orders/mock"
	paymentsdomain "github.com/guliangiam/guliangiam/internal/domain/payments"
	paymentsmock "github.com/guliangiam/guliangiam/internal/domain/payments/mock"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	usermock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"go.uber.org/mock/gomock"
)

const (
	forgedOrgSessionID = "11111111-1111-1111-1111-111111111111"
	forgedOrgRequestID = "22222222-2222-2222-2222-222222222222"
)

type forgedOrgClock struct{}

func (forgedOrgClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type forgedOrgUUID struct{}

func (forgedOrgUUID) NewString() (string, error) { return "generated-user-id", nil }

func forgedOrgRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "user-1", OrganizationID: forgedOrgSessionID,
	})
	return req.WithContext(ctx)
}

func forgedOrgRouter(ctrl *gomock.Controller) (chi.Router, *usermock.MockRepository) {
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := usermock.NewMockRepository(ctrl)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	).AnyTimes()
	uow.EXPECT().Users().Return(repo).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appusers.Service{
		Clock: forgedOrgClock{}, UUID: forgedOrgUUID{}, UOW: uow,
	}})
	return router, repo
}

func expectUserCreated(t *testing.T, repo *usermock.MockRepository, username string) {
	t.Helper()
	repo.EXPECT().GetByUsername(gomock.Any(), forgedOrgSessionID, username).
		Return(nil, apperrors.NewNotFound("user not found"))
	repo.EXPECT().HasDeletedWithUsername(gomock.Any(), forgedOrgSessionID, username).
		Return(false, nil)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, user *userdomain.User) error {
			assert.Equal(t, forgedOrgSessionID, user.OrganizationID)
			return nil
		},
	)
}

// TestImportUsers_ForgedOuterAndRowOrganizationIDIgnored pins GAP-011 for
// POST /users/import: neither the query nor a per-row organizationId may move
// the write outside the trusted tenant.
func TestImportUsers_ForgedOuterAndRowOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	expectUserCreated(t, repo, "alice")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost,
		"/api/v1/users/import?organizationId="+forgedOrgRequestID,
		strings.NewReader(`{"users":[{"username":"alice","displayName":"Alice","organizationId":"`+forgedOrgRequestID+`"}]}`)))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestImportUsersRowCannotEscapeTrustedOuterOrg is the positive counterpart:
// even with a trusted outer org, a row that names another tenant is clamped to
// the outer org.
func TestImportUsersRowCannotEscapeTrustedOuterOrg(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	expectUserCreated(t, repo, "bob")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost,
		"/api/v1/users/import?organizationId="+forgedOrgSessionID,
		strings.NewReader(`{"users":[{"username":"bob","displayName":"Bob","organizationId":"`+forgedOrgRequestID+`"}]}`)))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestUploadUsers_ForgedQueryOrganizationIDIgnored pins the Excel import route.
func TestUploadUsers_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	book := excelize.NewFile()
	require.NoError(t, book.SetCellValue("Sheet1", "A2", "1"))
	require.NoError(t, book.SetCellValue("Sheet1", "B2", "seed"))
	require.NoError(t, book.SetCellValue("Sheet1", "C2", "carol"))
	require.NoError(t, book.SetCellValue("Sheet1", "D2", "Carol"))
	require.NoError(t, book.SetCellValue("Sheet1", "G2", "normal"))
	require.NoError(t, book.SetCellValue("Sheet1", "H2", "active"))
	buf, err := book.WriteToBuffer()
	require.NoError(t, err)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "users.xlsx")
	require.NoError(t, err)
	_, err = part.Write(buf.Bytes())
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	expectUserCreated(t, repo, "carol")

	req := forgedOrgRequest(http.MethodPost,
		"/api/v1/users/upload?organizationId="+forgedOrgRequestID, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestUploadUsersCSV_ForgedQueryOrganizationIDIgnored pins the CSV import route.
func TestUploadUsersCSV_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "users.csv")
	require.NoError(t, err)
	_, err = part.Write([]byte("username,displayName\ndave,Dave\n"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	expectUserCreated(t, repo, "dave")

	req := forgedOrgRequest(http.MethodPost,
		"/api/v1/users/upload-csv?organizationId="+forgedOrgRequestID, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestExportUsers_ForgedQueryOrganizationIDIgnored pins the export read path:
// a forged query value must not widen the roster beyond the trusted tenant.
func TestExportUsers_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
			assert.Equal(t, forgedOrgSessionID, query.OrganizationID)
			return nil, 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/users/export?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestImportUsersWithoutTenantScopeRejected pins the fail-fast guard: the
// JSON import must not run with an empty tenant.
func TestImportUsersWithoutTenantScopeRejected(t *testing.T) {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appusers.Service{}})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/import",
		strings.NewReader(`{"users":[{"username":"alice"}]}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

// TestListUsers_ForgedQueryOrganizationIDIgnored pins GET /users.
func TestListUsers_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
			assert.Equal(t, forgedOrgSessionID, query.OrganizationID)
			return nil, 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/users?organizationId="+forgedOrgRequestID+"&keyword=ali", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestGetUserCount_ForgedQueryOrganizationIDIgnored also proves the surviving
// /users/count registration is the trusted handler from chi_import.go.
func TestGetUserCount_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().Count(gomock.Any(), forgedOrgSessionID).Return(int64(3), nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/users/count?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestUserCountRouteRegisteredOnce guards the duplicate-registration contract:
// /users/count must be declared by exactly one non-test source file.
func TestUserCountRouteRegisteredOnce(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	declarations := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		require.NoError(t, err)
		declarations += strings.Count(string(data), `router.Get("/api/v1/users/count"`)
	}
	assert.Equal(t, 1, declarations)
}

func withAdminScope(req *http.Request, scope scopeauthorizer.AdminScope) *http.Request {
	return req.WithContext(scopeauthorizer.WithAdminScope(req.Context(), scope))
}

// otherTenant is a valid UUID that the targeted admin scope does not cover.
const otherTenant = "44444444-4444-4444-4444-444444444444"

// TestListUserOrders_Ownership pins same-tenant horizontal privilege separation
// on GET /users/{id}/orders: self reads pass, cross-user reads require a
// covering platform administrator scope, and denials never touch the repo.
func TestListUserOrders_Ownership(t *testing.T) {
	t.Run("self_read_returns_own_orders", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		router, repo, _ := forgedOrgCommerceRouter(ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, query ordersdomain.ListQuery) ([]*ordersdomain.Order, int64, error) {
				assert.Equal(t, "user-1", query.User)
				return []*ordersdomain.Order{{
					ID: "o-1", OrganizationID: forgedOrgSessionID, User: "user-1",
				}}, int64(1), nil
			},
		)

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet, "/api/v1/users/user-1/orders", nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"o-1"`)
	})

	t.Run("cross_user_without_scope_denied", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		router, repo, _ := forgedOrgCommerceRouter(ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Times(0)

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet, "/api/v1/users/user-2/orders", nil))
		assert.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("cross_user_with_all_scope_allowed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		router, repo, _ := forgedOrgCommerceRouter(ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, query ordersdomain.ListQuery) ([]*ordersdomain.Order, int64, error) {
				assert.Equal(t, "user-2", query.User)
				return []*ordersdomain.Order{{
					ID: "o-2", OrganizationID: forgedOrgSessionID, User: "user-2",
				}}, int64(1), nil
			},
		)

		req := withAdminScope(
			forgedOrgRequest(http.MethodGet, "/api/v1/users/user-2/orders", nil),
			scopeauthorizer.AdminScope{Class: scopeauthorizer.AdminScopeClassAll},
		)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"o-2"`)
	})

	t.Run("targeted_scope_for_other_tenant_denied", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		router, repo, _ := forgedOrgCommerceRouter(ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Times(0)

		req := withAdminScope(
			forgedOrgRequest(http.MethodGet, "/api/v1/users/user-2/orders", nil),
			scopeauthorizer.AdminScope{
				Class:         scopeauthorizer.AdminScopeClassTargeted,
				TargetTenants: []uuid.UUID{uuid.MustParse(otherTenant)},
			},
		)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		assert.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("authorized_cross_tenant_target_allowed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		router, repo, _ := forgedOrgCommerceRouter(ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, query ordersdomain.ListQuery) ([]*ordersdomain.Order, int64, error) {
				assert.Equal(t, "user-2", query.User)
				return []*ordersdomain.Order{{
					ID: "o-3", OrganizationID: forgedOrgSessionID, User: "user-2",
				}}, int64(1), nil
			},
		)

		// ScopeAuthorizerMiddleware injects this only after authorizing an
		// explicit cross-tenant target; the unit router has no middleware, so
		// simulate that trust decision directly.
		req := forgedOrgRequest(http.MethodGet, "/api/v1/users/user-2/orders", nil)
		req = req.WithContext(tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSystem))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"o-3"`)
	})
}

// TestListUserPayments_Ownership mirrors the orders ownership contract for
// GET /users/{id}/payments.
func TestListUserPayments_Ownership(t *testing.T) {
	t.Run("self_read_returns_own_payments", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		router, _, repo := forgedOrgCommerceRouter(ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, query paymentsdomain.ListQuery) ([]*paymentsdomain.Payment, int64, error) {
				assert.Equal(t, "user-1", query.User)
				return []*paymentsdomain.Payment{{
					ID: "p-1", OrganizationID: forgedOrgSessionID, User: "user-1",
				}}, int64(1), nil
			},
		)

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet, "/api/v1/users/user-1/payments", nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"p-1"`)
	})

	t.Run("cross_user_without_scope_denied", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		router, _, repo := forgedOrgCommerceRouter(ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Times(0)

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet, "/api/v1/users/user-2/payments", nil))
		assert.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("cross_user_with_all_scope_allowed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		router, _, repo := forgedOrgCommerceRouter(ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, query paymentsdomain.ListQuery) ([]*paymentsdomain.Payment, int64, error) {
				assert.Equal(t, "user-2", query.User)
				return []*paymentsdomain.Payment{{
					ID: "p-2", OrganizationID: forgedOrgSessionID, User: "user-2",
				}}, int64(1), nil
			},
		)

		req := withAdminScope(
			forgedOrgRequest(http.MethodGet, "/api/v1/users/user-2/payments", nil),
			scopeauthorizer.AdminScope{Class: scopeauthorizer.AdminScopeClassAll},
		)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"p-2"`)
	})

	t.Run("targeted_scope_for_other_tenant_denied", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		router, _, repo := forgedOrgCommerceRouter(ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Times(0)

		req := withAdminScope(
			forgedOrgRequest(http.MethodGet, "/api/v1/users/user-2/payments", nil),
			scopeauthorizer.AdminScope{
				Class:         scopeauthorizer.AdminScopeClassTargeted,
				TargetTenants: []uuid.UUID{uuid.MustParse(otherTenant)},
			},
		)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		assert.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("authorized_cross_tenant_target_allowed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		router, _, repo := forgedOrgCommerceRouter(ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, query paymentsdomain.ListQuery) ([]*paymentsdomain.Payment, int64, error) {
				assert.Equal(t, "user-2", query.User)
				return []*paymentsdomain.Payment{{
					ID: "p-3", OrganizationID: forgedOrgSessionID, User: "user-2",
				}}, int64(1), nil
			},
		)

		req := forgedOrgRequest(http.MethodGet, "/api/v1/users/user-2/payments", nil)
		req = req.WithContext(tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSystem))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"p-3"`)
	})
}

func forgedOrgCommerceRouter(ctrl *gomock.Controller) (chi.Router, *ordersmock.MockRepository, *paymentsmock.MockRepository) {
	uow := appuowmock.NewMockCommerceUoW(ctrl)
	orderRepo := ordersmock.NewMockRepository(ctrl)
	paymentRepo := paymentsmock.NewMockRepository(ctrl)
	uow.EXPECT().Orders().Return(orderRepo).AnyTimes()
	uow.EXPECT().Payments().Return(paymentRepo).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{
		OrdersService:   apporders.Service{UOW: uow},
		PaymentsService: apppayments.Service{UOW: uow},
	})
	return router, orderRepo, paymentRepo
}

// TestListUserOrders_ForgedQueryOrganizationIDIgnored pins the user orders
// sub-resource.
func TestListUserOrders_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo, _ := forgedOrgCommerceRouter(ctrl)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query ordersdomain.ListQuery) ([]*ordersdomain.Order, int64, error) {
			assert.Equal(t, forgedOrgSessionID, query.OrganizationID)
			assert.Equal(t, "user-1", query.User)
			return nil, 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/users/user-1/orders?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestListUserPayments_ForgedQueryOrganizationIDIgnored pins the user payments
// sub-resource.
func TestListUserPayments_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, _, repo := forgedOrgCommerceRouter(ctrl)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query paymentsdomain.ListQuery) ([]*paymentsdomain.Payment, int64, error) {
			assert.Equal(t, forgedOrgSessionID, query.OrganizationID)
			assert.Equal(t, "user-1", query.User)
			return nil, 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/users/user-1/payments?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}
