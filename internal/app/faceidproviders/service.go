package faceidproviders

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   uow.AuthUoW
}

// 租户作用域说明：本包不做 RLS 提升。faceidproviders 的 organizationId 由 query/body
// 传入（chi.go:16），普通会话受 RLS 限制在本组织；跨组织需 ScopeAuthorizer 先放行。
type CreateFaceIdProviderCommand struct {
	OrganizationID  string
	Name            string
	ProviderType    string
	AccessKeyID     string
	AccessKeySecret string
	Endpoint        string
	Config          map[string]any
	IsEnabled       *bool
}

func (s Service) NewFaceIdProvider(command CreateFaceIdProviderCommand) (*domain.FaceIdProvider, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	isEnabled := true
	if command.IsEnabled != nil {
		isEnabled = *command.IsEnabled
	}

	config := make(map[string]any)
	for k, v := range command.Config {
		config[k] = v
	}

	return &domain.FaceIdProvider{
		ID:              id,
		OrganizationID:  command.OrganizationID,
		Name:            command.Name,
		ProviderType:    command.ProviderType,
		AccessKeyID:     command.AccessKeyID,
		AccessKeySecret: command.AccessKeySecret,
		Endpoint:        command.Endpoint,
		Config:          config,
		IsEnabled:       isEnabled,
		CreatedAt:       now,
		UpdatedAt:       now,
		Revision:        1,
	}, nil
}
