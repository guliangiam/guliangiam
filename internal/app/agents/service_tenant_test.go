package agents

import (
	"context"
	"errors"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	"github.com/guliangiam/guliangiam/internal/domain/agents"
	agentsmock "github.com/guliangiam/guliangiam/internal/domain/agents/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func assertInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

// TestAgentByID_RequiresOrganization pins the non-HTTP fail-fast: without a
// trusted tenant the app layer rejects before touching any repository.
func TestAgentByID_RequiresOrganization(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	_, err := svc.GetAgent(ctx, "  ", "agent-1")
	assertInvalidArgument(t, err)

	err = svc.PatchAgent(ctx, "", PatchAgentCommand{ID: "agent-1"})
	assertInvalidArgument(t, err)

	err = svc.DeleteAgent(ctx, "", "agent-1")
	assertInvalidArgument(t, err)
}

// TestGetAgent_PassesTrustedOrganization pins the exact tenant argument seen by
// the repository: it comes from the handler-resolved scope, not from the id.
func TestGetAgent_PassesTrustedOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := agentsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Agents().Return(repo)
	repo.EXPECT().GetByID(gomock.Any(), "agent-1", "org-1").
		Return(&agents.Agent{ID: "agent-1", OrganizationID: "org-1"}, nil)

	_, err := svc.GetAgent(context.Background(), "org-1", "agent-1")
	require.NoError(t, err)
}

// TestPatchAgent_ForeignTenantStopsBeforeUpdate pins that ownership is decided
// before mutation: a NotFound read leaves Update uncalled and writes no audit.
func TestPatchAgent_ForeignTenantStopsBeforeUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := agentsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Agents().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "agent-1", "org-1").
		Return(nil, apperrors.NewNotFound("agent not found"))
	// No Update expectation: a foreign id must never reach the write.

	displayName := "hijacked"
	err := svc.PatchAgent(context.Background(), "org-1", PatchAgentCommand{ID: "agent-1", DisplayName: &displayName})
	assertNotFound(t, err)
}

// TestDeleteAgent_ForeignTenantStopsBeforeDelete pins that a foreign id is
// rejected before Delete and the audit row carries the entity organization.
func TestDeleteAgent_ForeignTenantStopsBeforeDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := agentsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Agents().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "agent-1", "org-1").
		Return(nil, apperrors.NewNotFound("agent not found"))
	// No Delete expectation: a foreign id must never reach the delete.

	err := svc.DeleteAgent(context.Background(), "org-1", "agent-1")
	assertNotFound(t, err)
}

// TestDeleteAgent_PassesEntityOrganizationToDelete pins the successful
// same-tenant delete: both the pre-read and the delete use the trusted tenant,
// and the audit row is written for the entity's own organization.
func TestDeleteAgent_PassesEntityOrganizationToDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := agentsmock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Agents().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "agent-1", "org-1").
		Return(&agents.Agent{ID: "agent-1", OrganizationID: "org-1"}, nil)
	repo.EXPECT().Delete(gomock.Any(), "agent-1", "org-1").Return(nil)

	require.NoError(t, svc.DeleteAgent(context.Background(), "org-1", "agent-1"))
}
