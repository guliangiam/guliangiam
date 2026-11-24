package applications

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// TestManagementUseCases_RequireOrganizationID pins the app-layer fail-fast:
// every by-ID management use case rejects an empty trusted tenant before the
// repository is consulted.
func TestManagementUseCases_RequireOrganizationID(t *testing.T) {
	repo := &mockApplicationRepo{apps: []*domain.Application{
		{ID: "app-1", OrganizationID: "org-1", Slug: "test-app", Revision: 1},
	}}
	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UOW: testUnitOfWork{appRepo: repo}}
	ctx := context.Background()

	cases := map[string]func() error{
		// get 不再 fail-fast：transport 只在超管/系统作用域放行空租户，
		// 应用层改走系统域 GetByID（见 get_application.go）。空租户 + 非系统
		// 作用域的组合在 transport 已被拒绝，到不了应用层。
		"delete-summary": func() error {
			_, err := svc.GetApplicationDeleteSummary(ctx, "", "app-1")
			return err
		},
		"patch": func() error {
			name := "renamed"
			return svc.PatchApplication(ctx, PatchApplicationCommand{ID: "app-1", DisplayName: &name})
		},
		"delete": func() error {
			return svc.DeleteApplication(ctx, DeleteApplicationCommand{ID: "app-1"})
		},
		"provider-bindings": func() error {
			return svc.ReplaceProviderBindings(ctx, "", "app-1", "", nil)
		},
		"export": func() error {
			_, err := svc.ExportApplication(ctx, "", "app-1")
			return err
		},
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			err := call()
			var appErr *apperrors.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
		})
	}

	// 空租户的 GetApplication 走系统域 GetByID（不经过租户过滤的
	// GetByIDForOrganization），其余用例仍不得触达仓储。
	assert.Zero(t, repo.scopedGetCalls, "missing tenant must not reach the scoped repository")
	assert.Len(t, repo.apps, 1, "missing tenant must not mutate the application")
}
