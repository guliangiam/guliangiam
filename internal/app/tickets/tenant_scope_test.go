package tickets

import (
	"context"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/tickets"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestByIDRequiresOrganizationScope pins the app-level fail-fast for the four
// by-ID faces: an empty trusted tenant is rejected before the UOW or repository
// is touched, so a caller cannot degrade into "all tenants".
func TestByIDRequiresOrganizationScope(t *testing.T) {
	svc := Service{}
	tests := []struct {
		name string
		call func() error
	}{
		{"get", func() error {
			_, err := svc.GetTicket(context.Background(), "", "ticket-1")
			return err
		}},
		{"patch", func() error {
			return svc.PatchTicket(context.Background(), PatchTicketCommand{ID: "ticket-1"})
		}},
		{"delete", func() error {
			return svc.DeleteTicket(context.Background(), "", "ticket-1")
		}},
		{"messages", func() error {
			_, _, err := svc.ListTicketMessages(context.Background(), "", "ticket-1", 1, 10)
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			require.Error(t, err)
			assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
		})
	}
}

// regressedScopeRepo simulates a storage predicate that no longer filters by
// organization_id and therefore hands back a foreign row.
type regressedScopeRepo struct {
	ticket  *domain.Ticket
	updates int
	deletes int
}

func (r *regressedScopeRepo) GetByID(_ context.Context, _, _ string) (*domain.Ticket, error) {
	return r.ticket, nil
}

func (r *regressedScopeRepo) List(_ context.Context, _ domain.ListQuery) ([]*domain.Ticket, int64, error) {
	return nil, 0, nil
}

func (r *regressedScopeRepo) Create(_ context.Context, _ *domain.Ticket) error { return nil }

func (r *regressedScopeRepo) Update(_ context.Context, _ *domain.Ticket) error {
	r.updates++
	return nil
}

func (r *regressedScopeRepo) Delete(_ context.Context, _, _ string) error {
	r.deletes++
	return nil
}

// TestByIDRejectsForeignEntityReturnedByRepository pins the app-level entity
// check: even if a storage dialect regresses its tenant predicate, a foreign
// ticket stays indistinguishable from a missing one and is never written.
func TestByIDRejectsForeignEntityReturnedByRepository(t *testing.T) {
	const sessionOrg = "session-org"
	foreign := &domain.Ticket{
		ID: "foreign-ticket", OrganizationID: "other-org",
		Status: domain.StatusOpen, Revision: 1,
	}
	repo := &regressedScopeRepo{ticket: foreign}

	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockCommerceUoW(ctrl)
	tx := appuowmock.NewMockUnitOfWork(ctrl)
	uow.EXPECT().Tickets().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, tx)
		},
	).AnyTimes()
	tx.EXPECT().Tickets().Return(repo).AnyTimes()

	svc := Service{Clock: fakeClock{}, UOW: uow}
	tests := []struct {
		name string
		call func() error
	}{
		{"get", func() error {
			_, err := svc.GetTicket(context.Background(), sessionOrg, foreign.ID)
			return err
		}},
		{"patch", func() error {
			return svc.PatchTicket(context.Background(), PatchTicketCommand{ID: foreign.ID, OrganizationID: sessionOrg})
		}},
		{"delete", func() error {
			return svc.DeleteTicket(context.Background(), sessionOrg, foreign.ID)
		}},
		{"messages", func() error {
			_, _, err := svc.ListTicketMessages(context.Background(), sessionOrg, foreign.ID, 1, 10)
			return err
		}},
		{"add-message", func() error {
			_, err := svc.AddTicketMessage(context.Background(), sessionOrg, foreign.ID, "agent", "reply", "")
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			require.Error(t, err)
			assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
		})
	}
	assert.Zero(t, repo.updates)
	assert.Zero(t, repo.deletes)
}
