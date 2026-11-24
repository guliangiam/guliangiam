package outbox

import (
	"strings"
	"testing"
)

// TestMarkDeadLetterCopiesOrganizationID 回归：MySQL 镜像同样必须复制
// organization_id（OPEN-API-37 下不宣称可用，但 SQL 语义保持与 Postgres 对齐）。
func TestMarkDeadLetterCopiesOrganizationID(t *testing.T) {
	insertLine := "id, organization_id, aggregate_type, aggregate_id, event_type, event_id, payload, headers"
	selectLine := "SELECT id, organization_id, aggregate_type, aggregate_id, event_type, id, payload, headers"

	if !strings.Contains(sqlMarkDead_Letter, insertLine) {
		t.Fatalf("dead letter INSERT 列清单缺少 organization_id: %s", sqlMarkDead_Letter)
	}
	if !strings.Contains(sqlMarkDead_Letter, selectLine) {
		t.Fatalf("dead letter SELECT 未复制 organization_id: %s", sqlMarkDead_Letter)
	}
}
