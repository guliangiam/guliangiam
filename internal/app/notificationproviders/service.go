package notificationproviders

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   uow.InfraUoW
}

type CreateNotificationProviderCommand struct {
	OrganizationID string
	Name           string
	ProviderType   string
	WebhookURL     string
	AccessToken    string
	Config         map[string]any
	IsEnabled      *bool
}

func (s Service) NewNotificationProvider(command CreateNotificationProviderCommand) (*domain.NotificationProvider, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	isEnabled := true
	if command.IsEnabled != nil {
		isEnabled = *command.IsEnabled
	}

	config := command.Config
	if config == nil {
		config = map[string]any{}
	}

	return &domain.NotificationProvider{
		ID:             id,
		OrganizationID: command.OrganizationID,
		Name:           command.Name,
		ProviderType:   command.ProviderType,
		WebhookURL:     command.WebhookURL,
		AccessToken:    command.AccessToken,
		Config:         config,
		IsEnabled:      isEnabled,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}, nil
}
