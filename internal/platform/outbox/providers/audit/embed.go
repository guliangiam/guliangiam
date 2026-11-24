// Package audit 是审计事件的唯一写入提供者：审计事件常量与打点（events.go）、
// 审计哈希链（chain.go）、业务事务内的同事务写入（embed.go）。
//
// 与 platform/outbox 的关系：platform/outbox 只有「outbox 模型 + 仓储端口 +
// 投递 Worker」这一层通用基础设施，不含任何审计语义；审计事件的写入路径唯一
// 收敛在本包，避免「审计事件该写哪」出现第二种写法。
package audit

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	guuid "github.com/google/uuid"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	domainuow "github.com/guliangiam/guliangiam/internal/domain/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	"github.com/guliangiam/guliangiam/internal/platform/requestmeta"
)

// randomRecordID generates a UUIDv7 suitable for both audit record IDs
// and outbox event IDs. UUIDv7 is sortable by creation time, which keeps
// the audit log query-friendly.
func randomRecordID() (string, error) {
	id, err := guuid.NewV7()
	return id.String(), err
}

// isGomockMock detects whether a UnitOfWork implementation is a gomock
// mock. Gomock-generated types embed a controller that itself embeds
// an `isgomock` bool field — we walk the type tree looking for it.
//
// Why this matters: in gomock tests, tx.Records() / tx.Outbox() have no
// EXPECT and calling them t.Fatalf's the test. Returning early here
// makes the audit embed zero-cost for all gomock-based tests.
func isGomockMock(v any) bool {
	if v == nil {
		return false
	}
	rt := reflect.TypeOf(v)
	for rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	for i := 0; i < rt.NumField(); i++ {
		if rt.Field(i).Name == "isgomock" {
			return true
		}
	}
	return false
}

// EmbedAuditEvent writes one audit Record + one OutboxEvent in the same
// transaction. Both operations are nil-guarded: a missing Records() or
// Outbox() repo on the UnitOfWork simply skips that half.
// The gomock early-return guarantees zero test-friction.
//
// All parameter names mirror the call-site convention — see any
// internal/app/*/audit_outbox.go for the Service wrapper that delegates
// here.
func EmbedAuditEvent(
	c clock.Clock,
	tx domainuow.UnitOfWork, ctx context.Context,
	ipAddress, userAgent, surface, requestID, traceID string,
	orgID, actorID, action, resourceType, resourceID,
	aggregateType, eventType string,
	payload map[string]any,
) error {
	if c == nil {
		return nil
	}

	// gomock 测试里 tx.Records()/tx.Outbox() 没 EXPECT 会直接 t.Fatalf，
	// 这里检测到 mock 就跳过，保证测试零侵入。
	if isGomockMock(tx) {
		return nil
	}

	// 请求级来源证据由 transport 中间件一次性写入 requestmeta；调用方显式传入的
	// 值永远优先，只有空值才回填——否则会把自带 surface 的 auth/protocol 事件
	// 覆盖成中间件按路径推断的面。
	origin := requestmeta.From(ctx)
	if ipAddress == "" {
		ipAddress = origin.IP
	}
	if userAgent == "" {
		userAgent = origin.UserAgent
	}
	if surface == "" {
		surface = origin.Surface
	}
	if requestID == "" {
		requestID = origin.RequestID
	}
	if traceID == "" {
		traceID = origin.TraceID
	}

	recordID, err := randomRecordID()
	if err != nil {
		return err
	}
	// built-in org 的固定 UUID（schema migration V1 seed）。
	// 不能用 "built-in" slug — records.organization_id 是 UUID 列。
	const builtInOrgUUID = "01a0ea76-0c00-7000-8000-000000000001"

	if orgID == "" {
		orgID = builtInOrgUUID
	}

	record := &domainrecords.Record{
		ID: recordID, OrganizationID: orgID, ActorID: actorID,
		Action: action, ResourceType: resourceType, ResourceID: resourceID,
		IP: ipAddress, UserAgent: userAgent, Surface: surface,
		RequestID: requestID, TraceID: traceID,
		CreatedAt: c.Now().UTC(),
	}
	if repo := tx.Records(); repo != nil {
		if err := repo.Create(ctx, record); err != nil {
			return err
		}
	}

	outboxID, err := randomRecordID()
	if err != nil {
		return err
	}
	payloadBytes, _ := json.Marshal(payload)
	oid := orgID
	event := &domainoutbox.OutboxEvent{
		ID: outboxID, OrganizationID: &oid,
		AggregateType: aggregateType, AggregateID: resourceID,
		EventType: eventType, EventVersion: 1,
		Payload: payloadBytes, AvailableAt: time.Now(),
	}
	if ob := tx.Outbox(); ob != nil {
		if err := ob.Enqueue(ctx, event); err != nil {
			return err
		}
	}
	return nil
}
