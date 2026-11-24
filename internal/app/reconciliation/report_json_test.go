package reconciliation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	scimdomain "github.com/guliangiam/guliangiam/internal/domain/scim"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReport_JSONIsObjectNotBase64 pins BE-C-19: the report payloads must
// serialize as JSON objects. A bare []byte field would emit base64 strings
// and break every consumer the moment one is wired.
func TestReport_JSONIsObjectNotBase64(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeReportRepo{reports: []*scimdomain.ReconciliationReport{{
		ID: "r-1", OrganizationID: "org-1", ConnectorID: "c-1", Status: "completed",
		StartedAt: now, FinishedAt: now,
		UserReport:  []byte(`{"added":1,"updated":2}`),
		GroupReport: []byte(`{"removed":3}`),
		CreatedAt:   now,
	}}}
	response, err := NewService(repo).List(context.Background(), ListQuery{OrganizationID: "org-1"})
	require.NoError(t, err)
	require.Len(t, response.Items, 1)

	encoded, err := json.Marshal(response.Items[0])
	require.NoError(t, err)

	var decoded struct {
		UserReport  map[string]int `json:"userReport"`
		GroupReport map[string]int `json:"groupReport"`
	}
	require.NoError(t, json.Unmarshal(encoded, &decoded), "report payloads must decode as objects, not base64 strings")
	assert.Equal(t, map[string]int{"added": 1, "updated": 2}, decoded.UserReport)
	assert.Equal(t, map[string]int{"removed": 3}, decoded.GroupReport)
	assert.Contains(t, string(encoded), `"userReport":{"added":1,"updated":2}`)
}
