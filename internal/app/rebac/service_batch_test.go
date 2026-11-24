package rebac

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// batchGuardUoW 的嵌入接口为 nil：上限/必填校验必须在开启事务前短路，
// 一旦这些守卫失效而触碰 UOW，测试会以 panic 失败而不是静默通过。
type batchGuardUoW struct{ appuow.UnitOfWork }

func TestService_CreateTuples_RejectsOversizedBatch(t *testing.T) {
	svc := NewService(nil, nil, batchGuardUoW{})
	commands := make([]CreateTupleCommand, maxCreateTuplesBatch+1)
	for i := range commands {
		commands[i] = CreateTupleCommand{
			OrganizationID: "org-1", NamespaceID: "ns-1",
			Subject: "user:a", Relation: "member", Object: "doc:1",
		}
	}
	_, err := svc.CreateTuples(context.Background(), commands)
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
	assert.Contains(t, err.Error(), "too many tuples")
}

func TestService_CreateTuples_RequiresFieldsBeforeTransaction(t *testing.T) {
	svc := NewService(nil, nil, batchGuardUoW{})
	cases := []CreateTupleCommand{
		{NamespaceID: "ns-1", Subject: "user:a", Relation: "member", Object: "doc:1"},
		{OrganizationID: "org-1", Subject: "user:a", Relation: "member", Object: "doc:1"},
		{OrganizationID: "org-1", NamespaceID: "ns-1", Relation: "member", Object: "doc:1"},
		{OrganizationID: "org-1", NamespaceID: "ns-1", Subject: "user:a", Object: "doc:1"},
		{OrganizationID: "org-1", NamespaceID: "ns-1", Subject: "user:a", Relation: "member"},
	}
	for _, cmd := range cases {
		_, err := svc.CreateTuples(context.Background(), []CreateTupleCommand{cmd})
		require.Error(t, err)
		assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
	}
}
