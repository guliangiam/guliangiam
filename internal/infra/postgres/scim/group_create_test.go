package scim

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/scim"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

type failingExecer struct {
	calls  int
	failAt int
}

func (f *failingExecer) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	f.calls++
	if f.calls == f.failAt {
		return pgconn.CommandTag{}, errors.New("boom")
	}
	return pgconn.NewCommandTag("INSERT 1"), nil
}

// 成员写入失败必须中断并返回错误（由 CreateGroup 的事务回滚），不能继续写后续成员。
func TestCreateGroupRows_MemberFailurePropagates(t *testing.T) {
	group := &domain.ScimGroup{
		ID:             "group-1",
		OrganizationID: "org-1",
		DisplayName:    "Engineering",
		Members: []domain.Member{
			{Value: "user-1"},
			{Value: "user-2"},
		},
	}

	exec := &failingExecer{failAt: 2} // 第 1 次写组成功，第 2 次写第一个成员失败
	err := createGroupRows(context.Background(), exec, group, time.Now())

	require.Error(t, err)
	require.Contains(t, err.Error(), "add scim group member user-1")
	require.Equal(t, 2, exec.calls, "失败后不得继续写后续成员")
}

func TestCreateGroupRows_WritesAllMembers(t *testing.T) {
	group := &domain.ScimGroup{
		ID:          "group-1",
		DisplayName: "Engineering",
		Members:     []domain.Member{{Value: "user-1"}, {Value: "user-2"}},
	}

	exec := &failingExecer{failAt: 99}
	require.NoError(t, createGroupRows(context.Background(), exec, group, time.Now()))
	require.Equal(t, 3, exec.calls)
}
