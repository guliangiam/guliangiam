package payments

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// ReconService 是对账业务用例接口（app 层 Service）。
//
// Clean Architecture：transport 层只依赖此接口，不直接 import domain。
// 实现持有 domain.ReconRepo + domain.ReconRunner（ReconcileWorker）——同包注入，
// 不跨层。
//
// 方法设计为薄包装：domain 类型不跨 transport 边界，handler 层在收到
// ReconService 返回值后立即映射到自己的 view struct。
type ReconService interface {
	// ListBatches 返回组织下的对账批次（分页）。
	ListBatches(ctx context.Context, orgID string, page, pageSize int) ([]*domain.ReconBatch, int64, error)
	// ListItems 返回指定 batch 的差异项列表（仅当 batch 属于可信租户）。
	ListItems(ctx context.Context, organizationID, batchID string) ([]*domain.ReconItem, error)
	// ResolveItem 人工处置一条差异项（仅当 batch 属于可信租户）。
	ResolveItem(ctx context.Context, organizationID, itemID, operator string) error
	// ApproveItem 复核一条已处置差异项；approver 不得等于 resolved_by。
	ApproveItem(ctx context.Context, organizationID, itemID, approver string) error
	// RunManual 手动触发已存在的 pending batch 立即执行（仅当 batch 属于可信租户）。
	RunManual(ctx context.Context, organizationID, batchID string) error
	// CreateAndRun 创建 pending batch 后立即执行（RunRecon 模式 2）。
	CreateAndRun(ctx context.Context, orgID, channel string, batchDate, now time.Time) (batchID string, err error)
}

type reconService struct {
	Recon  domain.ReconRepo
	Runner domain.ReconRunner
	UUID   platformuuid.Generator
	UOW    appuow.UnitOfWork
}

// NewReconService 构造 ReconService。
//
// recon 由 bootstrap 注入（infrapgpayments.NewReconRepository）；
// runner 由 bootstrap 注入（*ReconcileWorker，已实现 domain.ReconRunner）；
// uuid 用于生成 pay_recon_batches.id（UUID 主键），nil 时回退平台 V7。
func NewReconService(recon domain.ReconRepo, runner domain.ReconRunner, uuid platformuuid.Generator, uow ...appuow.UnitOfWork) ReconService {
	if uuid == nil {
		uuid = platformuuid.V7Generator{}
	}
	var work appuow.UnitOfWork
	if len(uow) > 0 {
		work = uow[0]
	}
	return &reconService{Recon: recon, Runner: runner, UUID: uuid, UOW: work}
}

func (s *reconService) ListBatches(ctx context.Context, orgID string, page, pageSize int) ([]*domain.ReconBatch, int64, error) {
	limit, offset := reconBatchWindow(page, pageSize)
	return s.Recon.Batches(ctx, orgID, limit, offset)
}

// reconBatchWindow returns the LIMIT and OFFSET for one batch-list page. Both
// values come from the same clamped size, and the offset is computed by the
// shared helper so a hostile page cannot wrap the multiplication negative.
func reconBatchWindow(page, pageSize int) (limit, offset int) {
	limit = platformpagination.ClampPageSize(pageSize)
	return limit, platformpagination.Offset(page, limit)
}

func (s *reconService) ListItems(ctx context.Context, organizationID, batchID string) ([]*domain.ReconItem, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	return s.Recon.ListItemsForOrganization(ctx, batchID, organizationID)
}

func (s *reconService) ResolveItem(ctx context.Context, organizationID, itemID, operator string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.Recon.ResolveItemForOrganization(ctx, itemID, organizationID, operator)
}

func (s *reconService) ApproveItem(ctx context.Context, organizationID, itemID, approver string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if approver == "" {
		return apperrors.NewInvalidArgument("approver is required")
	}
	// BE-C-18：仓储支持原子能力时，approve + 审计 + Outbox 同事务提交；
	// 审计失败会回滚 approve，不再产生"已复核但无审计"的静默缺口。
	if auditor, ok := s.Recon.(domain.ReconApprovalAuditor); ok {
		_, err := auditor.ApproveItemWithAudit(ctx, itemID, organizationID, approver, func(originalOperator string) (*domainrecords.Record, *domainoutbox.OutboxEvent, error) {
			return s.approvalAudit(organizationID, itemID, approver, originalOperator)
		})
		return err
	}
	// 旧路径回退（MySQL 镜像 / 未实现原子能力的仓储）：两步提交，审计尽力而为。
	originalOperator, err := s.Recon.ApproveItemForOrganization(ctx, itemID, organizationID, approver)
	if err != nil {
		return err
	}
	s.recordApproval(ctx, organizationID, itemID, approver, originalOperator)
	return nil
}

// approvalAudit 构造复核审计的 record + outbox 事件（同一 ID 关联）。
func (s *reconService) approvalAudit(organizationID, itemID, approver, originalOperator string) (*domainrecords.Record, *domainoutbox.OutboxEvent, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, nil, apperrors.WrapInternal(err, "generate recon approval audit id")
	}
	payload, _ := json.Marshal(map[string]any{
		"itemId": itemID, "approver": approver, "originalOperator": originalOperator,
	})
	org := organizationID
	event := &domainoutbox.OutboxEvent{
		ID: id, OrganizationID: &org,
		AggregateType: "pay_recon_item", AggregateID: itemID,
		EventType: "payment.recon.item.approved", EventVersion: 1,
		Payload: payload, MaxAttempts: 3, AvailableAt: time.Now().UTC(),
	}
	record := &domainrecords.Record{
		ID: id, OrganizationID: organizationID, ActorID: approver,
		Action: "payment.recon.item.approved", ResourceType: "pay_recon_item",
		ResourceID: itemID, Surface: domainrecords.SurfaceManagement,
		Metadata: map[string]any{"approver": approver, "originalOperator": originalOperator}, CreatedAt: time.Now().UTC(),
	}
	return record, event, nil
}

func (s *reconService) recordApproval(ctx context.Context, organizationID, itemID, approver, originalOperator string) {
	if s.UOW == nil {
		return
	}
	record, event, err := s.approvalAudit(organizationID, itemID, approver, originalOperator)
	if err != nil {
		slog.WarnContext(ctx, "recon approval audit id", "error", err)
		return
	}
	err = s.UOW.WithinTransaction(tenantctx.System(ctx), func(txCtx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Records().Create(txCtx, record); err != nil {
			return err
		}
		return tx.Outbox().Enqueue(txCtx, event)
	})
	if err != nil {
		slog.WarnContext(ctx, "recon approval audit", "error", err)
	}
}

func (s *reconService) RunManual(ctx context.Context, organizationID, batchID string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	// Ownership gate: batchId is caller-supplied, so a foreign batch must look
	// missing before the worker is asked to run it.
	if _, err := s.Recon.GetBatchForOrganization(ctx, batchID, organizationID); err != nil {
		return err
	}
	return s.Runner.RunManual(ctx, batchID)
}

func (s *reconService) CreateAndRun(ctx context.Context, orgID, channel string, batchDate, now time.Time) (string, error) {
	if err := requireOrganizationID(orgID); err != nil {
		return "", err
	}
	// pay_recon_batches.id is a UUID primary key. The "one batch per
	// org/channel/date" rule is enforced by the partial unique constraint
	// uq_pay_recon_batches_org_channel_date, not by the id value.
	batchID, err := s.UUID.NewString()
	if err != nil {
		return "", apperrors.WrapInternal(err, "generate recon batch id")
	}
	batch := &domain.ReconBatch{
		ID:             batchID,
		OrganizationID: orgID,
		Channel:        channel,
		BatchDate:      batchDate,
		Status:         string(domain.ReconBatchStatusPending),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := batch.Validate(); err != nil {
		return "", err
	}
	if err := s.Recon.CreateBatch(ctx, batch); err != nil {
		return "", err
	}
	return batchID, s.Runner.RunManual(ctx, batchID)
}
