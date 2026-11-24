package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/guliangiam/guliangiam/internal/domain/gateway"
	domainnotification "github.com/guliangiam/guliangiam/internal/domain/notifications"
)

// evaluateAlerts checks if alert conditions are met and creates notification records
// for the notification worker to deliver asynchronously.
func (s *Service) evaluateAlerts(ctx context.Context, site *gateway.Site) {
	if !site.EnableAlert || len(site.AlertProviderIDs) == 0 {
		return
	}

	logger := slog.Default().With("component", "gateway-alert", keySiteID, site.ID, "siteName", site.Name)

	// Deduplicate before querying: a duplicated provider id in
	// site.AlertProviderIDs would otherwise produce a second, identical alert
	// notification and a redundant provider read for the same alert event.
	seen := make(map[string]bool, len(site.AlertProviderIDs))
	providerIDs := make([]string, 0, len(site.AlertProviderIDs))
	for _, providerID := range site.AlertProviderIDs {
		if providerID == "" || seen[providerID] {
			continue
		}
		seen[providerID] = true
		providerIDs = append(providerIDs, providerID)
	}

	for _, providerID := range providerIDs {
		// Look up the notification provider to verify it exists and is enabled
		provider, err := s.UOW.NotificationProviders().GetByID(ctx, providerID)
		if err != nil {
			logger.Warn("failed to fetch notification provider for alert",
				"provider_id", providerID,
				"error", err,
			)
			continue
		}

		if !provider.IsEnabled {
			logger.Info("skipping disabled notification provider",
				"provider_id", providerID,
			)
			continue
		}

		// Create a notification record; the notification worker will deliver it asynchronously.
		notificationID, uuidErr := s.UUID.NewString()
		if uuidErr != nil {
			logger.Error("failed to generate notification ID", "error", uuidErr)
			continue
		}

		now := s.Clock.Now()
		subject := fmt.Sprintf("Site Alert: %s is unhealthy", site.Name)
		content := fmt.Sprintf("Site %q (host: %s) health check failed at %s. Last status: %s.",
			site.Name, site.Host, now.Format(time.RFC3339), site.LastHealthStatus)

		notification := &domainnotification.Notification{
			ID:             notificationID,
			OrganizationID: site.OrganizationID,
			ProviderID:     providerID,
			Recipient:      provider.WebhookURL,
			Subject:        subject,
			Content:        content,
			Status:         "pending",
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		if createErr := s.UOW.Notifications().Create(ctx, notification); createErr != nil {
			logger.Error("failed to create alert notification",
				"provider_id", providerID,
				"error", createErr,
			)
			continue
		}

		logger.Info("alert notification created",
			"provider_id", providerID,
			"notification_id", notificationID,
		)
	}
}

// AlertResult contains the result of an alert evaluation.
type AlertResult struct {
	SiteID    string   `json:"siteId"`
	SiteName  string   `json:"siteName"`
	AlertSent bool     `json:"alertSent"`
	Providers []string `json:"providers,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// SendSiteAlert manually triggers an alert for a site.
func (s *Service) SendSiteAlert(ctx context.Context, siteID string) (*AlertResult, error) {
	site, err := s.UOW.Gateway().GetSiteByID(ctx, siteID)
	if err != nil {
		return nil, err
	}

	result := &AlertResult{
		SiteID:    site.ID,
		SiteName:  site.Name,
		Providers: site.AlertProviderIDs,
	}

	if !site.EnableAlert {
		result.Error = "alerts are not enabled for this site"
		return result, nil
	}

	if len(site.AlertProviderIDs) == 0 {
		result.Error = "no alert providers configured"
		return result, nil
	}

	// Send alert through each configured provider by creating notification records
	s.evaluateAlerts(ctx, site)
	result.AlertSent = true

	return result, nil
}
