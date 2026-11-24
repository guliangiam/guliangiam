package rules

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/rules"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   uow.AuditUoW
}

type CreateRuleCommand struct {
	OrganizationID string
	Name           string
	Scope          string
	RuleType       string
	Conditions     []map[string]any
	Action         string
	Priority       int
	IsEnabled      *bool
}

func (s Service) NewRule(command CreateRuleCommand) (*domain.Rule, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	isEnabled := true
	if command.IsEnabled != nil {
		isEnabled = *command.IsEnabled
	}

	conditions := make([]map[string]any, 0, len(command.Conditions))
	conditions = append(conditions, command.Conditions...)

	return &domain.Rule{
		ID:             id,
		OrganizationID: command.OrganizationID,
		Name:           command.Name,
		Scope:          command.Scope,
		RuleType:       command.RuleType,
		Conditions:     conditions,
		Action:         command.Action,
		Priority:       command.Priority,
		IsEnabled:      isEnabled,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}, nil
}

// 租户作用域说明：by-ID 路由在 transport 解析可信租户，app 层拒绝空租户，
// 仓储按 organization_id 过滤；跨组织写入需上游 ScopeAuthorizer 授权。
