package mcpservers

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

type mockScanner struct {
	values []interface{}
	err    error
}

func (m *mockScanner) Scan(dest ...interface{}) error {
	if m.err != nil {
		return m.err
	}
	for i, d := range dest {
		switch v := m.values[i].(type) {
		case int64:
			if ptr, ok := d.(*int64); ok {
				*ptr = v
			}
		case string:
			if ptr, ok := d.(*string); ok {
				*ptr = v
			}
		case []byte:
			if ptr, ok := d.(*[]byte); ok {
				*ptr = v
			}
		case bool:
			if ptr, ok := d.(*bool); ok {
				*ptr = v
			}
		case time.Time:
			if ptr, ok := d.(*time.Time); ok {
				*ptr = v
			}
		case nil:
			// skip
		}
	}
	return nil
}

func TestScanMcpServer(t *testing.T) {
	now := time.Now()
	tools, _ := json.Marshal([]domain.Tool{
		{Name: "search", Description: "Search the web", IsAllowed: true},
	})

	t.Run("full_server", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"mcp-001", "org-001", "web-search", "Web Search",
				"https://mcp.example.com", "token-abc", "app-001",
				tools, true, now, now, int64(1),
			},
		}

		server, err := scanMcpServer(sc)
		require.NoError(t, err)

		assert.Equal(t, "mcp-001", server.ID)
		assert.Equal(t, "org-001", server.OrganizationID)
		assert.Equal(t, "web-search", server.Name)
		assert.Equal(t, "https://mcp.example.com", server.URL)
		assert.Equal(t, "app-001", server.Application)
		assert.Len(t, server.Tools, 1)
		assert.Equal(t, "search", server.Tools[0].Name)
		assert.True(t, server.IsEnabled)
	})

	t.Run("null_tools", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"mcp-002", "org-001", "empty-server", "Empty Server",
				"https://mcp2.example.com", "", "",
				[]byte("null"), false, now, now, int64(1),
			},
		}

		server, err := scanMcpServer(sc)
		require.NoError(t, err)
		assert.Equal(t, []domain.Tool{}, server.Tools)
	})

	t.Run("scan_error", func(t *testing.T) {
		sc := &mockScanner{err: errors.New("db error")}
		_, err := scanMcpServer(sc)
		assert.Error(t, err)
	})
}

func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

func sealedMcpToken(t *testing.T, serverID, plaintext string) string {
	t.Helper()
	sealed, err := crypto.GlobalFieldEncryptor().EncryptColumn(mcpServerAAD(serverID), plaintext)
	require.NoError(t, err)
	require.True(t, crypto.IsEnvelopeToken(sealed))
	return sealed
}

func mcpServerValues(serverID, organizationID, token string) []interface{} {
	now := time.Now()
	tools, _ := json.Marshal([]domain.Tool{{Name: "search", IsAllowed: true}})
	return []interface{}{
		serverID, organizationID, "web-search", "Web Search",
		"https://mcp.example.com", token, "app-001",
		tools, true, now, now, int64(1),
	}
}

func TestScanMcpServer_SealedTokenRoundTrip(t *testing.T) {
	sc := &mockScanner{values: mcpServerValues("mcp-001", "org-001", sealedMcpToken(t, "mcp-001", "token-abc"))}

	server, err := scanMcpServer(sc)
	require.NoError(t, err)
	assert.Equal(t, "token-abc", server.Token, "repository reads must keep the plaintext available inside the process")
	assert.Equal(t, "mcp-001", server.ID)
}

func TestScanMcpServer_LegacyPlaintextFallsBackWithMinimalWarn(t *testing.T) {
	logs := captureWarnings(t)
	sc := &mockScanner{values: mcpServerValues("mcp-002", "org-002", "legacy-token-value")}

	server, err := scanMcpServer(sc)
	require.NoError(t, err)
	assert.Equal(t, "legacy-token-value", server.Token)
	assert.Contains(t, logs.String(), "mcp-002")
	assert.Contains(t, logs.String(), "org-002")
	assert.NotContains(t, logs.String(), "legacy-token-value", "the WARN must never include the token")
}

func TestScanMcpServer_AADBindingRejectsAnotherRow(t *testing.T) {
	sc := &mockScanner{values: mcpServerValues("mcp-003", "org-003", sealedMcpToken(t, "other-id", "token-abc"))}

	_, err := scanMcpServer(sc)
	require.Error(t, err)
	assert.ErrorIs(t, err, crypto.ErrAADMismatch)
}

func TestScanMcpServer_EmptyTokenStaysEmpty(t *testing.T) {
	sc := &mockScanner{values: mcpServerValues("mcp-004", "org-004", "")}

	server, err := scanMcpServer(sc)
	require.NoError(t, err)
	assert.Empty(t, server.Token)
}

func TestSealMcpServerToken(t *testing.T) {
	t.Run("empty_stays_empty", func(t *testing.T) {
		sealed, err := sealMcpServerToken(&domain.McpServer{ID: "mcp-1"})
		require.NoError(t, err)
		assert.Empty(t, sealed)
	})

	t.Run("seals_with_row_aad", func(t *testing.T) {
		sealed, err := sealMcpServerToken(&domain.McpServer{ID: "mcp-1", Token: "token-abc"})
		require.NoError(t, err)
		require.True(t, crypto.IsEnvelopeToken(sealed))
		assert.NotContains(t, sealed, "token-abc")
		header, _, _ := strings.Cut(sealed, ":")
		assert.True(t, strings.HasSuffix(header, "-aad"), "v2 AAD envelope expected")

		opened, err := crypto.GlobalFieldEncryptor().DecryptColumn(mcpServerAAD("mcp-1"), sealed)
		require.NoError(t, err)
		assert.Equal(t, "token-abc", opened)
	})

	t.Run("oversized_envelope_is_refused", func(t *testing.T) {
		token := strings.Repeat("t", 120)
		_, err := sealMcpServerToken(&domain.McpServer{ID: "mcp-1", Token: token})
		require.Error(t, err)
		assert.NotContains(t, err.Error(), token)
	})
}
