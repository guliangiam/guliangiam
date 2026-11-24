package workload

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	domain "github.com/guliangiam/guliangiam/internal/domain/workload"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var testNow = time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return testNow }

type mockRepo struct {
	pools            []*domain.Pool
	trusts           []*domain.FederationTrust
	accounts         []*domain.ServiceAccount
	createPoolErr    error
	createTrustErr   error
	createAccountErr error
	deleteAccountRev int64
}

func (m *mockRepo) ListPools(_ context.Context, q domain.ListPoolsQuery) ([]*domain.Pool, int64, error) {
	return m.pools, int64(len(m.pools)), nil
}
func (m *mockRepo) GetPool(_ context.Context, id, orgID string) (*domain.Pool, error) {
	for _, p := range m.pools {
		if p.ID == id && p.OrganizationID == orgID {
			return p, nil
		}
	}
	return nil, nil
}
func (m *mockRepo) CreatePool(_ context.Context, p *domain.Pool) error {
	if m.createPoolErr != nil {
		return m.createPoolErr
	}
	m.pools = append(m.pools, p)
	return nil
}
func (m *mockRepo) ListTrusts(_ context.Context, q domain.ListTrustsQuery) ([]*domain.FederationTrust, int64, error) {
	items := []*domain.FederationTrust{}
	for _, trust := range m.trusts {
		if (q.OrganizationID == "" || trust.OrganizationID == q.OrganizationID) && (q.PoolID == "" || trust.PoolID == q.PoolID) {
			items = append(items, trust)
		}
	}
	return items, int64(len(items)), nil
}
func (m *mockRepo) GetTrust(_ context.Context, id, orgID string) (*domain.FederationTrust, error) {
	for _, trust := range m.trusts {
		if trust.ID == id && trust.OrganizationID == orgID {
			return trust, nil
		}
	}
	return nil, nil
}
func (m *mockRepo) FindActiveTrustsByIssuer(_ context.Context, issuer string) ([]*domain.FederationTrust, error) {
	items := []*domain.FederationTrust{}
	for _, trust := range m.trusts {
		if trust.Issuer == issuer && trust.Status == domain.TrustStatusActive {
			items = append(items, trust)
		}
	}
	return items, nil
}
func (m *mockRepo) CreateTrust(_ context.Context, trust *domain.FederationTrust) error {
	if m.createTrustErr != nil {
		return m.createTrustErr
	}
	m.trusts = append(m.trusts, trust)
	return nil
}
func (m *mockRepo) SetTrustStatus(_ context.Context, id, orgID string, status domain.FederationTrustStatus) error {
	trust, _ := m.GetTrust(context.Background(), id, orgID)
	if trust == nil || trust.Status == domain.TrustStatusDeleted {
		return apperrors.NewConflict("trust status conflict")
	}
	trust.Status = status
	return nil
}
func (m *mockRepo) DeleteTrust(_ context.Context, id, orgID string) error {
	trust, _ := m.GetTrust(context.Background(), id, orgID)
	if trust == nil || trust.Status == domain.TrustStatusDeleted {
		return apperrors.NewConflict("trust delete conflict")
	}
	trust.Status = domain.TrustStatusDeleted
	return nil
}
func (m *mockRepo) ListAccounts(_ context.Context, q domain.ListAccountsQuery) ([]*domain.ServiceAccount, int64, error) {
	return m.accounts, int64(len(m.accounts)), nil
}
func (m *mockRepo) findAccount(id, orgID string) *domain.ServiceAccount {
	for _, account := range m.accounts {
		if account.ID == id && account.OrganizationID == orgID {
			return account
		}
	}
	return nil
}
func cloneAccount(account *domain.ServiceAccount) *domain.ServiceAccount {
	if account == nil {
		return nil
	}
	cloned := *account
	cloned.Scopes = append([]string{}, account.Scopes...)
	if account.LastRotatedAt != nil {
		rotatedAt := *account.LastRotatedAt
		cloned.LastRotatedAt = &rotatedAt
	}
	return &cloned
}
func (m *mockRepo) GetAccount(_ context.Context, id, orgID string) (*domain.ServiceAccount, error) {
	return cloneAccount(m.findAccount(id, orgID)), nil
}
func (m *mockRepo) GetAccountByClientID(_ context.Context, clientID string) (*domain.ServiceAccount, error) {
	for _, account := range m.accounts {
		if account.ClientID == clientID {
			return cloneAccount(account), nil
		}
	}
	return nil, nil
}
func (m *mockRepo) GetAccountByName(_ context.Context, orgID, name string) (*domain.ServiceAccount, error) {
	for _, account := range m.accounts {
		if account.OrganizationID == orgID && account.Name == name {
			return cloneAccount(account), nil
		}
	}
	return nil, nil
}
func (m *mockRepo) CreateAccount(_ context.Context, account *domain.ServiceAccount) error {
	if m.createAccountErr != nil {
		return m.createAccountErr
	}
	m.accounts = append(m.accounts, account)
	return nil
}
func (m *mockRepo) SetAccountStatus(_ context.Context, id, orgID string, status domain.ServiceAccountStatus, revision int64) error {
	account := m.findAccount(id, orgID)
	if account == nil || account.Revision != revision || account.Status == domain.SAStatusDeleted {
		return apperrors.NewConflict("service account revision conflict")
	}
	account.Status = status
	account.Revision++
	return nil
}
func (m *mockRepo) RotateAccountSecret(_ context.Context, id, orgID, hash string, rotatedAt time.Time, revision int64) error {
	account := m.findAccount(id, orgID)
	if account == nil || account.Revision != revision || account.Status == domain.SAStatusDeleted {
		return apperrors.NewConflict("service account revision conflict")
	}
	account.ClientSecretHash = hash
	account.LastRotatedAt = &rotatedAt
	account.Revision++
	return nil
}
func (m *mockRepo) DeleteAccount(_ context.Context, id, orgID string, revision int64) error {
	account := m.findAccount(id, orgID)
	if account == nil || account.Revision != revision || account.Status != domain.SAStatusDisabled {
		return apperrors.NewConflict("service account revision conflict")
	}
	m.deleteAccountRev = revision
	account.Status = domain.SAStatusDeleted
	account.Revision++
	return nil
}

type mockTokenRepo struct {
	activeByUser map[string]int64
	revokedUsers []string
}

func (m *mockTokenRepo) GetByID(context.Context, string) (*tokendomain.Token, error) {
	return nil, nil
}
func (m *mockTokenRepo) GetByIDForOrganization(context.Context, string, string) (*tokendomain.Token, error) {
	return nil, nil
}
func (m *mockTokenRepo) GetByTokenHash(context.Context, string) (*tokendomain.Token, error) {
	return nil, nil
}
func (m *mockTokenRepo) GetByDeviceSecretHash(context.Context, string) (*tokendomain.Token, error) {
	return nil, nil
}
func (m *mockTokenRepo) GetRefreshByTokenHash(context.Context, string) (*tokendomain.RefreshToken, error) {
	return nil, nil
}
func (m *mockTokenRepo) List(_ context.Context, q tokendomain.ListQuery) ([]*tokendomain.Token, int64, error) {
	return nil, m.activeByUser[q.UserID], nil
}
func (m *mockTokenRepo) Create(context.Context, *tokendomain.Token) error { return nil }
func (m *mockTokenRepo) UpdateDeviceSecret(context.Context, string, string, int) error {
	return nil
}
func (m *mockTokenRepo) CreateRefresh(context.Context, *tokendomain.RefreshToken) error {
	return nil
}
func (m *mockTokenRepo) Revoke(context.Context, string, time.Time) error { return nil }
func (m *mockTokenRepo) RevokeRefresh(context.Context, string, time.Time) error {
	return nil
}
func (m *mockTokenRepo) RevokeByQuery(_ context.Context, q tokendomain.RevokeQuery, _ time.Time) (int64, error) {
	count := m.activeByUser[q.UserID]
	m.activeByUser[q.UserID] = 0
	m.revokedUsers = append(m.revokedUsers, q.UserID)
	return count, nil
}
func (m *mockTokenRepo) RevokeByFamilyID(context.Context, string, time.Time) (int64, error) {
	return 0, nil
}
func (m *mockTokenRepo) RevokeRefreshByUser(context.Context, string, time.Time) (int64, error) {
	return 0, nil
}
func (m *mockTokenRepo) RevokeByAuthorizationCodeID(context.Context, string, time.Time) (int64, error) {
	return 0, nil
}
func (m *mockTokenRepo) Delete(context.Context, string) error { return nil }

type mockRecordsRepo struct{ records []*domainrecords.Record }

func (m *mockRecordsRepo) Create(_ context.Context, record *domainrecords.Record) error {
	m.records = append(m.records, record)
	return nil
}
func (m *mockRecordsRepo) List(context.Context, domainrecords.ListQuery) ([]*domainrecords.Record, int64, error) {
	return nil, 0, nil
}
func (m *mockRecordsRepo) Search(context.Context, domainrecords.SearchCriteria) ([]*domainrecords.Record, int64, error) {
	return nil, 0, nil
}
func (m *mockRecordsRepo) LoadLastChainEntry(context.Context, string) (seq int64, hash []byte, err error) {
	return 0, nil, nil
}
func (m *mockRecordsRepo) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}
func (m *mockRecordsRepo) InsertAnchor(context.Context, *domainrecords.ChainAnchor) error {
	return nil
}
func (m *mockRecordsRepo) ListChainEntries(context.Context, string) ([]domainrecords.ChainEntry, error) {
	return nil, nil
}

type mockOutboxRepo struct{ events []*domainoutbox.OutboxEvent }

func (m *mockOutboxRepo) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	m.events = append(m.events, event)
	return nil
}
func (m *mockOutboxRepo) EnqueueBatch(_ context.Context, events []*domainoutbox.OutboxEvent) error {
	m.events = append(m.events, events...)
	return nil
}

type mockUoW struct {
	appuow.UnitOfWork
	repo         *mockRepo
	tokens       *mockTokenRepo
	records      *mockRecordsRepo
	outbox       *mockOutboxRepo
	transactions int
}

func (u *mockUoW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	u.transactions++
	return fn(ctx, u)
}
func (u *mockUoW) Workload() domain.Repository       { return u.repo }
func (u *mockUoW) Tokens() tokendomain.Repository    { return u.tokens }
func (u *mockUoW) Records() domainrecords.Repository { return u.records }
func (u *mockUoW) Outbox() domainoutbox.Repository   { return u.outbox }

func newSvc(repo *mockRepo) (Service, *mockUoW) {
	uow := &mockUoW{repo: repo, tokens: &mockTokenRepo{activeByUser: map[string]int64{}}, records: &mockRecordsRepo{}, outbox: &mockOutboxRepo{}}
	svc := NewService(uow)
	svc.Clock = fixedClock{}
	return svc, uow
}

func poolFixture(orgID string) *domain.Pool {
	return &domain.Pool{ID: "pool", OrganizationID: orgID, Name: "pool", CreatedAt: testNow, UpdatedAt: testNow, Revision: 1}
}

func validTrustCommand() CreateTrustCommand {
	return CreateTrustCommand{OrganizationID: "org", PoolID: "pool", Name: "trust", Issuer: "https://issuer.example.com", JWKSURI: "https://issuer.example.com/jwks.json", Audience: []string{"api"}, Subjects: []string{"repo:org/*"}}
}

func accountFixture() *domain.ServiceAccount {
	return &domain.ServiceAccount{ID: "sa", OrganizationID: "org", PoolID: "pool", Name: "robot", Description: "CI", ClientID: "gl_cid_existing", ClientSecretHash: "old-hash", Scopes: []string{"orders:read"}, AccessTokenTTLSeconds: 900, Status: domain.SAStatusActive, CreatedAt: testNow, UpdatedAt: testNow, Revision: 1}
}

func TestCreatePoolUsesTransaction(t *testing.T) {
	repo := &mockRepo{}
	svc, uow := newSvc(repo)
	dto, err := svc.CreatePool(context.Background(), CreatePoolCommand{OrganizationID: "org", Name: "pool", DisplayName: "Pool"})
	if err != nil {
		t.Fatal(err)
	}
	if dto.ID == "" || uow.transactions != 1 || len(repo.pools) != 1 {
		t.Fatalf("pool create did not use transaction: dto=%+v transactions=%d", dto, uow.transactions)
	}
}

func TestCreateTrustValidationMatrix(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CreateTrustCommand)
	}{
		{name: "organization", mutate: func(c *CreateTrustCommand) { c.OrganizationID = "" }},
		{name: "pool", mutate: func(c *CreateTrustCommand) { c.PoolID = "" }},
		{name: "name", mutate: func(c *CreateTrustCommand) { c.Name = "" }},
		{name: "issuer required", mutate: func(c *CreateTrustCommand) { c.Issuer = "" }},
		{name: "issuer https", mutate: func(c *CreateTrustCommand) { c.Issuer = "http://issuer.example.com" }},
		{name: "jwks required", mutate: func(c *CreateTrustCommand) { c.JWKSURI = "" }},
		{name: "jwks https", mutate: func(c *CreateTrustCommand) { c.JWKSURI = "http://issuer.example.com/jwks" }},
		{name: "audience required", mutate: func(c *CreateTrustCommand) { c.Audience = nil }},
		{name: "audience item", mutate: func(c *CreateTrustCommand) { c.Audience = []string{" "} }},
		{name: "subjects required", mutate: func(c *CreateTrustCommand) { c.Subjects = nil }},
		{name: "subject wildcard", mutate: func(c *CreateTrustCommand) { c.Subjects = []string{"*"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &mockRepo{pools: []*domain.Pool{poolFixture("org")}}
			svc, _ := newSvc(repo)
			cmd := validTrustCommand()
			test.mutate(&cmd)
			if _, err := svc.CreateTrust(context.Background(), cmd); err == nil {
				t.Fatal("expected validation error")
			}
			if len(repo.trusts) != 0 {
				t.Fatal("invalid trust must not be persisted")
			}
		})
	}
}

func TestCreateTrustSuccessAndAuditOutbox(t *testing.T) {
	repo := &mockRepo{pools: []*domain.Pool{poolFixture("org")}}
	svc, uow := newSvc(repo)
	dto, err := svc.CreateTrust(context.Background(), validTrustCommand())
	if err != nil {
		t.Fatal(err)
	}
	if dto.JWKSURI != "https://issuer.example.com/jwks.json" || len(dto.Audience) != 1 || len(dto.Subjects) != 1 {
		t.Fatalf("unexpected trust DTO: %+v", dto)
	}
	if len(uow.records.records) != 1 || len(uow.outbox.events) != 1 {
		t.Fatalf("audit/outbox missing: records=%d events=%d", len(uow.records.records), len(uow.outbox.events))
	}
	if uow.outbox.events[0].EventType != "federation_trust.created" {
		t.Fatalf("event type = %q", uow.outbox.events[0].EventType)
	}
}

func TestCreateTrustRejectsPoolFromAnotherOrganization(t *testing.T) {
	repo := &mockRepo{pools: []*domain.Pool{poolFixture("other-org")}}
	svc, _ := newSvc(repo)
	if _, err := svc.CreateTrust(context.Background(), validTrustCommand()); err == nil {
		t.Fatal("cross-organization pool must be rejected")
	}
}

func TestPreviewSubjectMatches(t *testing.T) {
	svc, _ := newSvc(&mockRepo{})
	matches, err := svc.PreviewSubjectMatches([]string{"repo:org/*", "exact"}, []string{"repo:org/app", "other", "exact"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 3 || !matches[0].Allowed || matches[1].Allowed || !matches[2].Allowed {
		t.Fatalf("unexpected matches: %+v", matches)
	}
	if _, err := svc.PreviewSubjectMatches([]string{"*"}, []string{"anything"}); err == nil {
		t.Fatal("invalid preview patterns must be rejected")
	}
}

func TestAccessTokenTTLValidation(t *testing.T) {
	for _, test := range []struct {
		input int
		want  int
		valid bool
	}{{0, 900, true}, {60, 60, true}, {3600, 3600, true}, {59, 0, false}, {3601, 0, false}, {-1, 0, false}} {
		got, err := validateAccessTokenTTL(test.input)
		if (err == nil) != test.valid || got != test.want {
			t.Fatalf("validateAccessTokenTTL(%d) = %d, %v", test.input, got, err)
		}
	}
}

func TestCreateAccountDefaultsSecretAndAudit(t *testing.T) {
	repo := &mockRepo{pools: []*domain.Pool{poolFixture("org")}}
	svc, uow := newSvc(repo)
	dto, err := svc.CreateAccount(context.Background(), CreateAccountCommand{OrganizationID: "org", PoolID: "pool", Name: "robot", Description: "CI bot"})
	if err != nil {
		t.Fatal(err)
	}
	parsedID, err := uuid.Parse(dto.ID)
	if err != nil || parsedID.Version() != 7 {
		t.Fatalf("service account ID must be UUIDv7: %q, %v", dto.ID, err)
	}
	if !strings.HasPrefix(dto.ClientID, serviceAccountIDPrefix) || len(dto.ClientID) != len(serviceAccountIDPrefix)+32 {
		t.Fatalf("unexpected client ID: %q", dto.ClientID)
	}
	if !strings.HasPrefix(dto.ClientSecretPlain, serviceAccountSecretPrefix) || dto.AccessTokenTTLSeconds != 900 || len(dto.Scopes) != 0 || dto.LastRotatedAt != nil || !dto.SecretSet {
		t.Fatalf("unexpected account DTO: %+v", dto)
	}
	stored := repo.accounts[0]
	if !strings.HasPrefix(stored.ClientSecretHash, "$argon2id$") || stored.ClientSecretHash == dto.ClientSecretPlain {
		t.Fatal("only an Argon2id secret hash may be persisted")
	}
	verified, err := platformcrypto.VerifyClientSecret(dto.ClientSecretPlain, stored.ClientSecretHash)
	if err != nil || !verified {
		t.Fatalf("stored hash does not verify one-time secret: %v", err)
	}
	if len(uow.records.records) != 1 || len(uow.outbox.events) != 1 {
		t.Fatal("create audit/outbox missing")
	}
	payload := string(uow.outbox.events[0].Payload)
	if strings.Contains(payload, dto.ClientSecretPlain) || strings.Contains(payload, stored.ClientSecretHash) || strings.Contains(payload, "clientSecretHash") || strings.Contains(payload, "clientSecretPlain") {
		t.Fatal("audit/outbox payload must not contain secret material")
	}
}

func TestAccountSecretCannotBeReadAgain(t *testing.T) {
	account := accountFixture()
	repo := &mockRepo{accounts: []*domain.ServiceAccount{account}}
	svc, _ := newSvc(repo)
	got, err := svc.GetAccount(context.Background(), "org", "pool", "sa")
	if err != nil {
		t.Fatal(err)
	}
	if got.ClientSecretPlain != "" || !got.SecretSet {
		t.Fatalf("get must not return plaintext: %+v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "clientSecretPlain") || strings.Contains(string(encoded), "old-hash") || strings.Contains(string(encoded), "clientSecretHash") {
		t.Fatalf("secret material leaked in DTO: %s", encoded)
	}
}

func TestRotateAccountSecretKeepsClientIDAndRevokesTokens(t *testing.T) {
	account := accountFixture()
	oldHash := account.ClientSecretHash
	repo := &mockRepo{accounts: []*domain.ServiceAccount{account}}
	svc, uow := newSvc(repo)
	uow.tokens.activeByUser["sa"] = 2
	dto, err := svc.RotateAccountSecret(context.Background(), RotateAccountSecretCommand{OrganizationID: "org", PoolID: "pool", AccountID: "sa", Revision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if dto.ClientID != account.ClientID || dto.ClientSecretPlain == "" || dto.ClientSecretPlain == oldHash || dto.LastRotatedAt == nil || dto.Revision != 2 {
		t.Fatalf("unexpected rotation DTO: %+v", dto)
	}
	verified, err := platformcrypto.VerifyClientSecret(dto.ClientSecretPlain, account.ClientSecretHash)
	if err != nil || !verified || account.ClientSecretHash == oldHash {
		t.Fatalf("new secret hash was not persisted: %v", err)
	}
	if len(uow.tokens.revokedUsers) != 1 || uow.tokens.revokedUsers[0] != "sa" || uow.tokens.activeByUser["sa"] != 0 {
		t.Fatalf("rotation did not revoke account tokens: %+v", uow.tokens.revokedUsers)
	}
	if len(uow.outbox.events) != 1 || strings.Contains(string(uow.outbox.events[0].Payload), dto.ClientSecretPlain) {
		t.Fatal("rotation audit/outbox invalid")
	}
}

func TestSetAccountStatusRevisionAndDeletedGuard(t *testing.T) {
	account := accountFixture()
	repo := &mockRepo{accounts: []*domain.ServiceAccount{account}}
	svc, uow := newSvc(repo)
	uow.tokens.activeByUser["sa"] = 1
	dto, err := svc.SetAccountStatus(context.Background(), SetAccountStatusCommand{OrganizationID: "org", PoolID: "pool", AccountID: "sa", Status: domain.SAStatusDisabled, Revision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if dto.Status != "disabled" || dto.Revision != 2 || uow.tokens.activeByUser["sa"] != 0 {
		t.Fatalf("disable failed: %+v", dto)
	}
	enabled, err := svc.SetAccountStatus(context.Background(), SetAccountStatusCommand{OrganizationID: "org", PoolID: "pool", AccountID: "sa", Status: domain.SAStatusActive, Revision: 2})
	if err != nil || enabled.Status != "active" || enabled.Revision != 3 {
		t.Fatalf("enable failed: dto=%+v err=%v", enabled, err)
	}
	if len(uow.tokens.revokedUsers) != 1 {
		t.Fatalf("enabling an account must not revoke tokens again: %v", uow.tokens.revokedUsers)
	}
	if _, err := svc.SetAccountStatus(context.Background(), SetAccountStatusCommand{OrganizationID: "org", PoolID: "pool", AccountID: "sa", Status: domain.SAStatusActive, Revision: 1}); apperrors.FromError(err).Code != apperrors.CodeConflict {
		t.Fatalf("stale revision must conflict: %v", err)
	}
	account.Status = domain.SAStatusDeleted
	if _, err := svc.SetAccountStatus(context.Background(), SetAccountStatusCommand{OrganizationID: "org", PoolID: "pool", AccountID: "sa", Status: domain.SAStatusActive, Revision: 3}); err == nil || !strings.Contains(err.Error(), "deleted") {
		t.Fatalf("deleted service account must not be revived: %v", err)
	}
}

func TestAccountDeletePrecheckAndDeleteLifecycle(t *testing.T) {
	account := accountFixture()
	repo := &mockRepo{accounts: []*domain.ServiceAccount{account}}
	svc, uow := newSvc(repo)
	uow.tokens.activeByUser["sa"] = 2
	precheck, err := svc.GetAccountDeletePrecheck(context.Background(), "org", "pool", "sa")
	if err != nil {
		t.Fatal(err)
	}
	if precheck.ActiveTokenCount != 2 || precheck.CanDelete || precheck.Status != "active" || len(precheck.Scopes) != 1 {
		t.Fatalf("unexpected precheck: %+v", precheck)
	}
	deleteCmd := DeleteAccountCommand{OrganizationID: "org", PoolID: "pool", AccountID: "sa", Revision: 1}
	if err := svc.DeleteAccount(context.Background(), deleteCmd); apperrors.FromError(err).Code != apperrors.CodeConflict {
		t.Fatalf("active tokens must block deletion: %v", err)
	}
	if account.Status != domain.SAStatusActive {
		t.Fatal("rejected deletion must not disable account")
	}
	if _, err := svc.SetAccountStatus(context.Background(), SetAccountStatusCommand{OrganizationID: "org", PoolID: "pool", AccountID: "sa", Status: domain.SAStatusDisabled, Revision: 1}); err != nil {
		t.Fatal(err)
	}
	precheck, err = svc.GetAccountDeletePrecheck(context.Background(), "org", "pool", "sa")
	if err != nil || !precheck.CanDelete || precheck.ActiveTokenCount != 0 {
		t.Fatalf("disabled account should be deletable: %+v, %v", precheck, err)
	}
	deleteCmd.Revision = 2
	if err := svc.DeleteAccount(context.Background(), deleteCmd); err != nil {
		t.Fatal(err)
	}
	if account.Status != domain.SAStatusDeleted || account.Revision != 4 || repo.deleteAccountRev != 3 {
		t.Fatalf("delete must disable then delete with CAS revisions: %+v", account)
	}
	if len(uow.tokens.revokedUsers) != 3 {
		t.Fatalf("disable and delete token revocations missing: %v", uow.tokens.revokedUsers)
	}
	if len(uow.outbox.events) != 2 || uow.outbox.events[1].EventType != "service_account.deleted" {
		t.Fatalf("status/delete audit events missing: %+v", uow.outbox.events)
	}
}

func TestCreateAccountRepositoryErrorDoesNotReturnSecret(t *testing.T) {
	repo := &mockRepo{pools: []*domain.Pool{poolFixture("org")}, createAccountErr: errors.New("db failed")}
	svc, _ := newSvc(repo)
	dto, err := svc.CreateAccount(context.Background(), CreateAccountCommand{OrganizationID: "org", PoolID: "pool", Name: "robot"})
	if err == nil || dto != nil {
		t.Fatalf("failed create must not return credentials: dto=%+v err=%v", dto, err)
	}
}

func TestTrustStatusAndDeleteLifecycle(t *testing.T) {
	trust := &domain.FederationTrust{ID: "trust", OrganizationID: "org", PoolID: "pool", Name: "trust", Issuer: "https://issuer.example.com", JWKSURI: "https://issuer.example.com/jwks", Audience: []string{"api"}, Subjects: []string{"repo:org/*"}, Status: domain.TrustStatusActive, CreatedAt: testNow, UpdatedAt: testNow, Revision: 1}
	repo := &mockRepo{trusts: []*domain.FederationTrust{trust}}
	svc, uow := newSvc(repo)
	disabled, err := svc.SetTrustStatus(context.Background(), SetTrustStatusCommand{OrganizationID: "org", PoolID: "pool", TrustID: "trust", Status: domain.TrustStatusDisabled})
	if err != nil || disabled.Status != "disabled" {
		t.Fatalf("disable trust failed: %+v, %v", disabled, err)
	}
	enabled, err := svc.SetTrustStatus(context.Background(), SetTrustStatusCommand{OrganizationID: "org", PoolID: "pool", TrustID: "trust", Status: domain.TrustStatusActive})
	if err != nil || enabled.Status != "active" {
		t.Fatalf("enable trust failed: %+v, %v", enabled, err)
	}
	if err := svc.DeleteTrust(context.Background(), DeleteTrustCommand{OrganizationID: "org", PoolID: "pool", TrustID: "trust"}); err != nil {
		t.Fatal(err)
	}
	if trust.Status != domain.TrustStatusDeleted || len(uow.outbox.events) != 3 {
		t.Fatalf("trust lifecycle or audit failed: status=%s events=%d", trust.Status, len(uow.outbox.events))
	}
	if _, err := svc.SetTrustStatus(context.Background(), SetTrustStatusCommand{OrganizationID: "org", PoolID: "pool", TrustID: "trust", Status: domain.TrustStatusActive}); err == nil {
		t.Fatal("deleted trust must not be revived")
	}
}

func TestDTOJSONUsesCamelCaseAndOmitsSecret(t *testing.T) {
	trust := TrustDTO{ID: "id", OrganizationID: "org", PoolID: "pool", JWKSURI: "https://issuer.example.com/jwks", Audience: []string{}, Subjects: []string{}}
	encoded, err := json.Marshal(trust)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"id", "organizationId", "poolId", "jwksUri", "audience", "subjects", "createdAt", "updatedAt", "revision"} {
		if _, ok := fields[field]; !ok {
			t.Fatalf("camelCase field %s missing from %s", field, encoded)
		}
	}
	if _, ok := fields["issuerUrl"]; ok {
		t.Fatal("issuerUrl alias is forbidden")
	}
	account := AccountDTO{ID: "id", OrganizationID: "org", PoolID: "pool", ClientID: "client", Scopes: []string{}, SecretSet: true}
	encoded, err = json.Marshal(account)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "clientSecretPlain") || strings.Contains(string(encoded), "clientSecretHash") || strings.Contains(string(encoded), "accountId") {
		t.Fatalf("account DTO leaked forbidden field: %s", encoded)
	}
}

func TestWorkloadValidation_Answers422(t *testing.T) {
	svc := Service{Clock: fixedClock{}}
	ctx := context.Background()

	checks := []struct {
		name string
		err  error
	}{
		{"issuer", func() error {
			_, err := svc.CreateTrust(ctx, CreateTrustCommand{OrganizationID: "org", PoolID: "pool", Name: "trust", Issuer: "http://insecure", JWKSURI: "https://issuer.example.com/jwks", Audience: []string{"api"}, Subjects: []string{"repo:org/*"}})
			return err
		}()},
		{"jwks_uri", func() error {
			_, err := svc.CreateTrust(ctx, CreateTrustCommand{OrganizationID: "org", PoolID: "pool", Name: "trust", Issuer: "https://issuer.example.com", JWKSURI: "http://insecure", Audience: []string{"api"}, Subjects: []string{"repo:org/*"}})
			return err
		}()},
		{"audience_empty", func() error {
			_, err := svc.CreateTrust(ctx, CreateTrustCommand{OrganizationID: "org", PoolID: "pool", Name: "trust", Issuer: "https://issuer.example.com", JWKSURI: "https://issuer.example.com/jwks", Audience: nil, Subjects: []string{"repo:org/*"}})
			return err
		}()},
		{"trust_status", func() error {
			_, err := svc.SetTrustStatus(ctx, SetTrustStatusCommand{OrganizationID: "org", PoolID: "pool", TrustID: "trust", Status: "paused"})
			return err
		}()},
		{"account_status", func() error {
			_, err := svc.SetAccountStatus(ctx, SetAccountStatusCommand{OrganizationID: "org", PoolID: "pool", AccountID: "acc", Revision: 1, Status: "paused"})
			return err
		}()},
		{"revision", validateAccountMutation("org", "pool", "acc", 0)},
		{"ttl_low", func() error { _, err := validateAccessTokenTTL(30); return err }()},
		{"ttl_high", func() error { _, err := validateAccessTokenTTL(5000); return err }()},
		{"values_empty", validateNonEmptyValues("audience", nil)},
		{"value_blank", validateNonEmptyValuesAllowEmpty("audience", []string{" "})},
	}
	for _, tt := range checks {
		t.Run(tt.name, func(t *testing.T) {
			appErr := apperrors.FromError(tt.err)
			if appErr.Code != apperrors.CodeUnprocessableEntity {
				t.Fatalf("error code = %q, want 422 (err=%v)", appErr.Code, tt.err)
			}
			if appErr.Meta["field"] == nil {
				t.Fatalf("meta = %v, want field", appErr.Meta)
			}
		})
	}
}
