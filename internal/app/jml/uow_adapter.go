package jml

import (
	"context"
	"encoding/json"
	"time"

	igaapp "github.com/guliangiam/guliangiam/internal/app/iga"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	sessionsdomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	tokensdomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	usersdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
)

// LifecyclePort 是 users/scim 服务注入的事务感知 JML 端口。tx 参数必须是
// 调用方 WithinTransaction 回调里的事务 UoW —— 绑定创建、SoD 复检、会话/
// 令牌吊销与 outbox 写入全部与业务事实同事务（ADR-030 Decision 4）。
type LifecyclePort interface {
	OnUserCreated(ctx context.Context, tx appuow.UnitOfWork, userID, orgID string, userPayload []byte, initialRoles []string) error
	OnRoleBindingChanged(ctx context.Context, tx appuow.UnitOfWork, userID, orgID string, addedRoles, removedRoles []string, userPayload []byte) error
	OnUserDeactivated(ctx context.Context, tx appuow.UnitOfWork, userID, orgID string, userPayload []byte) error
}

// UoWLifecycle 是 LifecyclePort 的生产实现：每次调用从事务 UoW 构造一个
// 绑定该事务的 Orchestrator（UoW 不把 tx 放进 ctx，适配器必须显式接 tx）。
type UoWLifecycle struct {
	Clock clock.Clock
}

func NewUoWLifecycle(clk clock.Clock) UoWLifecycle { return UoWLifecycle{Clock: clk} }

func (l UoWLifecycle) timeNow() time.Time {
	if l.Clock != nil {
		return l.Clock.Now()
	}
	return time.Now()
}

func (l UoWLifecycle) orchestratorFor(tx appuow.UnitOfWork) (*Orchestrator, error) {
	roles, err := newRoleBindingAdapter(tx.Users())
	if err != nil {
		return nil, err
	}
	return &Orchestrator{
		Users:    tx.Users(),
		Roles:    roles,
		IGA:      igaapp.NewJMLAdapter(tx, l.Clock),
		Outbox:   outboxEnqueueAdapter{repo: tx.Outbox()},
		Sessions: sessionRevokerAdapter{repo: tx.Sessions(), now: l.timeNow},
		Tokens:   tokenRevokerAdapter{repo: tx.Tokens(), now: l.timeNow},
		TimeNow:  l.timeNow,
	}, nil
}

func (l UoWLifecycle) OnUserCreated(ctx context.Context, tx appuow.UnitOfWork, userID, orgID string, userPayload []byte, initialRoles []string) error {
	o, err := l.orchestratorFor(tx)
	if err != nil {
		return err
	}
	return o.OnUserCreated(ctx, userID, orgID, userPayload, initialRoles)
}

func (l UoWLifecycle) OnRoleBindingChanged(ctx context.Context, tx appuow.UnitOfWork, userID, orgID string, addedRoles, removedRoles []string, userPayload []byte) error {
	o, err := l.orchestratorFor(tx)
	if err != nil {
		return err
	}
	return o.OnRoleBindingChanged(ctx, userID, orgID, addedRoles, removedRoles, userPayload)
}

func (l UoWLifecycle) OnUserDeactivated(ctx context.Context, tx appuow.UnitOfWork, userID, orgID string, userPayload []byte) error {
	o, err := l.orchestratorFor(tx)
	if err != nil {
		return err
	}
	return o.OnUserDeactivated(ctx, userID, orgID, userPayload)
}

// SafeUserPayload 构造 JML 出站事件的用户载荷：只含元数据，严禁凭据/哈希。
func SafeUserPayload(u *usersdomain.User) []byte {
	if u == nil {
		return nil
	}
	b, err := json.Marshal(map[string]any{
		"user_id":         u.ID,
		"username":        u.Username,
		"organization_id": u.OrganizationID,
		"external_id":     u.ExternalID,
		"active":          !u.IsForbidden,
	})
	if err != nil {
		return nil
	}
	return b
}

// ─── 仓储适配器 ─────────────────────────────────────────────────────────

// roleBindingCapability 是 postgres users.Repository 为 JML 提供的精细化
// 绑定能力（结构满足，不进 domain users.Repository 接口）。MySQL 镜像未
// 实现（OPEN-API-37）：断言失败 → JML 不可用，bootstrap 据此跳过注入。
type roleBindingCapability interface {
	CreateRoleBinding(ctx context.Context, binding *usersdomain.UserRoleBinding) error
	DeleteRoleBinding(ctx context.Context, userID, roleID string) error
	DeleteRoleBindingsByUser(ctx context.Context, userID string) error
	ListRoleBindingsByUser(ctx context.Context, userID string) ([]usersdomain.UserRoleBinding, error)
}

type roleBindingAdapter struct{ repo roleBindingCapability }

func newRoleBindingAdapter(repo usersdomain.Repository) (RoleBindingRepository, error) {
	capability, ok := repo.(roleBindingCapability)
	if !ok {
		return nil, apperrors.NewUnavailable("users repository lacks JML role binding capability")
	}
	return roleBindingAdapter{repo: capability}, nil
}

func (a roleBindingAdapter) Create(ctx context.Context, binding *usersdomain.UserRoleBinding) error {
	return a.repo.CreateRoleBinding(ctx, binding)
}

func (a roleBindingAdapter) DeleteByUser(ctx context.Context, userID string) error {
	return a.repo.DeleteRoleBindingsByUser(ctx, userID)
}

func (a roleBindingAdapter) Delete(ctx context.Context, userID, roleID string) error {
	return a.repo.DeleteRoleBinding(ctx, userID, roleID)
}

func (a roleBindingAdapter) ListByUser(ctx context.Context, userID string) ([]*usersdomain.UserRoleBinding, error) {
	bindings, err := a.repo.ListRoleBindingsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]*usersdomain.UserRoleBinding, 0, len(bindings))
	for i := range bindings {
		out = append(out, &bindings[i])
	}
	return out, nil
}

// outboxEnqueueAdapter 把 JML 的 platform OutboxItem Insert 适配到事务
// UoW 的 domain outbox Enqueue（同一事务写入，无需能力断言）。
type outboxEnqueueAdapter struct{ repo domainoutbox.Repository }

func (a outboxEnqueueAdapter) Insert(ctx context.Context, item *platformoutbox.OutboxItem) error {
	if a.repo == nil {
		return apperrors.NewUnavailable("outbox repository unavailable")
	}
	return a.repo.Enqueue(ctx, &domainoutbox.OutboxEvent{
		ID:             item.ID,
		OrganizationID: item.OrganizationID,
		AggregateType:  item.AggregateType,
		AggregateID:    item.AggregateID,
		EventType:      item.EventType,
		EventVersion:   item.EventVersion,
		Payload:        item.Payload,
		Headers:        item.Headers,
		MaxAttempts:    item.MaxAttempts,
		AvailableAt:    item.AvailableAt,
	})
}

type sessionRevokerAdapter struct {
	repo sessionsdomain.Repository
	now  func() time.Time
}

func (a sessionRevokerAdapter) RevokeAll(ctx context.Context, userID string) error {
	if a.repo == nil {
		return apperrors.NewUnavailable("sessions repository unavailable")
	}
	_, err := a.repo.RevokeByUser(ctx, userID, a.now())
	return err
}

type tokenRevokerAdapter struct {
	repo tokensdomain.Repository
	now  func() time.Time
}

func (a tokenRevokerAdapter) RevokeAll(ctx context.Context, userID string) error {
	if a.repo == nil {
		return apperrors.NewUnavailable("tokens repository unavailable")
	}
	now := a.now()
	if _, err := a.repo.RevokeRefreshByUser(ctx, userID, now); err != nil {
		return err
	}
	_, err := a.repo.RevokeByQuery(ctx, tokensdomain.RevokeQuery{UserID: userID}, now)
	return err
}
