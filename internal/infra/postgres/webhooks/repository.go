package webhooks

import (
	"context"
	"encoding/json"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Webhook, error) {
	return r.getByID(ctx, queryGetWebhookByID, id)
}

// GetByIDForOrganization binds the by-ID read to the trusted tenant; a foreign
// or missing id is the same NotFound.
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Webhook, error) {
	return r.getByID(ctx, queryGetWebhookByIDForOrganization, id, organizationID)
}

func (r Repository) getByID(ctx context.Context, query string, args ...any) (*domain.Webhook, error) {
	var row webhookRow
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
		&row.URL, &row.Method, &row.Headers, &row.Events,
		&row.IsEnabled, &row.Secret, &row.RetryCount, &row.TimeoutMs,
		&row.Organization, &row.ContentType, &row.CustomBody, &row.CustomHeaders,
		&row.CertID, &row.IsError, &row.ErrorText,
		&row.TokenFields, &row.ObjectFields, &row.IsUserExtended,
		&row.SingleOrgOnly, &row.MaxRetries, &row.RetryInterval, &row.UseExponentialBackoff,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("webhook not found")
		}
		return nil, err
	}

	webhook, err := toWebhook(row)
	if err != nil {
		return nil, apperrors.NewInternal("decode webhook")
	}
	return webhook, nil
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Webhook, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListWebhooks, query.OrganizationID, query.Keyword, query.IsEnabled, query.Method, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Webhook, 0)
	for rows.Next() {
		var row webhookRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
			&row.URL, &row.Method, &row.Headers, &row.Events,
			&row.IsEnabled, &row.Secret, &row.RetryCount, &row.TimeoutMs,
			&row.Organization, &row.ContentType, &row.CustomBody, &row.CustomHeaders,
			&row.CertID, &row.IsError, &row.ErrorText,
			&row.TokenFields, &row.ObjectFields, &row.IsUserExtended,
			&row.SingleOrgOnly, &row.MaxRetries, &row.RetryInterval, &row.UseExponentialBackoff,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		webhook, err := toWebhook(row)
		if err != nil {
			return nil, 0, apperrors.NewInternal("decode webhook")
		}
		items = append(items, webhook)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountWebhooks, query.OrganizationID, query.Keyword, query.IsEnabled, query.Method).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, webhook *domain.Webhook) error {
	events, err := shared.MarshalJSON(webhook.Events)
	if err != nil {
		return err
	}

	if err := encryptWebhookSecrets(webhook); err != nil {
		return err
	}

	// custom_headers 是 NOT NULL 的 jsonb：空字符串会被 ::jsonb 判为非法 JSON
	// （SQLSTATE 22P02），因此空值统一写成 {}。
	customHeaders := normalizeJSONObject(webhook.CustomHeaders)
	tokenFields := normalizeStringSlice(webhook.TokenFields)
	objectFields := normalizeStringSlice(webhook.ObjectFields)

	_, err = r.db.Exec(ctx, queryInsertWebhook,
		webhook.ID, webhook.OrganizationID, webhook.Name, webhook.DisplayName,
		webhook.URL, webhook.Method, webhook.Headers, events,
		webhook.IsEnabled, webhook.Secret, webhook.RetryCount, webhook.TimeoutMs,
		webhook.Organization, webhook.ContentType, webhook.CustomBody, customHeaders,
		webhook.CertID, webhook.IsError, webhook.ErrorText,
		tokenFields, objectFields, webhook.IsUserExtended,
		webhook.SingleOrgOnly, webhook.MaxRetries, webhook.RetryInterval, webhook.UseExponentialBackoff,
		webhook.CreatedAt, webhook.UpdatedAt, webhook.Revision,
	)
	return err
}

func (r Repository) Update(ctx context.Context, webhook *domain.Webhook) error {
	events, err := shared.MarshalJSON(webhook.Events)
	if err != nil {
		return err
	}

	if err := encryptWebhookSecrets(webhook); err != nil {
		return err
	}

	customHeaders := normalizeJSONObject(webhook.CustomHeaders)
	tokenFields := normalizeStringSlice(webhook.TokenFields)
	objectFields := normalizeStringSlice(webhook.ObjectFields)

	var newRevision int64
	err = r.db.QueryRow(ctx, queryUpdateWebhook,
		webhook.DisplayName, webhook.URL, webhook.Method, webhook.Headers, events,
		webhook.IsEnabled, webhook.Secret, webhook.RetryCount, webhook.TimeoutMs,
		webhook.Organization, webhook.ContentType, webhook.CustomBody, customHeaders,
		webhook.CertID, webhook.IsError, webhook.ErrorText,
		tokenFields, objectFields, webhook.IsUserExtended,
		webhook.SingleOrgOnly, webhook.MaxRetries, webhook.RetryInterval, webhook.UseExponentialBackoff,
		// App 层已 Revision++，这里要传自增前的值做乐观锁守卫。
		webhook.UpdatedAt, webhook.Revision-1, webhook.ID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("webhook revision conflict")
		}
		return err
	}
	webhook.Revision = newRevision
	return nil
}

// normalizeJSONObject 把空字符串归一到合法的空 JSON 对象。
func normalizeJSONObject(value string) string {
	if strings.TrimSpace(value) == "" {
		return "{}"
	}
	return value
}

// normalizeStringSlice 把 nil 归一到空切片：token_fields / object_fields 是
// NOT NULL 的数组列，nil 会被写成 NULL 并触发 23502。
func normalizeStringSlice(value []string) []string {
	if value == nil {
		return []string{}
	}
	return value
}

func (r Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, queryDeleteWebhook, id)
	return err
}

func (r Repository) ListByOrganizationAndEvent(ctx context.Context, organizationID, event string) ([]*domain.Webhook, error) {
	// Wrap event in a JSON array for the @> containment check
	eventJSON, err := json.Marshal([]string{event})
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, queryListWebhooksByOrgAndEvent, organizationID, eventJSON)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*domain.Webhook, 0)
	for rows.Next() {
		var row webhookRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
			&row.URL, &row.Method, &row.Headers, &row.Events,
			&row.IsEnabled, &row.Secret, &row.RetryCount, &row.TimeoutMs,
			&row.Organization, &row.ContentType, &row.CustomBody, &row.CustomHeaders,
			&row.CertID, &row.IsError, &row.ErrorText,
			&row.TokenFields, &row.ObjectFields, &row.IsUserExtended,
			&row.SingleOrgOnly, &row.MaxRetries, &row.RetryInterval, &row.UseExponentialBackoff,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, err
		}
		webhook, err := toWebhook(row)
		if err != nil {
			return nil, err
		}
		items = append(items, webhook)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return items, nil
}

var _ domain.Repository = Repository{}

func max(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
