package auth

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	outboxdomain "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	recordsmock "github.com/guliangiam/guliangiam/internal/domain/records/mock"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
)

// 自助改密是口令-only 场景：定向更新不得清空既有访问密钥。
func TestChangePassword_PreservesAccessCredentials(t *testing.T) {
	rotatedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	org := newTestOrg()
	user := newTestUser(func(u *userdomain.User) {
		u.Credentials.AccessKey = "AK-1"
		u.Credentials.AccessSecretHash = "SECRET-HASH-1"
		u.Credentials.AccessSecretRotatedAt = &rotatedAt
	})
	uow := newTestUow(org, user, newTestApp())
	uow.tokenRepo = &testTokenRepository{}
	svc := newTestService(uow)

	_, _, err := svc.ChangePassword(context.Background(), ChangePasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		OldPassword:      testPassword,
		NewPassword:      "Xq7!mK2#vR9z",
	})
	require.NoError(t, err)

	assert.Equal(t, "AK-1", user.Credentials.AccessKey, "access key must survive a self password change")
	assert.Equal(t, "SECRET-HASH-1", user.Credentials.AccessSecretHash, "access secret hash must survive a self password change")
	require.NotNil(t, user.Credentials.AccessSecretRotatedAt)
	assert.Equal(t, rotatedAt, *user.Credentials.AccessSecretRotatedAt)
}

type capturingOutbox struct {
	events []*outboxdomain.OutboxEvent
}

func (c *capturingOutbox) Enqueue(ctx context.Context, item *outboxdomain.OutboxEvent) error {
	c.events = append(c.events, item)
	return nil
}

func (c *capturingOutbox) EnqueueBatch(ctx context.Context, items []*outboxdomain.OutboxEvent) error {
	c.events = append(c.events, items...)
	return nil
}

// 自助改密必须同事务写审计 + outbox，actor 为本人，payload 不含口令/哈希。
func TestChangePassword_WritesAuditAndOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	records := recordsmock.NewMockRepository(ctrl)

	var captured *recordsdomain.Record
	records.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, record *recordsdomain.Record) error {
			captured = record
			return nil
		},
	)

	org := newTestOrg()
	user := newTestUser()
	uow := newTestUow(org, user, newTestApp())
	uow.recordsRepo = records
	uow.outboxRepo = &capturingOutbox{}
	uow.tokenRepo = &testTokenRepository{}
	svc := newTestService(uow)

	const previousSessionID = "sess-old"
	_, _, err := svc.ChangePassword(context.Background(), ChangePasswordCommand{
		OrganizationSlug:  testOrgSlug,
		Username:          testUsername,
		OldPassword:       testPassword,
		NewPassword:       "Xq7!mK2#vR9z",
		PreviousSessionID: previousSessionID,
	})
	require.NoError(t, err)

	require.NotNil(t, captured)
	assert.Equal(t, "auth.password.changed", captured.Action)
	assert.Equal(t, user.ID, captured.ActorID)
	assert.Equal(t, user.ID, captured.ResourceID)
	assert.Equal(t, recordsdomain.SurfaceAuthentication, captured.Surface)

	outbox := uow.outboxRepo.(*capturingOutbox)
	require.Len(t, outbox.events, 1)
	assert.Equal(t, "auth.password.changed", outbox.events[0].EventType)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(outbox.events[0].Payload, &payload))
	assert.Equal(t, user.ID, payload["userId"])
	assert.Equal(t, previousSessionID, payload["sessionId"])
	assert.Equal(t, "self", payload["method"])
	assert.Len(t, payload, 3, "payload must only carry userId/sessionId/method")
	assert.NotContains(t, string(outbox.events[0].Payload), "password")
	assert.NotContains(t, string(outbox.events[0].Payload), "hash")
}

type capturingNotificationPublisher struct {
	commands []NotificationCommand
	err      error
}

func (c *capturingNotificationPublisher) Publish(ctx context.Context, command NotificationCommand) error {
	c.commands = append(c.commands, command)
	return c.err
}

// 自助改密成功后向本人发送安全提醒；该动作不属于口令写事务。
func TestChangePassword_NotifiesSecurityInbox(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	uow := newTestUow(org, user, newTestApp())
	uow.tokenRepo = &testTokenRepository{}
	svc := newTestService(uow)
	publisher := &capturingNotificationPublisher{}
	svc.Notifications = publisher

	_, _, err := svc.ChangePassword(context.Background(), ChangePasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		OldPassword:      testPassword,
		NewPassword:      "Xq7!mK2#vR9z",
	})
	require.NoError(t, err)

	require.Len(t, publisher.commands, 1)
	notification := publisher.commands[0]
	assert.Equal(t, user.OrganizationID, notification.OrganizationID)
	assert.Equal(t, user.ID, notification.UserID)
	assert.Equal(t, "security", notification.Category)
	assert.Equal(t, "auth.password.changed", notification.EventType)
	assert.Equal(t, "/account", notification.Link)
	assert.NotEmpty(t, notification.Title)
	assert.NotEmpty(t, notification.Content)
}

// 通知失败只记录日志，不影响已经提交的改密结果。
func TestChangePassword_NotificationFailureDoesNotFailChange(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	uow := newTestUow(org, user, newTestApp())
	uow.tokenRepo = &testTokenRepository{}
	svc := newTestService(uow)
	svc.Notifications = &capturingNotificationPublisher{err: assert.AnError}

	result, _, err := svc.ChangePassword(context.Background(), ChangePasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		OldPassword:      testPassword,
		NewPassword:      "Xq7!mK2#vR9z",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "changed", result.Status)
}
