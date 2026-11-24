package scim

import (
	"context"
	"testing"

	connectors "github.com/guliangiam/guliangiam/internal/domain/connectors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/stretchr/testify/require"
)

// TestJML_OutboxEventIDsAreUUIDs 验证 SCIM 出站事件使用 UUID 主键
// （outbox.id 列类型 UUID），且每次尝试/每个 connector 都是独立新事件。
func TestJML_OutboxEventIDsAreUUIDs(t *testing.T) {
	repo := newTestOutboxRepo()

	require.NoError(t, EmitJoinerProvisioning(
		context.Background(),
		"org-1",
		"user-alice",
		[]string{"role-1"},
		[]byte(`{"userName":"alice"}`),
		repo,
		platformuuid.V7Generator{},
		[]string{"c-a", "c-b"},
	))

	events := repo.Events()
	// 2 connectors × (1 user.created + 1 role.member.added) = 4
	require.Len(t, events, 4)

	seen := make(map[string]bool, len(events))
	perAggregate := map[string]int{}
	for _, e := range events {
		require.True(t, platformuuid.IsValid(e.ID),
			"outbox id %q is not a UUID (outbox.id is UUID; Postgres rejects it)", e.ID)
		require.False(t, seen[e.ID], "duplicate outbox id %q", e.ID)
		seen[e.ID] = true
		perAggregate[e.AggregateID]++

		// ID 改为随机 UUID 后，聚合关联信息必须仍可从独立字段获得。
		require.Equal(t, connectors.AggregateType, e.AggregateType)
		require.NotEmpty(t, e.EventType)
	}
	require.Equal(t, 2, perAggregate["user-alice"], "user.created events keep aggregate_id")
	require.Equal(t, 2, perAggregate["role-1"], "role.member.added events keep aggregate_id")
}
