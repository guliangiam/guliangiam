package webhooks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	eventdomain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	domain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

func (s Service) ListWebhooks(ctx context.Context, q ListWebhooksQuery) ([]WebhookListItem, int64, error) {
	items, total, err := s.UOW.Webhooks().List(ctx, domain.ListQuery(q))
	if err != nil {
		return nil, 0, err
	}
	r := make([]WebhookListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}
func (s Service) GetWebhook(ctx context.Context, id, organizationID string) (*WebhookDetail, error) {
	w, err := loadWebhookForTenant(ctx, s.UOW.Webhooks(), id, organizationID)
	if err != nil {
		return nil, err
	}
	d := toDetail(w)
	return &d, nil
}

// CreateWebhook creates a webhook and returns its signing secret ONCE.
//
// The plaintext secret is never persisted or returned by any read endpoint: it
// exists only in this response so the caller can configure HMAC verification on
// the receiving side (mirrors application clientSecretPlain). Deliveries without
// a secret are refused by the worker, so generating one here is what makes the
// endpoint usable at all (DEF-45).
func (s Service) CreateWebhook(ctx context.Context, c CreateWebhookCommand) (string, error) {
	if strings.TrimSpace(c.OrganizationID) == "" {
		return "", apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return "", apperrors.NewInvalidArgument("name is required")
	}
	if err := validateWebhookURL(c.URL); err != nil {
		return "", err
	}
	if err := validateWebhookHeaders(c.Headers); err != nil {
		return "", err
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return "", err
	}
	secret, err := newWebhookSigningSecret()
	if err != nil {
		return "", err
	}
	now := s.Clock.Now()
	isEnabled := true
	if c.IsEnabled != nil {
		isEnabled = *c.IsEnabled
	}
	w := &domain.Webhook{ID: id, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		URL: c.URL, Method: c.Method, Headers: c.Headers, Events: append([]string(nil), c.Events...),
		IsEnabled: isEnabled, RetryCount: c.RetryCount, TimeoutMs: c.TimeoutMs,
		Organization: c.Organization, ContentType: c.ContentType, CustomBody: c.CustomBody, CustomHeaders: c.CustomHeaders,
		CertID: c.CertID, IsError: c.IsError, ErrorText: c.ErrorText,
		TokenFields: append([]string(nil), c.TokenFields...), ObjectFields: append([]string(nil), c.ObjectFields...),
		IsUserExtended: c.IsUserExtended, SingleOrgOnly: c.SingleOrgOnly,
		Secret:     secret,
		MaxRetries: c.MaxRetries, RetryInterval: c.RetryInterval, UseExponentialBackoff: c.UseExponentialBackoff,
		CreatedAt: now, UpdatedAt: now, Revision: 1}
	// 事务失败时不泄露 secret 给调用方；只有提交成功才返回一次。
	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Webhooks().Create(ctx, w); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			w.OrganizationID, "",
			"webhook.created", "Webhook", w.ID,
			"Webhook", "webhook.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return "", err
	}
	return secret, nil
}

// webhookSecretPrefix is the operator-visible marker for webhook signing keys
// (documented as whsec_* in the webhook guide).
const webhookSecretPrefix = "whsec_"

// newWebhookSigningSecret returns a 256-bit random secret in hex. crypto/rand is
// the only source here: UUIDv7 ordering has no bearing on signing secrecy.
func newWebhookSigningSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", apperrors.WrapInternal(err, "generate webhook signing secret")
	}
	return webhookSecretPrefix + hex.EncodeToString(buf), nil
}

func (s Service) PatchWebhook(ctx context.Context, organizationID string, c PatchWebhookCommand) error {
	if c.URL != nil {
		if err := validateWebhookURL(*c.URL); err != nil {
			return err
		}
	}
	if c.Headers != nil {
		if err := validateWebhookHeaders(*c.Headers); err != nil {
			return err
		}
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		w, err := loadWebhookForTenant(ctx, tx.Webhooks(), c.ID, organizationID)
		if err != nil {
			return err
		}
		if c.Revision > 0 && w.Revision != c.Revision {
			return apperrors.NewConflict("webhook revision mismatch")
		}
		if c.DisplayName != nil {
			w.DisplayName = *c.DisplayName
		}
		if c.URL != nil {
			w.URL = *c.URL
		}
		if c.Method != nil {
			w.Method = *c.Method
		}
		if c.Headers != nil {
			w.Headers = *c.Headers
		}
		if c.Secret != nil {
			// PATCH convention: nil keeps the stored secret; a non-nil value
			// replaces it (empty string clears it, which disables signing).
			w.Secret = *c.Secret
		}
		if c.Events != nil {
			w.Events = append([]string(nil), c.Events...)
		}
		if c.IsEnabled != nil {
			w.IsEnabled = *c.IsEnabled
		}
		if c.RetryCount != nil {
			w.RetryCount = *c.RetryCount
		}
		if c.TimeoutMs != nil {
			w.TimeoutMs = *c.TimeoutMs
		}
		if c.Organization != nil {
			w.Organization = *c.Organization
		}
		if c.ContentType != nil {
			w.ContentType = *c.ContentType
		}
		if c.CustomBody != nil {
			w.CustomBody = *c.CustomBody
		}
		if c.CustomHeaders != nil {
			w.CustomHeaders = *c.CustomHeaders
		}
		if c.CertID != nil {
			w.CertID = *c.CertID
		}
		if c.IsError != nil {
			w.IsError = *c.IsError
		}
		if c.ErrorText != nil {
			w.ErrorText = *c.ErrorText
		}
		if c.TokenFields != nil {
			w.TokenFields = append([]string(nil), c.TokenFields...)
		}
		if c.ObjectFields != nil {
			w.ObjectFields = append([]string(nil), c.ObjectFields...)
		}
		if c.IsUserExtended != nil {
			w.IsUserExtended = *c.IsUserExtended
		}
		if c.SingleOrgOnly != nil {
			w.SingleOrgOnly = *c.SingleOrgOnly
		}
		if c.MaxRetries != nil {
			w.MaxRetries = *c.MaxRetries
		}
		if c.RetryInterval != nil {
			w.RetryInterval = *c.RetryInterval
		}
		if c.UseExponentialBackoff != nil {
			w.UseExponentialBackoff = *c.UseExponentialBackoff
		}
		w.UpdatedAt = s.Clock.Now()
		w.Revision++
		if err := tx.Webhooks().Update(ctx, w); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			w.OrganizationID, "",
			"webhook.updated", "Webhook", w.ID,
			"Webhook", "webhook.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) DeleteWebhook(ctx context.Context, id, organizationID string) error {
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		w, err := loadWebhookForTenant(ctx, tx.Webhooks(), id, organizationID)
		if err != nil {
			return err
		}
		if err := tx.Webhooks().Delete(ctx, id); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			w.OrganizationID, "",
			"webhook.deleted", "Webhook", id,
			"Webhook", "webhook.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

// platformTenantScope reports whether ctx carries an authenticated platform
// (super_admin/system) scope. Only those callers may act across tenants; an
// empty trusted organization is otherwise a client error.
func platformTenantScope(ctx context.Context) bool {
	scope := tenantctx.TenantScopeFromContext(ctx)
	return scope.Mode == tenantctx.TenantFilterSuperAdmin || scope.Mode == tenantctx.TenantFilterSystem
}

// requireOrganizationID fails fast when a by-ID management call reaches the app
// layer without a tenant and without a platform scope.
func requireOrganizationID(ctx context.Context, organizationID string) error {
	if strings.TrimSpace(organizationID) == "" && !platformTenantScope(ctx) {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return nil
}

// loadWebhookForTenant resolves a webhook through a tenant-scoped repository
// read. A regular caller with an empty trusted tenant is a client error; an
// authenticated platform scope keeps the all-tenant system lookup.
func loadWebhookForTenant(ctx context.Context, repo domain.Repository, id, organizationID string) (*domain.Webhook, error) {
	if err := requireOrganizationID(ctx, organizationID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(organizationID) == "" {
		return repo.GetByID(ctx, id)
	}

	webhook, err := repo.GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	// The scoped read is the ownership gate; this assertion keeps the
	// not-found shape if a repository regression ever returns a foreign row.
	if err := ensureWebhookVisibleToTenant(webhook, organizationID); err != nil {
		return nil, err
	}
	return webhook, nil
}

// loadWebhookEventForTenant resolves a webhook event through the parent
// webhook's organization_id. webhook_events has no organization_id column, so
// the repository binds the predicate through EXISTS(parent).
func loadWebhookEventForTenant(ctx context.Context, repo eventdomain.Repository, id, organizationID string) (*eventdomain.WebhookEvent, error) {
	if err := requireOrganizationID(ctx, organizationID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(organizationID) == "" {
		return repo.GetByID(ctx, id)
	}
	return repo.GetByIDForOrganization(ctx, id, organizationID)
}

// ensureWebhookVisibleToTenant is a defence-in-depth assertion over a row that
// was already loaded through a tenant-scoped read. A cross-tenant row is
// reported as not-found exactly like a missing one.
func ensureWebhookVisibleToTenant(webhook *domain.Webhook, organizationID string) error {
	if webhook == nil {
		return apperrors.NewNotFound("webhook not found")
	}
	if organizationID != "" && webhook.OrganizationID != organizationID {
		return apperrors.NewNotFound("webhook not found")
	}
	return nil
}
