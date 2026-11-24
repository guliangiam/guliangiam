package roles

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/roles"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock         clock.Clock
	UUID          platformuuid.Generator
	UOW           uow.IdentityUoW
	CasbinManager domain.PolicyManager
}

// 租户作用域说明：本包不做 RLS 提升。roles 的 organizationId 由 transport 层
// 用 TrustedOrganizationID 收敛（query 读、body 写、bulk-import 每行）；角色变更
// 同步写入 Casbin 策略，落库与策略使用同一个可信租户。
type CreateRoleCommand struct {
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	GroupIDs       []string
	Users          []string
	RoleIDs        []string
	Domains        []string
	IsEnabled      *bool
}

func (s *Service) NewRole(command CreateRoleCommand) (*domain.Role, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	isEnabled := true
	if command.IsEnabled != nil {
		isEnabled = *command.IsEnabled
	}

	return &domain.Role{
		ID:             id,
		OrganizationID: command.OrganizationID,
		Name:           command.Name,
		DisplayName:    command.DisplayName,
		Description:    command.Description,
		GroupIDs:       append([]string(nil), command.GroupIDs...),
		Users:          append([]string(nil), command.Users...),
		RoleIDs:        append([]string(nil), command.RoleIDs...),
		Domains:        append([]string(nil), command.Domains...),
		IsEnabled:      isEnabled,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}, nil
}
