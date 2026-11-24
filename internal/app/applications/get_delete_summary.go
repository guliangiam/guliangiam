package applications

import (
	"context"

	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) GetApplicationDeleteSummary(ctx context.Context, organizationID, id string) (*ApplicationDeleteSummary, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	if id == "" {
		return nil, apperrors.NewInvalidArgument("application id is required")
	}

	// 先做租户内读取：跨租户 id 在到达 token/consent 计数前就已 NotFound。
	application, err := s.UOW.Applications().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	activeOnly := true
	_, activeTokenCount, err := s.UOW.Tokens().List(ctx, tokendomain.ListQuery{
		Page:          1,
		PageSize:      1,
		ApplicationID: id,
		ActiveOnly:    &activeOnly,
	})
	if err != nil {
		return nil, err
	}

	activeConsentCount, err := s.UOW.Consents().CountActiveByApplication(ctx, id)
	if err != nil {
		return nil, err
	}

	return &ApplicationDeleteSummary{
		ApplicationID:        application.ID,
		Slug:                 application.Slug,
		DisplayName:          application.DisplayName,
		ProviderBindingCount: int64(len(application.ProviderBindings)),
		ActiveTokenCount:     activeTokenCount,
		ActiveConsentCount:   activeConsentCount,
	}, nil
}
