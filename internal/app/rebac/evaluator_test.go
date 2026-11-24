package rebac

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/rebac"
	"github.com/stretchr/testify/assert"
)

// mockStore 是 TupleStore 的内存实现，方便构造测试数据。
type mockStore struct {
	tuples       []*domain.Tuple
	latestZookie string
}

func (m *mockStore) FetchAllTuples(ctx context.Context, orgID, nsID, atZookie string) ([]*domain.Tuple, error) {
	// atZookie 非空时过滤 zookie > atZookie 的元组
	if atZookie == "" {
		return m.tuples, nil
	}
	out := make([]*domain.Tuple, 0, len(m.tuples))
	for _, t := range m.tuples {
		if t.Zookie == "" || t.Zookie <= atZookie {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *mockStore) GetLatestZookie(ctx context.Context, orgID, nsID string) (string, error) {
	if m.latestZookie == "" {
		return "42", nil // 默认值，老测试不需要精确 revision
	}
	return m.latestZookie, nil
}

// T 快速构造 tuple 字面量
func T(subj, rel, obj string) *domain.Tuple {
	return &domain.Tuple{Subject: subj, Relation: rel, Object: obj}
}

func TestCheck_DirectHit(t *testing.T) {
	engine := NewEngine(&mockStore{tuples: []*domain.Tuple{
		T("user:u1", "admin", "folder:f1"),
		T("user:u1", "write", "folder:f1"),
	}})

	res, err := engine.Check(context.Background(), domain.CheckRequest{
		Tuple: domain.TupleKey{Subject: "user:u1", Relation: "admin", Object: "folder:f1"},
	})
	assert.NoError(t, err)
	assert.True(t, res.Allowed, "direct hit should return true")

	res2, _ := engine.Check(context.Background(), domain.CheckRequest{
		Tuple: domain.TupleKey{Subject: "user:u1", Relation: "delete", Object: "folder:f1"},
	})
	assert.False(t, res2.Allowed, "missing relation should return false")
}

func TestCheck_OneHopInheritance(t *testing.T) {
	// u2 has_role role:editor → role:editor write folder:*
	engine := NewEngine(&mockStore{tuples: []*domain.Tuple{
		T("user:u2", "has_role", "role:editor"),
		T("role:editor", "write", "folder:*"),
		T("role:editor", "read", "folder:*"),
	}})

	res, err := engine.Check(context.Background(), domain.CheckRequest{
		Tuple: domain.TupleKey{Subject: "user:u2", Relation: "write", Object: "folder:*"},
	})
	assert.NoError(t, err)
	assert.True(t, res.Allowed, "1-hop inheritance should return true")

	res2, _ := engine.Check(context.Background(), domain.CheckRequest{
		Tuple: domain.TupleKey{Subject: "user:u2", Relation: "delete", Object: "folder:*"},
	})
	assert.False(t, res2.Allowed)
}

func TestCheck_TwoHopRoleChain(t *testing.T) {
	// u1 has_role role:admin → role:admin parent role:editor → role:editor write folder:*
	engine := NewEngine(&mockStore{tuples: []*domain.Tuple{
		T("user:u1", "has_role", "role:admin"),
		T("role:admin", "parent", "role:editor"),
		T("role:editor", "write", "folder:*"),
	}})

	res, err := engine.Check(context.Background(), domain.CheckRequest{
		Tuple: domain.TupleKey{Subject: "user:u1", Relation: "write", Object: "folder:*"},
	})
	assert.NoError(t, err)
	assert.True(t, res.Allowed, "2-hop role chain should return true")
}

func TestCheck_CycleDetection(t *testing.T) {
	// user:u member group:g1, group:g1 member group:g2, group:g2 member group:g1 (cycle!)
	engine := NewEngine(&mockStore{tuples: []*domain.Tuple{
		T("group:g1", "member", "group:g2"),
		T("group:g2", "member", "group:g1"),
		T("user:u", "member", "group:g1"),
		T("group:g1", "read", "doc:d1"),
		T("group:g2", "read", "doc:d2"),
	}})

	// 应该不会无限循环
	res, err := engine.Check(context.Background(), domain.CheckRequest{
		Tuple:      domain.TupleKey{Subject: "user:u", Relation: "read", Object: "doc:d2"},
		DepthLimit: 4,
	})
	assert.NoError(t, err)
	// 从 u→g1→g2→d2 是有效的
	assert.True(t, res.Allowed)
}

func TestCheck_BoundaryCaseObjectWildcard(t *testing.T) {
	// role:admin write folder:* 覆盖 folder:f1
	engine := NewEngine(&mockStore{tuples: []*domain.Tuple{
		T("role:admin", "write", "folder:*"),
	}})

	res, _ := engine.Check(context.Background(), domain.CheckRequest{
		Tuple: domain.TupleKey{Subject: "role:admin", Relation: "write", Object: "folder:f1"},
	})
	// 当前实现是 exact match，folder:* != folder:f1 → false
	// （通配符匹配需要 relation_defs 展开规则，留给后续迭代）
	assert.False(t, res.Allowed, "wildcard matching not implemented yet — exact match only")
}

func TestCheck_SubjectRelationMembership(t *testing.T) {
	sr := "member"
	// u5 member group:g1 → group:g1 has_role role:member → role:member read folder:*
	engine := NewEngine(&mockStore{tuples: []*domain.Tuple{
		{Subject: "user:u5", SubjectRelation: &sr, Relation: "belongs_to", Object: "group:g1"},
		T("group:g1", "has_role", "role:member"),
		T("role:member", "read", "folder:*"),
	}})

	res, err := engine.Check(context.Background(), domain.CheckRequest{
		Tuple: domain.TupleKey{Subject: "user:u5", Relation: "read", Object: "folder:*"},
	})
	assert.NoError(t, err)
	assert.True(t, res.Allowed, "3-hop subject_relation chain should resolve")
}

func TestCheck_DepthLimitTermination(t *testing.T) {
	// 5 层链，depthLimit=2 时在第 2 层终止
	engine := NewEngine(&mockStore{tuples: []*domain.Tuple{
		T("a", "link", "b"),
		T("b", "link", "c"),
		T("c", "link", "d"),
		T("d", "link", "e"),
		T("e", "read", "doc:d1"),
	}})

	res, _ := engine.Check(context.Background(), domain.CheckRequest{
		Tuple:      domain.TupleKey{Subject: "a", Relation: "read", Object: "doc:d1"},
		DepthLimit: 2,
	})
	// 深度 2 时 a→b→c 已用完，无法到 e
	assert.False(t, res.Allowed, "depth limit should stop at depth 2")

	res2, _ := engine.Check(context.Background(), domain.CheckRequest{
		Tuple:      domain.TupleKey{Subject: "a", Relation: "read", Object: "doc:d1"},
		DepthLimit: 8,
	})
	assert.True(t, res2.Allowed, "depth 8 should reach e")
}

func TestExpand_RootNode(t *testing.T) {
	engine := NewEngine(&mockStore{tuples: []*domain.Tuple{
		T("user:u1", "admin", "folder:f1"),
		T("user:u2", "admin", "folder:f1"),
		T("role:editor", "admin", "folder:f1"),
		T("user:u3", "member", "role:editor"),
	}})

	res, err := engine.Expand(context.Background(), domain.ExpandRequest{
		Relation: "admin",
		Object:   "folder:f1",
	})
	assert.NoError(t, err)
	assert.Equal(t, "folder:f1", res.Root.Subject)
	assert.True(t, res.Root.Expanded)
	// 有多个 direct admin，u3 通过 role:editor 到达 folder:f1
	assert.GreaterOrEqual(t, len(res.Root.Children), 2)
}

func TestBatchCheck_MultipleSubjects(t *testing.T) {
	engine := NewEngine(&mockStore{tuples: []*domain.Tuple{
		T("user:u1", "admin", "folder:f1"),
		T("user:u2", "write", "folder:f1"),
		T("user:u3", "read", "folder:f1"),
	}})

	results, err := engine.BatchCheck(context.Background(), "org-1", "ns-1", []domain.TupleKey{
		{Subject: "user:u1", Relation: "admin", Object: "folder:f1"},
		{Subject: "user:u1", Relation: "write", Object: "folder:f1"},
		{Subject: "user:u3", Relation: "read", Object: "folder:f1"},
	}, "")
	assert.NoError(t, err)
	assert.Len(t, results, 3)
	assert.True(t, results[0].Allowed)
	assert.False(t, results[1].Allowed)
	assert.True(t, results[2].Allowed)
}
