package payments

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	infrapgoutbox "github.com/guliangiam/guliangiam/internal/infra/postgres/outbox"
	infrapgrecords "github.com/guliangiam/guliangiam/internal/infra/postgres/records"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// ─── ChannelConfigsRepo ────────────────────────────────────────────────

type ChannelConfigsRepository struct{ db shared.DBTX }

func NewChannelConfigsRepository(db shared.DBTX) ChannelConfigsRepository {
	return ChannelConfigsRepository{db: db}
}

var _ domain.ChannelConfigsRepo = ChannelConfigsRepository{}

// scanPayChannelConfig scans a pay_channel_configs row into a domain entity.
// SELECT 列顺序: id, organization_id, provider_id, channel, environment,
// config_sealed, config_fingerprint, config_set (jsonb), version_no, status,
// test_passed_at, activated_at, disabled_at, created_at, updated_at, deleted_at, revision.
func scanPayChannelConfig(sc interface{ Scan(...interface{}) error }) (*domain.PayChannelConfig, error) {
	var c domain.PayChannelConfig
	var providerID *string
	var configSetBytes []byte
	err := sc.Scan(
		&c.ID, &c.OrganizationID, &providerID, &c.Channel, &c.Environment,
		&c.ConfigSealed, &c.ConfigFingerprint, &configSetBytes,
		&c.VersionNo, &c.Status,
		&c.TestPassedAt, &c.ActivatedAt, &c.DisabledAt,
		&c.CreatedAt, &c.UpdatedAt, &c.DeletedAt,
		&c.Revision,
	)
	if err != nil {
		return nil, err
	}
	if providerID != nil {
		c.ProviderID = *providerID
	}
	if len(configSetBytes) > 0 && string(configSetBytes) != jsonNullLiteral && string(configSetBytes) != "[]" {
		if err := json.Unmarshal(configSetBytes, &c.ConfigSet); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal config_set")
		}
	}
	if c.ConfigSet == nil {
		c.ConfigSet = []string{}
	}
	return &c, nil
}

func (r ChannelConfigsRepository) GetByID(ctx context.Context, id string) (*domain.PayChannelConfig, error) {
	c, err := scanPayChannelConfig(r.db.QueryRow(ctx, QueryChannelConfigGetByID, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("channel config not found")
		}
		return nil, err
	}
	return c, nil
}

func (r ChannelConfigsRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.PayChannelConfig, error) {
	c, err := scanPayChannelConfig(r.db.QueryRow(ctx, QueryChannelConfigGetByIDForOrganization, id, organizationID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("channel config not found")
		}
		return nil, err
	}
	return c, nil
}

func (r ChannelConfigsRepository) GetActive(ctx context.Context, organizationID, channel, environment string) (*domain.PayChannelConfig, error) {
	c, err := scanPayChannelConfig(r.db.QueryRow(ctx, QueryChannelConfigGetActive, organizationID, channel, environment))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("no active channel config for " + channel)
		}
		return nil, err
	}
	return c, nil
}

func (r ChannelConfigsRepository) List(ctx context.Context, organizationID string) ([]*domain.PayChannelConfig, error) {
	rows, err := r.db.Query(ctx, QueryChannelConfigList, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*domain.PayChannelConfig
	for rows.Next() {
		c, err := scanPayChannelConfig(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r ChannelConfigsRepository) Create(ctx context.Context, c *domain.PayChannelConfig) error {
	var configSetBytes []byte
	if len(c.ConfigSet) > 0 {
		var err error
		configSetBytes, err = json.Marshal(c.ConfigSet)
		if err != nil {
			return apperrors.WrapInternalf(err, "marshal config_set")
		}
	}
	var providerID any
	if c.ProviderID != "" {
		providerID = c.ProviderID
	}
	_, err := r.db.Exec(ctx, QueryChannelConfigInsert,
		c.ID, c.OrganizationID, providerID, c.Channel, c.Environment,
		c.ConfigSealed, c.ConfigFingerprint, configSetBytes,
		c.VersionNo, c.Status,
		c.TestPassedAt, c.ActivatedAt, c.DisabledAt,
		c.CreatedAt, c.UpdatedAt, c.DeletedAt,
		c.Revision,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "insert channel config")
	}
	return nil
}

func (r ChannelConfigsRepository) Update(ctx context.Context, c *domain.PayChannelConfig) error {
	newRevision := c.Revision + 1
	tag, err := r.db.Exec(ctx, QueryChannelConfigUpdate,
		c.Status, c.TestPassedAt, c.ActivatedAt, c.DisabledAt,
		c.UpdatedAt, newRevision, c.ID, c.Revision, c.OrganizationID,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "update channel config")
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewConflictf("channel config %s update conflict", c.ID)
	}
	c.Revision = newRevision
	return nil
}

// ─── ReconRepo ────────────────────────────────────────────────────────────

type ReconRepository struct{ db shared.DBTX }

func NewReconRepository(db shared.DBTX) ReconRepository { return ReconRepository{db: db} }

var _ domain.ReconRepo = ReconRepository{}

// scanReconBatch scans a pay_recon_batches row into domain.ReconBatch.
// SELECT 列顺序: id, organization_id, channel, batch_date, status,
// diff_count, total_local, total_channel, created_at, updated_at, deleted_at, revision.
func scanReconBatch(sc interface{ Scan(...interface{}) error }) (*domain.ReconBatch, error) {
	var b domain.ReconBatch
	err := sc.Scan(
		&b.ID, &b.OrganizationID, &b.Channel, &b.BatchDate, &b.Status,
		&b.DiffCount, &b.TotalLocal, &b.TotalChannel,
		&b.CreatedAt, &b.UpdatedAt, &b.DeletedAt, &b.Revision,
	)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// scanReconItem scans a pay_recon_items row into domain.ReconItem.
// SELECT 列顺序: id, batch_id, diff_type, channel_order_no, local_payment_id,
// channel_amount, local_amount, channel_state, local_state,
// resolved, resolved_by, resolved_at, note, created_at.
func scanReconItem(sc interface{ Scan(...interface{}) error }) (*domain.ReconItem, error) {
	var i domain.ReconItem
	err := sc.Scan(
		&i.ID, &i.BatchID, &i.DiffType, &i.ChannelOrderNo, &i.LocalPaymentID,
		&i.ChannelAmount, &i.LocalAmount, &i.ChannelState, &i.LocalState,
		&i.Resolved, &i.ResolutionState, &i.ApprovedBy, &i.ApprovedAt,
		&i.ResolvedBy, &i.ResolvedAt, &i.Note, &i.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (r ReconRepository) Batches(ctx context.Context, organizationID string, limit, offset int) ([]*domain.ReconBatch, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, ReconBatchCount, organizationID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, ReconBatchList, organizationID, limit, offset)
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
	b, err := scanReconBatch(r.db.QueryRow(ctx, ReconBatchGetByID, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("recon batch not found")
		}
		return nil, err
	}
	return b, nil
}

func (r ReconRepository) GetBatchForOrganization(ctx context.Context, id, organizationID string) (*domain.ReconBatch, error) {
	b, err := scanReconBatch(r.db.QueryRow(ctx, ReconBatchGetByIDForOrganization, id, organizationID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("recon batch not found")
		}
		return nil, err
	}
	return b, nil
}

func (r ReconRepository) CreateBatch(ctx context.Context, b *domain.ReconBatch) error {
	_, err := r.db.Exec(ctx, ReconBatchInsert,
		b.ID, b.OrganizationID, b.Channel, b.BatchDate, b.Status,
		b.DiffCount, b.TotalLocal, b.TotalChannel,
		b.CreatedAt, b.UpdatedAt, b.DeletedAt, b.Revision,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "insert recon batch")
	}
	return nil
}

func (r ReconRepository) UpdateBatch(ctx context.Context, b *domain.ReconBatch) error {
	newRevision := b.Revision + 1
	tag, err := r.db.Exec(ctx, ReconBatchUpdate,
		b.Status, b.DiffCount, b.TotalLocal, b.TotalChannel,
		b.UpdatedAt, newRevision, b.ID, b.Revision,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "update recon batch")
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewConflictf("recon batch %s update conflict", b.ID)
	}
	b.Revision = newRevision
	return nil
}

func (r ReconRepository) CreateItem(ctx context.Context, item *domain.ReconItem) error {
	_, err := r.db.Exec(ctx, ReconItemInsert,
		item.ID, item.BatchID, item.DiffType, item.ChannelOrderNo, item.LocalPaymentID,
		item.ChannelAmount, item.LocalAmount, item.ChannelState, item.LocalState,
		item.Resolved, item.ResolvedBy, item.ResolvedAt, item.Note, item.CreatedAt,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "insert recon item")
	}
	return nil
}

func (r ReconRepository) ListItems(ctx context.Context, batchID string) ([]*domain.ReconItem, error) {
	return r.listItems(ctx, ReconItemList, batchID)
}

func (r ReconRepository) ListItemsForOrganization(ctx context.Context, batchID, organizationID string) ([]*domain.ReconItem, error) {
	return r.listItems(ctx, ReconItemListForOrganization, batchID, organizationID)
}

func (r ReconRepository) listItems(ctx context.Context, query string, args ...any) ([]*domain.ReconItem, error) {
	rows, err := r.db.Query(ctx, query, args...)
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
	var originalOperator *string
	if err := r.db.QueryRow(ctx, ReconItemApproveForOrganization, approver, now, itemID, organizationID).Scan(&originalOperator); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperrors.NewUnprocessableEntity("recon item must be resolved, unapproved, and approver must differ from resolver")
		}
		return "", apperrors.WrapInternalf(err, "approve recon item")
	}
	if originalOperator == nil {
		return "", nil
	}
	return *originalOperator, nil
}

// ApproveItemWithAudit 实现 domain.ReconApprovalAuditor：approve + 审计记录 +
// Outbox 事件同事务提交（BE-C-18）。db 为连接池时自开事务；db 已绑定调用方
// 事务（UoW tx）时顺序复用同一事务，由调用方提交/回滚。
func (r ReconRepository) ApproveItemWithAudit(ctx context.Context, itemID, organizationID, approver string, buildAudit func(originalOperator string) (*domainrecords.Record, *domainoutbox.OutboxEvent, error)) (string, error) {
	beginner, ok := r.db.(interface {
		Begin(ctx context.Context) (pgx.Tx, error)
	})
	if !ok {
		return r.approveWithAuditOn(ctx, r.db, itemID, organizationID, approver, buildAudit)
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "begin recon approval transaction")
	}
	defer tx.Rollback(ctx)
	// records/outbox 是 FORCE RLS 平台表，审计写入需要系统作用域（与旧
	// recordApproval 的 tenantctx.System 事务一致）。approve 语句自带显式
	// organization_id 过滤，提权不放宽租户边界。LOCAL(true) 避免污染连接池。
	if _, err := tx.Exec(ctx,
		`SELECT set_config('app.current_role', 'super_admin', true), set_config('app.current_organization_id', '', true)`,
	); err != nil {
		return "", apperrors.WrapInternal(err, "set recon approval system scope")
	}
	original, err := r.approveWithAuditOn(ctx, tx, itemID, organizationID, approver, buildAudit)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", apperrors.WrapInternalf(err, "commit recon approval transaction")
	}
	return original, nil
}

func (r ReconRepository) approveWithAuditOn(ctx context.Context, db shared.DBTX, itemID, organizationID, approver string, buildAudit func(originalOperator string) (*domainrecords.Record, *domainoutbox.OutboxEvent, error)) (string, error) {
	scoped := ReconRepository{db: db}
	original, err := scoped.ApproveItemForOrganization(ctx, itemID, organizationID, approver)
	if err != nil {
		return "", err
	}
	if buildAudit == nil {
		return original, nil
	}
	record, event, err := buildAudit(original)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "build recon approval audit")
	}
	if record != nil {
		if err := infrapgrecords.NewRepository(db).Create(ctx, record); err != nil {
			return "", apperrors.WrapInternalf(err, "persist recon approval record")
		}
	}
	if event != nil {
		if err := infrapgoutbox.NewRepository(db, nil).Enqueue(ctx, event); err != nil {
			return "", apperrors.WrapInternalf(err, "enqueue recon approval event")
		}
	}
	return original, nil
}

var _ domain.ReconApprovalAuditor = ReconRepository{}

func (r ReconRepository) ResolveItem(ctx context.Context, itemID, operator string) error {
	now := time.Now().UTC()
	var emptyNote *string
	tag, err := r.db.Exec(ctx, ReconItemResolve, operator, now, emptyNote, itemID)
	if err != nil {
		return apperrors.WrapInternalf(err, "resolve recon item")
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewInvalidArgumentf("recon item %s not found or already resolved", itemID)
	}
	return nil
}

func (r ReconRepository) ResolveItemForOrganization(ctx context.Context, itemID, organizationID, operator string) error {
	now := time.Now().UTC()
	var emptyNote *string
	tag, err := r.db.Exec(ctx, ReconItemResolveForOrganization, operator, now, emptyNote, itemID, organizationID)
	if err != nil {
		return apperrors.WrapInternalf(err, "resolve recon item")
	}
	if tag.RowsAffected() == 0 {
		// A foreign batch and a missing item look identical; management callers
		// must not be able to probe existence across tenants.
		return apperrors.NewNotFoundf("recon item %s not found", itemID)
	}
	return nil
}
