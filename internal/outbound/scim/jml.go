package scim

import (
	"context"
	"encoding/json"
	"time"

	connectors "github.com/guliangiam/guliangiam/internal/domain/connectors"
	platformoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// ──────────────────────────────────────────────────────────────────────
// Outbox item 通用辅助
// ──────────────────────────────────────────────────────────────────────

// buildConnectorHeaders 构造 Handler 读取的 headers JSON。
func buildConnectorHeaders(connectorID, orgID string) []byte {
	h := map[string]string{
		"connector_id":    connectorID,
		"organization_id": orgID,
	}
	b, _ := json.Marshal(h)
	return b
}

// newSCIMOutboxEvent 构造一条 SCIM 出站 OutboxEvent。
// outbox.id 是 UUID 列：每次 Enqueue 都是独立新事件，用平台随机 UUID；
// 聚合关联信息仍由 AggregateType / AggregateID / EventType 独立字段承载。
func newSCIMOutboxEvent(idGen platformuuid.Generator, orgID, aggregateID, eventType string, payload, headers []byte) (*platformoutbox.OutboxEvent, error) {
	id, err := idGen.NewString()
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim jml: generate outbox id for %s", eventType)
	}
	return &platformoutbox.OutboxEvent{
		ID:             id,
		OrganizationID: &orgID,
		AggregateType:  connectors.AggregateType,
		AggregateID:    aggregateID,
		EventType:      eventType,
		EventVersion:   1,
		Payload:        payload,
		Headers:        headers,
		MaxAttempts:    12,
		AvailableAt:    time.Now(),
	}, nil
}

// enqueueForConnectors 对每个 connectorID 写一条 outbox item。
// connectorIDs 为空时直接 return nil（不投递）。
func enqueueForConnectors(ctx context.Context, repo platformoutbox.Repository, idGen platformuuid.Generator, orgID, aggregateID, eventType string, payload []byte, connectorIDs []string) error {
	if len(connectorIDs) == 0 {
		return nil
	}
	if idGen == nil {
		idGen = platformuuid.V7Generator{}
	}
	for _, cid := range connectorIDs {
		headers := buildConnectorHeaders(cid, orgID)
		evt, err := newSCIMOutboxEvent(idGen, orgID, aggregateID, eventType, payload, headers)
		if err != nil {
			return err
		}
		if err := repo.Enqueue(ctx, evt); err != nil {
			return apperrors.WrapInternalf(err, "scim jml: enqueue %s for connector %s", eventType, cid)
		}
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────
// JML 三个辅助函数
// ──────────────────────────────────────────────────────────────────────

// SCIM PATCH 请求体的 JSON 字段名。
const (
	jsonOperations = "Operations"
	jsonMembers    = "members"
	jsonPath       = "path"
	jsonValue      = "value"
)

// EmitJoinerProvisioning 触发 Joiner 出站：写 user.created + （如果有初始角色）role.member.added 到 Outbox。
// 调用方需先 Enqueue user.created，这里做的是**自动追加**角色/组成员变更。
// userPayload 是完整 SCIM User JSON（由业务层负责映射）。
// idGen 为 nil 时回退到平台 V7 生成器。
func EmitJoinerProvisioning(ctx context.Context, orgID, userID string, initialRoles []string, userPayload []byte, outboxRepo platformoutbox.Repository, idGen platformuuid.Generator, connectorIDs []string) error {
	// user.created
	if err := enqueueForConnectors(ctx, outboxRepo, idGen, orgID, userID, connectors.EvtUserCreated, userPayload, connectorIDs); err != nil {
		return err
	}

	// 初始角色 → role.member.added
	for _, roleID := range initialRoles {
		memberBody, _ := json.Marshal(map[string]any{
			jsonOperations: []map[string]any{
				{"op": "add", jsonPath: jsonMembers, jsonValue: map[string]any{jsonValue: userID}},
			},
		})
		if err := enqueueForConnectors(ctx, outboxRepo, idGen, orgID, roleID, connectors.EvtRoleMemberAdded, memberBody, connectorIDs); err != nil {
			return err
		}
	}
	return nil
}

// EmitMoverProvisioning 触发 Mover 出站：角色变更 → role.member.added / role.member.removed。
// 角色在 SCIM 中常作为 Group 下发，故走 group.member.* 事件类型。
// idGen 为 nil 时回退到平台 V7 生成器。
func EmitMoverProvisioning(ctx context.Context, orgID, userID string, addedRoles, removedRoles []string, outboxRepo platformoutbox.Repository, idGen platformuuid.Generator, connectorIDs []string) error {
	// 新增角色
	for _, roleID := range addedRoles {
		body, _ := json.Marshal(map[string]any{
			jsonOperations: []map[string]any{
				{"op": "add", jsonPath: jsonMembers, jsonValue: map[string]any{jsonValue: userID}},
			},
		})
		if err := enqueueForConnectors(ctx, outboxRepo, idGen, orgID, roleID, connectors.EvtRoleMemberAdded, body, connectorIDs); err != nil {
			return err
		}
	}
	// 移除角色
	for _, roleID := range removedRoles {
		body, _ := json.Marshal(map[string]any{
			jsonOperations: []map[string]any{
				{"op": "remove", jsonPath: jsonMembers, jsonValue: map[string]any{jsonValue: userID}},
			},
		})
		if err := enqueueForConnectors(ctx, outboxRepo, idGen, orgID, roleID, connectors.EvtRoleMemberRemoved, body, connectorIDs); err != nil {
			return err
		}
	}
	return nil
}

// EmitLeaverProvisioning 触发 Leaver 出站：user.deactivated + group.member.removed（清所有角色）。
// allRoleIDs 是用户当前持有的所有角色，Leaver 会对每个角色发 role.member.removed。
// idGen 为 nil 时回退到平台 V7 生成器。
func EmitLeaverProvisioning(ctx context.Context, orgID, userID string, allRoleIDs []string, outboxRepo platformoutbox.Repository, idGen platformuuid.Generator, connectorIDs []string) error {
	// user.deactivated —— Handler 会自动构造 active=false PATCH
	if err := enqueueForConnectors(ctx, outboxRepo, idGen, orgID, userID, connectors.EvtUserDeactivated, nil, connectorIDs); err != nil {
		return err
	}

	// 清理所有角色
	for _, roleID := range allRoleIDs {
		body, _ := json.Marshal(map[string]any{
			jsonOperations: []map[string]any{
				{"op": "remove", jsonPath: jsonMembers, jsonValue: map[string]any{jsonValue: userID}},
			},
		})
		if err := enqueueForConnectors(ctx, outboxRepo, idGen, orgID, roleID, connectors.EvtRoleMemberRemoved, body, connectorIDs); err != nil {
			return err
		}
	}
	return nil
}
