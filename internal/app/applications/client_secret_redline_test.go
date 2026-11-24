package applications

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
)

// 本文件锁定四条凭据红线的可核验落点：
//  1. 创建/轮换在同一事务内写审计 Record + Outbox Event；
//  2. 审计/Outbox payload 不含明文 secret、不含 argon2id 哈希；
//  3. 事务失败回滚时不得留下孤立的审计/Outbox 行；
//  4. 轮换同样按可信 organization_id 读取，跨租户表现为 404。

// redlineRecords/redlineOutbox 只记录本事务缓冲区的写入；redlineUoW 只有在
// 事务函数返回 nil 时才把缓冲区合并到已提交集合，模拟数据库回滚语义。
type redlineRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *redlineRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type redlineOutbox struct {
	domainoutbox.Repository
	events []*domainoutbox.OutboxEvent
}

func (r *redlineOutbox) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	r.events = append(r.events, event)
	return nil
}

// redlineTx 是事务内可见的 UnitOfWork 视图，持有本事务自己的审计/Outbox 缓冲。
type redlineTx struct {
	appuow.UnitOfWork
	apps    domain.Repository
	records *redlineRecords
	outbox  *redlineOutbox
}

func (u *redlineTx) Applications() domain.Repository   { return u.apps }
func (u *redlineTx) Records() recordsdomain.Repository { return u.records }
func (u *redlineTx) Outbox() domainoutbox.Repository   { return u.outbox }

// redlineUoW 是可回滚的事务宿主。
type redlineUoW struct {
	appuow.AuthUoW
	apps      domain.Repository
	committed *redlineRecords
	outbox    *redlineOutbox
}

func (u *redlineUoW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	tx := &redlineTx{apps: u.apps, records: &redlineRecords{}, outbox: &redlineOutbox{}}
	if err := fn(ctx, tx); err != nil {
		// 模拟 ROLLBACK：丢弃事务内写入的审计/Outbox 行。
		return err
	}
	u.committed.created = append(u.committed.created, tx.records.created...)
	u.outbox.events = append(u.outbox.events, tx.outbox.events...)
	return nil
}

// redlineFailRepo 在指定方法上注入仓储失败，用于验证回滚路径。
type redlineFailRepo struct {
	*mockApplicationRepo
	failCreateSecret bool
	failRotateSecret bool
}

func (r *redlineFailRepo) CreateClientSecret(ctx context.Context, organizationID string, secret *domain.ClientSecret) error {
	if r.failCreateSecret {
		return errors.New("injected CreateClientSecret failure")
	}
	return r.mockApplicationRepo.CreateClientSecret(ctx, organizationID, secret)
}

func (r *redlineFailRepo) RotateClientSecret(ctx context.Context, organizationID, applicationID string, next *domain.ClientSecret, rotatedAt, graceUntil time.Time) error {
	if r.failRotateSecret {
		return errors.New("injected RotateClientSecret failure")
	}
	return r.mockApplicationRepo.RotateClientSecret(ctx, organizationID, applicationID, next, rotatedAt, graceUntil)
}

func resetRedlineSecretStore() {
	fakeClientSecretStore = map[string][]*domain.ClientSecret{}
}

func newRedlineService(t *testing.T, repo domain.Repository) (Service, *redlineRecords, *redlineOutbox) {
	t.Helper()
	committed := &redlineRecords{}
	outbox := &redlineOutbox{}
	svc := NewService(
		fakeClock{fixedTime: time.Unix(0, 0).UTC()},
		&fakeUUIDGenerator{fixedID: "redline-app-id", altID: "redline-secret-id"},
		&redlineUoW{apps: repo, committed: committed, outbox: outbox},
	)
	return svc, committed, outbox
}

func decodeRedlinePayload(t *testing.T, event *domainoutbox.OutboxEvent) map[string]any {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(event.Payload, &payload))
	return payload
}

// assertNoSecretMaterial 断言审计记录序列化结果与 Outbox payload 都不含
// 一次性明文，也不含 argon2id 哈希。
func assertNoSecretMaterial(t *testing.T, record *recordsdomain.Record, event *domainoutbox.OutboxEvent, plaintext string) {
	t.Helper()

	recordJSON, err := json.Marshal(record)
	require.NoError(t, err)
	for _, blob := range []string{string(recordJSON), string(event.Payload)} {
		if plaintext != "" {
			assert.NotContains(t, blob, plaintext, "audit/outbox 不得包含一次性明文 secret")
		}
		assert.NotContains(t, blob, "$argon2id$", "audit/outbox 不得包含 secret 哈希")
	}

	payload := decodeRedlinePayload(t, event)
	for _, forbidden := range []string{"clientSecret", "clientSecretPlain", "secretHash", "hash"} {
		_, exists := payload[forbidden]
		assert.Falsef(t, exists, "payload 不得包含字段 %q", forbidden)
	}
}

func TestCreateApplicationEmbedAuditOutboxInSameTransaction(t *testing.T) {
	resetRedlineSecretStore()
	repo := &mockApplicationRepo{}
	svc, records, outbox := newRedlineService(t, repo)

	result, err := svc.CreateApplication(context.Background(), CreateApplicationCommand{
		OrganizationID:          "org-1",
		ActorID:                 "actor-1",
		Slug:                    "redline-app",
		DisplayName:             "Redline Application",
		AppType:                 "web",
		Status:                  "active",
		TokenEndpointAuthMethod: string(domain.TokenEndpointAuthMethodClientSecretBasic),
		RedirectURIs:            []string{"https://example.com/callback"},
		GrantTypes:              []string{"authorization_code"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.ClientSecretPlain, "创建响应必须携带一次性明文")

	require.Len(t, records.created, 1, "同一事务必须写一条审计 Record")
	require.Len(t, outbox.events, 1, "同一事务必须写一条 Outbox Event")

	record := records.created[0]
	event := outbox.events[0]

	assert.Equal(t, "org-1", record.OrganizationID)
	assert.Equal(t, "actor-1", record.ActorID)
	assert.Equal(t, "application.created", record.Action)
	assert.Equal(t, "Application", record.ResourceType)
	assert.Equal(t, result.Application.ID, record.ResourceID)

	assert.Equal(t, "Application", event.AggregateType)
	assert.Equal(t, result.Application.ID, event.AggregateID)
	assert.Equal(t, "application.created", event.EventType)
	require.NotNil(t, event.OrganizationID)
	assert.Equal(t, "org-1", *event.OrganizationID)

	assertNoSecretMaterial(t, record, event, result.ClientSecretPlain)

	payload := decodeRedlinePayload(t, event)
	assert.Equal(t, "client_secret_basic", payload["tokenEndpointAuthMethod"])
	assert.Equal(t, result.Application.ClientID, payload["clientId"])
	assert.Equal(t, result.ClientSecretPrefix, payload["clientSecretPrefix"],
		"payload 只允许出现非秘密前缀")
	assert.NotEqual(t, result.ClientSecretPlain, result.ClientSecretPrefix,
		"前缀截断后不得等于完整明文")
}

func TestCreateApplicationRollbackLeavesNoAuditOrOutbox(t *testing.T) {
	resetRedlineSecretStore()
	repo := &redlineFailRepo{mockApplicationRepo: &mockApplicationRepo{}, failCreateSecret: true}
	svc, records, outbox := newRedlineService(t, repo)

	_, err := svc.CreateApplication(context.Background(), CreateApplicationCommand{
		OrganizationID:          "org-1",
		ActorID:                 "actor-1",
		Slug:                    "redline-rollback",
		DisplayName:             "Redline Rollback",
		AppType:                 "web",
		Status:                  "active",
		TokenEndpointAuthMethod: string(domain.TokenEndpointAuthMethodClientSecretBasic),
		RedirectURIs:            []string{"https://example.com/callback"},
		GrantTypes:              []string{"authorization_code"},
	})
	require.Error(t, err, "secret 写入失败必须让整个事务失败")

	assert.Empty(t, records.created, "回滚后不得留下孤立审计行")
	assert.Empty(t, outbox.events, "回滚后不得留下孤立 Outbox 行")
}

func TestRotateClientSecretEmbedAuditOutboxInSameTransaction(t *testing.T) {
	resetRedlineSecretStore()
	repo := &mockApplicationRepo{apps: []*domain.Application{{
		ID:                      "redline-app-id",
		OrganizationID:          "org-1",
		ClientID:                "gl_cid_redline",
		TokenEndpointAuthMethod: domain.TokenEndpointAuthMethodClientSecretBasic,
	}}}
	svc, records, outbox := newRedlineService(t, repo)

	result, err := svc.RotateClientSecret(context.Background(), RotateClientSecretCommand{
		ID:             "redline-app-id",
		OrganizationID: "org-1",
		ActorID:        "actor-1",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.ClientSecretPlain)

	require.Len(t, records.created, 1, "轮换必须写一条审计 Record")
	require.Len(t, outbox.events, 1, "轮换必须写一条 Outbox Event")

	record := records.created[0]
	event := outbox.events[0]

	assert.Equal(t, "org-1", record.OrganizationID)
	assert.Equal(t, "actor-1", record.ActorID)
	assert.Equal(t, "application.client_secret.rotated", record.Action)
	assert.Equal(t, "application.client_secret.rotated", event.EventType)

	assertNoSecretMaterial(t, record, event, result.ClientSecretPlain)

	payload := decodeRedlinePayload(t, event)
	assert.Equal(t, result.ClientSecretPrefix, payload["clientSecretPrefix"])
	assert.Equal(t, result.GraceUntil.UTC().Format(time.RFC3339), payload["graceUntil"])
}

func TestRotateClientSecretRollbackLeavesNoAuditOrOutbox(t *testing.T) {
	resetRedlineSecretStore()
	repo := &redlineFailRepo{
		mockApplicationRepo: &mockApplicationRepo{apps: []*domain.Application{{
			ID:                      "redline-app-id",
			OrganizationID:          "org-1",
			ClientID:                "gl_cid_redline",
			TokenEndpointAuthMethod: domain.TokenEndpointAuthMethodClientSecretBasic,
		}}},
		failRotateSecret: true,
	}
	svc, records, outbox := newRedlineService(t, repo)

	_, err := svc.RotateClientSecret(context.Background(), RotateClientSecretCommand{
		ID:             "redline-app-id",
		OrganizationID: "org-1",
		ActorID:        "actor-1",
	})
	require.Error(t, err)

	assert.Empty(t, records.created, "回滚后不得留下孤立审计行")
	assert.Empty(t, outbox.events, "回滚后不得留下孤立 Outbox 行")
}

// TestRotateClientSecretCrossTenantLooksMissing 锁定跨租户轮换的拒绝路径：
// 用另一个 organization_id 轮换时，仓储显式 organization_id 过滤使应用不可见，
// 返回 404（不是 403），从而不泄露应用是否存在。
func TestRotateClientSecretCrossTenantLooksMissing(t *testing.T) {
	resetRedlineSecretStore()
	repo := &mockApplicationRepo{apps: []*domain.Application{{
		ID:                      "redline-app-id",
		OrganizationID:          "org-1",
		ClientID:                "gl_cid_redline",
		TokenEndpointAuthMethod: domain.TokenEndpointAuthMethodClientSecretBasic,
	}}}
	svc, records, outbox := newRedlineService(t, repo)

	_, err := svc.RotateClientSecret(context.Background(), RotateClientSecretCommand{
		ID:             "redline-app-id",
		OrganizationID: "org-2",
		ActorID:        "actor-2",
	})
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "application not found"))
	var appErr interface{ HTTPStatus() int }
	if errors.As(err, &appErr) {
		assert.Equal(t, 404, appErr.HTTPStatus())
	}

	assert.Empty(t, records.created, "跨租户轮换不得产生审计")
	assert.Empty(t, outbox.events, "跨租户轮换不得产生 Outbox")
}
