package scim

import (
	"context"
	"sort"
	"sync"
	"time"

	connectors "github.com/guliangiam/guliangiam/internal/domain/connectors"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// StubConnectorRepo 是 ConnectorRepository 的内存实现，仅用于测试。
type StubConnectorRepo struct {
	mu   sync.Mutex
	data map[string]*connectors.Connector // key = connectorID + "|" + orgID
}

// NewStubConnectorRepo 构造空的 Stub。
func NewStubConnectorRepo() *StubConnectorRepo {
	return &StubConnectorRepo{data: make(map[string]*connectors.Connector)}
}

// key 生成 map key。
func key(connectorID, orgID string) string {
	return connectorID + "|" + orgID
}

// cloneConnector 隔离调用方与 Stub 内部状态；BearerToken 只在内存对象上流动。
func cloneConnector(c *connectors.Connector) *connectors.Connector {
	if c == nil {
		return nil
	}
	clone := *c
	return &clone
}

// Get 按 connectorID + orgID 查找 connector。
func (s *StubConnectorRepo) Get(ctx context.Context, connectorID, orgID string) (*connectors.Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.data[key(connectorID, orgID)]
	if !ok {
		return nil, apperrors.Newf(apperrors.CodeNotFound, "connector not found: %s (org=%s)", connectorID, orgID)
	}
	return cloneConnector(c), nil
}

// List 返回指定组织的全部 connector，按名称排序。
func (s *StubConnectorRepo) List(ctx context.Context, orgID string) ([]*connectors.Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*connectors.Connector, 0, len(s.data))
	for _, c := range s.data {
		if c.OrganizationID == orgID {
			out = append(out, cloneConnector(c))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].ID < out[j].ID
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// Create 新增一个 connector；同组织下 ID 已存在时返回 Conflict。
func (s *StubConnectorRepo) Create(ctx context.Context, orgID string, c *connectors.Connector) error {
	if c == nil {
		return apperrors.NewInvalidArgument("connector is required")
	}
	if orgID == "" {
		return apperrors.NewInvalidArgument("organizationID is required")
	}
	if c.OrganizationID != "" && c.OrganizationID != orgID {
		return apperrors.NewInvalidArgument("connector organization mismatch")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(c.ID, orgID)
	if _, exists := s.data[k]; exists {
		return apperrors.Newf(apperrors.CodeConflict, "connector already exists: %s", c.ID)
	}
	stored := cloneConnector(c)
	stored.OrganizationID = orgID
	if stored.Revision == 0 {
		stored.Revision = 1
	}
	now := time.Now().UTC()
	if stored.CreatedAt.IsZero() {
		stored.CreatedAt = now
	}
	if stored.UpdatedAt.IsZero() {
		stored.UpdatedAt = now
	}
	s.data[k] = stored
	*c = *cloneConnector(stored)
	return nil
}

// Update 以 revision 乐观锁更新 connector；不匹配时返回 Conflict。
func (s *StubConnectorRepo) Update(ctx context.Context, orgID string, c *connectors.Connector) error {
	if c == nil {
		return apperrors.NewInvalidArgument("connector is required")
	}
	if orgID == "" {
		return apperrors.NewInvalidArgument("organizationID is required")
	}
	if c.Revision <= 0 {
		return apperrors.NewInvalidArgument("connector revision is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(c.ID, orgID)
	current, exists := s.data[k]
	if !exists {
		return apperrors.NewNotFound("connector not found")
	}
	if current.Revision != c.Revision {
		return apperrors.NewConflict("connector revision conflict")
	}
	stored := cloneConnector(c)
	stored.OrganizationID = orgID
	stored.Revision = current.Revision + 1
	stored.UpdatedAt = time.Now().UTC()
	s.data[k] = stored
	*c = *cloneConnector(stored)
	return nil
}

// Delete 删除指定组织下的 connector；不存在时返回 NotFound。
func (s *StubConnectorRepo) Delete(ctx context.Context, orgID, connectorID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(connectorID, orgID)
	if _, exists := s.data[k]; !exists {
		return apperrors.NewNotFound("connector not found")
	}
	delete(s.data, k)
	return nil
}

// Upsert 测试注入用。
func (s *StubConnectorRepo) Upsert(c *connectors.Connector) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key(c.ID, c.OrganizationID)] = cloneConnector(c)
}

// GetAll 返回所有已注册的 connector（测试辅助）。
func (s *StubConnectorRepo) GetAll() []*connectors.Connector {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*connectors.Connector, 0, len(s.data))
	for _, c := range s.data {
		out = append(out, cloneConnector(c))
	}
	return out
}

// 编译期断言
var _ connectors.ConnectorRepository = (*StubConnectorRepo)(nil)
