package invitations

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domainverifications "github.com/guliangiam/guliangiam/internal/domain/verifications"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   uow.IdentityUoW
	// SenderResolver resolves the organization-scoped email sender used by
	// Send/Resend (BE-A-02). Nil keeps the legacy no-delivery behaviour for
	// embedders without messaging; the skip is logged so it stays observable.
	SenderResolver domainverifications.SenderResolver
}

// 租户作用域说明：本包不做 RLS 提升。邀请创建使用 body 的 organizationId；校验/接受按
// code 查询（/api/v1/invitations/verify 为公开路由），可见性取决于调用方上下文与 RLS。
type CreateInvitationCommand struct {
	OrganizationID string
	Email          string
	RoleIDs        []string
	GroupIDs       []string
	InvitedBy      string
	// ValidDays 是邀请有效天数；0 表示使用默认值（7 天），允许范围 1..365。
	ValidDays int
}

type RevokeInvitationCommand struct {
	ID             string
	OrganizationID string
}

type ResendInvitationCommand struct {
	ID             string
	OrganizationID string
}

type AcceptInvitationCommand struct {
	Code string
}

type ValidateInvitationCodeResult struct {
	Valid          bool
	OrganizationID string
	Email          string
	RoleIDs        []string
	GroupIDs       []string
}
