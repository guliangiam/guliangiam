package scim

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	connectors "github.com/guliangiam/guliangiam/internal/domain/connectors"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	outboxplatform "github.com/guliangiam/guliangiam/internal/platform/outbox"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──── 测试用 outbox repository stub ────

// testOutboxRepo 是 domain/outbox.Repository 的内存实现，记录所有 Enqueue 的事件供断言。
type testOutboxRepo struct {
	mu     sync.Mutex
	events []*domainoutbox.OutboxEvent
}

func newTestOutboxRepo() *testOutboxRepo { return &testOutboxRepo{} }

func (r *testOutboxRepo) Enqueue(ctx context.Context, item *domainoutbox.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, item)
	return nil
}
func (r *testOutboxRepo) EnqueueBatch(ctx context.Context, items []*domainoutbox.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, items...)
	return nil
}
func (r *testOutboxRepo) Events() []*domainoutbox.OutboxEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*domainoutbox.OutboxEvent, len(r.events))
	copy(out, r.events)
	return out
}

// ──── 测试辅助：构造 OutboxItem + Headers ────

func buildHeaders(connectorID string) []byte {
	h := map[string]string{
		"connector_id":    connectorID,
		"organization_id": "org-1",
	}
	b, _ := json.Marshal(h)
	return b
}

func buildItem(eventType, aggregateID string, headers, payload []byte) *outboxplatform.OutboxItem {
	return &outboxplatform.OutboxItem{
		ID:            "evt-test-" + eventType + "-" + aggregateID,
		AggregateType: connectors.AggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payload,
		Headers:       headers,
		Status:        outboxplatform.StatusPending,
		MaxAttempts:   12,
	}
}

// ──── EventType → (method, path) 映射表测试 ────

func TestHandler_EventTypeRouting(t *testing.T) {
	type expected struct {
		method    string
		pathMatch string // strings.Contains
	}

	payloadJSON := func(id string) []byte {
		if id == "" {
			return []byte(`{"userName":"alice","displayName":"Alice"}`)
		}
		return []byte(fmt.Sprintf(`{"id":%q,"userName":"alice","displayName":"Alice"}`, id))
	}

	testCases := []struct {
		name        string
		eventType   string
		aggregateID string
		payload     []byte
		want        expected
	}{
		{
			name:        "user.created → POST /Users",
			eventType:   connectors.EvtUserCreated,
			aggregateID: "u-1",
			payload:     payloadJSON(""),
			want:        expected{method: http.MethodPost, pathMatch: "/Users"},
		},
		{
			name:        "user.updated → PUT /Users/{id} from payload.id",
			eventType:   connectors.EvtUserUpdated,
			aggregateID: "u-1",
			payload:     payloadJSON("u-1-payload"),
			want:        expected{method: http.MethodPut, pathMatch: "/Users/u-1-payload"},
		},
		{
			name:        "user.deactivated → PATCH /Users/{id}",
			eventType:   connectors.EvtUserDeactivated,
			aggregateID: "u-2",
			payload:     nil,
			want:        expected{method: http.MethodPatch, pathMatch: "/Users/u-2"},
		},
		{
			name:        "group.created → POST /Groups",
			eventType:   connectors.EvtGroupCreated,
			aggregateID: "g-1",
			payload:     []byte(`{"displayName":"Engineering"}`),
			want:        expected{method: http.MethodPost, pathMatch: "/Groups"},
		},
		{
			name:        "group.updated → PUT /Groups/{id}",
			eventType:   connectors.EvtGroupUpdated,
			aggregateID: "g-2",
			payload:     []byte(`{"id":"g-2","displayName":"Engineering Updated"}`),
			want:        expected{method: http.MethodPut, pathMatch: "/Groups/g-2"},
		},
		{
			name:        "group.deleted → DELETE /Groups/{id}",
			eventType:   connectors.EvtGroupDeleted,
			aggregateID: "g-3",
			payload:     nil,
			want:        expected{method: http.MethodDelete, pathMatch: "/Groups/g-3"},
		},
		{
			name:        "group.member.added → PATCH /Groups/{gid}",
			eventType:   connectors.EvtGroupMemberAdded,
			aggregateID: "g-4",
			payload:     []byte(`{"Operations":[{"op":"add","path":"members","value":{"value":"u-1"}}]}`),
			want:        expected{method: http.MethodPatch, pathMatch: "/Groups/g-4"},
		},
		{
			name:        "group.member.removed → PATCH /Groups/{gid}",
			eventType:   connectors.EvtGroupMemberRemoved,
			aggregateID: "g-5",
			payload:     []byte(`{"Operations":[{"op":"remove","path":"members","value":{"value":"u-1"}}]}`),
			want:        expected{method: http.MethodPatch, pathMatch: "/Groups/g-5"},
		},
		{
			name:        "role.member.added → PATCH /Groups/{gid}",
			eventType:   connectors.EvtRoleMemberAdded,
			aggregateID: "role-1",
			payload:     []byte(`{"Operations":[{"op":"add","path":"members","value":{"value":"u-1"}}]}`),
			want:        expected{method: http.MethodPatch, pathMatch: "/Groups/role-1"},
		},
		{
			name:        "role.member.removed → PATCH /Groups/{gid}",
			eventType:   connectors.EvtRoleMemberRemoved,
			aggregateID: "role-2",
			payload:     []byte(`{"Operations":[{"op":"remove","path":"members","value":{"value":"u-1"}}]}`),
			want:        expected{method: http.MethodPatch, pathMatch: "/Groups/role-2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 启动 httptest server，捕获 method + path
			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer srv.Close()

			// 注册 connector
			stubRepo := NewStubConnectorRepo()
			stubRepo.Upsert(&connectors.Connector{
				ID:             "c-1",
				OrganizationID: "org-1",
				Name:           "Test SCIM",
				SCIMBaseURL:    srv.URL,
				BearerToken:    "token-1",
				IsEnabled:      true,
				TimeoutMs:      2000,
				MaxRatePerMin:  300,
			})

			h := NewHandler(stubRepo)
			item := buildItem(tc.eventType, tc.aggregateID, buildHeaders("c-1"), tc.payload)
			err := h.Handle(context.Background(), item)

			require.NoError(t, err)
			assert.Equal(t, tc.want.method, gotMethod, "HTTP method mismatch")
			assert.Contains(t, gotPath, tc.want.pathMatch, "path mismatch: got=%s want contains %s", gotPath, tc.want.pathMatch)
		})
	}
}

// ──── 连接 / 错误场景 ────

func TestHandler_ConnectorNotFound(t *testing.T) {
	stubRepo := NewStubConnectorRepo() // 空的
	h := NewHandler(stubRepo)

	item := buildItem(connectors.EvtUserCreated, "u-1", buildHeaders("c-not-exist"), nil)
	err := h.Handle(context.Background(), item)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connector not found")
	// 断言错误类型是 NotFound（Worker 会 MarkDead）
	var appErr *apperrors.Error
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestHandler_ConnectorDisabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("disabled connector should NOT make HTTP request")
	}))
	defer srv.Close()

	stubRepo := NewStubConnectorRepo()
	stubRepo.Upsert(&connectors.Connector{
		ID:             "c-disabled",
		OrganizationID: "org-1",
		Name:           "Disabled",
		SCIMBaseURL:    srv.URL,
		BearerToken:    "token",
		IsEnabled:      false, // 关键：disabled
		TimeoutMs:      2000,
		MaxRatePerMin:  300,
	})

	h := NewHandler(stubRepo)
	item := buildItem(connectors.EvtUserCreated, "u-1", buildHeaders("c-disabled"), nil)
	err := h.Handle(context.Background(), item)

	assert.NoError(t, err, "disabled connector should silently succeed")
}

func TestHandler_HTTP200_MarkSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	stubRepo := NewStubConnectorRepo()
	stubRepo.Upsert(&connectors.Connector{
		ID: "c-1", OrganizationID: "org-1", IsEnabled: true,
		SCIMBaseURL: srv.URL, BearerToken: "token", TimeoutMs: 2000, MaxRatePerMin: 0,
	})

	h := NewHandler(stubRepo)
	item := buildItem(connectors.EvtUserCreated, "u-1", buildHeaders("c-1"), []byte(`{"userName":"a"}`))
	err := h.Handle(context.Background(), item)
	assert.NoError(t, err)
}

func TestHandler_HTTP404_MarkDead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Not Found"}`))
	}))
	defer srv.Close()

	stubRepo := NewStubConnectorRepo()
	stubRepo.Upsert(&connectors.Connector{
		ID: "c-1", OrganizationID: "org-1", IsEnabled: true,
		SCIMBaseURL: srv.URL, BearerToken: "token", TimeoutMs: 2000, MaxRatePerMin: 0,
	})

	h := NewHandler(stubRepo)
	item := buildItem(connectors.EvtUserCreated, "u-1", buildHeaders("c-1"), []byte(`{"userName":"a"}`))
	err := h.Handle(context.Background(), item)

	require.Error(t, err)
	var appErr *apperrors.Error
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code, "4xx → InvalidArgument → MarkDead")
}

func TestHandler_HTTP500_MarkRetry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	stubRepo := NewStubConnectorRepo()
	stubRepo.Upsert(&connectors.Connector{
		ID: "c-1", OrganizationID: "org-1", IsEnabled: true,
		SCIMBaseURL: srv.URL, BearerToken: "token", TimeoutMs: 2000, MaxRatePerMin: 0,
	})

	h := NewHandler(stubRepo)
	item := buildItem(connectors.EvtUserCreated, "u-1", buildHeaders("c-1"), []byte(`{"userName":"a"}`))
	err := h.Handle(context.Background(), item)

	require.Error(t, err)
	var appErr *apperrors.Error
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInternal, appErr.Code, "5xx → Internal → MarkRetry")
}

func TestHandler_HTTP429_MarkRetry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	stubRepo := NewStubConnectorRepo()
	stubRepo.Upsert(&connectors.Connector{
		ID: "c-1", OrganizationID: "org-1", IsEnabled: true,
		SCIMBaseURL: srv.URL, BearerToken: "token", TimeoutMs: 2000, MaxRatePerMin: 0,
	})

	h := NewHandler(stubRepo)
	item := buildItem(connectors.EvtUserCreated, "u-1", buildHeaders("c-1"), []byte(`{"userName":"a"}`))
	err := h.Handle(context.Background(), item)

	require.Error(t, err)
	var appErr *apperrors.Error
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInternal, appErr.Code, "429 → Internal → MarkRetry")
}

func TestHandler_ConnectionTimeout_MarkRetry(t *testing.T) {
	// 指向一个不会响应的地址（用 httptest + 人为 slow 模拟超时）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // 比 TimeoutMs=1000 长
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	stubRepo := NewStubConnectorRepo()
	stubRepo.Upsert(&connectors.Connector{
		ID: "c-1", OrganizationID: "org-1", IsEnabled: true,
		SCIMBaseURL: srv.URL, BearerToken: "token", TimeoutMs: 1000, MaxRatePerMin: 0,
	})

	h := NewHandler(stubRepo)
	item := buildItem(connectors.EvtUserCreated, "u-1", buildHeaders("c-1"), []byte(`{"userName":"a"}`))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := h.Handle(ctx, item)

	require.Error(t, err)
	var appErr *apperrors.Error
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInternal, appErr.Code, "transport error → Internal → MarkRetry")
}

func TestHandler_MissingConnectorID_InHeaders(t *testing.T) {
	stubRepo := NewStubConnectorRepo()
	h := NewHandler(stubRepo)

	item := buildItem(connectors.EvtUserCreated, "u-1", nil, []byte(`{}`))
	err := h.Handle(context.Background(), item)

	require.Error(t, err)
	var appErr *apperrors.Error
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
	assert.Contains(t, err.Error(), "missing connector_id")
}

func TestHandler_UnknownEventType_MarkDead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unknown event type should not make HTTP")
	}))
	defer srv.Close()

	stubRepo := NewStubConnectorRepo()
	stubRepo.Upsert(&connectors.Connector{
		ID: "c-1", OrganizationID: "org-1", IsEnabled: true,
		SCIMBaseURL: srv.URL, BearerToken: "token", TimeoutMs: 2000, MaxRatePerMin: 0,
	})

	h := NewHandler(stubRepo)
	item := buildItem("unknown.event", "u-1", buildHeaders("c-1"), nil)
	err := h.Handle(context.Background(), item)

	require.Error(t, err)
	var appErr *apperrors.Error
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code, "unknown event → MarkDead")
}

func TestHandler_UserDeactivated_AutoPatchBody(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = readBody(t, r)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	stubRepo := NewStubConnectorRepo()
	stubRepo.Upsert(&connectors.Connector{
		ID: "c-1", OrganizationID: "org-1", IsEnabled: true,
		SCIMBaseURL: srv.URL, BearerToken: "token", TimeoutMs: 2000, MaxRatePerMin: 0,
	})

	h := NewHandler(stubRepo)
	item := buildItem(connectors.EvtUserDeactivated, "u-deactived-99", buildHeaders("c-1"), nil)
	err := h.Handle(context.Background(), item)

	require.NoError(t, err)
	assert.Contains(t, gotBody, `"path":"active"`, "deactivated PATCH body should target active field")
	assert.Contains(t, gotBody, `"value":false`, "deactivated PATCH body should set active=false")
	assert.Contains(t, gotBody, `"op":"replace"`, "should use replace op")
}

// readBody 读请求 body 并返回字符串（测试辅助）。
func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	buf := make([]byte, 1<<20)
	n, _ := r.Body.Read(buf)
	return strings.TrimSpace(string(buf[:n]))
}

// ──── JML 辅助函数测试 ────

func TestJML_EmitJoinerProvisioning(t *testing.T) {
	repo := newTestOutboxRepo()
	userPayload := []byte(`{"userName":"alice","displayName":"Alice"}`)

	err := EmitJoinerProvisioning(
		context.Background(),
		"org-1",
		"user-alice",
		[]string{"role-engineer", "role-qa"},
		userPayload,
		repo,
		platformuuid.V7Generator{},
		[]string{"c-scim-okta"},
	)
	require.NoError(t, err)

	events := repo.Events()
	// 期望：1 user.created + 2 role.member.added = 3
	assert.Len(t, events, 3, "1 user.created + 2 roles = 3 events")

	// 检查 aggregate_type 都是 scim_outbound
	for _, e := range events {
		assert.Equal(t, connectors.AggregateType, e.AggregateType)
	}

	// 分类
	var userCreatedEvt *domainoutbox.OutboxEvent
	roleCount := 0
	for _, e := range events {
		if e.EventType == connectors.EvtUserCreated {
			userCreatedEvt = e
		}
		if e.EventType == connectors.EvtRoleMemberAdded {
			roleCount++
		}
	}

	require.NotNil(t, userCreatedEvt, "user.created must exist")
	assert.Equal(t, "user-alice", userCreatedEvt.AggregateID)
	assert.Equal(t, userPayload, userCreatedEvt.Payload)

	assert.Equal(t, 2, roleCount, "2 roles → 2 role.member.added events")
}

func TestJML_EmitJoiner_NoInitialRoles(t *testing.T) {
	repo := newTestOutboxRepo()
	err := EmitJoinerProvisioning(
		context.Background(),
		"org-1",
		"user-bob",
		nil, // 没有初始角色
		[]byte(`{"userName":"bob"}`),
		repo,
		platformuuid.V7Generator{},
		[]string{"c-1"},
	)
	require.NoError(t, err)

	events := repo.Events()
	assert.Len(t, events, 1, "no roles → only user.created")
	assert.Equal(t, connectors.EvtUserCreated, events[0].EventType)
}

func TestJML_EmitLeaverProvisioning(t *testing.T) {
	repo := newTestOutboxRepo()

	err := EmitLeaverProvisioning(
		context.Background(),
		"org-1",
		"user-alice",
		[]string{"role-engineer", "role-qa", "role-admin"},
		repo,
		platformuuid.V7Generator{},
		[]string{"c-scim-okta"},
	)
	require.NoError(t, err)

	events := repo.Events()
	// 1 deactivated + 3 role.removed = 4
	assert.Len(t, events, 4, "1 deactivated + 3 role.removed = 4 events")

	var deactivatedCount, removedCount int
	for _, e := range events {
		switch e.EventType {
		case connectors.EvtUserDeactivated:
			deactivatedCount++
		case connectors.EvtRoleMemberRemoved:
			removedCount++
		}
	}
	assert.Equal(t, 1, deactivatedCount, "exactly 1 deactivated")
	assert.Equal(t, 3, removedCount, "exactly 3 role.member.removed")
}

func TestJML_EmitLeaver_NoRoles(t *testing.T) {
	repo := newTestOutboxRepo()
	err := EmitLeaverProvisioning(
		context.Background(),
		"org-1",
		"user-bob",
		nil, // 无角色
		repo,
		platformuuid.V7Generator{},
		[]string{"c-1"},
	)
	require.NoError(t, err)

	events := repo.Events()
	assert.Len(t, events, 1, "no roles → only deactivated")
	assert.Equal(t, connectors.EvtUserDeactivated, events[0].EventType)
}

func TestJML_EmitMoverProvisioning(t *testing.T) {
	repo := newTestOutboxRepo()

	err := EmitMoverProvisioning(
		context.Background(),
		"org-1",
		"user-alice",
		[]string{"role-manager"},  // 新增
		[]string{"role-engineer"}, // 移除
		repo,
		platformuuid.V7Generator{},
		[]string{"c-1"},
	)
	require.NoError(t, err)

	events := repo.Events()
	assert.Len(t, events, 2)

	var added, removed int
	for _, e := range events {
		if e.EventType == connectors.EvtRoleMemberAdded {
			added++
		}
		if e.EventType == connectors.EvtRoleMemberRemoved {
			removed++
		}
	}
	assert.Equal(t, 1, added)
	assert.Equal(t, 1, removed)
}

func TestJML_EmptyConnectorIDs_NoEnqueue(t *testing.T) {
	repo := newTestOutboxRepo()

	err := EmitJoinerProvisioning(
		context.Background(),
		"org-1",
		"user-alice",
		[]string{"role-1"},
		[]byte(`{}`),
		repo,
		platformuuid.V7Generator{},
		nil, // 空 connectorIDs
	)
	require.NoError(t, err)
	assert.Len(t, repo.Events(), 0, "empty connectorIDs → no enqueue")
}

func TestJML_MultipleConnectors(t *testing.T) {
	repo := newTestOutboxRepo()

	err := EmitJoinerProvisioning(
		context.Background(),
		"org-1",
		"user-alice",
		nil,
		[]byte(`{"userName":"a"}`),
		repo,
		platformuuid.V7Generator{},
		[]string{"c-a", "c-b", "c-c"}, // 3 个 connector
	)
	require.NoError(t, err)

	events := repo.Events()
	assert.Len(t, events, 3, "3 connectors → 3 user.created events")

	// 检查 headers 里 connector_id 各不相同
	seen := make(map[string]bool)
	for _, e := range events {
		var headers map[string]string
		require.NoError(t, json.Unmarshal(e.Headers, &headers))
		seen[headers["connector_id"]] = true
	}
	assert.Len(t, seen, 3)
	assert.Contains(t, seen, "c-a")
	assert.Contains(t, seen, "c-b")
	assert.Contains(t, seen, "c-c")
}

// ──── Connector domain 辅助测试 ────

func TestConnectors_IsValidEventType(t *testing.T) {
	for _, et := range connectors.ValidEventTypes {
		assert.True(t, connectors.IsValidEventType(et), et)
	}
	assert.False(t, connectors.IsValidEventType("foo.bar"))
	assert.False(t, connectors.IsValidEventType(""))
}

func TestConnectors_Defaults(t *testing.T) {
	assert.Equal(t, 5000, connectors.DefaultTimeoutMs)
	assert.Equal(t, 300, connectors.DefaultMaxRatePerMin)
	assert.Equal(t, "scim_outbound", connectors.AggregateType)
}
