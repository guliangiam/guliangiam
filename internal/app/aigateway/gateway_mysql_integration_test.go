package aigateway

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	mysqlag "github.com/guliangiam/guliangiam/internal/infra/mysql/aigateway"
	"github.com/stretchr/testify/require"
)

// TestGatewayService_Chat_MySQLUsage proves a completion request executes with
// the AI Gateway usage repository backed by real MySQL (OPEN-API-73).
func TestGatewayService_Chat_MySQLUsage(t *testing.T) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	const orgID = "01a0ea76-0c00-7000-8000-000000000001"
	_, err = db.ExecContext(ctx, `INSERT IGNORE INTO organizations (id, slug, display_name, revision) VALUES (?, 'built-in', 'Built-in', 1)`, orgID)
	require.NoError(t, err)

	upstream := &mockUpstream{
		name: "openai_compat", proto: "openai",
		chatResp: &domain.CanonicalResponse{
			Content:     []domain.CanonicalContentBlock{{Type: "text", Text: "hello"}},
			InputTokens: 10, OutputTokens: 5, TotalTokens: 15,
		},
	}
	registry := newMockRegistry()
	registry.Register("openai", upstream)
	resolution := &domain.RouteResolution{
		Provider:     &domain.LlmProvider{ID: "p-1", OrganizationID: orgID, Name: "openai"},
		Model:        &domain.LlmModel{ID: "m-1", ProviderModelName: "gpt-4o"},
		Route:        &domain.LlmRoute{ID: "r-1", AliasID: "a-1", ModelID: "m-1", Priority: 1},
		ProtocolFace: "openai",
	}
	_, err = db.ExecContext(ctx, `INSERT IGNORE INTO llm_providers (id, organization_id, name, provider_type, base_url, api_key_sealed, api_key_fingerprint, supported_protocols) VALUES ('p-1', ?, 'openai-mysql', 'openai', 'https://stub.invalid/v1', '', '', '["openai"]')`, orgID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT IGNORE INTO llm_models (id, organization_id, provider_id, provider_model_name, capabilities) VALUES ('m-1', ?, 'p-1', 'gpt-4o', '[]')`, orgID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT IGNORE INTO llm_model_aliases (id, organization_id, alias) VALUES ('a-1', ?, 'gpt-4')`, orgID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM llm_model_aliases WHERE id = 'a-1'`)
		_, _ = db.Exec(`DELETE FROM llm_models WHERE id = 'm-1'`)
		_, _ = db.Exec(`DELETE FROM llm_providers WHERE id = 'p-1'`)
	})
	usageLogs := mysqlag.NewUsageLogsRepository(db)
	svc := NewGatewayService(
		mockClock{t: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)},
		mockUUID{s: "uuid-fixed"}, &mockRouter{res: resolution}, registry,
		&stubQuotaRepo{},
		&mockPriceRepo{pv: &domain.LlmPriceVersion{ID: "pv-1", OrganizationID: orgID, ModelID: "m-1", InputPricePer1M: 5, OutputPricePer1M: 15, Currency: "USD"}},
		&usageLogs,
		&stubRepo{}, nil, &stubProviderRepo{}, &stubHealthRepo{}, nil, nil,
	)
	resp, err := svc.Chat(ctx, "hash-xyz", "gpt-4", makeValidReq())
	require.NoError(t, err)
	require.Equal(t, "hello", resp.Content[0].Text)

	logs, total, err := usageLogs.List(ctx, orgID, 10, 0)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(1))
	require.NotEmpty(t, logs)
	latest := logs[0]
	require.Equal(t, orgID, latest.OrganizationID)
	require.Equal(t, domain.UsageStatusSucceeded, latest.Status)
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM llm_usage_logs WHERE id = ?`, latest.ID) })

	failureUpstream := &mockUpstream{
		name: "openai_compat", proto: "openai", chatErr: errors.New("upstream unavailable"),
	}
	failureRegistry := newMockRegistry()
	failureRegistry.Register("openai", failureUpstream)
	failureService := NewGatewayService(
		mockClock{t: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)},
		mockUUID{s: "uuid-fixed-2"}, &mockRouter{res: resolution}, failureRegistry,
		&stubQuotaRepo{},
		&mockPriceRepo{pv: &domain.LlmPriceVersion{ID: "pv-1", OrganizationID: orgID, ModelID: "m-1", InputPricePer1M: 5, OutputPricePer1M: 15, Currency: "USD"}},
		&usageLogs,
		&stubRepo{}, nil, &stubProviderRepo{}, &stubHealthRepo{}, nil, nil,
	)
	_, err = failureService.Chat(ctx, "hash-xyz", "gpt-4", makeValidReq())
	require.Error(t, err)
	failureLogs, _, err := usageLogs.List(ctx, orgID, 10, 0)
	require.NoError(t, err)
	require.NotEmpty(t, failureLogs)
	foundFailure := false
	for _, item := range failureLogs {
		if item.Status == domain.UsageStatusFailed {
			foundFailure = true
			t.Cleanup(func(id string) func() {
				return func() { _, _ = db.Exec(`DELETE FROM llm_usage_logs WHERE id = ?`, id) }
			}(item.ID))
			break
		}
	}
	require.True(t, foundFailure, "failure path must persist a failed usage log")
}
