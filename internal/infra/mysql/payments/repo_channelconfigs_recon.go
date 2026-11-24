package payments

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ─── ChannelConfigsRepo ────────────────────────────────────────────────

type ChannelConfigsRepository struct{ db mysqlshared.DBTX }

func NewChannelConfigsRepository(db mysqlshared.DBTX) ChannelConfigsRepository {
	return ChannelConfigsRepository{db: db}
}

var _ domain.ChannelConfigsRepo = ChannelConfigsRepository{}

func (r ChannelConfigsRepository) GetByID(ctx context.Context, id string) (*domain.PayChannelConfig, error) {
	c, err := scanChannelConfig(r.db.QueryRowContext(ctx, queryChannelConfigGetByID, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("channel config not found")
		}
		return nil, err
	}
	return c, nil
}

func (r ChannelConfigsRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.PayChannelConfig, error) {
	c, err := scanChannelConfig(r.db.QueryRowContext(ctx, queryChannelConfigGetByIDForOrganization, id, organizationID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("channel config not found")
		}
		return nil, err
	}
	return c, nil
}

func (r ChannelConfigsRepository) GetActive(ctx context.Context, organizationID, channel, environment string) (*domain.PayChannelConfig, error) {
	c, err := scanChannelConfig(r.db.QueryRowContext(ctx, queryChannelConfigGetActive, organizationID, channel, environment))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("no active channel config for " + channel)
		}
		return nil, err
	}
	return c, nil
}

func (r ChannelConfigsRepository) List(ctx context.Context, organizationID string) ([]*domain.PayChannelConfig, error) {
	rows, err := r.db.QueryContext(ctx, queryChannelConfigList, organizationID, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*domain.PayChannelConfig
	for rows.Next() {
		c, err := scanChannelConfig(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r ChannelConfigsRepository) Create(ctx context.Context, c *domain.PayChannelConfig) error {
	var configSetStr sql.NullString
	if len(c.ConfigSet) > 0 {
		b, err := json.Marshal(c.ConfigSet)
		if err != nil {
			return apperrors.WrapInternalf(err, "marshal config_set")
		}
		configSetStr = sql.NullString{String: string(b), Valid: true}
	}
	var testPassedAt sql.NullTime
	if c.TestPassedAt != nil {
		testPassedAt = sql.NullTime{Time: *c.TestPassedAt, Valid: true}
	}
	var activatedAt sql.NullTime
	if c.ActivatedAt != nil {
		activatedAt = sql.NullTime{Time: *c.ActivatedAt, Valid: true}
	}
	var disabledAt sql.NullTime
	if c.DisabledAt != nil {
		disabledAt = sql.NullTime{Time: *c.DisabledAt, Valid: true}
	}
	var deletedAt sql.NullTime
	if c.DeletedAt != nil {
		deletedAt = sql.NullTime{Time: *c.DeletedAt, Valid: true}
	}
	providerID := sql.NullString{String: c.ProviderID, Valid: c.ProviderID != ""}
	_, err := r.db.ExecContext(ctx, queryChannelConfigInsert,
		c.ID, c.OrganizationID, providerID, c.Channel, c.Environment,
		c.ConfigSealed, c.ConfigFingerprint, configSetStr,
		c.VersionNo, c.Status,
		testPassedAt, activatedAt, disabledAt,
		c.CreatedAt, c.UpdatedAt, deletedAt,
		c.Revision,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "insert channel config")
	}
	return nil
}

func (r ChannelConfigsRepository) Update(ctx context.Context, c *domain.PayChannelConfig) error {
	var testPassedAt sql.NullTime
	if c.TestPassedAt != nil {
		testPassedAt = sql.NullTime{Time: *c.TestPassedAt, Valid: true}
	}
	var activatedAt sql.NullTime
	if c.ActivatedAt != nil {
		activatedAt = sql.NullTime{Time: *c.ActivatedAt, Valid: true}
	}
	var disabledAt sql.NullTime
	if c.DisabledAt != nil {
		disabledAt = sql.NullTime{Time: *c.DisabledAt, Valid: true}
	}
	newRevision := c.Revision + 1
	res, err := r.db.ExecContext(ctx, queryChannelConfigUpdate,
		c.Status, testPassedAt, activatedAt, disabledAt,
		c.UpdatedAt, newRevision, c.ID, c.Revision, c.OrganizationID,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "update channel config")
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return apperrors.NewConflictf("channel config %s update conflict", c.ID)
	}
	c.Revision = newRevision
	return nil
}

// ─── ReconRepo ────────────────────────────────────────────────────────────

type ReconRepository struct{ db mysqlshared.DBTX }

func NewReconRepository(db mysqlshared.DBTX) ReconRepository { return ReconRepository{db: db} }

var _ domain.ReconRepo = ReconRepository{}

func (r ReconRepository) Batches(ctx context.Context, organizationID string, limit, offset int) ([]*domain.ReconBatch, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, reconBatchCount, organizationID, organizationID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, reconBatchList, organizationID, organizationID, mysqlshared.ClampPageSize(limit), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []*domain.ReconBatch
	for rows.Next() {
		b, err := scanReconBatch(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, b)
	}
	return items, total, rows.Err()
}

func (r ReconRepository) GetBatch(ctx context.Context, id string) (*domain.ReconBatch, error) {
	b, err := scanReconBatch(r.db.QueryRowContext(ctx, reconBatchGetByID, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("recon batch not found")
		}
		return nil, err
	}
	return b, nil
}

func (r ReconRepository) GetBatchForOrganization(ctx context.Context, id, organizationID string) (*domain.ReconBatch, error) {
	b, err := scanReconBatch(r.db.QueryRowContext(ctx, reconBatchGetByIDForOrganization, id, organizationID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("recon batch not found")
		}
		return nil, err
	}
	return b, nil
}

func (r ReconRepository) CreateBatch(ctx context.Context, b *domain.ReconBatch) error {
	var deletedAt sql.NullTime
	if b.DeletedAt != nil {
		deletedAt = sql.NullTime{Time: *b.DeletedAt, Valid: true}
	}
	_, err := r.db.ExecContext(ctx, reconBatchInsert,
		b.ID, b.OrganizationID, b.Channel, b.BatchDate, b.Status,
		b.DiffCount, b.TotalLocal, b.TotalChannel,
		b.CreatedAt, b.UpdatedAt, deletedAt, b.Revision,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "insert recon batch")
	}
	return nil
}

func (r ReconRepository) UpdateBatch(ctx context.Context, b *domain.ReconBatch) error {
	newRevision := b.Revision + 1
	res, err := r.db.ExecContext(ctx, reconBatchUpdate,
		b.Status, b.DiffCount, b.TotalLocal, b.TotalChannel,
		b.UpdatedAt, newRevision, b.ID, b.Revision,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "update recon batch")
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return apperrors.NewConflictf("recon batch %s update conflict", b.ID)
	}
	b.Revision = newRevision
	return nil
}

func (r ReconRepository) CreateItem(ctx context.Context, item *domain.ReconItem) error {
	var channelOrderNo sql.NullString
	if item.ChannelOrderNo != nil {
		channelOrderNo = sql.NullString{String: *item.ChannelOrderNo, Valid: true}
	}
	var localPaymentID sql.NullString
	if item.LocalPaymentID != nil {
		localPaymentID = sql.NullString{String: *item.LocalPaymentID, Valid: true}
	}
	var channelAmount sql.NullFloat64
	if item.ChannelAmount != nil {
		channelAmount = sql.NullFloat64{Float64: *item.ChannelAmount, Valid: true}
	}
	var localAmount sql.NullFloat64
	if item.LocalAmount != nil {
		localAmount = sql.NullFloat64{Float64: *item.LocalAmount, Valid: true}
	}
	var channelState sql.NullString
	if item.ChannelState != nil {
		channelState = sql.NullString{String: *item.ChannelState, Valid: true}
	}
	var localState sql.NullString
	if item.LocalState != nil {
		localState = sql.NullString{String: *item.LocalState, Valid: true}
	}
	// resolved_by / note 有 DEFAULT 值，通常在 item.Resolved 时才设置
	var resolvedBy sql.NullString
	if item.ResolvedBy != nil {
		resolvedBy = sql.NullString{String: *item.ResolvedBy, Valid: true}
	}
	var resolvedAt sql.NullTime
	if item.ResolvedAt != nil {
		resolvedAt = sql.NullTime{Time: *item.ResolvedAt, Valid: true}
	}
	var note sql.NullString
	if item.Note != nil {
		note = sql.NullString{String: *item.Note, Valid: true}
	}
	_, err := r.db.ExecContext(ctx, reconItemInsert,
		item.ID, item.BatchID, item.DiffType, channelOrderNo, localPaymentID,
		channelAmount, localAmount, channelState, localState,
		item.Resolved, resolvedBy, resolvedAt, note, item.CreatedAt,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "insert recon item")
	}
	return nil
}

func (r ReconRepository) ListItems(ctx context.Context, batchID string) ([]*domain.ReconItem, error) {
	return r.listItems(ctx, reconItemList, batchID)
}

func (r ReconRepository) ListItemsForOrganization(ctx context.Context, batchID, organizationID string) ([]*domain.ReconItem, error) {
	return r.listItems(ctx, reconItemListForOrganization, batchID, organizationID)
}

func (r ReconRepository) listItems(ctx context.Context, query string, args ...any) ([]*domain.ReconItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*domain.ReconItem
	for rows.Next() {
		i, err := scanReconItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func (r ReconRepository) ApproveItemForOrganization(ctx context.Context, itemID, organizationID, approver string) (string, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, reconItemApproveForOrganization, approver, now, itemID, approver, organizationID)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "approve recon item")
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return "", apperrors.NewUnprocessableEntity("recon item must be resolved, unapproved, and approver must differ from resolver")
	}
	var resolvedBy sql.NullString
	if err := r.db.QueryRowContext(ctx, reconItemResolvedBy, itemID).Scan(&resolvedBy); err != nil {
		return "", apperrors.WrapInternalf(err, "load recon resolver")
	}
	if !resolvedBy.Valid {
		return "", nil
	}
	return resolvedBy.String, nil
}

func (r ReconRepository) ResolveItem(ctx context.Context, itemID, operator string) error {
	now := time.Now().UTC()
	var emptyNote sql.NullString
	res, err := r.db.ExecContext(ctx, reconItemResolve, operator, now, emptyNote, itemID)
	if err != nil {
		return apperrors.WrapInternalf(err, "resolve recon item")
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return apperrors.NewInvalidArgumentf("recon item %s not found or already resolved", itemID)
	}
	return nil
}

func (r ReconRepository) ResolveItemForOrganization(ctx context.Context, itemID, organizationID, operator string) error {
	now := time.Now().UTC()
	var emptyNote sql.NullString
	res, err := r.db.ExecContext(ctx, reconItemResolveForOrganization, operator, now, emptyNote, itemID, organizationID)
	if err != nil {
		return apperrors.WrapInternalf(err, "resolve recon item")
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return apperrors.NewNotFoundf("recon item %s not found", itemID)
	}
	return nil
}
