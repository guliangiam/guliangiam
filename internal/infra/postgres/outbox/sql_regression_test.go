package outbox

import (
	"strings"
	"testing"
)

// TestMarkDeadLetterCopiesOrganizationID 回归：dead letter 必须继承原 outbox
// 行的 organization_id，否则非超管角色在 RLS 下看不到死信，运维只能跨租户排查。
func TestMarkDeadLetterCopiesOrganizationID(t *testing.T) {
	insertLine := "id, organization_id, aggregate_type, aggregate_id, event_type, event_id, payload, headers"
	selectLine := "SELECT id, organization_id, aggregate_type, aggregate_id, event_type, id, payload, headers"

	if !strings.Contains(sqlMarkDead_Letter, insertLine) {
		t.Fatalf("dead letter INSERT 列清单缺少 organization_id: %s", sqlMarkDead_Letter)
	}
	if !strings.Contains(sqlMarkDead_Letter, selectLine) {
		t.Fatalf("dead letter SELECT 未复制 organization_id: %s", sqlMarkDead_Letter)
	}

	insertCount := len(strings.Split(insertLine, ","))
	selectCount := len(strings.Split(strings.TrimPrefix(selectLine, "SELECT "), ","))
	if insertCount != selectCount {
		t.Fatalf("dead letter 列数不匹配: insert=%d select=%d", insertCount, selectCount)
	}
}

// TestMarkDeadLetterTargetsTenantVisibleTable 回归：写入目标必须是带
// organization_id + RLS 的 outbox_dead_letters（V14 声明）。
func TestMarkDeadLetterTargetsTenantVisibleTable(t *testing.T) {
	if !strings.Contains(sqlMarkDead_Letter, "INSERT INTO guliangiam.outbox_dead_letters") {
		t.Fatalf("dead letter 写入目标错误: %s", sqlMarkDead_Letter)
	}
	if !strings.Contains(sqlMarkDead_Letter, "ON CONFLICT (id) DO NOTHING") {
		t.Fatal("dead letter 写入缺少幂等冲突处理")
	}
}
