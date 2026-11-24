package b2b

import (
	"context"
	stderrors "errors"
	"fmt"
	"net"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainb2b "github.com/guliangiam/guliangiam/internal/domain/b2b"
	domainuow "github.com/guliangiam/guliangiam/internal/domain/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// DNSResolver 抽象 DNS TXT 查询 —— 生产实现用 net.DefaultResolver，
// 单元测试注入 fake。避免直接硬编码 net.DefaultResolver 导致无法 mock。
type DNSResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

// defaultDNSResolver 是 net.DefaultResolver 的薄包装 —— 满足 DNSResolver 接口。
type defaultDNSResolver struct{}

func (defaultDNSResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return net.DefaultResolver.LookupTXT(ctx, name)
}

// 确认 defaultDNSResolver 实现 DNSResolver（编译期接口检查）。
var _ DNSResolver = defaultDNSResolver{}

// Statuses 是组织域名状态封闭集的重导出。transport 层只允许依赖 app 层
// （scripts/check-layers.sh Rule 2），直接 import domain/b2b 会形成越层依赖。
var Statuses = domainb2b.Statuses

// DefaultMaxVerificationAttempts 是 DNS TXT 续验的失败预算。
// 预算耗尽才进入 failed 终态；此前保持 pending 并按指数退避重试。
const DefaultMaxVerificationAttempts = 5

// VerificationPolicy 控制单次 DNS 续验失败后的重试行为。
type VerificationPolicy struct {
	MaxAttempts int
	Backoff     func(attempts int) time.Duration
}

// DefaultVerificationPolicy 用于 Cron 与手动触发：5 次预算，1/2/4/8/16 分钟退避。
func DefaultVerificationPolicy() VerificationPolicy {
	return VerificationPolicy{
		MaxAttempts: DefaultMaxVerificationAttempts,
		Backoff:     VerificationBackoff,
	}
}

// VerificationBackoff 是 min(2^(attempts-1) 分钟, 60 分钟)。
// attempts 从 1 起（首次失败），因此第一次重试等 1 分钟。
func VerificationBackoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	const maxMinutes = 60
	minutes := 1
	for i := 1; i < attempts && minutes < maxMinutes; i++ {
		minutes *= 2
	}
	if minutes > maxMinutes {
		minutes = maxMinutes
	}
	return time.Duration(minutes) * time.Minute
}

// normalizeVerificationPolicy 把零值策略收敛到默认值，避免调用方漏传导致
// maxAttempts=0（domain 会按 1 处理，语义突变）或 nil backoff panic。
func normalizeVerificationPolicy(policy VerificationPolicy) VerificationPolicy {
	if policy.MaxAttempts < 1 {
		policy.MaxAttempts = DefaultMaxVerificationAttempts
	}
	if policy.Backoff == nil {
		policy.Backoff = VerificationBackoff
	}
	return policy
}

// Service 是 B2B 域名治理应用服务层 —— functional.md §15 MODULE-B2B。
//
// 依赖注入：Clock / UUID / UnitOfWork / DNSResolver。
// 类型断言获取 B2BUoW（domain/uow/B2BUoW 是独立接口，未嵌入 UnitOfWork 组合）。
type Service struct {
	Clock       clock.Clock
	UUID        platformuuid.Generator
	UOW         appuow.UnitOfWork
	DNSResolver DNSResolver
}

func NewService(c clock.Clock, uuid platformuuid.Generator, uow appuow.UnitOfWork) Service {
	return NewServiceWithDNS(c, uuid, uow, defaultDNSResolver{})
}

func NewServiceWithDNS(c clock.Clock, uuid platformuuid.Generator, uow appuow.UnitOfWork, dns DNSResolver) Service {
	return Service{Clock: c, UUID: uuid, UOW: uow, DNSResolver: dns}
}

// ── DTO / Command ───────────────────────────────────────────────────────────

type ClaimDomainCommand struct {
	OrganizationID string
	// Domain 不能是 IP / 裸 hostname —— Validate 已覆盖
	Domain string
}

type DomainDTO struct {
	ID                string    `json:"id"`
	OrganizationID    string    `json:"organizationId"`
	Domain            string    `json:"domain"`
	VerificationToken string    `json:"verificationToken"`
	Status            string    `json:"status"`
	VerifiedAt        time.Time `json:"verifiedAt,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`

	// 续验预算字段仅用于 Cron/服务内部流转，不允许通过 HTTP 暴露（B7-g 边界）。
	Attempts      int       `json:"-"`
	LastError     string    `json:"-"`
	NextRetryAt   time.Time `json:"-"`
	LastCheckedAt time.Time `json:"-"`
}

// ToDTO 内部转换 —— app 层 DTO vs domain 层实体，遵循 Clean Architecture 原则。
func toDTO(d *domainb2b.OrganizationDomain) DomainDTO {
	return DomainDTO{
		ID:                d.ID,
		OrganizationID:    d.OrganizationID,
		Domain:            d.Domain,
		VerificationToken: d.VerificationToken,
		Status:            d.Status,
		VerifiedAt:        d.VerifiedAt,
		CreatedAt:         d.CreatedAt,
		UpdatedAt:         d.UpdatedAt,
		Attempts:          d.Attempts,
		LastError:         d.LastError,
		NextRetryAt:       d.NextRetryAt,
		LastCheckedAt:     d.LastCheckedAt,
	}
}

// ── helper ─────────────────────────────────────────────────────────────────

func b2bRepoFrom(uow appuow.UnitOfWork) (domainb2b.Repository, error) {
	tx, ok := uow.(domainuow.B2BUoW)
	if !ok {
		return nil, apperrors.NewInternalf("B2BUoW 未实现 — UnitOfWork 类型 %T", uow)
	}
	return tx.B2BDomains(), nil
}

func (s Service) b2bRepo() (domainb2b.Repository, error) {
	return b2bRepoFrom(s.UOW)
}

func (s Service) embedAuditEvent(tx appuow.UnitOfWork, ctx context.Context, orgID, action, resourceID string, payload map[string]any) error {
	clockProvider := s.Clock
	if clockProvider == nil {
		clockProvider = clock.SystemClock{}
	}
	return audit.EmbedAuditEvent(
		clockProvider, tx, ctx,
		"", "", "management", "", "",
		orgID, "", action, "organization_domain", resourceID,
		"organization_domain", action, payload,
	)
}

// ── ClaimDomain ────────────────────────────────────────────────────────────
//
// 业务逻辑（§15.1 步骤 1-2）：
//
//  1. 生成一次性 DNS TXT token（格式 GuliangIAM-Verification=v1:<uuid>）
//  2. 创建 pending 状态 OrganizationDomain
//  3. 返回 token + 域名 + status
//
// DB 唯一约束 uq_org_domain 兜底重复认领 —— domain.Validate 先在应用层做格式检查。

func (s Service) ClaimDomain(ctx context.Context, cmd ClaimDomainCommand) (DomainDTO, error) {
	if strings.TrimSpace(cmd.OrganizationID) == "" {
		return DomainDTO{}, apperrors.NewInvalidArgument("organization id is required")
	}

	now := s.Clock.Now()
	id, err := s.UUID.NewString()
	if err != nil {
		return DomainDTO{}, apperrors.WrapInternalf(err, "generate domain id")
	}
	tokenUUID, err := s.UUID.NewString()
	if err != nil {
		return DomainDTO{}, apperrors.WrapInternalf(err, "generate verification token")
	}
	d := &domainb2b.OrganizationDomain{
		ID:                id,
		OrganizationID:    cmd.OrganizationID,
		Domain:            cmd.Domain,
		VerificationToken: fmt.Sprintf("GuliangIAM-Verification=v1:%s", tokenUUID),
		Status:            domainb2b.StatusPending,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := d.Validate(); err != nil {
		return DomainDTO{}, err
	}

	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo, err := b2bRepoFrom(tx)
		if err != nil {
			return err
		}
		if err := repo.Create(ctx, d); err != nil {
			return err
		}
		return s.embedAuditEvent(tx, ctx, d.OrganizationID, "b2b.domain.claimed", d.ID, map[string]any{
			fieldDomain: d.Domain, fieldStatus: d.Status,
		})
	})
	if err != nil {
		return DomainDTO{}, err
	}

	return toDTO(d), nil
}

// ── VerifyDomain (Cron entry) ──────────────────────────────────────────────
//
// 业务逻辑（§15.1 步骤 3-4）：
//
//  1. 查询 TXT 记录 _guliangiam-verification.<domain>
//  2. 匹配 pending.verification_token → RecordVerificationSuccess → verified
//  3. 不匹配 / TXT 不存在 / 查询错误 → RecordVerificationFailure：
//     预算内保持 pending + 指数退避；预算耗尽才进入 failed 终态。
//
// 并发：Update 带 status='pending'；其他实例已终态化时返回 Conflict 且不写审计。
// 重试调度（保持 pending）只持久化预算字段，不写审计 —— 审计仅记录
// verified / verification_failed 两个既有动作，避免中间态审计/outbox 噪声。

// VerifySingle 对单条 pending 记录执行 DNS 验证并持久化续验结果。供 Cron worker 调用。
func (s Service) VerifySingle(
	ctx context.Context, pending *domainb2b.OrganizationDomain, policy VerificationPolicy,
) (DomainDTO, error) {
	policy = normalizeVerificationPolicy(policy)

	txtRecord := "_guliangiam-verification." + pending.Domain
	records, lookupErr := s.DNSResolver.LookupTXT(ctx, txtRecord)
	now := s.Clock.Now()

	matched := false
	if lookupErr == nil {
		for _, r := range records {
			if r == pending.VerificationToken {
				matched = true
				break
			}
		}
	}

	action := ""
	if matched {
		if err := pending.RecordVerificationSuccess(now); err != nil {
			return DomainDTO{}, err
		}
		action = "b2b.domain.verified"
	} else {
		reason := classifyVerificationFailure(records, lookupErr)
		if err := pending.RecordVerificationFailure(reason, now, policy.MaxAttempts, policy.Backoff); err != nil {
			return DomainDTO{}, err
		}
		if pending.Status == domainb2b.StatusFailed {
			action = "b2b.domain.verification_failed"
		}
	}

	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo, err := b2bRepoFrom(tx)
		if err != nil {
			return err
		}
		if err := repo.Update(ctx, pending); err != nil {
			return err
		}
		if action == "" {
			return nil
		}
		return s.embedAuditEvent(tx, ctx, pending.OrganizationID, action, pending.ID, map[string]any{
			fieldDomain: pending.Domain, fieldStatus: pending.Status,
		})
	})
	if err != nil {
		return DomainDTO{}, err
	}
	return toDTO(pending), nil
}

// classifyVerificationFailure 返回「分类码 + 简短说明」，不包含 token、原始 DNS
// 错误文本或解析器地址；由 domain 层统一截断到 last_error VARCHAR(255)。
func classifyVerificationFailure(records []string, lookupErr error) string {
	if lookupErr != nil {
		var dnsErr *net.DNSError
		if stderrors.As(lookupErr, &dnsErr) {
			switch {
			case dnsErr.IsNotFound:
				return "dns_nxdomain: TXT record not found"
			case dnsErr.IsTimeout:
				return "dns_timeout: TXT lookup timed out"
			}
		}
		return "dns_error: TXT lookup failed"
	}
	if len(records) == 0 {
		return "txt_missing: no TXT record found"
	}
	return "token_mismatch: TXT record does not contain the expected token"
}

// ClaimPending 原子认领至多 limit 条到期 pending 域名并加 lease 租约 ——
// Cron worker 入口。在事务内执行，使 tenantctx.System(ctx) 的 RLS GUC 生效
// （跨租户读写 organization_domains）；事务提交后租约落库，DNS 查询在事务外进行。
func (s Service) ClaimPending(ctx context.Context, limit int, lease time.Duration) ([]DomainDTO, error) {
	if limit <= 0 {
		return nil, apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"claim limit must be positive",
			map[string]any{"field": "limit", "value": limit, "min": 1},
		)
	}
	if lease <= 0 {
		return nil, apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"claim lease must be positive",
			map[string]any{"field": "lease", "value": lease.String(), "min": "1ns"},
		)
	}
	var claimed []*domainb2b.OrganizationDomain
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo, err := b2bRepoFrom(tx)
		if err != nil {
			return err
		}
		claimed, err = repo.ClaimPending(ctx, limit, lease)
		return err
	})
	if err != nil {
		return nil, err
	}
	out := make([]DomainDTO, len(claimed))
	for i, d := range claimed {
		out[i] = toDTO(d)
	}
	return out, nil
}

// ── FindForSSO (邮箱域发现) ─────────────────────────────────────────────────

// FindForSSO 从邮箱地址提取域名 → 查已验证组织域名。
//
// 输入：alice@corp.example.com
// 输出：OrganizationDomain 对应的 OrganizationID（供 auth 流程自动路由 IdP）
// 无匹配 → 返回 nil, nil（不报错，未认领邮箱域）。
func (s Service) FindForSSO(ctx context.Context, email string) (*DomainDTO, error) {
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return nil, nil // 无有效域名
	}
	domainName := domainb2b.NormalizeDomain(email[at+1:])
	if domainName == "" {
		return nil, nil
	}

	repo, err := s.b2bRepo()
	if err != nil {
		return nil, err
	}
	d, err := repo.FindVerifiedByDomain(ctx, domainName)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, nil
	}
	dto := toDTO(d)
	return &dto, nil
}

// ── ListDomains (管理侧) ────────────────────────────────────────────────────

type ListDomainsQuery struct {
	Page, PageSize int
	OrganizationID string
	Status         string
}

func (s Service) ListDomains(ctx context.Context, q ListDomainsQuery) ([]DomainDTO, int64, error) {
	if strings.TrimSpace(q.OrganizationID) == "" {
		return nil, 0, apperrors.NewInvalidArgument("organization id is required")
	}
	repo, err := s.b2bRepo()
	if err != nil {
		return nil, 0, err
	}

	list, total, err := repo.List(ctx, domainb2b.ListQuery{
		Page:           q.Page,
		PageSize:       q.PageSize,
		OrganizationID: q.OrganizationID,
		Status:         q.Status,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]DomainDTO, len(list))
	for i, d := range list {
		out[i] = toDTO(d)
	}
	return out, total, nil
}

// ── GetDomainByID ──────────────────────────────────────────────────────────
//
// Handler: GET /api/v1/organizations/{orgId}/domains/{domainId}（手动触发前置）。
// 仓储已有 GetByID —— 这里加一层 app service 包装 + 归属权校验。

func (s Service) GetDomainByID(ctx context.Context, id string) (DomainDTO, error) {
	repo, err := s.b2bRepo()
	if err != nil {
		return DomainDTO{}, err
	}
	d, err := repo.GetByID(ctx, id)
	if err != nil {
		return DomainDTO{}, err
	}
	return toDTO(d), nil
}

// ── DeleteDomain ────────────────────────────────────────────────────────────

func (s Service) DeleteDomain(ctx context.Context, id, organizationID string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organization id is required")
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo, err := b2bRepoFrom(tx)
		if err != nil {
			return err
		}
		// 租户谓词在仓储层显式生效；跨租户与不存在统一 NotFound，避免存在性探测。
		d, err := repo.GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if d.OrganizationID != organizationID {
			return apperrors.NewNotFound("organization_domain not found")
		}
		if err := repo.Delete(ctx, id, organizationID); err != nil {
			return err
		}
		return s.embedAuditEvent(tx, ctx, d.OrganizationID, "b2b.domain.deleted", d.ID, map[string]any{
			fieldDomain: d.Domain, fieldStatus: d.Status,
		})
	})
}

// ── VerifyDomainByID（手动触发） ────────────────────────────────────────────
//
// handler.go VerifyDomain 调用链：GetByIDForOrganization → VerifySingle。
// 返回 verified bool 表示最终状态；DNS 不匹配时按续验预算保持 pending 或进入
// failed 终态（与 Cron 同一套语义）。

func (s Service) VerifyDomainByID(ctx context.Context, id, organizationID string) (DomainDTO, bool, error) {
	if strings.TrimSpace(organizationID) == "" {
		return DomainDTO{}, false, apperrors.NewInvalidArgument("organization id is required")
	}
	repo, err := s.b2bRepo()
	if err != nil {
		return DomainDTO{}, false, err
	}
	// 租户谓词在仓储层显式生效；跨租户与不存在统一 NotFound，避免存在性探测。
	d, err := repo.GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return DomainDTO{}, false, err
	}
	candidate := *d
	out, err := s.VerifySingle(ctx, &candidate, DefaultVerificationPolicy())
	if err != nil {
		return DomainDTO{}, false, err
	}
	return out, out.Status == domainb2b.StatusVerified, nil
}
