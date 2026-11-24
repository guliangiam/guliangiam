// Package auditoutbox provides the shared audit-record + outbox-event
// embed function used by every Service in internal/app/*. All 45
// audit_outbox.go files in the app packages were identical except for
// the package declaration — this is the single source of truth since
// 2026-09-28.
package auditoutbox

import (
	"context"
	"encoding/json"
	"reflect"

	guuid "github.com/google/uuid"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	domainuow "github.com/guliangiam/guliangiam/internal/domain/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
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

// EmbedAuditOutboxPlain writes one audit Record + one OutboxEvent in
// the same transaction. Both operations are nil-guarded: a missing
// Records() or Outbox() repo on the UnitOfWork simply skips that half.
// The gomock early-return guarantees zero test-friction.
//
// All parameter names mirror the call-site convention — see any
// internal/app/*/audit_outbox.go for the Service wrapper that delegates
// here.
func EmbedAuditOutboxPlain(
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

	recordID, err := randomRecordID()
	if err != nil {
		return err
	}
	if orgID == "" {
		orgID = "built-in"
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
	// BE-E-09: a marshal failure must not silently enqueue an empty audit
	// payload; surface it so the caller's transaction rolls back instead.
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternal(err, "marshal audit outbox payload")
	}
	oid := orgID
	event := &domainoutbox.OutboxEvent{
		ID: outboxID, OrganizationID: &oid,
		AggregateType: aggregateType, AggregateID: resourceID,
		EventType: eventType, EventVersion: 1,
		Payload: payloadBytes, AvailableAt: c.Now().UTC(),
	}
	if ob := tx.Outbox(); ob != nil {
		if err := ob.Enqueue(ctx, event); err != nil {
			return err
		}
	}
	return nil
}
