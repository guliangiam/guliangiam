package iga

import (
	"context"
	"fmt"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/iga"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/pgerror"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

var _ domain.Repository = Repository{}

var campaignConstraints = []string{
	"uq_access_campaigns_org_name",
	"ck_access_campaigns_status",
	"fk_access_campaigns_organization_id_organizations",
}

var sodRuleConstraints = []string{
	"uq_sod_rules_org_name",
	"uq_sod_rules_org_roles",
	"fk_sod_rules_role_a_id_roles",
	"fk_sod_rules_role_b_id_roles",
	"fk_sod_rules_organization_id_organizations",
}

func (r Repository) ListCampaigns(ctx context.Context, q domain.ListCampaignsQuery) ([]*domain.AccessCampaign, int64, error) {
	args := []any{q.OrganizationID}
	extra := ""
	if q.Status != nil && *q.Status != "" {
		extra, args = shared.AppendParamCondition(extra, "status", string(*q.Status), args)
	}

	var total int64
	if err := r.db.QueryRow(ctx, qCountCampaignsBase+extra, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, qListCampaignsBase+extra+" ORDER BY updated_at DESC"+fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2), append(append([]any{}, args...), q.PageSize, shared.PageOffset(q.Page, q.PageSize))...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []*domain.AccessCampaign{}
	for rows.Next() {
		var c domain.AccessCampaign
		var statusStr string
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.Name, &statusStr, &c.StartDate, &c.EndDate, &c.CreatedAt, &c.UpdatedAt, &c.Revision); err != nil {
			return nil, 0, err
		}
		c.Status = domain.CampaignStatus(statusStr)
		items = append(items, &c)
	}
	return items, total, rows.Err()
}

func (r Repository) GetCampaign(ctx context.Context, id, orgID string) (*domain.AccessCampaign, error) {
	var c domain.AccessCampaign
	var statusStr string
	if err := r.db.QueryRow(ctx, qGetCampaign, id, orgID).Scan(&c.ID, &c.OrganizationID, &c.Name, &statusStr, &c.StartDate, &c.EndDate, &c.CreatedAt, &c.UpdatedAt, &c.Revision); err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("access campaign not found")
		}
		return nil, err
	}
	c.Status = domain.CampaignStatus(statusStr)
	return &c, nil
}

func (r Repository) CreateCampaign(ctx context.Context, c *domain.AccessCampaign) error {
	_, err := r.db.Exec(ctx, qInsertCampaign, c.ID, c.OrganizationID, c.Name, c.StartDate, c.EndDate)
	return pgerror.Map(err, campaignConstraints...)
}

func (r Repository) UpdateCampaignStatus(ctx context.Context, orgID, id string, status domain.CampaignStatus) error {
	cmd, err := r.db.Exec(ctx, qUpdateCampaignStatus, string(status), id, orgID)
	if err != nil {
		return pgerror.Map(err, campaignConstraints...)
	}
	if cmd.RowsAffected() == 0 {
		return apperrors.NewNotFound("access campaign not found")
	}
	return nil
}

func (r Repository) DeleteCampaign(ctx context.Context, id, orgID string) error {
	cmd, err := r.db.Exec(ctx, qDeleteCampaign, id, orgID)
	if err != nil {
		return pgerror.Map(err, campaignConstraints...)
	}
	if cmd.RowsAffected() == 0 {
		return apperrors.NewNotFound("access campaign not found")
	}
	return nil
}

func (r Repository) ListSODRules(ctx context.Context, q domain.ListSODRulesQuery) ([]*domain.SODRule, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM guliangiam.sod_rules WHERE organization_id = $1", q.OrganizationID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, qListSODRules+fmt.Sprintf(" LIMIT %d OFFSET %d", q.PageSize, shared.PageOffset(q.Page, q.PageSize)), q.OrganizationID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []*domain.SODRule{}
	for rows.Next() {
		var s domain.SODRule
		if err := rows.Scan(&s.ID, &s.OrganizationID, &s.Name, &s.RoleAID, &s.RoleBID, &s.CreatedAt, &s.UpdatedAt, &s.Revision); err != nil {
			return nil, 0, err
		}
		items = append(items, &s)
	}
	return items, total, rows.Err()
}

// ListSODRulesByRoleIDs 单查询返回与 roleIDs 任一端相关的全部 SOD 规则。
// 不在 domain.Repository 接口内（MySQL 镜像受 OPEN-API-37 约束），
// app 层通过可选接口断言使用，未实现时回退分页扫描。
func (r Repository) ListSODRulesByRoleIDs(ctx context.Context, orgID string, roleIDs []string) ([]*domain.SODRule, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, qListSODRulesByRoleIDs, orgID, roleIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*domain.SODRule{}
	for rows.Next() {
		var s domain.SODRule
		if err := rows.Scan(&s.ID, &s.OrganizationID, &s.Name, &s.RoleAID, &s.RoleBID, &s.CreatedAt, &s.UpdatedAt, &s.Revision); err != nil {
			return nil, err
		}
		items = append(items, &s)
	}
	return items, rows.Err()
}

func (r Repository) GetSODRule(ctx context.Context, id, orgID string) (*domain.SODRule, error) {
	var s domain.SODRule
	if err := r.db.QueryRow(ctx, qGetSODRule, id, orgID).Scan(&s.ID, &s.OrganizationID, &s.Name, &s.RoleAID, &s.RoleBID, &s.CreatedAt, &s.UpdatedAt, &s.Revision); err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("sod rule not found")
		}
		return nil, err
	}
	return &s, nil
}

func (r Repository) CreateSODRule(ctx context.Context, s *domain.SODRule) error {
	_, err := r.db.Exec(ctx, qInsertSODRule, s.ID, s.OrganizationID, s.Name, s.RoleAID, s.RoleBID)
	return pgerror.Map(err, sodRuleConstraints...)
}

func (r Repository) UpdateSODRule(ctx context.Context, orgID, id, name, roleA, roleB string) error {
	cmd, err := r.db.Exec(ctx, qUpdateSODRule, name, roleA, roleB, id, orgID)
	if err != nil {
		return pgerror.Map(err, sodRuleConstraints...)
	}
	if cmd.RowsAffected() == 0 {
		return apperrors.NewNotFound("sod rule not found")
	}
	return nil
}

func (r Repository) DeleteSODRule(ctx context.Context, id, orgID string) error {
	cmd, err := r.db.Exec(ctx, qDeleteSODRule, id, orgID)
	if err != nil {
		return pgerror.Map(err, sodRuleConstraints...)
	}
	if cmd.RowsAffected() == 0 {
		return apperrors.NewNotFound("sod rule not found")
	}
	return nil
}

func (r Repository) ListLifecycle(ctx context.Context, q domain.ListLifecycleQuery) ([]*domain.AccessLifecycle, int64, error) {
	args := []any{q.OrganizationID}
	extra := ""
	if q.UserID != nil && *q.UserID != "" {
		extra, args = shared.AppendParamCondition(extra, "user_id", *q.UserID, args)
	}
	if q.RoleID != nil && *q.RoleID != "" {
		extra, args = shared.AppendParamCondition(extra, "role_id", *q.RoleID, args)
	}
	if q.Status != nil && *q.Status != "" {
		extra, args = shared.AppendParamCondition(extra, "status", string(*q.Status), args)
	}

	var total int64
	if err := r.db.QueryRow(ctx, qCountLifecycleBase+extra, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, qListLifecycleBase+extra+" ORDER BY expires_at DESC"+fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2), append(append([]any{}, args...), q.PageSize, shared.PageOffset(q.Page, q.PageSize))...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []*domain.AccessLifecycle{}
	for rows.Next() {
		var l domain.AccessLifecycle
		var statusStr string
		if err := rows.Scan(&l.ID, &l.OrganizationID, &l.UserID, &l.RoleID, &l.GrantedAt, &l.ExpiresAt, &statusStr, &l.CreatedAt, &l.Revision); err != nil {
			return nil, 0, err
		}
		l.Status = domain.LifecycleStatus(statusStr)
		items = append(items, &l)
	}
	return items, total, rows.Err()
}

func (r Repository) CreateLifecycle(ctx context.Context, l *domain.AccessLifecycle) error {
	_, err := r.db.Exec(ctx, qInsertLifecycle, l.ID, l.OrganizationID, l.UserID, l.RoleID,
		nullableUUID(l.TicketID), nullableUUID(l.EnforcerID), l.Resource, l.Action,
		nullableUUID(l.PolicyID), l.GrantedAt, l.StartsAt, l.ExpiresAt, l.Status)
	return err
}

func (r Repository) GetLifecycle(ctx context.Context, id, orgID string) (*domain.AccessLifecycle, error) {
	l, err := scanLifecycle(r.db.QueryRow(ctx, qGetLifecycle, id, orgID))
	if err == pgx.ErrNoRows {
		return nil, apperrors.NewNotFound("lifecycle grant not found")
	}
	return l, err
}

func (r Repository) UpdateLifecycleStatus(ctx context.Context, orgID, id string, status domain.LifecycleStatus) error {
	_, err := r.db.Exec(ctx, qUpdateLifecycleStatus, string(status), id, orgID)
	return err
}

func (r Repository) RevokeLifecycle(ctx context.Context, id, orgID string) error {
	_, err := r.db.Exec(ctx, qRevokeLifecycle, id, orgID)
	return err
}

func (r Repository) ExpireLifecycle(ctx context.Context, orgID string, before time.Time) (int64, error) {
	cmd, err := r.db.Exec(ctx, qExpireLifecycle, orgID, before)
	if err != nil {
		return 0, err
	}
	return cmd.RowsAffected(), nil
}
