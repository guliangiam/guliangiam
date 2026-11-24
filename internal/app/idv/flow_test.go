package idv

import (
	"context"
	"sync"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/idv"
	domainusers "github.com/guliangiam/guliangiam/internal/domain/users"
	usermock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"go.uber.org/mock/gomock"
)

type flowRepo struct {
	mu      sync.Mutex
	items   map[string]*domain.Verification
	byRef   map[string]*domain.Verification
	creates int
	updates int
}

func newFlowRepo() *flowRepo {
	return &flowRepo{items: map[string]*domain.Verification{}, byRef: map[string]*domain.Verification{}}
}

func (r *flowRepo) Create(_ context.Context, v *domain.Verification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *v
	r.items[v.ID] = &cp
	if v.ProviderRef != nil {
		r.byRef[v.Provider+"|"+*v.ProviderRef] = &cp
	}
	r.creates++
	return nil
}

func (r *flowRepo) Update(_ context.Context, v *domain.Verification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[v.ID]; !ok {
		return apperrors.NewNotFound("verification not found")
	}
	cp := *v
	r.items[v.ID] = &cp
	r.updates++
	return nil
}

func (r *flowRepo) GetByID(_ context.Context, id, organizationID string) (*domain.Verification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.items[id]
	if !ok || v.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("verification not found")
	}
	cp := *v
	return &cp, nil
}

func (r *flowRepo) GetByProviderRef(_ context.Context, provider, providerRef string) (*domain.Verification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.byRef[provider+"|"+providerRef]
	if !ok {
		return nil, apperrors.NewNotFound("verification not found")
	}
	cp := *v
	return &cp, nil
}

func (r *flowRepo) List(context.Context, domain.ListQuery) ([]*domain.Verification, int64, error) {
	return nil, 0, nil
}

func (r *flowRepo) Delete(_ context.Context, id, organizationID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.items[id]
	if !ok || v.OrganizationID != organizationID {
		return apperrors.NewNotFound("verification not found")
	}
	delete(r.items, id)
	return nil
}

func (r *flowRepo) snapshot(id string) domain.Verification {
	r.mu.Lock()
	defer r.mu.Unlock()
	return *r.items[id]
}

type flowResolver struct {
	credentials domain.ProviderCredentials
	err         error
}

func (r flowResolver) ResolveIDVProvider(context.Context, string, string) (*domain.ProviderCredentials, error) {
	if r.err != nil {
		return nil, r.err
	}
	cp := r.credentials
	return &cp, nil
}

type flowVerifier struct {
	initiate    *domain.InitiateResult
	initiateErr error
	result      *domain.VerificationResult
	resultErr   error
	callback    *domain.VerificationResult
	callbackErr error
}

func (v *flowVerifier) InitiateVerification(context.Context, domain.InitiateOptions) (*domain.InitiateResult, error) {
	return v.initiate, v.initiateErr
}
func (v *flowVerifier) GetVerificationResult(context.Context, string) (*domain.VerificationResult, error) {
	return v.result, v.resultErr
}
func (v *flowVerifier) VerifyCallback(context.Context, []byte, map[string]string) (*domain.VerificationResult, error) {
	return v.callback, v.callbackErr
}
func (v *flowVerifier) Probe(context.Context) error { return nil }

type flowFactory struct {
	verifier  *flowVerifier
	reference string
	refErr    error
}

func (f flowFactory) Build(domain.ProviderCredentials) (domain.IdentityVerifier, error) {
	return f.verifier, nil
}

func (f flowFactory) CallbackReference(string, []byte) (string, error) {
	if f.refErr != nil {
		return "", f.refErr
	}
	if f.reference == "" {
		return "", apperrors.NewNotFound("verification not found")
	}
	return f.reference, nil
}

func newFlowService(t *testing.T, repo *flowRepo, resolver IDVProviderResolver, verifier *flowVerifier) Service {
	t.Helper()
	return newFlowServiceWithFactory(t, repo, resolver, flowFactory{verifier: verifier})
}

func newFlowServiceWithFactory(t *testing.T, repo *flowRepo, resolver IDVProviderResolver, factory domain.VerifierFactory) Service {
	t.Helper()
	ctrl := gomock.NewController(t)
	uow := mock.NewMockUnitOfWork(ctrl)
	uow.EXPECT().IDV().Return(repo).AnyTimes()
	userRepo := usermock.NewMockRepository(ctrl)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id, _ string) (*domainusers.User, error) {
			return &domainusers.User{ID: id}, nil
		}).AnyTimes()
	userRepo.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	uow.EXPECT().Users().Return(userRepo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			tx := mock.NewMockUnitOfWork(ctrl)
			tx.EXPECT().IDV().Return(repo).AnyTimes()
			tx.EXPECT().Users().Return(userRepo).AnyTimes()
			return fn(ctx, tx)
		},
	).AnyTimes()

	fixed := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	return Service{
		UOW:       uow,
		Clock:     func() time.Time { return fixed },
		Providers: resolver,
		Factory:   factory,
	}
}

func TestInitiatePersistsPendingVerification(t *testing.T) {
	repo := newFlowRepo()
	verifier := &flowVerifier{initiate: &domain.InitiateResult{
		VerificationID: "provider-ref-1",
		RedirectURL:    "https://provider.test/flow",
	}}
	svc := newFlowService(t, repo, flowResolver{credentials: domain.ProviderCredentials{Provider: "jumio"}}, verifier)

	outcome, err := svc.Initiate(context.Background(), InitiateCommand{
		OrganizationID: "org-1", UserID: stringPtr("user-1"), ReturnURL: "https://iam.test/return",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.Provider != "jumio" || outcome.RedirectURL != "https://provider.test/flow" {
		t.Errorf("unexpected outcome: %+v", outcome)
	}
	stored := repo.snapshot(outcome.VerificationID)
	if stored.Status != domain.StatusPending {
		t.Errorf("status = %s, want pending", stored.Status)
	}
	if stored.ProviderRef == nil || *stored.ProviderRef != "provider-ref-1" {
		t.Error("provider reference was not persisted")
	}
	if repo.creates != 1 {
		t.Errorf("expected one insert, got %d", repo.creates)
	}
}

func TestInitiateWithoutProviderIsRejected(t *testing.T) {
	if _, err := (Service{UOW: nil}).Initiate(context.Background(), InitiateCommand{OrganizationID: "org-1"}); err == nil {
		t.Fatal("expected Unavailable when no provider is wired")
	}
}

func TestRefreshAppliesApproval(t *testing.T) {
	repo := newFlowRepo()
	verifier := &flowVerifier{result: &domain.VerificationResult{
		VerificationID: "provider-ref-1",
		Status:         domain.StatusApproved,
		ProviderName:   "jumio",
		FirstName:      "Zhang",
		LastName:       "San",
	}}
	svc := newFlowService(t, repo, flowResolver{credentials: domain.ProviderCredentials{Provider: "jumio"}}, verifier)

	pending := &domain.Verification{
		ID: "ver-1", OrganizationID: "org-1", Provider: "jumio",
		Status: domain.StatusPending, ProviderRef: stringPtr("provider-ref-1"),
		InitiatedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Create(context.Background(), pending); err != nil {
		t.Fatal(err)
	}

	dto, err := svc.Refresh(context.Background(), "ver-1", "org-1", "admin-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Status != string(domain.StatusApproved) {
		t.Errorf("status = %s, want approved", dto.Status)
	}
	if dto.CompletedAt == nil {
		t.Error("expected completedAt to be set")
	}
}

func TestRefreshKeepsPendingOnProviderOutage(t *testing.T) {
	repo := newFlowRepo()
	verifier := &flowVerifier{resultErr: apperrors.NewUnavailable("provider down")}
	svc := newFlowService(t, repo, flowResolver{credentials: domain.ProviderCredentials{Provider: "jumio"}}, verifier)

	pending := &domain.Verification{
		ID: "ver-2", OrganizationID: "org-1", Provider: "jumio",
		Status: domain.StatusPending, ProviderRef: stringPtr("provider-ref-2"),
		InitiatedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Create(context.Background(), pending); err != nil {
		t.Fatal(err)
	}

	dto, err := svc.Refresh(context.Background(), "ver-2", "org-1", "admin-1")
	if err != nil {
		t.Fatalf("refresh must be best-effort, got %v", err)
	}
	if dto.Status != string(domain.StatusPending) {
		t.Errorf("status = %s, want pending", dto.Status)
	}
	if repo.updates != 0 {
		t.Error("provider outage must not rewrite the record")
	}
}

func TestRefreshIsIdempotentForTerminalRecords(t *testing.T) {
	repo := newFlowRepo()
	verifier := &flowVerifier{result: &domain.VerificationResult{
		VerificationID: "provider-ref-3", Status: domain.StatusRejected, ProviderName: "jumio",
	}}
	svc := newFlowService(t, repo, flowResolver{credentials: domain.ProviderCredentials{Provider: "jumio"}}, verifier)

	approved := &domain.Verification{
		ID: "ver-3", OrganizationID: "org-1", Provider: "jumio",
		Status: domain.StatusApproved, ProviderRef: stringPtr("provider-ref-3"),
		InitiatedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Create(context.Background(), approved); err != nil {
		t.Fatal(err)
	}

	dto, err := svc.Refresh(context.Background(), "ver-3", "org-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Status != string(domain.StatusApproved) {
		t.Errorf("terminal status must not be overwritten, got %s", dto.Status)
	}
	if repo.updates != 0 {
		t.Error("terminal record must not be updated")
	}
}

func TestHandleCallbackUpdatesByIdempotently(t *testing.T) {
	repo := newFlowRepo()
	verifier := &flowVerifier{callback: &domain.VerificationResult{
		VerificationID: "provider-ref-4", Status: domain.StatusRejected, ProviderName: "jumio",
	}}
	svc := newFlowService(t, repo, flowResolver{credentials: domain.ProviderCredentials{Provider: "jumio"}}, verifier)

	pending := &domain.Verification{
		ID: "ver-4", OrganizationID: "org-1", Provider: "jumio",
		Status: domain.StatusPending, ProviderRef: stringPtr("provider-ref-4"),
		InitiatedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Create(context.Background(), pending); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.HandleCallback(context.Background(), "org-1", "jumio", []byte(`{}`), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := repo.snapshot("ver-4").Status; got != domain.StatusRejected {
		t.Errorf("status = %s, want rejected", got)
	}

	// A duplicate callback must not update again.
	if _, err := svc.HandleCallback(context.Background(), "org-1", "jumio", []byte(`{}`), nil); err != nil {
		t.Fatalf("unexpected error on duplicate callback: %v", err)
	}
	if repo.updates != 1 {
		t.Errorf("expected exactly one update, got %d", repo.updates)
	}
}

func TestCompleteRejectsCrossTenantAccess(t *testing.T) {
	repo := newFlowRepo()
	svc := newFlowService(t, repo, flowResolver{credentials: domain.ProviderCredentials{Provider: "jumio"}}, &flowVerifier{})
	v := &domain.Verification{ID: "ver-5", OrganizationID: "org-1", Status: domain.StatusPending}
	if err := repo.Create(context.Background(), v); err != nil {
		t.Fatal(err)
	}

	_, err := svc.Complete(context.Background(), CompleteCommand{
		ID: "ver-5", OrganizationID: "org-2", Status: domain.StatusApproved,
	})
	if err == nil {
		t.Fatal("expected cross-tenant access to be rejected")
	}
}

func TestCompleteRejectsNonPersistableStatus(t *testing.T) {
	_, err := (Service{}).Complete(context.Background(), CompleteCommand{ID: "v", OrganizationID: "o", Status: domain.StatusError})
	if err == nil {
		t.Fatal("expected status error to be rejected")
	}
}

func TestHandlePublicCallbackResolvesTenantAndAppliesResult(t *testing.T) {
	repo := newFlowRepo()
	verifier := &flowVerifier{callback: &domain.VerificationResult{
		VerificationID: "ref-pub-1", Status: domain.StatusApproved, ProviderName: "jumio",
	}}
	svc := newFlowServiceWithFactory(t, repo, flowResolver{credentials: domain.ProviderCredentials{Provider: "jumio"}},
		flowFactory{verifier: verifier, reference: "ref-pub-1"})

	pending := &domain.Verification{
		ID: "ver-pub-1", OrganizationID: "org-1", Provider: "jumio",
		Status: domain.StatusPending, ProviderRef: stringPtr("ref-pub-1"),
		InitiatedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Create(context.Background(), pending); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.HandlePublicCallback(context.Background(), "JUMIO", []byte(`{"status":"APPROVED_VERIFIED"}`), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := repo.snapshot("ver-pub-1").Status; got != domain.StatusApproved {
		t.Errorf("status = %s, want approved", got)
	}
	if repo.updates != 1 {
		t.Errorf("expected one update, got %d", repo.updates)
	}
}

func TestHandlePublicCallbackIsIdempotentForDuplicateDelivery(t *testing.T) {
	repo := newFlowRepo()
	verifier := &flowVerifier{callback: &domain.VerificationResult{
		VerificationID: "ref-pub-2", Status: domain.StatusRejected, ProviderName: "jumio",
	}}
	svc := newFlowServiceWithFactory(t, repo, flowResolver{credentials: domain.ProviderCredentials{Provider: "jumio"}},
		flowFactory{verifier: verifier, reference: "ref-pub-2"})

	pending := &domain.Verification{
		ID: "ver-pub-2", OrganizationID: "org-1", Provider: "jumio",
		Status: domain.StatusPending, ProviderRef: stringPtr("ref-pub-2"),
		InitiatedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Create(context.Background(), pending); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		if _, err := svc.HandlePublicCallback(context.Background(), "jumio", []byte(`{"status":"DENIED"}`), nil); err != nil {
			t.Fatalf("delivery %d: unexpected error: %v", i, err)
		}
	}
	if repo.updates != 1 {
		t.Errorf("duplicate delivery must produce exactly one update, got %d", repo.updates)
	}
}

func TestHandlePublicCallbackSkipsUnchangedStatus(t *testing.T) {
	repo := newFlowRepo()
	verifier := &flowVerifier{callback: &domain.VerificationResult{
		VerificationID: "ref-pub-3", Status: domain.StatusPending, ProviderName: "jumio",
	}}
	svc := newFlowServiceWithFactory(t, repo, flowResolver{credentials: domain.ProviderCredentials{Provider: "jumio"}},
		flowFactory{verifier: verifier, reference: "ref-pub-3"})

	pending := &domain.Verification{
		ID: "ver-pub-3", OrganizationID: "org-1", Provider: "jumio",
		Status: domain.StatusPending, ProviderRef: stringPtr("ref-pub-3"),
		InitiatedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Create(context.Background(), pending); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.HandlePublicCallback(context.Background(), "jumio", []byte(`{"status":"PENDING"}`), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.updates != 0 {
		t.Error("a callback that does not change the status must not write audit/outbox rows")
	}
}

func TestHandlePublicCallbackUnknownReferenceIsNotFound(t *testing.T) {
	repo := newFlowRepo()
	svc := newFlowServiceWithFactory(t, repo, flowResolver{credentials: domain.ProviderCredentials{Provider: "jumio"}},
		flowFactory{verifier: &flowVerifier{}, reference: "ref-does-not-exist"})

	_, err := svc.HandlePublicCallback(context.Background(), "jumio", []byte(`{}`), nil)
	if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestHandlePublicCallbackRejectsDisabledProvider(t *testing.T) {
	repo := newFlowRepo()
	svc := newFlowServiceWithFactory(t, repo, flowResolver{err: apperrors.NewNotFound("provider disabled")},
		flowFactory{verifier: &flowVerifier{}, reference: "ref-pub-4"})

	pending := &domain.Verification{
		ID: "ver-pub-4", OrganizationID: "org-1", Provider: "jumio",
		Status: domain.StatusPending, ProviderRef: stringPtr("ref-pub-4"),
		InitiatedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Create(context.Background(), pending); err != nil {
		t.Fatal(err)
	}

	_, err := svc.HandlePublicCallback(context.Background(), "jumio", []byte(`{}`), nil)
	if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected NotFound without leaking existence, got %v", err)
	}
}

func TestHandlePublicCallbackPropagatesSignatureRejection(t *testing.T) {
	repo := newFlowRepo()
	verifier := &flowVerifier{callbackErr: apperrors.NewUnauthorized("bad signature")}
	svc := newFlowServiceWithFactory(t, repo, flowResolver{credentials: domain.ProviderCredentials{Provider: "jumio"}},
		flowFactory{verifier: verifier, reference: "ref-pub-5"})

	pending := &domain.Verification{
		ID: "ver-pub-5", OrganizationID: "org-1", Provider: "jumio",
		Status: domain.StatusPending, ProviderRef: stringPtr("ref-pub-5"),
		InitiatedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Create(context.Background(), pending); err != nil {
		t.Fatal(err)
	}

	_, err := svc.HandlePublicCallback(context.Background(), "jumio", []byte(`{}`), nil)
	if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected Unauthorized, got %v", err)
	}
	if repo.updates != 0 {
		t.Error("a rejected signature must not touch the record")
	}
}

func TestHandlePublicCallbackRejectsProviderMismatch(t *testing.T) {
	repo := newFlowRepo()
	verifier := &flowVerifier{callback: &domain.VerificationResult{
		VerificationID: "ref-pub-6", Status: domain.StatusApproved, ProviderName: "aliyun_idv",
	}}
	svc := newFlowServiceWithFactory(t, repo, flowResolver{credentials: domain.ProviderCredentials{Provider: "jumio"}},
		flowFactory{verifier: verifier, reference: "ref-pub-6"})

	pending := &domain.Verification{
		ID: "ver-pub-6", OrganizationID: "org-1", Provider: "jumio",
		Status: domain.StatusPending, ProviderRef: stringPtr("ref-pub-6"),
		InitiatedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Create(context.Background(), pending); err != nil {
		t.Fatal(err)
	}

	_, err := svc.HandlePublicCallback(context.Background(), "jumio", []byte(`{}`), nil)
	if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected Unauthorized for provider mismatch, got %v", err)
	}
}
