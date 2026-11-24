package risk

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	notificationproviderdomain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	notificationdomain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	domain "github.com/guliangiam/guliangiam/internal/domain/risk"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
)

const (
	// criticalAlertMaxProviders bounds how many enabled providers we fan out to.
	// Every extra provider multiplies the notification rows by the recipient
	// count, so the cap keeps a misconfigured tenant from flooding the queue.
	criticalAlertMaxProviders = 5
	// criticalAlertMaxRecipients bounds user + administrator recipients.
	criticalAlertMaxRecipients = 10
	// criticalAlertAdminPageSize bounds the administrator lookup. Administrators
	// are fetched with an is_admin filter, so this is a page size, not a scan.
	criticalAlertAdminPageSize = 50
)

// criticalAlertEvent is the deny reason as seen by the notification layer.
// It deliberately carries no password, token, or full user profile.
type criticalAlertEvent struct {
	OrganizationID string
	UserID         string
	DecisionID     string
	Channel        string
	Score          int
	IP             string
	UserAgent      string
}

// notifyCriticalLoginDenied satisfies AC-RISK-001: a critical (deny) decision
// rejects the login AND notifies both the affected user and the organization's
// administrators.
//
// Delivery is asynchronous: rows go into the notifications table with status
// "pending" and the notification worker pushes them through the tenant's
// enabled providers. This keeps several network round-trips out of the login
// path while still leaving a durable, auditable record.
//
// Every failure here is logged and swallowed. The deny decision has already
// been enforced and audited; a tenant with no provider configured, or an
// unreachable SMTP server, must not turn a security rejection into a 500. The
// missing-notification case is itself audited so operators can see it.
func (s *Service) notifyCriticalLoginDenied(ctx context.Context, tx appuow.UnitOfWork, event criticalAlertEvent, now time.Time) {
	// 站内收件箱与外发台账解耦：先把告警写进通知中心，再尝试外部渠道。
	// 这样即使租户没有配置通知提供商，管理员仍能在通知页面看到告警。
	s.notifyCriticalAlertInbox(ctx, tx, event, now)

	providers, err := s.enabledAlertProviders(ctx, tx, event.OrganizationID)
	if err != nil {
		slog.WarnContext(ctx, "critical risk alert: provider lookup failed",
			"organization_id", event.OrganizationID, "decision_id", event.DecisionID, "error", err)
		s.auditAlertDispatch(ctx, tx, event, now, 0, 0, "provider_lookup_failed")
		return
	}
	if len(providers) == 0 {
		slog.WarnContext(ctx, "critical risk alert: no enabled notification provider",
			"organization_id", event.OrganizationID, "decision_id", event.DecisionID)
		s.auditAlertDispatch(ctx, tx, event, now, 0, 0, "no_provider")
		return
	}

	recipients := s.criticalAlertRecipients(ctx, tx, event)
	if len(recipients) == 0 {
		slog.WarnContext(ctx, "critical risk alert: no reachable recipient",
			"organization_id", event.OrganizationID, "decision_id", event.DecisionID)
		s.auditAlertDispatch(ctx, tx, event, now, len(providers), 0, "no_recipient")
		return
	}

	subject := "[Security Alert] Login blocked by risk assessment"
	content := buildCriticalAlertContent(event, now)
	created := 0
	for _, provider := range providers {
		for _, recipient := range recipients {
			notificationID, idErr := s.UUID.NewString()
			if idErr != nil {
				slog.ErrorContext(ctx, "critical risk alert: generate notification id",
					"organization_id", event.OrganizationID, "error", idErr)
				continue
			}
			notification := &notificationdomain.Notification{
				ID:             notificationID,
				OrganizationID: event.OrganizationID,
				ProviderID:     provider.ID,
				Recipient:      recipient,
				Subject:        subject,
				Content:        content,
				Variables: map[string]any{
					"category":    "risk.critical_login_denied",
					keyDecisionID: event.DecisionID,
					keyChannel:    event.Channel,
					keyScore:      event.Score,
					"loginUserId": event.UserID,
					"ip":          event.IP,
					"detectedAt":  now.Format(time.RFC3339),
				},
				Status:    "pending",
				CreatedAt: now,
				UpdatedAt: now,
			}
			if createErr := tx.Notifications().Create(ctx, notification); createErr != nil {
				slog.ErrorContext(ctx, "critical risk alert: create notification failed",
					"organization_id", event.OrganizationID, "provider_id", provider.ID, "error", createErr)
				continue
			}
			created++
		}
	}

	slog.InfoContext(ctx, "critical risk alert dispatched",
		"organization_id", event.OrganizationID, "decision_id", event.DecisionID,
		"channel", event.Channel, "providers", len(providers),
		"recipients", len(recipients), "notifications", created)

	if created == 0 {
		s.auditAlertDispatch(ctx, tx, event, now, len(providers), len(recipients), "create_failed")
		return
	}
	s.auditAlertDispatch(ctx, tx, event, now, len(providers), len(recipients), "")
}

// enabledAlertProviders returns the tenant's enabled notification providers,
// capped at criticalAlertMaxProviders so one tenant cannot multiply the fan-out.
func (s *Service) enabledAlertProviders(ctx context.Context, tx appuow.UnitOfWork, organizationID string) ([]*notificationproviderdomain.NotificationProvider, error) {
	enabled := true
	providers, _, err := tx.NotificationProviders().List(ctx, notificationproviderdomain.ListQuery{
		OrganizationID: organizationID,
		IsEnabled:      &enabled,
		Page:           1,
		PageSize:       criticalAlertMaxProviders,
	})
	if err != nil {
		return nil, err
	}
	if len(providers) > criticalAlertMaxProviders {
		providers = providers[:criticalAlertMaxProviders]
	}
	return providers, nil
}

// criticalAlertRecipients resolves the affected user plus the organization's
// administrators into a de-duplicated email list.
//
// The user is addressed by id; administrators are found with an is_admin filter
// rather than by paging the whole tenant. Recipients without an email address
// are skipped — an inbox we cannot address would only produce a failed row.
func (s *Service) criticalAlertRecipients(ctx context.Context, tx appuow.UnitOfWork, event criticalAlertEvent) []string {
	seen := make(map[string]struct{}, criticalAlertMaxRecipients)
	recipients := make([]string, 0, criticalAlertMaxRecipients)

	add := func(email string) {
		normalized := strings.ToLower(strings.TrimSpace(email))
		if normalized == "" {
			return
		}
		if _, ok := seen[normalized]; ok {
			return
		}
		seen[normalized] = struct{}{}
		recipients = append(recipients, normalized)
	}

	if event.UserID != "" {
		user, err := tx.Users().GetByID(ctx, event.UserID)
		if err != nil {
			slog.WarnContext(ctx, "critical risk alert: affected user lookup failed",
				"organization_id", event.OrganizationID, "user_id", event.UserID, "error", err)
		} else {
			add(user.Profile.Email)
		}
	}

	isAdmin := true
	admins, _, err := tx.Users().List(ctx, userdomain.ListQuery{
		OrganizationID: event.OrganizationID,
		IsAdmin:        &isAdmin,
		Page:           1,
		PageSize:       criticalAlertAdminPageSize,
	})
	if err != nil {
		slog.WarnContext(ctx, "critical risk alert: administrator lookup failed",
			"organization_id", event.OrganizationID, "error", err)
	} else {
		for _, admin := range admins {
			if len(recipients) >= criticalAlertMaxRecipients {
				break
			}
			add(admin.Profile.Email)
		}
	}

	if len(recipients) > criticalAlertMaxRecipients {
		recipients = recipients[:criticalAlertMaxRecipients]
	}
	return recipients
}

// auditAlertDispatch records how the alert fan-out went. reason is empty on a
// successful dispatch; otherwise it names the step that produced no rows so the
// audit trail distinguishes "nothing to notify" from "notified".
func (s *Service) auditAlertDispatch(ctx context.Context, tx appuow.UnitOfWork, event criticalAlertEvent, now time.Time, providers, recipients int, reason string) {
	payload := map[string]any{
		keyDecisionID: event.DecisionID,
		keyOrgID:      event.OrganizationID,
		keyChannel:    event.Channel,
		"providers":   providers,
		"recipients":  recipients,
		"detectedAt":  now.Format(time.RFC3339),
	}
	if reason != "" {
		payload["skippedReason"] = reason
	}
	if err := s.embedAuditEvent(
		tx, ctx,
		event.IP, event.UserAgent, "", "", "",
		event.OrganizationID, event.UserID,
		"risk.critical_alert.dispatched", "risk_decision", event.DecisionID,
		"risk", "risk.critical_alert.dispatched",
		payload,
	); err != nil {
		slog.ErrorContext(ctx, "critical risk alert: audit write failed",
			"organization_id", event.OrganizationID, "decision_id", event.DecisionID, "error", err)
	}
}

func buildCriticalAlertContent(event criticalAlertEvent, now time.Time) string {
	var builder strings.Builder
	builder.WriteString("A login attempt was blocked by the adaptive risk engine.\n\n")
	fmt.Fprintf(&builder, "Detection time: %s\n", now.Format(time.RFC3339))
	fmt.Fprintf(&builder, "Login channel: %s\n", event.Channel)
	fmt.Fprintf(&builder, "Risk score: %d\n", event.Score)
	fmt.Fprintf(&builder, "Decision ID: %s\n", event.DecisionID)
	if event.IP != "" {
		fmt.Fprintf(&builder, "Source IP: %s\n", event.IP)
	}
	builder.WriteString("\nIf you do not recognize this activity, review the user's session and credential state in the " +
		"security console. Do not reply to this message with credentials.")
	return builder.String()
}

// Ensure the compile-time port contract stays in sync: risk.Service must
// satisfy the domain port consumed by auth and the other login surfaces.
var _ domain.LoginRiskEvaluator = (*Service)(nil)
