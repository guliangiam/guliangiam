package mcpservers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
)

func TestGetMcpServer_MasksTokenAndNeverReturnsPlaintext(t *testing.T) {
	now := time.Now()
	repo := &mockMcpServerRepo{mcpServers: []*domain.McpServer{{
		ID: "mcp-1", OrganizationID: "org-1", Name: "server", Token: "tok-secret-123",
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}}}
	svc := newSvc(repo)

	detail, err := svc.GetMcpServer(context.Background(), "org-1", "mcp-1")
	require.NoError(t, err)
	assert.True(t, detail.TokenSet)
	assert.Equal(t, "to***23", detail.TokenMasked)
	assert.NotEqual(t, "tok-secret-123", detail.TokenMasked)

	encoded, err := json.Marshal(detail)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "tok-secret-123")
	assert.NotContains(t, string(encoded), `"token":`)
	assert.Contains(t, string(encoded), `"tokenSet":true`)
}

func TestGetMcpServer_UnconfiguredToken(t *testing.T) {
	now := time.Now()
	repo := &mockMcpServerRepo{mcpServers: []*domain.McpServer{{
		ID: "mcp-2", OrganizationID: "org-1", Name: "server", CreatedAt: now, UpdatedAt: now, Revision: 1,
	}}}
	svc := newSvc(repo)

	detail, err := svc.GetMcpServer(context.Background(), "org-1", "mcp-2")
	require.NoError(t, err)
	assert.False(t, detail.TokenSet)
	assert.Empty(t, detail.TokenMasked)

	encoded, err := json.Marshal(detail)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), `"token"`)
	assert.NotContains(t, string(encoded), "tokenMasked")
}

func TestListMcpServers_OmitsToken(t *testing.T) {
	now := time.Now()
	repo := &mockMcpServerRepo{mcpServers: []*domain.McpServer{{
		ID: "mcp-1", OrganizationID: "org-1", Name: "server", Token: "tok-secret-123",
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}}}
	svc := newSvc(repo)

	items, total, err := svc.ListMcpServers(context.Background(), ListMcpServersQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)

	encoded, err := json.Marshal(items)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "tok-secret-123")
	assert.NotContains(t, string(encoded), `"token"`)
}

func TestMaskToken(t *testing.T) {
	assert.Empty(t, maskToken(""))
	assert.Equal(t, "******", maskToken("short"))
	assert.Equal(t, "to***23", maskToken("tok-secret-123"))
}
