package iga

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/iga"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type UnitOfWork = appuow.UnitOfWork
type (
	AccessCampaign          = domain.AccessCampaign
	SODRule                 = domain.SODRule
	AccessLifecycle         = domain.AccessLifecycle
	CampaignStatus          = domain.CampaignStatus
	LifecycleStatus         = domain.LifecycleStatus
	ListCampaignsQuery      = domain.ListCampaignsQuery
	ListSODRulesQuery       = domain.ListSODRulesQuery
	ListLifecycleQuery      = domain.ListLifecycleQuery
	ListAccessRequestsQuery = domain.ListAccessRequestsQuery
	RequestStatus           = domain.RequestStatus
	CreateCampaignCommand   = domain.CreateCampaignCommand
	CreateSODRuleCommand    = domain.CreateSODRuleCommand
)

const (
	auditKeyCampaignID = "campaignId"
	auditKeyName       = "name"
	auditKeySODRuleID  = "sodRuleId"
)

type Service struct {
	Clock clock.Clock
	UOW   appuow.IGAUoW
}

func NewService(uow UnitOfWork) Service { return Service{Clock: clock.SystemClock{}, UOW: uow} }

func fmtTime(t time.Time) string { return t.Format("2006-01-02T15:04:05.000Z07:00") }

// ─── DTOs ──────────────────────────────────────────────────────────────────

type CampaignDTO struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organizationId"`
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	StartDate      *string `json:"startDate"`
	EndDate        *string `json:"endDate"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
	Revision       int64   `json:"revision"`
}
type SODRuleDTO struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	RoleAID        string `json:"roleAId"`
	RoleBID        string `json:"roleBId"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
	Revision       int64  `json:"revision"`
}
type LifecycleDTO struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	UserID         string `json:"userId"`
	RoleID         string `json:"roleId"`
	Status         string `json:"status"`
	GrantedAt      string `json:"grantedAt"`
	ExpiresAt      string `json:"expiresAt"`
	CreatedAt      string `json:"createdAt"`
	Revision       int64  `json:"revision"`
}

func toCampaign(c *AccessCampaign) CampaignDTO {
	d := CampaignDTO{
		ID:             c.ID,
		OrganizationID: c.OrganizationID,
		Name:           c.Name,
		Status:         string(c.Status),
		CreatedAt:      fmtTime(c.CreatedAt),
		UpdatedAt:      fmtTime(c.UpdatedAt),
		Revision:       c.Revision,
	}
	if c.StartDate != nil {
		s := fmtTime(*c.StartDate)
		d.StartDate = &s
	}
	if c.EndDate != nil {
		s := fmtTime(*c.EndDate)
		d.EndDate = &s
	}
	return d
}
func toSOD(s *SODRule) SODRuleDTO {
	return SODRuleDTO{
		ID:             s.ID,
		OrganizationID: s.OrganizationID,
		Name:           s.Name,
		RoleAID:        s.RoleAID,
		RoleBID:        s.RoleBID,
		CreatedAt:      fmtTime(s.CreatedAt),
		UpdatedAt:      fmtTime(s.UpdatedAt),
		Revision:       s.Revision,
	}
}
func toLifecycle(l *AccessLifecycle) LifecycleDTO {
	return LifecycleDTO{
		ID:             l.ID,
		OrganizationID: l.OrganizationID,
		UserID:         l.UserID,
		RoleID:         l.RoleID,
		Status:         string(l.Status),
		GrantedAt:      fmtTime(l.GrantedAt),
		ExpiresAt:      fmtTime(l.ExpiresAt),
		CreatedAt:      fmtTime(l.CreatedAt),
		Revision:       l.Revision,
	}
}

// ─── Campaigns ─────────────────────────────────────────────────────────────

func (s Service) ListCampaigns(ctx context.Context, q ListCampaignsQuery) ([]CampaignDTO, int64, error) {
	if q.OrganizationID == "" {
		return nil, 0, apperrors.NewInvalidArgument("organizationId required")
	}
	items, total, err := s.UOW.IGA().ListCampaigns(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	out := make([]CampaignDTO, len(items))
	for i, c := range items {
		out[i] = toCampaign(c)
	}
	return out, total, nil
}

func (s Service) GetCampaign(ctx context.Context, orgID, id string) (*CampaignDTO, error) {
	c, err := s.UOW.IGA().GetCampaign(ctx, id, orgID)
	if err != nil {
		return nil, err
	}
	d := toCampaign(c)
	return &d, nil
}

func (s Service) CreateCampaign(ctx context.Context, cmd CreateCampaignCommand) (*CampaignDTO, error) {
	if cmd.OrganizationID == "" || cmd.Name == "" {
		return nil, apperrors.NewInvalidArgument("organizationId and name required")
	}
	if err := validateCampaignWindow(cmd.StartDate, cmd.EndDate); err != nil {
		return nil, err
	}
	now := s.now()
	c := &AccessCampaign{
		ID: uuid.NewString(), OrganizationID: cmd.OrganizationID, Name: cmd.Name,
		Status: domain.CampaignDraft, StartDate: cmd.StartDate, EndDate: cmd.EndDate,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.IGA().CreateCampaign(ctx, c); err != nil {
			return err
		}
		return s.embedAuditEvent(tx, ctx, c.OrganizationID, "",
			"iga.campaign.created", "access_campaign", c.ID, "access_campaign", "iga.campaign.created",
			map[string]any{auditKeyCampaignID: c.ID, auditKeyName: c.Name})
	})
	if err != nil {
		return nil, err
	}
	return s.GetCampaign(ctx, cmd.OrganizationID, c.ID)
}

// validateCampaignWindow 拒绝 startDate >= endDate 的非法窗口（BE-D-5/16）。
func validateCampaignWindow(start, end *time.Time) error {
	if start != nil && end != nil && !end.After(*start) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "endDate must be after startDate", map[string]any{"field": "endDate", "expected": "after startDate"})
	}
	return nil
}

// campaignStatusTransitions 是服务层状态机白名单（BE-D-16）：
// DB CHECK 只约束成员值，completed/cancelled 为终态，不允许任意跳变。
var campaignStatusTransitions = map[CampaignStatus][]CampaignStatus{
	domain.CampaignDraft:     {domain.CampaignActive, domain.CampaignCancelled},
	domain.CampaignActive:    {domain.CampaignCompleted, domain.CampaignCancelled},
	domain.CampaignCompleted: {},
	domain.CampaignCancelled: {},
}

func validCampaignTransition(from, to CampaignStatus) bool {
	for _, allowed := range campaignStatusTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// UpdateCampaignStatus transitions the campaign through its lifecycle.
// 状态机在服务层裁决（DB CHECK 仅约束成员值，不约束跳转）。
func (s Service) UpdateCampaignStatus(ctx context.Context, orgID, id string, status CampaignStatus) (*CampaignDTO, error) {
	if status == "" {
		return nil, apperrors.NewInvalidArgument("status required")
	}
	switch status {
	case domain.CampaignDraft, domain.CampaignActive, domain.CampaignCompleted, domain.CampaignCancelled:
	default:
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "status must be one of draft, active, completed, cancelled", map[string]any{"field": "status", "value": string(status)})
	}
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		current, err := tx.IGA().GetCampaign(ctx, id, orgID)
		if err != nil {
			return err
		}
		if current.Status == status {
			return apperrors.NewConflict("campaign is already in the requested status")
		}
		if !validCampaignTransition(current.Status, status) {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invalid campaign status transition", map[string]any{"from": string(current.Status), "to": string(status), "allowed": campaignStatusTransitions[current.Status]})
		}
		if err := tx.IGA().UpdateCampaignStatus(ctx, orgID, id, status); err != nil {
			return err
		}
		return s.embedAuditEvent(tx, ctx, orgID, "",
			"iga.campaign.status_updated", "access_campaign", id, "access_campaign", "iga.campaign.status_updated",
			map[string]any{auditKeyCampaignID: id, "from": string(current.Status), "to": string(status)})
	})
	if err != nil {
		return nil, err
	}
	return s.GetCampaign(ctx, orgID, id)
}

// DeleteCampaign removes a campaign. Soft or hard — the infra layer decides
// (current schema has no soft-delete column, so this is a hard DELETE).
func (s Service) DeleteCampaign(ctx context.Context, orgID, id string) error {
	if orgID == "" || id == "" {
		return apperrors.NewInvalidArgument("organizationId and id required")
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if _, err := tx.IGA().GetCampaign(ctx, id, orgID); err != nil {
			return err
		}
		if err := tx.IGA().DeleteCampaign(ctx, id, orgID); err != nil {
			return err
		}
		return s.embedAuditEvent(tx, ctx, orgID, "",
			"iga.campaign.deleted", "access_campaign", id, "access_campaign", "iga.campaign.deleted",
			map[string]any{auditKeyCampaignID: id})
	})
}

// ─── SOD Rules ─────────────────────────────────────────────────────────────

func (s Service) ListSODRules(ctx context.Context, q ListSODRulesQuery) ([]SODRuleDTO, int64, error) {
	if q.OrganizationID == "" {
		return nil, 0, apperrors.NewInvalidArgument("organizationId required")
	}
	items, total, err := s.UOW.IGA().ListSODRules(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	out := make([]SODRuleDTO, len(items))
	for i, r := range items {
		out[i] = toSOD(r)
	}
	return out, total, nil
}

func (s Service) GetSODRule(ctx context.Context, orgID, id string) (*SODRuleDTO, error) {
	r, err := s.UOW.IGA().GetSODRule(ctx, id, orgID)
	if err != nil {
		return nil, err
	}
	d := toSOD(r)
	return &d, nil
}

// normalizeSODPair 将角色对规范化为字典序（BE-D-16）：反向对 (B,A) 与
// (A,B) 归一后命中同一唯一约束，绕过不再可能。
func normalizeSODPair(roleA, roleB string) (string, string) {
	if roleB < roleA {
		return roleB, roleA
	}
	return roleA, roleB
}

func (s Service) CreateSODRule(ctx context.Context, cmd CreateSODRuleCommand) (*SODRuleDTO, error) {
	if cmd.OrganizationID == "" || cmd.Name == "" || cmd.RoleAID == "" || cmd.RoleBID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId, name, roleAId, roleBId all required")
	}
	if cmd.RoleAID == cmd.RoleBID {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "roleAId and roleBId must differ", map[string]any{"field": "roleBId", "value": cmd.RoleBID, "expected": "different from roleAId"})
	}
	roleA, roleB := normalizeSODPair(cmd.RoleAID, cmd.RoleBID)
	now := s.now()
	r := &SODRule{
		ID: uuid.NewString(), OrganizationID: cmd.OrganizationID, Name: cmd.Name,
		RoleAID: roleA, RoleBID: roleB, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.IGA().CreateSODRule(ctx, r); err != nil {
			return err
		}
		return s.embedAuditEvent(tx, ctx, r.OrganizationID, "",
			"iga.sod.created", "sod_rule", r.ID, "sod_rule", "iga.sod.created",
			map[string]any{auditKeySODRuleID: r.ID, auditKeyName: r.Name, "roleAId": r.RoleAID, "roleBId": r.RoleBID})
	})
	if err != nil {
		return nil, err
	}
	d := toSOD(r)
	return &d, nil
}

// UpdateSODRule patches the name / roleA / roleB of an existing rule.
// Keeps the roleA != roleB invariant.
func (s Service) UpdateSODRule(ctx context.Context, orgID, id, name, roleA, roleB string) (*SODRuleDTO, error) {
	if orgID == "" || id == "" || name == "" || roleA == "" || roleB == "" {
		return nil, apperrors.NewInvalidArgument("organizationId, id, name, roleAId, roleBId all required")
	}
	if roleA == roleB {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "roleAId and roleBId must differ", map[string]any{"field": "roleBId", "value": roleB, "expected": "different from roleAId"})
	}
	roleA, roleB = normalizeSODPair(roleA, roleB)
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.IGA().UpdateSODRule(ctx, orgID, id, name, roleA, roleB); err != nil {
			return err
		}
		return s.embedAuditEvent(tx, ctx, orgID, "",
			"iga.sod.updated", "sod_rule", id, "sod_rule", "iga.sod.updated",
			map[string]any{auditKeySODRuleID: id, auditKeyName: name, "roleAId": roleA, "roleBId": roleB})
	})
	if err != nil {
		return nil, err
	}
	return s.GetSODRule(ctx, orgID, id)
}

// DeleteSODRule removes an SOD rule by id + org.
func (s Service) DeleteSODRule(ctx context.Context, orgID, id string) error {
	if orgID == "" || id == "" {
		return apperrors.NewInvalidArgument("organizationId and id required")
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.IGA().DeleteSODRule(ctx, id, orgID); err != nil {
			return err
		}
		return s.embedAuditEvent(tx, ctx, orgID, "",
			"iga.sod.deleted", "sod_rule", id, "sod_rule", "iga.sod.deleted",
			map[string]any{auditKeySODRuleID: id})
	})
}

// ─── Lifecycle ─────────────────────────────────────────────────────────────

func (s Service) ListLifecycle(ctx context.Context, q ListLifecycleQuery) ([]LifecycleDTO, int64, error) {
	if q.OrganizationID == "" {
		return nil, 0, apperrors.NewInvalidArgument("organizationId required")
	}
	var items []*AccessLifecycle
	var total int64
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var err error
		items, total, err = tx.IGA().ListLifecycle(ctx, q)
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]LifecycleDTO, len(items))
	for i, l := range items {
		out[i] = toLifecycle(l)
	}
	return out, total, nil
}

// GetLifecycle returns a single lifecycle grant by id + org.
func (s Service) GetLifecycle(ctx context.Context, orgID, id string) (*LifecycleDTO, error) {
	if orgID == "" || id == "" {
		return nil, apperrors.NewInvalidArgument("organizationId and id required")
	}
	var lifecycle *AccessLifecycle
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var err error
		lifecycle, err = tx.IGA().GetLifecycle(ctx, id, orgID)
		return err
	})
	if err != nil {
		return nil, err
	}
	d := toLifecycle(lifecycle)
	return &d, nil
}

// CreateLifecycle creates a lifecycle grant with an explicit expiresAt
// (REST-style POST, unlike the GrantAccess semantic command which uses TTL).
func (s Service) CreateLifecycle(ctx context.Context, orgID, userID, roleID string, expiresAt time.Time) (*LifecycleDTO, error) {
	if orgID == "" || userID == "" || roleID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId, userId, roleId all required")
	}
	if expiresAt.IsZero() {
		return nil, apperrors.NewInvalidArgument("expiresAt required")
	}
	now := s.now()
	if !expiresAt.After(now) {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "expiresAt must be in the future", map[string]any{"field": "expiresAt", "value": expiresAt.UTC().Format(time.RFC3339), "expected": "future"})
	}
	l, err := s.grantWindow(ctx, orgID, userID, roleID, now, expiresAt, "*", "*")
	if err != nil {
		return nil, err
	}
	d := toLifecycle(l)
	return &d, nil
}

// UpdateLifecycleStatus transitions a lifecycle grant. The valid state-machine
// allows only active → {revoked, expired}; any other transition (revoked/expired
// → active, cross-terminal, or same-state) is rejected.
func (s Service) UpdateLifecycleStatus(ctx context.Context, orgID, id string, status LifecycleStatus) (*LifecycleDTO, error) {
	if orgID == "" || id == "" || status == "" {
		return nil, apperrors.NewInvalidArgument("organizationId, id, status all required")
	}
	// 1. Enum membership — block arbitrary strings that slip past DB CHECK.
	if status != domain.LifecycleRevoked && status != domain.LifecycleExpired {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "lifecycle status must be 'revoked' or 'expired'", map[string]any{"field": "status", "value": string(status), "allowed": append([]string{}, terminalLifecycleStatuses...)})
	}
	completed, err := s.reclaimLifecycleByID(ctx, orgID, id, status, "")
	if err != nil {
		return nil, err
	}
	if !completed {
		return nil, apperrors.NewConflict("lifecycle grant is not active; cannot transition status")
	}
	return s.GetLifecycle(ctx, orgID, id)
}

func (s Service) GrantAccess(ctx context.Context, orgID, userID, roleID string, ttl time.Duration) (*LifecycleDTO, error) {
	if orgID == "" || userID == "" || roleID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId, userId, roleId all required")
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	now := s.now()
	l, err := s.grantWindow(ctx, orgID, userID, roleID, now, now.Add(ttl), "*", "*")
	if err != nil {
		return nil, err
	}
	d := toLifecycle(l)
	return &d, nil
}

func (s Service) RevokeAccess(ctx context.Context, orgID, grantID string) error {
	_, err := s.reclaimLifecycleByID(ctx, orgID, grantID, domain.LifecycleRevoked, "")
	return err
}

// ExpireAccesses reclaims every artifact for the expired grants in one batch.
// The whole batch (list + per-grant lock/reclaim/audit) runs in a SINGLE
// transaction (BE-D-17), avoiding the previous N+1 independent transactions.
func (s Service) ExpireAccesses(ctx context.Context, orgID string, before time.Time) (int64, error) {
	if orgID == "" {
		return 0, apperrors.NewInvalidArgument("organizationId required")
	}
	var count int64
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		items, err := tx.IGA().ListExpiringLifecycle(ctx, orgID, before, 1000)
		if err != nil {
			return err
		}
		for _, item := range items {
			completed, err := s.reclaimLifecycleInTx(ctx, tx, item.OrganizationID, item.ID, domain.LifecycleExpired, "")
			if err != nil {
				return err
			}
			if completed {
				count++
			}
		}
		return nil
	})
	if err != nil {
		return count, err
	}
	return count, nil
}

var _ = errors.New

// 安全边界：PEP 中间件（APIAuthorizationMiddleware）已通过 Casbin 验证了权限。
// RLS 提升只在 service 内部生效（事务内 LOCAL set_config），事务结束自动清除。
