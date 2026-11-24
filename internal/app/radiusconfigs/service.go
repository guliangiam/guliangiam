package radiusconfigs

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   uow.InfraUoW
}

// 租户作用域说明：本包不做 RLS 提升。radiusconfigs 的 organizationId 来自 query/body
// （chi.go:16），仓储按 organization_id 过滤；跨组织访问需 ScopeAuthorizer 放行。
type CreateRadiusConfigCommand struct {
	OrganizationID string
	Name           string
	Host           string
	Port           int
	SharedSecret   string
	AuthProtocol   string
	IsEnabled      *bool
}

func (s Service) NewRadiusConfig(command CreateRadiusConfigCommand) (*domain.RadiusConfig, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	isEnabled := true
	if command.IsEnabled != nil {
		isEnabled = *command.IsEnabled
	}

	return &domain.RadiusConfig{
		ID:             id,
		OrganizationID: command.OrganizationID,
		Name:           command.Name,
		Host:           command.Host,
		Port:           command.Port,
		SharedSecret:   command.SharedSecret,
		AuthProtocol:   command.AuthProtocol,
		IsEnabled:      isEnabled,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}, nil
}
