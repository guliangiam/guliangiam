package radiusaccountings

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/radiusaccountings"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.RadiusAccounting, error) {
	var ra domain.RadiusAccounting
	err := r.db.QueryRow(ctx, QueryGetByID, id, organizationID).Scan(
		&ra.ID, &ra.OrganizationID, &ra.Name, &ra.Username, &ra.ServiceType, &ra.NasID, &ra.NasIPAddr,
		&ra.NasPortID, &ra.NasPortType, &ra.NasPort, &ra.FramedIPAddr, &ra.FramedIPNetmask,
		&ra.AcctSessionID, &ra.AcctSessionTime, &ra.AcctInputTotal, &ra.AcctOutputTotal,
		&ra.AcctInputPackets, &ra.AcctOutputPackets, &ra.AcctTerminateCause, &ra.AcctStartTime,
		&ra.AcctStopTime, &ra.LastUpdate, &ra.CreatedAt, &ra.UpdatedAt, &ra.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("radius accounting not found")
		}
		return nil, err
	}
	return &ra, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.RadiusAccounting, int64, error) {
	offset := shared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.Query(ctx, QueryList, q.OrganizationID, q.Keyword, q.Username, shared.ClampPageSize(q.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.RadiusAccounting, 0)
	for rows.Next() {
		var ra domain.RadiusAccounting
		if err := rows.Scan(&ra.ID, &ra.OrganizationID, &ra.Name, &ra.Username, &ra.AcctSessionID, &ra.AcctStartTime, &ra.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, &ra)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, QueryCount, q.OrganizationID, q.Keyword, q.Username).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, ra *domain.RadiusAccounting) error {
	_, err := r.db.Exec(ctx, QueryInsert,
		ra.ID, ra.OrganizationID, ra.Name, ra.Username, ra.ServiceType, ra.NasID, ra.NasIPAddr,
		ra.NasPortID, ra.NasPortType, ra.NasPort, ra.FramedIPAddr, ra.FramedIPNetmask,
		ra.AcctSessionID, ra.AcctSessionTime, ra.AcctInputTotal, ra.AcctOutputTotal,
		ra.AcctInputPackets, ra.AcctOutputPackets, ra.AcctTerminateCause, ra.AcctStartTime,
		ra.AcctStopTime, ra.LastUpdate, ra.CreatedAt, ra.UpdatedAt, ra.Revision,
	)
	return err
}

func (r Repository) Update(ctx context.Context, ra *domain.RadiusAccounting) error {
	tag, err := r.db.Exec(ctx, QueryUpdate,
		ra.AcctSessionTime, ra.AcctInputTotal, ra.AcctOutputTotal, ra.AcctInputPackets,
		ra.AcctOutputPackets, ra.AcctTerminateCause, ra.AcctStopTime, ra.LastUpdate,
		ra.UpdatedAt, ra.Revision, ra.ID, ra.OrganizationID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("radius accounting not found")
	}
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, QueryDelete, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("radius accounting not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
