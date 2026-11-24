package workload

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/workload"
)

// ── 内联 mock: domain/workload.Repository ────────────────────────────────────
// workload.Handler 依赖 domain.Repository（app 层是薄包装）。
// Handler 只调用 Trust 相关方法（4 个），Pool/Account 方法给空实现。
type mockRepo struct {
	trusts                []*domain.FederationTrust
	createErr             error
	listErr               error
	lastCreated           *domain.FederationTrust
	lastSetID, lastSetOrg string
	lastSetVal            domain.FederationTrustStatus
}

func (m *mockRepo) ListTrusts(ctx context.Context, q domain.ListTrustsQuery) ([]*domain.FederationTrust, int64, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	out := make([]*domain.FederationTrust, 0, len(m.trusts))
	for _, t := range m.trusts {
		if t.OrganizationID == q.OrganizationID {
			out = append(out, t)
		}
	}
	return out, int64(len(out)), nil
}
func (m *mockRepo) CreateTrust(ctx context.Context, t *domain.FederationTrust) error {
	m.lastCreated = t
	return m.createErr
}
func (m *mockRepo) DeleteTrust(ctx context.Context, id, orgID string) error { return nil }
func (m *mockRepo) SetTrustStatus(ctx context.Context, id, orgID string, s domain.FederationTrustStatus) error {
	m.lastSetID, m.lastSetOrg, m.lastSetVal = id, orgID, s
	return nil
}
func (m *mockRepo) ListPools(ctx context.Context, q domain.ListPoolsQuery) ([]*domain.Pool, int64, error) {
	return nil, 0, nil
}
func (m *mockRepo) CreatePool(ctx context.Context, p *domain.Pool) error { return nil }
func (m *mockRepo) ListAccounts(ctx context.Context, q domain.ListAccountsQuery) ([]*domain.ServiceAccount, int64, error) {
	return nil, 0, nil
}
func (m *mockRepo) CreateAccount(ctx context.Context, a *domain.ServiceAccount) error { return nil }
func (m *mockRepo) GetPool(ctx context.Context, id, orgID string) (*domain.Pool, error) {
	return &domain.Pool{ID: id, OrganizationID: orgID}, nil
}
func (m *mockRepo) GetTrust(ctx context.Context, id, orgID string) (*domain.FederationTrust, error) {
	return nil, nil
}
func (m *mockRepo) FindActiveTrustsByIssuer(ctx context.Context, issuer string) ([]*domain.FederationTrust, error) {
	return nil, nil
}
func (m *mockRepo) GetAccount(ctx context.Context, id, orgID string) (*domain.ServiceAccount, error) {
	return nil, nil
}
func (m *mockRepo) GetAccountByClientID(ctx context.Context, clientID string) (*domain.ServiceAccount, error) {
	return nil, nil
}
func (m *mockRepo) GetAccountByName(ctx context.Context, orgID, name string) (*domain.ServiceAccount, error) {
	return nil, nil
}
func (m *mockRepo) SetAccountStatus(ctx context.Context, id, orgID string, status domain.ServiceAccountStatus, revision int64) error {
	return nil
}
func (m *mockRepo) RotateAccountSecret(ctx context.Context, id, orgID, hash string, rotatedAt time.Time, revision int64) error {
	return nil
}
func (m *mockRepo) DeleteAccount(ctx context.Context, id, orgID string, revision int64) error {
	return nil
}

func newHandler(repo *mockRepo) *Handler {
	// 直接注入 domain.Repo — Handler 只做参数校验 + 调用 Repo + DTO 转换
	// 依赖 appworkload.FederationTrust（= domain 层别名）
	return NewHandler(repo)
}

// ── 1. ListTrusts: orgID 空 → InvalidArgument ────────────────────────────────
func TestListTrusts_OrgIDRequired(t *testing.T) {
	h := newHandler(&mockRepo{})
	_, _, err := h.ListTrusts(context.Background(), "  ", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "org_id is required") {
		t.Fatalf("got: %v", err)
	}
}

// ── 2. ListTrusts: 转换 + 过滤 orgID ─────────────────────────────────────────
func TestListTrusts_FilterAndTransform(t *testing.T) {
	now := time.Now().UTC()
	repo := &mockRepo{trusts: []*domain.FederationTrust{
		{ID: "t1", OrganizationID: "org1", PoolID: "pool1", Name: "A", Issuer: "https://a", Status: domain.TrustStatusActive, CreatedAt: now},
		{ID: "t2", OrganizationID: "org2", PoolID: "pool2", Name: "B", Issuer: "https://b", Status: domain.TrustStatusDisabled, CreatedAt: now},
		{ID: "t3", OrganizationID: "org1", PoolID: "pool1", Name: "C", Issuer: "https://c", Status: domain.TrustStatusActive, CreatedAt: now},
	}}
	h := newHandler(repo)

	out, total, err := h.ListTrusts(context.Background(), "org1", "")
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total=%d want 2", total)
	}
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
	if out[0].OrgID != "org1" || out[1].OrgID != "org1" {
		t.Fatalf("wrong org filter: %+v", out)
	}
	if out[0].Status != "active" {
		t.Fatalf("status=%s", out[0].Status)
	}
}

// ── 3. CreateTrust: 参数校验矩阵 ─────────────────────────────────────────────
func TestCreateTrust_Validation(t *testing.T) {
	h := newHandler(&mockRepo{})
	tests := []struct {
		name, orgID string
		trust       CreateTrustRequest
		need        string
	}{
		{"org empty", "  ", CreateTrustRequest{}, "org_id is required"},
		{"pool missing", "o", CreateTrustRequest{Name: "n", Issuer: "i"}, "pool_id, name, and issuer are required"},
		{"name missing", "o", CreateTrustRequest{PoolID: "p", Issuer: "i"}, "pool_id, name, and issuer are required"},
		{"issuer missing", "o", CreateTrustRequest{PoolID: "p", Name: "n"}, "pool_id, name, and issuer are required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.CreateTrust(context.Background(), tc.orgID, "t1", tc.trust)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.need) {
				t.Fatalf("got: %v, want contains %q", err, tc.need)
			}
		})
	}
}

// ── 4. CreateTrust: 显式 JWKS/audience/subjects + status ─────────────────────
func TestCreateTrust_ExplicitSecurityConfiguration(t *testing.T) {
	repo := &mockRepo{}
	h := newHandler(repo)
	_, err := h.CreateTrust(context.Background(), "org1", "t1", CreateTrustRequest{
		PoolID: "pool1", Name: "Trust A", Issuer: "https://issuer.example.com/",
		JWKSURI: "https://issuer.example.com/jwks.json", Audience: []string{"api"}, Subjects: []string{"repo:org/*"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.lastCreated == nil {
		t.Fatal("CreateTrust not called")
	}
	if repo.lastCreated.JWKSURI != "https://issuer.example.com/jwks.json" {
		t.Fatalf("JWKSURI changed unexpectedly: %q", repo.lastCreated.JWKSURI)
	}
	if len(repo.lastCreated.Audience) != 1 || repo.lastCreated.Audience[0] != "api" {
		t.Fatalf("audience: %v", repo.lastCreated.Audience)
	}
	if len(repo.lastCreated.Subjects) != 1 || repo.lastCreated.Subjects[0] != "repo:org/*" {
		t.Fatalf("subjects: %v", repo.lastCreated.Subjects)
	}
	if repo.lastCreated.Status != domain.TrustStatusActive {
		t.Fatalf("status default: %v", repo.lastCreated.Status)
	}
}

// ── 5. CreateTrust: Repo error → wrap ────────────────────────────────────────
func TestCreateTrust_RepoError(t *testing.T) {
	repo := &mockRepo{createErr: errors.New("db down")}
	h := newHandler(repo)
	_, err := h.CreateTrust(context.Background(), "o", "t", CreateTrustRequest{
		PoolID: "p", Name: "n", Issuer: "https://issuer.example.com", JWKSURI: "https://issuer.example.com/jwks",
		Audience: []string{"api"}, Subjects: []string{"repo:org/*"},
	})
	if err == nil {
		t.Fatal("expected wrapped error")
	}
	if !strings.Contains(err.Error(), "db down") {
		t.Fatalf("got: %v", err)
	}
}

// ── 6. DeleteTrust: 参数校验 ────────────────────────────────────────────────
func TestDeleteTrust_Validation(t *testing.T) {
	h := newHandler(&mockRepo{})
	if err := h.DeleteTrust(context.Background(), "", "t"); err == nil {
		t.Fatal("org err")
	}
	if err := h.DeleteTrust(context.Background(), "o", ""); err == nil {
		t.Fatal("id err")
	}
}

// ── 7. EnableTrust / DisableTrust: 直接透传 SetTrustStatus ───────────────────
func TestSetTrustStatus_Passthrough(t *testing.T) {
	repo := &mockRepo{}
	h := newHandler(repo)
	h.EnableTrust(context.Background(), "org1", "t1")
	if repo.lastSetID != "t1" || repo.lastSetOrg != "org1" || repo.lastSetVal != domain.TrustStatusActive {
		t.Fatalf("EnableTrust: %s %s %v", repo.lastSetID, repo.lastSetOrg, repo.lastSetVal)
	}
	h.DisableTrust(context.Background(), "org1", "t1")
	if repo.lastSetVal != domain.TrustStatusDisabled {
		t.Fatalf("DisableTrust: %v", repo.lastSetVal)
	}
}
