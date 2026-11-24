package idv

import (
	"context"
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

// orderFlowVerifier implements the optional OrderResultQuerier extension so the
// refresh path can be exercised with a persisted BizSeq.
type orderFlowVerifier struct {
	flowVerifier
	gotBizSeq string
	orderUsed bool
}

func (v *orderFlowVerifier) GetVerificationResultWithOrder(_ context.Context, _, merchantBizID string) (*domain.VerificationResult, error) {
	v.orderUsed = true
	v.gotBizSeq = merchantBizID
	return v.result, v.resultErr
}

func TestRefreshPrefersOrderAwareQuery(t *testing.T) {
	repo := newFlowRepo()
	verifier := &orderFlowVerifier{flowVerifier: flowVerifier{result: &domain.VerificationResult{
		VerificationID: "txn-1", Status: domain.StatusApproved, ProviderName: "aliyun_idv", GatewayCode: "200",
	}}}
	ctrl := gomock.NewController(t)
	uow := mock.NewMockUnitOfWork(ctrl)
	uow.EXPECT().IDV().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			tx := mock.NewMockUnitOfWork(ctrl)
			tx.EXPECT().IDV().Return(repo).AnyTimes()
			return fn(ctx, tx)
		},
	).AnyTimes()
	svc := Service{
		UOW: uow, Clock: func() time.Time { return time.Now() },
		Providers: flowResolver{credentials: domain.ProviderCredentials{Provider: "aliyun_idv"}},
		Factory:   orderFactory{verifier: verifier},
	}

	pending := &domain.Verification{
		ID: "ver-o", OrganizationID: "org-1", Provider: "aliyun_idv",
		Status: domain.StatusPending, ProviderRef: stringPtr("txn-1"),
		BizSeq: stringPtr("0123456789abcdef0123456789abcdef"), ProductCode: stringPtr("eKYC_PRO"),
		InitiatedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Create(context.Background(), pending); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refresh(context.Background(), "ver-o", "org-1", "admin-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !verifier.orderUsed || verifier.gotBizSeq != "0123456789abcdef0123456789abcdef" {
		t.Errorf("expected order-aware query with the persisted bizSeq, used=%v seq=%q", verifier.orderUsed, verifier.gotBizSeq)
	}
}

type orderFactory struct{ verifier *orderFlowVerifier }

func (f orderFactory) Build(domain.ProviderCredentials) (domain.IdentityVerifier, error) {
	return f.verifier, nil
}
func (f orderFactory) CallbackReference(string, []byte) (string, error) {
	return "", apperrors.NewNotFound("verification not found")
}

func TestInitiateRejectsAlreadyVerifiedUser(t *testing.T) {
	repo := newFlowRepo()
	verifier := &flowVerifier{initiate: &domain.InitiateResult{VerificationID: "ref-1"}}
	ctrl := gomock.NewController(t)
	uow := mock.NewMockUnitOfWork(ctrl)
	uow.EXPECT().IDV().Return(repo).AnyTimes()
	userRepo := usermock.NewMockRepository(ctrl)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&domainusers.User{ID: "user-1", IsVerified: true}, nil).AnyTimes()
	uow.EXPECT().Users().Return(userRepo).AnyTimes()
	svc := Service{
		UOW: uow, Clock: func() time.Time { return time.Now() },
		Providers: flowResolver{credentials: domain.ProviderCredentials{Provider: "jumio"}},
		Factory:   flowFactory{verifier: verifier},
	}

	_, err := svc.Initiate(context.Background(), InitiateCommand{
		OrganizationID: "org-1", UserID: stringPtr("user-1"), Action: string(domain.ActionRealNameVerifyCN),
	})
	if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("already-verified user must be rejected with 422, got %v", err)
	}
	if repo.creates != 0 {
		t.Errorf("no verification row may be created for an already-verified user, got %d", repo.creates)
	}
}

func TestInitiateRejectsUnsupportedActionWith422Meta(t *testing.T) {
	repo := newFlowRepo()
	verifier := &flowVerifier{initiate: &domain.InitiateResult{VerificationID: "ref-1"}}
	svc := newFlowService(t, repo, flowResolver{credentials: domain.ProviderCredentials{Provider: "aliyun_idv"}}, verifier)

	_, err := svc.Initiate(context.Background(), InitiateCommand{
		OrganizationID: "org-1", Action: "bogus_action",
	})
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("unsupported action must be rejected with 422, got %v", err)
	}
	if appErr.Meta["field"] != "action" || appErr.Meta["value"] != "bogus_action" {
		t.Errorf("meta must carry field/value, got %v", appErr.Meta)
	}
	allowed, ok := appErr.Meta["allowed"].([]string)
	if !ok || len(allowed) != 3 {
		t.Errorf("meta must list the 3 allowed actions, got %v", appErr.Meta["allowed"])
	}
	if repo.creates != 0 {
		t.Errorf("no verification row may be created for an unsupported action, got %d", repo.creates)
	}
}

func TestInitiateCHNForcesDomesticAction(t *testing.T) {
	repo := newFlowRepo()
	verifier := &flowVerifier{initiate: &domain.InitiateResult{VerificationID: "ref-1", ProductCode: "ID_PRO"}}
	svc := newFlowService(t, repo, flowResolver{credentials: domain.ProviderCredentials{Provider: "aliyun_idv"}}, verifier)

	outcome, err := svc.Initiate(context.Background(), InitiateCommand{
		OrganizationID: "org-1", UserID: stringPtr("user-1"),
		Action: string(domain.ActionRealNameVerifyOverseas), Country: "CHN",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stored := repo.snapshot(outcome.VerificationID)
	if stored.Action == nil || *stored.Action != string(domain.ActionRealNameVerifyCN) {
		t.Errorf("CHN must force the domestic action, got %v", stored.Action)
	}
}

func TestInitiatePersistsBizSeqAndProductCode(t *testing.T) {
	repo := newFlowRepo()
	verifier := &flowVerifier{initiate: &domain.InitiateResult{VerificationID: "ref-1", ProductCode: "PV_FV"}}
	svc := newFlowService(t, repo, flowResolver{credentials: domain.ProviderCredentials{Provider: "aliyun_idv"}}, verifier)

	outcome, err := svc.Initiate(context.Background(), InitiateCommand{
		OrganizationID: "org-1", UserID: stringPtr("user-1"), Action: string(domain.ActionLoginVerify),
		TargetFaceURL: "https://cdn.test/face.jpg",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stored := repo.snapshot(outcome.VerificationID)
	if stored.BizSeq == nil || len(*stored.BizSeq) != 32 {
		t.Errorf("expected a 32-char bizSeq, got %v", stored.BizSeq)
	}
	if stored.ProductCode == nil || *stored.ProductCode != "PV_FV" {
		t.Errorf("productCode = %v, want PV_FV", stored.ProductCode)
	}
	if stored.Action == nil || *stored.Action != string(domain.ActionLoginVerify) {
		t.Errorf("action = %v, want login_verify", stored.Action)
	}
}
